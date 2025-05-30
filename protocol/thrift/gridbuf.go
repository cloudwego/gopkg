/*
 * Copyright 2025 CloudWeGo Authors
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

	"github.com/cloudwego/gopkg/gridbuf"
)

var GridBuffer GridBufferProtocol

type GridBufferProtocol struct{}

// Skip skips over the value for the given type using Go implementation.
func (p GridBufferProtocol) Skip(b *gridbuf.ReadBuffer, t TType, unknownFields []byte, receiveUnknownFields bool) ([]byte, error) {
	return p.skipType(b, t, defaultRecursionDepth, unknownFields, receiveUnknownFields)
}

func (p GridBufferProtocol) skipType(b *gridbuf.ReadBuffer, t TType, maxdepth int, unknownFields []byte, receiveUnknownFields bool, ) ([]byte, error) {
	if maxdepth == 0 {
		return unknownFields, errDepthLimitExceeded
	}
	if n := typeToSize[t]; n > 0 {
		buf := b.ReadN(int(n))[:n]
		if receiveUnknownFields {
			unknownFields = append(unknownFields, buf...)
		}
		return unknownFields, nil
	}
	var err error
	switch t {
	case STRING:
		tmp := b.ReadN(4)[:4]
		n := binary.BigEndian.Uint32(tmp)
		s := b.ReadN(int(n))[:n]
		if receiveUnknownFields {
			unknownFields = append(unknownFields, tmp...)
			unknownFields = append(unknownFields, s...)
		}
		return unknownFields, nil
	case MAP:
		buf := b.ReadN(6)[:6]
		if receiveUnknownFields {
			unknownFields = append(unknownFields, buf...)
		}
		kt, vt, sz := TType(buf[0]), TType(buf[1]), binary.BigEndian.Uint32(buf[2:])
		ksz, vsz := int(typeToSize[kt]), int(typeToSize[vt])
		if ksz > 0 && vsz > 0 { // fast path, fast skip
			mapkvsize := (int(sz) * (ksz + vsz))
			buf = b.ReadN(mapkvsize)[:mapkvsize]
			if receiveUnknownFields {
				unknownFields = append(unknownFields, buf...)
			}
			return unknownFields, nil
		}
		for j := int32(0); j < int32(sz); j++ {
			if ksz > 0 {
				kbuf := b.ReadN(ksz)[:ksz]
				if receiveUnknownFields {
					unknownFields = append(unknownFields, kbuf...)
				}
			} else if kt == STRING {
				tmp := b.ReadN(4)[:4]
				n := binary.BigEndian.Uint32(tmp)
				s := b.ReadN(int(n))[:n]
				if receiveUnknownFields {
					unknownFields = append(unknownFields, tmp...)
					unknownFields = append(unknownFields, s...)
				}
			} else {
				unknownFields, err = p.skipType(b, kt, maxdepth-1, unknownFields, receiveUnknownFields)
				if err != nil {
					return unknownFields, err
				}
			}
			if vsz > 0 {
				vbuf := b.ReadN(vsz)[:vsz]
				if receiveUnknownFields {
					unknownFields = append(unknownFields, vbuf...)
				}
			} else if vt == STRING {
				tmp := b.ReadN(4)[:4]
				n := binary.BigEndian.Uint32(tmp)
				s := b.ReadN(int(n))[:n]
				if receiveUnknownFields {
					unknownFields = append(unknownFields, tmp...)
					unknownFields = append(unknownFields, s...)
				}
			} else {
				unknownFields, err = p.skipType(b, vt, maxdepth-1, unknownFields, receiveUnknownFields)
				if err != nil {
					return unknownFields, err
				}
			}
		}
		return unknownFields, nil
	case LIST, SET:
		buf := b.ReadN(5)[:5]
		if receiveUnknownFields {
			unknownFields = append(unknownFields, buf...)
		}
		vt, sz := TType(buf[0]), binary.BigEndian.Uint32(buf[1:])
		vsz := int(typeToSize[vt])
		if vsz > 0 { // fast path, fast skip
			listvsize := int(sz) * vsz
			buf = b.ReadN(listvsize)[:listvsize]
			if receiveUnknownFields {
				unknownFields = append(unknownFields, buf...)
			}
			return unknownFields, nil
		}
		for j := int32(0); j < int32(sz); j++ {
			if vsz > 0 {
				vbuf := b.ReadN(vsz)[:vsz]
				if receiveUnknownFields {
					unknownFields = append(unknownFields, vbuf...)
				}
			} else if vt == STRING {
				tmp := b.ReadN(4)[:4]
				n := binary.BigEndian.Uint32(tmp)
				s := b.ReadN(int(n))[:n]
				if receiveUnknownFields {
					unknownFields = append(unknownFields, tmp...)
					unknownFields = append(unknownFields, s...)
				}
			} else {
				unknownFields, err = p.skipType(b, vt, maxdepth-1, unknownFields, receiveUnknownFields)
				if err != nil {
					return unknownFields, err
				}
			}
		}
		return unknownFields, nil
	case STRUCT:
		for {
			buf := b.ReadN(1)[:1] // TType
			if receiveUnknownFields {
				unknownFields = append(unknownFields, buf...)
			}
			ft := TType(buf[0])
			if ft == STOP {
				return unknownFields, nil
			}
			buf = b.ReadN(2)[:2] // Field ID
			if receiveUnknownFields {
				unknownFields = append(unknownFields, buf...)
			}
			if sz := typeToSize[ft]; sz > 0 {
				buf = b.ReadN(int(sz))[:sz]
				if receiveUnknownFields {
					unknownFields = append(unknownFields, buf...)
				}
			} else if ft == STRING {
				tmp := b.ReadN(4)[:4]
				n := binary.BigEndian.Uint32(tmp)
				s := b.ReadN(int(n))[:n]
				if receiveUnknownFields {
					unknownFields = append(unknownFields, tmp...)
					unknownFields = append(unknownFields, s...)
				}
			} else {
				unknownFields, err = p.skipType(b, ft, maxdepth-1, unknownFields, receiveUnknownFields)
				if err != nil {
					return unknownFields, err
				}
			}
		}
	default:
		return unknownFields, NewProtocolException(INVALID_DATA, fmt.Sprintf("unknown data type %d", t))
	}
}
