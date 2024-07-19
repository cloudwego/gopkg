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
	"io"
	"sync"
)

var poolSkipDecoder = sync.Pool{
	New: func() interface{} {
		return &SkipDecoder{}
	},
}

// SkipDecoder scans the underlying io.Reader and returns the bytes of a type
type SkipDecoder struct {
	p skipReaderIface
}

// NewSkipDecoder ... call Release if no longer use
func NewSkipDecoder(r io.Reader) *SkipDecoder {
	p := poolSkipDecoder.Get().(*SkipDecoder)
	p.Reset(r)
	return p
}

// Reset ...
func (p *SkipDecoder) Reset(r io.Reader) {
	// fast path without returning to pool if remote.ByteBuffer && *skipByteBuffer
	if buf, ok := r.(remoteByteBuffer); ok {
		if p.p != nil {
			r, ok := p.p.(*skipByteBuffer)
			if ok {
				r.Reset(buf)
				return
			}
			p.p.Release()
		}
		p.p = newSkipByteBuffer(buf)
		return
	}

	// not remote.ByteBuffer

	if p.p != nil {
		p.p.Release()
	}
	p.p = newSkipReader(r)
}

// Release ...
func (p *SkipDecoder) Release() {
	p.p.Release()
	p.p = nil
	poolSkipDecoder.Put(p)
}

// Next skips a specific type and returns its bytes
func (p *SkipDecoder) Next(t TType) (buf []byte, err error) {
	if err := p.skip(t, defaultRecursionDepth); err != nil {
		return nil, err
	}
	return p.p.Bytes()
}

func (p *SkipDecoder) skip(t TType, maxdepth int) error {
	if maxdepth == 0 {
		return errDepthLimitExceeded
	}
	if sz := typeToSize[t]; sz > 0 {
		_, err := p.p.Next(int(sz))
		return err
	}
	switch t {
	case STRING:
		b, err := p.p.Next(4)
		if err != nil {
			return err
		}
		sz := int(binary.BigEndian.Uint32(b))
		if sz < 0 {
			return errNegativeSize
		}
		if _, err := p.p.Next(sz); err != nil {
			return err
		}
	case STRUCT:
		for {
			b, err := p.p.Next(1) // TType
			if err != nil {
				return err
			}
			tp := TType(b[0])
			if tp == STOP {
				break
			}
			if _, err := p.p.Next(2); err != nil { // Field ID
				return err
			}
			if err := p.skip(tp, maxdepth-1); err != nil {
				return err
			}
		}
	case MAP:
		b, err := p.p.Next(6) // 1 byte key TType, 1 byte value TType, 4 bytes Len
		if err != nil {
			return err
		}
		kt, vt, sz := TType(b[0]), TType(b[1]), int32(binary.BigEndian.Uint32(b[2:]))
		if sz < 0 {
			return errNegativeSize
		}
		ksz, vsz := int(typeToSize[kt]), int(typeToSize[vt])
		if ksz > 0 && vsz > 0 {
			_, err := p.p.Next(int(sz) * (ksz + vsz))
			return err
		}
		for i := int32(0); i < sz; i++ {
			if err := p.skip(kt, maxdepth-1); err != nil {
				return err
			}
			if err := p.skip(vt, maxdepth-1); err != nil {
				return err
			}
		}
	case SET, LIST:
		b, err := p.p.Next(5) // 1 byte value type, 4 bytes Len
		if err != nil {
			return err
		}
		vt, sz := TType(b[0]), int32(binary.BigEndian.Uint32(b[1:]))
		if sz < 0 {
			return errNegativeSize
		}
		if vsz := typeToSize[vt]; vsz > 0 {
			_, err := p.p.Next(int(sz) * int(vsz))
			return err
		}
		for i := int32(0); i < sz; i++ {
			if err := p.skip(vt, maxdepth-1); err != nil {
				return err
			}
		}
	default:
		return NewProtocolException(INVALID_DATA, fmt.Sprintf("unknown data type %d", t))
	}
	return nil
}
