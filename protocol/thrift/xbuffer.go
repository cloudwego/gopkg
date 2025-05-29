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

	"github.com/cloudwego/gopkg/xbuf"
)

var XBuffer XBufferProtocol

type XBufferProtocol struct{}

// Skip skips over the value for the given type using Go implementation.
func (p XBufferProtocol) Skip(b *xbuf.XReadBuffer, uf *UnknownFields, t TType) error {
	return p.skipType(b, uf, t, defaultRecursionDepth)
}

func (p XBufferProtocol) skipType(b *xbuf.XReadBuffer, uf *UnknownFields, t TType, maxdepth int) error {
	if maxdepth == 0 {
		return errDepthLimitExceeded
	}
	if n := typeToSize[t]; n > 0 {
		buf := b.ReadN(int(n))
		if uf != nil {
			uf.Append(buf)
		}
		return nil
	}
	var err error
	switch t {
	case STRING:
		p.skipstr(b, uf)
		return nil
	case MAP:
		buf := b.ReadN(6)
		if uf != nil {
			uf.Append(buf)
		}
		kt, vt, sz := TType(buf[0]), TType(buf[1]), binary.BigEndian.Uint32(buf[2:])
		ksz, vsz := int(typeToSize[kt]), int(typeToSize[vt])
		if ksz > 0 && vsz > 0 { // fast path, fast skip
			mapkvsize := (int(sz) * (ksz + vsz))
			buf = b.ReadN(mapkvsize)
			if uf != nil {
				uf.Append(buf)
			}
			return nil
		}
		for j := int32(0); j < int32(sz); j++ {
			if ksz > 0 {
				kbuf := b.ReadN(ksz)
				if uf != nil {
					uf.Append(kbuf)
				}
			} else if kt == STRING {
				p.skipstr(b, uf)
			} else {
				err = p.skipType(b, uf, kt, maxdepth-1)
				if err != nil {
					return err
				}
			}
			if vsz > 0 {
				vbuf := b.ReadN(vsz)
				if uf != nil {
					uf.Append(vbuf)
				}
			} else if vt == STRING {
				p.skipstr(b, uf)
			} else {
				err = p.skipType(b, uf, vt, maxdepth-1)
				if err != nil {
					return err
				}
			}
		}
		return nil
	case LIST, SET:
		buf := b.ReadN(5)
		if uf != nil {
			uf.Append(buf)
		}
		vt, sz := TType(buf[0]), binary.BigEndian.Uint32(buf[1:])
		vsz := int(typeToSize[vt])
		if vsz > 0 { // fast path, fast skip
			listvsize := int(sz) * vsz
			buf = b.ReadN(listvsize)
			if uf != nil {
				uf.Append(buf)
			}
			return nil
		}
		for j := int32(0); j < int32(sz); j++ {
			if vsz > 0 {
				vbuf := b.ReadN(vsz)
				if uf != nil {
					uf.Append(vbuf)
				}
			} else if vt == STRING {
				p.skipstr(b, uf)
			} else {
				err = p.skipType(b, uf, vt, maxdepth-1)
				if err != nil {
					return err
				}
			}
		}
		return nil
	case STRUCT:
		for {
			buf := b.ReadN(1) // TType
			if uf != nil {
				uf.Append(buf)
			}
			ft := TType(buf[0])
			if ft == STOP {
				return nil
			}
			buf = b.ReadN(2) // Field ID
			if uf != nil {
				uf.Append(buf)
			}
			if typeToSize[ft] > 0 {
				buf = b.ReadN(int(typeToSize[ft]))
				if uf != nil {
					uf.Append(buf)
				}
			} else if ft == STRING {
				p.skipstr(b, uf)
			} else {
				err = p.skipType(b, uf, ft, maxdepth-1)
				if err != nil {
					return err
				}
			}
		}
	default:
		return NewProtocolException(INVALID_DATA, fmt.Sprintf("unknown data type %d", t))
	}
}

func (p XBufferProtocol) skipstr(b *xbuf.XReadBuffer, uf *UnknownFields) {
	tmp := b.ReadN(4)
	n := binary.BigEndian.Uint32(tmp)
	s := b.ReadN(int(n))
	if uf != nil {
		uf.Append(tmp)
		uf.Append(s)
	}
}

type UnknownFields struct {
	buf []byte
}

func (p *UnknownFields) Append(buf []byte) {
	p.buf = append(p.buf, buf...)
}

func (p *UnknownFields) Bytes() []byte {
	return p.buf
}
