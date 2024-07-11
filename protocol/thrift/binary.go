/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package thrift

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/cloudwego/gopkg/internal/unsafe"
)

var Binary binaryProtocol

type binaryProtocol struct{}

func (binaryProtocol) WriteMessageBegin(buf []byte, name string, typeID TMessageType, seq int32) int {
	binary.BigEndian.PutUint32(buf, uint32(msgVersion1)|uint32(typeID&msgTypeMask))
	binary.BigEndian.PutUint32(buf[4:], uint32(len(name)))
	off := 8 + copy(buf[8:], name)
	binary.BigEndian.PutUint32(buf[off:], uint32(seq))
	return off + 4
}

func (binaryProtocol) WriteFieldBegin(buf []byte, typeID TType, id int16) int {
	buf[0] = byte(typeID)
	binary.BigEndian.PutUint16(buf[1:], uint16(id))
	return 3
}

func (binaryProtocol) WriteFieldStop(buf []byte) int {
	buf[0] = byte(STOP)
	return 1
}

func (binaryProtocol) WriteMapBegin(buf []byte, kt, vt TType, size int) int {
	buf[0] = byte(kt)
	buf[1] = byte(vt)
	binary.BigEndian.PutUint32(buf[2:], uint32(size))
	return 6
}

func (binaryProtocol) WriteListBegin(buf []byte, et TType, size int) int {
	buf[0] = byte(et)
	binary.BigEndian.PutUint32(buf[1:], uint32(size))
	return 5
}

func (binaryProtocol) WriteSetBegin(buf []byte, et TType, size int) int {
	buf[0] = byte(et)
	binary.BigEndian.PutUint32(buf[1:], uint32(size))
	return 5
}

func (binaryProtocol) WriteBool(buf []byte, v bool) int {
	if v {
		buf[0] = 1
	} else {
		buf[0] = 0
	}
	return 1
}

func (binaryProtocol) WriteByte(buf []byte, v int8) int {
	buf[0] = byte(v)
	return 1
}

func (binaryProtocol) WriteI16(buf []byte, v int16) int {
	binary.BigEndian.PutUint16(buf, uint16(v))
	return 2
}

func (binaryProtocol) WriteI32(buf []byte, v int32) int {
	binary.BigEndian.PutUint32(buf, uint32(v))
	return 4
}

func (binaryProtocol) WriteI64(buf []byte, v int64) int {
	binary.BigEndian.PutUint64(buf, uint64(v))
	return 8
}

func (binaryProtocol) WriteDouble(buf []byte, v float64) int {
	binary.BigEndian.PutUint64(buf, math.Float64bits(v))
	return 8
}

func (binaryProtocol) WriteBinary(buf, v []byte) int {
	binary.BigEndian.PutUint32(buf, uint32(len(v)))
	return 4 + copy(buf[4:], v)
}

func (binaryProtocol) WriteBinaryNocopy(buf []byte, w NocopyWriter, v []byte) int {
	if w == nil || len(buf) < NocopyWriteThreshold {
		return Binary.WriteBinary(buf, v)
	}
	binary.BigEndian.PutUint32(buf, uint32(len(v)))
	_ = w.WriteDirect(v, len(buf[4:])) // always err == nil ?
	return 4
}

func (binaryProtocol) WriteString(buf []byte, v string) int {
	binary.BigEndian.PutUint32(buf, uint32(len(v)))
	return 4 + copy(buf[4:], v)
}

func (binaryProtocol) WriteStringNocopy(buf []byte, w NocopyWriter, v string) int {
	return Binary.WriteBinaryNocopy(buf, w, unsafe.StringToByteSlice(v))
}

// Append methods

func (binaryProtocol) AppendMessageBegin(buf []byte, name string, typeID TMessageType, seq int32) []byte {
	buf = appendUint32(buf, uint32(msgVersion1)|uint32(typeID&msgTypeMask))
	buf = Binary.AppendString(buf, name)
	return Binary.AppendI32(buf, seq)
}

func (binaryProtocol) AppendFieldBegin(buf []byte, typeID TType, id int16) []byte {
	return append(buf, byte(typeID), byte(uint16(id>>8)), byte(id))
}

