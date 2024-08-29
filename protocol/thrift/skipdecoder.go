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
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/gopkg/bufiox"
)

const defaultSkipDecoderSize = 4096

var poolSkipDecoder = sync.Pool{
	New: func() interface{} {
		return &SkipDecoder{}
	},
}

// SkipDecoder scans the underlying io.Reader and returns the bytes of a type
type SkipDecoder struct {
	r bufiox.Reader

	// for storing Next(ttype) buffer
	nextBuf []byte

	// for reusing buffer
	pendingBuf [][]byte
}

// NewSkipDecoder ... call Release if no longer use
func NewSkipDecoder(r bufiox.Reader) *SkipDecoder {
	p := poolSkipDecoder.Get().(*SkipDecoder)
	p.r = r
	return p
}

// Release releases the peekAck decoder, callers cannot use the returned data of Next after calling Release.
func (p *SkipDecoder) Release() {
	if cap(p.nextBuf) > 0 {
		mcache.Free(p.nextBuf)
	}
	*p = SkipDecoder{}
	poolSkipDecoder.Put(p)
}

// Next skips a specific type and returns its bytes.
// Callers cannot use the returned data after calling Release.
func (p *SkipDecoder) Next(t TType) (buf []byte, err error) {
	p.nextBuf = mcache.Malloc(0, defaultSkipDecoderSize)
	if err = p.skip(t, defaultRecursionDepth); err != nil {
		return
	}
	var offset int
	for _, b := range p.pendingBuf {
		offset += copy(p.nextBuf[offset:], b[offset:])
		mcache.Free(b)
	}
	p.pendingBuf = nil
	buf = p.nextBuf
	return
}

func (p *SkipDecoder) skip(t TType, maxdepth int) error {
	if maxdepth == 0 {
		return errDepthLimitExceeded
	}
	if sz := typeToSize[t]; sz > 0 {
		_, err := p.next(int(sz))
		return err
	}
	switch t {
	case STRING:
		b, err := p.next(4)
		if err != nil {
			return err
		}
		sz := int(binary.BigEndian.Uint32(b))
		if sz < 0 {
			return errNegativeSize
		}
		if _, err := p.next(sz); err != nil {
			return err
		}
	case STRUCT:
		for {
			b, err := p.next(1) // TType
			if err != nil {
				return err
			}
			tp := TType(b[0])
			if tp == STOP {
				break
			}
			if _, err := p.next(2); err != nil { // Field ID
				return err
			}
			if err := p.skip(tp, maxdepth-1); err != nil {
				return err
			}
		}
	case MAP:
		b, err := p.next(6) // 1 byte key TType, 1 byte value TType, 4 bytes Len
		if err != nil {
			return err
		}
		kt, vt, sz := TType(b[0]), TType(b[1]), int32(binary.BigEndian.Uint32(b[2:]))
		if sz < 0 {
			return errNegativeSize
		}
		ksz, vsz := int(typeToSize[kt]), int(typeToSize[vt])
		if ksz > 0 && vsz > 0 {
			_, err := p.next(int(sz) * (ksz + vsz))
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
		b, err := p.next(5) // 1 byte value type, 4 bytes Len
		if err != nil {
			return err
		}
		vt, sz := TType(b[0]), int32(binary.BigEndian.Uint32(b[1:]))
		if sz < 0 {
			return errNegativeSize
		}
		if vsz := typeToSize[vt]; vsz > 0 {
			_, err := p.next(int(sz) * int(vsz))
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

func (p *SkipDecoder) next(n int) (buf []byte, err error) {
	if buf, err = p.r.Next(n); err != nil {
		return
	}
	if cap(p.nextBuf)-len(p.nextBuf) < n {
		var ncap int
		for ncap = cap(p.nextBuf) * 2; ncap-len(p.nextBuf) < n; ncap *= 2 {
		}
		nbs := mcache.Malloc(ncap, ncap)
		p.pendingBuf = append(p.pendingBuf, p.nextBuf)
		p.nextBuf = nbs[:len(p.nextBuf)]
	}
	cn := copy(p.nextBuf[len(p.nextBuf):cap(p.nextBuf)], buf)
	p.nextBuf = p.nextBuf[:len(p.nextBuf)+cn]
	return
}
