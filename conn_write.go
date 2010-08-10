// Copyright 2010 Alexander Neumann. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pgsql

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
)

func (conn *Conn) flush() {
	err := conn.writer.Flush()
	if err != nil {
		panic(fmt.Sprintf("flush failed: %s", err))
	}
}

func (conn *Conn) write(b []byte) {
	_, err := conn.writer.Write(b)
	if err != nil {
		panic(fmt.Sprintf("write failed: %s", err))
	}
}

func (conn *Conn) writeByte(b byte) {
	err := conn.writer.WriteByte(b)
	if err != nil {
		panic(fmt.Sprintf("writeByte failed: %s", err))
	}
}

func (conn *Conn) writeFloat32(f float32) {
	var buf [4]byte
	b := buf[0:]

	binary.BigEndian.PutUint32(b, math.Float32bits(f))
	conn.write(b)
}

func (conn *Conn) writeFloat64(f float64) {
	var buf [8]byte
	b := buf[0:]

	binary.BigEndian.PutUint64(b, math.Float64bits(f))
	conn.write(b)
}

func (conn *Conn) writeFrontendMessageCode(code frontendMessageCode) {
	err := conn.writer.WriteByte(byte(code))
	if err != nil {
		panic(fmt.Sprintf("writeFrontendMessageCode failed: %s", err))
	}
}

func (conn *Conn) writeInt16(i int16) {
	var buf [2]byte
	b := buf[0:]

	binary.BigEndian.PutUint16(b, uint16(i))
	conn.write(b)
}

func (conn *Conn) writeInt32(i int32) {
	var buf [4]byte
	b := buf[0:]

	binary.BigEndian.PutUint32(b, uint32(i))
	conn.write(b)
}

func (conn *Conn) writeInt64(i int64) {
	var buf [8]byte
	b := buf[0:]

	binary.BigEndian.PutUint64(b, uint64(i))
	conn.write(b)
}

func (conn *Conn) writeString(s string) {
	_, err := conn.writer.WriteString(s)
	if err != nil {
		panic(fmt.Sprintf("writeString failed: %s", err))
	}
}

func (conn *Conn) writeString0(s string) {
	conn.writeString(s)
	conn.writeByte(0)
}

func (conn *Conn) writeFlush() {
	conn.writeFrontendMessageCode(_Flush)
	conn.writeInt32(4)

	conn.flush()
}

func (conn *Conn) writeBind(stmt *Statement) {
	values := make([]string, len(stmt.params))

	// Send Bind packet to server.
	var paramValuesLen int
	for i, param := range stmt.params {
		switch val := param.value.(type) {
		case bool:
			if val {
				values[i] = "t"
			} else {
				values[i] = "f"
			}

		case byte:
			values[i] = string([]byte{val})

		case float:
			values[i] = strconv.Ftoa(val, 'f', -1)

		case float32:
			values[i] = strconv.Ftoa32(val, 'f', -1)

		case float64:
			values[i] = strconv.Ftoa64(val, 'f', -1)

		case int:
			values[i] = strconv.Itoa(val)

		case int16:
			values[i] = strconv.Itoa(int(val))

		case int32:
			values[i] = strconv.Itoa(int(val))

		case int64:
			values[i] = strconv.Itoa64(val)

		case nil:

		case string:
			values[i] = val

		default:
			panic("unsupported parameter type")
		}

		paramValuesLen += len(values[i])
	}

	msgLen := int32(4 +
		len(stmt.portalName) + 1 +
		len(stmt.name) + 1 +
		2 + 2 +
		2 + len(stmt.params)*4 + paramValuesLen +
		2 + 2)

	conn.writeFrontendMessageCode(_Bind)
	conn.writeInt32(msgLen)
	conn.writeString0(stmt.portalName)
	conn.writeString0(stmt.name)
	conn.writeInt16(1)
	conn.writeInt16(int16(textFormat))
	conn.writeInt16(int16(len(stmt.params)))

	for i, param := range stmt.params {
		if param.value == nil {
			conn.writeInt32(-1)
		} else {
			conn.writeInt32(int32(len(values[i])))
			conn.writeString(values[i])
		}
	}

	conn.writeInt16(1)
	conn.writeInt16(binaryFormat)

	conn.writeFlush()
}

func (conn *Conn) writeClose(itemType byte, itemName string) {
	msgLen := int32(4 + 1 + len(itemName) + 1)

	conn.writeFrontendMessageCode(_Close)
	conn.writeInt32(msgLen)
	conn.writeByte(itemType)
	conn.writeString0(itemName)

	conn.flush()
}

func (conn *Conn) writeDescribe(stmt *Statement) {
	msgLen := int32(4 + 1 + len(stmt.portalName) + 1)

	conn.writeFrontendMessageCode(_Describe)
	conn.writeInt32(msgLen)
	conn.writeByte('P')
	conn.writeString0(stmt.portalName)

	conn.writeFlush()
}

func (conn *Conn) writeExecute(stmt *Statement) {
	msgLen := int32(4 + len(stmt.portalName) + 1 + 4)

	conn.writeFrontendMessageCode(_Execute)
	conn.writeInt32(msgLen)
	conn.writeString0(stmt.portalName)
	conn.writeInt32(0)

	conn.writeFlush()
}

func (conn *Conn) writeParse(stmt *Statement) {
	msgLen := int32(4 +
		len(stmt.name) + 1 +
		len(stmt.actualCommand) + 1 +
		2 + len(stmt.params)*4)

	conn.writeFrontendMessageCode(_Parse)
	conn.writeInt32(msgLen)
	conn.writeString0(stmt.name)
	conn.writeString0(stmt.actualCommand)

	conn.writeInt16(int16(len(stmt.params)))
	for _, param := range stmt.params {
		conn.writeInt32(int32(param.typ))
	}

	conn.writeFlush()
}

func (conn *Conn) writePasswordMessage(password string) {
	if conn.LogLevel >= LogDebug {
		defer conn.logExit(conn.logEnter("*Conn.writePasswordMessage"))
	}

	msgLen := int32(4 + len(password) + 1)

	conn.writeFrontendMessageCode(_PasswordMessage)
	conn.writeInt32(msgLen)
	conn.writeString0(password)

	conn.flush()
}

func (conn *Conn) writeQuery(command string) {
	conn.writeFrontendMessageCode(_Query)
	conn.writeInt32(int32(4 + len(command) + 1))
	conn.writeString0(command)

	conn.flush()
}

func (conn *Conn) writeStartup() {
	if conn.LogLevel >= LogDebug {
		defer conn.logExit(conn.logEnter("*Conn.writeStartup"))
	}

	msglen := int32(4 + 4 +
		len("user") + 1 + len(conn.params.User) + 1 +
		len("database") + 1 + len(conn.params.Database) + 1 + 1)

	conn.writeInt32(msglen)

	// For now we only support protocol version 3.0.
	conn.writeInt32(3 << 16)

	conn.writeString0("user")
	conn.writeString0(conn.params.User)

	conn.writeString0("database")
	conn.writeString0(conn.params.Database)

	conn.writeByte(0)

	conn.flush()
}

func (conn *Conn) writeSync() {
	conn.writeFrontendMessageCode(_Sync)
	conn.writeInt32(4)

	conn.writeFlush()
}

func (conn *Conn) writeTerminate() {
	conn.writeFrontendMessageCode(_Terminate)
	conn.writeInt32(4)

	conn.flush()
}