func (binaryProtocol) AppendFieldStop(buf []byte) []byte {
	return append(buf, byte(STOP))
}

func (binaryProtocol) AppendMapBegin(buf []byte, kt, vt TType, size int) []byte {
	return Binary.AppendI32(append(buf, byte(kt), byte(vt)), int32(size))
}

func (binaryProtocol) AppendListBegin(buf []byte, et TType, size int) []byte {
	return Binary.AppendI32(append(buf, byte(et)), int32(size))
}

func (binaryProtocol) AppendSetBegin(buf []byte, et TType, size int) []byte {
	return Binary.AppendI32(append(buf, byte(et)), int32(size))
}

func (binaryProtocol) AppendBinary(buf, v []byte) []byte {
	return append(Binary.AppendI32(buf, int32(len(v))), v...)
}

func (binaryProtocol) AppendString(buf []byte, v string) []byte {
	return append(Binary.AppendI32(buf, int32(len(v))), v...)
}

func (binaryProtocol) AppendBool(buf []byte, v bool) []byte {
	if v {
		return append(buf, 1)
	} else {
		return append(buf, 0)
	}
}

func (binaryProtocol) AppendByte(buf []byte, v int8) []byte {
	return append(buf, byte(v))
}

func (binaryProtocol) AppendI16(buf []byte, v int16) []byte {
	return append(buf, byte(uint16(v)>>8), byte(v))
}

func (binaryProtocol) AppendI32(buf []byte, v int32) []byte {
	return appendUint32(buf, uint32(v))
}

func (binaryProtocol) AppendI64(buf []byte, v int64) []byte {
	return appendUint64(buf, uint64(v))
}

func (binaryProtocol) AppendDouble(buf []byte, v float64) []byte {
	return appendUint64(buf, math.Float64bits(v))
}

func appendUint32(buf []byte, v uint32) []byte {
	return append(buf, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func appendUint64(buf []byte, v uint64) []byte {
	return append(buf, byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32),
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// Length methods

func (binaryProtocol) MessageBeginLength(name string, _ TMessageType, _ int32) int {
	return 4 + (4 + len(name)) + 4
}

func (binaryProtocol) FieldBeginLength() int           { return 3 }
func (binaryProtocol) FieldStopLength() int            { return 1 }
func (binaryProtocol) MapBeginLength() int             { return 6 }
func (binaryProtocol) ListBeginLength() int            { return 5 }
func (binaryProtocol) SetBeginLength() int             { return 5 }
func (binaryProtocol) BoolLength() int                 { return 1 }
func (binaryProtocol) ByteLength() int                 { return 1 }
func (binaryProtocol) I16Length() int                  { return 2 }
func (binaryProtocol) I32Length() int                  { return 4 }
func (binaryProtocol) I64Length() int                  { return 8 }
func (binaryProtocol) DoubleLength() int               { return 8 }
func (binaryProtocol) StringLength(v string) int       { return 4 + len(v) }
func (binaryProtocol) BinaryLength(v []byte) int       { return 4 + len(v) }
func (binaryProtocol) StringLengthNocopy(v string) int { return 4 + len(v) }
func (binaryProtocol) BinaryLengthNocopy(v []byte) int { return 4 + len(v) }

// Read methods

var (
	errReadMessage = NewProtocolException(INVALID_DATA, "ReadMessageBegin: buf too small")
	errBadVersion  = NewProtocolException(BAD_VERSION, "ReadMessageBegin: bad version")
)

func (binaryProtocol) ReadMessageBegin(buf []byte) (name string, typeID TMessageType, seq int32, l int, err error) {
	if len(buf) < 4 { // version+type header + name header
		return "", 0, 0, 0, errReadMessage
	}

	// read header for version and type
	header := binary.BigEndian.Uint32(buf)
	if header&msgVersionMask != msgVersion1 {
		return "", 0, 0, 0, errBadVersion
	}
	typeID = TMessageType(header & msgTypeMask)

	off := 4

	// read method name
	name, l, err1 := Binary.ReadString(buf[off:])
	if err1 != nil {
		return "", 0, 0, 0, errReadMessage
	}
	off += l

	// read seq
	seq, l, err2 := Binary.ReadI32(buf[off:])
	if err2 != nil {
		return "", 0, 0, 0, errReadMessage
	}
	off += l
	return name, typeID, seq, off, nil
}

var (
	errReadField = NewProtocolException(INVALID_DATA, "ReadFieldBegin: buf too small")
	errReadMap   = NewProtocolException(INVALID_DATA, "ReadMapBegin: buf too small")
	errReadList  = NewProtocolException(INVALID_DATA, "ReadListBegin: buf too small")
	errReadSet   = NewProtocolException(INVALID_DATA, "ReadSetBegin: buf too small")
	errReadStr   = NewProtocolException(INVALID_DATA, "ReadString: buf too small")
	errReadBin   = NewProtocolException(INVALID_DATA, "ReadBinary: buf too small")

	errReadBool   = NewProtocolException(INVALID_DATA, "ReadBool: len(buf) < 1")
	errReadByte   = NewProtocolException(INVALID_DATA, "ReadByte: len(buf) < 1")
	errReadI16    = NewProtocolException(INVALID_DATA, "ReadI16: len(buf) < 2")
	errReadI32    = NewProtocolException(INVALID_DATA, "ReadI32: len(buf) < 4")
	errReadI64    = NewProtocolException(INVALID_DATA, "ReadI64: len(buf) < 8")
	errReadDouble = NewProtocolException(INVALID_DATA, "ReadDouble: len(buf) < 8")
)

func (binaryProtocol) ReadFieldBegin(buf []byte) (typeID TType, id int16, l int, err error) {
	if len(buf) < 1 {
		return 0, 0, 0, errReadField
	}
	typeID = TType(buf[0])
	if typeID == STOP {
		return STOP, 0, 1, nil
	}
	if len(buf) < 3 {
		return 0, 0, 0, errReadField
	}
	return typeID, int16(binary.BigEndian.Uint16(buf[1:])), 3, nil
}

func (binaryProtocol) ReadMapBegin(buf []byte) (kt, vt TType, size, l int, err error) {
	if len(buf) < 6 {
		return 0, 0, 0, 0, errReadMap
	}
	return TType(buf[0]), TType(buf[1]), int(binary.BigEndian.Uint32(buf[2:])), 6, nil
}

func (binaryProtocol) ReadListBegin(buf []byte) (et TType, size, l int, err error) {
	if len(buf) < 5 {
		return 0, 0, 0, errReadList
	}
	return TType(buf[0]), int(binary.BigEndian.Uint32(buf[1:])), 5, nil
}

func (binaryProtocol) ReadSetBegin(buf []byte) (et TType, size, l int, err error) {
	if len(buf) < 5 {
		return 0, 0, 0, errReadSet
	}
	return TType(buf[0]), int(binary.BigEndian.Uint32(buf[1:])), 5, nil
}

func (binaryProtocol) ReadBinary(buf []byte) (b []byte, l int, err error) {
	sz, _, err := Binary.ReadI32(buf)
	if err != nil {
		return nil, 0, errReadBin
	}
	l = 4 + int(sz)
	if len(buf) < l {
		return nil, 4, errReadBin
	}
	// TODO: use span
	return []byte(string(buf[4:l])), l, nil
}

func (binaryProtocol) ReadString(buf []byte) (s string, l int, err error) {
	sz, _, err := Binary.ReadI32(buf)
	if err != nil {
		return "", 0, errReadStr
	}
	l = 4 + int(sz)
	if len(buf) < l {
		return "", 4, errReadStr
	}
	// TODO: use span
	return string(buf[4:l]), l, nil
}

func (binaryProtocol) ReadBool(buf []byte) (v bool, l int, err error) {
	if len(buf) < 1 {
		return false, 0, errReadBool
	}
	if buf[0] == 1 {
		return true, 1, nil
	}
	return false, 1, nil
}

func (binaryProtocol) ReadByte(buf []byte) (v int8, l int, err error) {
	if len(buf) < 1 {
		return 0, 0, errReadByte
	}
	return int8(buf[0]), 1, nil
}

func (binaryProtocol) ReadI16(buf []byte) (v int16, l int, err error) {
	if len(buf) < 2 {
		return 0, 0, errReadI16
	}
	return int16(binary.BigEndian.Uint16(buf)), 2, nil
}

func (binaryProtocol) ReadI32(buf []byte) (v int32, l int, err error) {
	if len(buf) < 4 {
		return 0, 0, errReadI32
	}
	return int32(binary.BigEndian.Uint32(buf)), 4, nil
}

func (binaryProtocol) ReadI64(buf []byte) (v int64, l int, err error) {
	if len(buf) < 8 {
		return 0, 0, errReadI64
	}
	return int64(binary.BigEndian.Uint64(buf)), 8, nil
}

func (binaryProtocol) ReadDouble(buf []byte) (v float64, l int, err error) {
	if len(buf) < 8 {
		return 0, 0, errReadDouble
	}
	return math.Float64frombits(binary.BigEndian.Uint64(buf)), 8, nil
}

var (
	errDepthLimitExceeded = NewProtocolException(DEPTH_LIMIT, "depth limit exceeded")
	errNegativeSize       = NewProtocolException(NEGATIVE_SIZE, "negative size")
)

var typeToSize = [256]int8{
	BOOL:   1,
	BYTE:   1,
	DOUBLE: 8,
	I16:    2,
	I32:    4,
	I64:    8,
}

func skipstr(b []byte) int {
	return 4 + int(binary.BigEndian.Uint32(b))
}

// Skip skips over the value for the given type using Go implementation.
func (binaryProtocol) Skip(b []byte, t TType) (int, error) {
	return skipType(b, t, defaultRecursionDepth)
}

func skipType(b []byte, t TType, maxdepth int) (int, error) {
	if maxdepth == 0 {
		return 0, errDepthLimitExceeded
	}
	if n := typeToSize[t]; n > 0 {
		return int(n), nil
	}
	switch t {
	case STRING:
		return skipstr(b), nil
	case MAP:
		i := 6
		kt, vt, sz := TType(b[0]), TType(b[1]), int32(binary.BigEndian.Uint32(b[2:]))
		if sz < 0 {
			return 0, errNegativeSize
		}
		ksz, vsz := int(typeToSize[kt]), int(typeToSize[vt])
		if ksz > 0 && vsz > 0 {
			return i + (int(sz) * (ksz + vsz)), nil
		}
		for j := int32(0); j < sz; j++ {
			if ksz > 0 {
				i += ksz
			} else if kt == STRING {
				i += skipstr(b[i:])
			} else if n, err := skipType(b[i:], kt, maxdepth-1); err != nil {
				return i, err
			} else {
				i += n
			}
			if vsz > 0 {
				i += vsz
			} else if vt == STRING {
				i += skipstr(b[i:])
			} else if n, err := skipType(b[i:], vt, maxdepth-1); err != nil {
				return i, err
			} else {
				i += n
			}
		}
		return i, nil
	case LIST, SET:
		i := 5
		vt, sz := TType(b[0]), int32(binary.BigEndian.Uint32(b[1:]))
		if sz < 0 {
			return 0, errNegativeSize
		}
		if typeToSize[vt] > 0 {
			return i + int(sz)*int(typeToSize[vt]), nil
		}
		for j := int32(0); j < sz; j++ {
			if vt == STRING {
				i += skipstr(b[i:])
			} else if n, err := skipType(b[i:], vt, maxdepth-1); err != nil {
				return i, err
			} else {
				i += n
			}
		}
		return i, nil
	case STRUCT:
		i := 0
		for {
			ft := TType(b[i])
			i += 1 // TType
			if ft == STOP {
				return i, nil
			}
			i += 2 // Field ID
			if typeToSize[ft] > 0 {
				i += int(typeToSize[ft])
			} else if n, err := skipType(b[i:], ft, maxdepth-1); err != nil {
				return i, err
			} else {
				i += n
			}
		}
	default:
		return 0, NewProtocolException(INVALID_DATA, fmt.Sprintf("unknown data type %d", t))
	}
}
