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

package mempool

import (
	"math/bits"
	"sync"
	"unsafe"
)

type memPool struct {
	sync.Pool

	Size int
}

var pools []*memPool

const (
	minMemPoolSize = 4 << 10   //	4KB, `Malloc` returns buf with cap >= the number
	maxMemPoolSize = 128 << 30 // 128GB, `Malloc` will panic if > the number
)

const (
	// footer is a [8]byte, it contains two parts: magic(58 bits) and index (6 bits):
	// * magic is for checking a []byte is created by this package
	// * index is for `pools`, the cap of a []byte is always equal to pools[i].Size
	// we use footer instead of header to ensure that `Free` is always safe regardless of the input provided.
	footerLen = 8

	footerMagicMask = uint64(0xFFFFFFFFFFFFFFC0) // 58 bits mask
	footerIndexMask = uint64(0x000000000000003F) // 6 bits mask
	footerMagic     = uint64(0xBADC0DEBADC0DEC0) // it ends with 6 zero bits which used by index
)

// bits2idx maps bits.Len to the index of `pools`
// for size < minMemPoolSize, bits2idx maps to `pools[0]` which is expected.
var bits2idx [64]int

func init() {
	i := 0
	for sz := minMemPoolSize; sz <= maxMemPoolSize; sz <<= 1 {
		p := &memPool{Size: sz}
		p.New = func() interface{} {
			b := make([]byte, 0, p.Size)
			b = b[:p.Size]
			return &b[0]
		}
		pools = append(pools, p)
		bits2idx[bits.Len(uint(p.Size))] = i
		i++
	}
}

// poolIndex returns index of a pool which fits the given size `sz`
func poolIndex(sz int) int {
	if sz <= minMemPoolSize {
		return 0
	}
	i := bits2idx[bits.Len(uint(sz))]
	if uint(sz)&(uint(sz)-1) == 0 {
		// if power of two, it fits perfectly
		// like `8192` should be in pools[1], but `8193` in pools[2]
		return i
	}
	return i + 1
}

type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

// Malloc creates a buf from pool.
// Tips for usage:
// * buf returned by Malloc may not be initialized with zeros, use at your own risk.
// * call `Free` when buf is no longer use, DO NOT REUSE buf after calling `Free`
// * use `buf = buf[:mempool.Cap(buf)]` to make use of the cap of a returned buf.
// * DO NOT USE `cap` or `append` to resize, coz bytes at the end of buf are used for storing malloc info.
func Malloc(size int) []byte {
	if size == 0 {
		return []byte{}
	}
	c := size + footerLen // reserve for footer
	i := poolIndex(c)
	pool := pools[i]
	p := pool.Get().(*byte)

	// prepare for return
	ret := []byte{}
	h := (*sliceHeader)(unsafe.Pointer(&ret))
	h.Data = unsafe.Pointer(p)
	h.Len = size
	h.Cap = pool.Size // update to the correct cap

	// add mallocMemMagic & index to the end of bytes
	// it will check later when `Free`
	*(*uint64)(unsafe.Add(h.Data, h.Cap-footerLen)) = footerMagic | uint64(i)
	return ret
}

// Cap returns the max cap of a buf can be resized to.
// See comment of `Malloc` for details
func Cap(buf []byte) int {
	if cap(buf)-len(buf) < footerLen || getFooter(buf)&footerMagicMask != footerMagic {
		panic("buf not malloc by this package or buf len changed without using Cap func")
	}
	return cap(buf) - footerLen
}

// Append appends bytes to the given `[]byte`.
// It frees `a` and creates a new one if needed.
// Please make sure you're calling the func like `b = mempool.Append(b, data...)`
func Append(a []byte, b ...byte) []byte {
	if cap(a)-len(a)-footerLen > len(b) {
		return append(a, b...)
	}
	return appendSlow(a, b)
}

func appendSlow(a, b []byte) []byte {
	ret := Malloc(len(a) + len(b))
	copy(ret, a)
	copy(ret[len(a):], b)
	Free(a)
	return ret
}

// AppendStr ... same as Append for string.
// See comment of `Append` for details.
func AppendStr(a []byte, b string) []byte {
	if cap(a)-len(a)-footerLen > len(b) {
		return append(a, b...)
	}
	return appendStrSlow(a, b)
}

func appendStrSlow(a []byte, b string) []byte {
	ret := Malloc(len(a) + len(b))
	copy(ret, a)
	copy(ret[len(a):], b)
	Free(a)
	return ret
}

// Free should be called when a buf is no longer used.
// See comment of `Malloc` for details.
func Free(buf []byte) {
	c := cap(buf)
	if c < minMemPoolSize {
		return
	}
	if uint(c)&uint(c-1) != 0 { // not malloc by this package
		return
	}
	size := len(buf)
	if c-size < footerLen { // size
		return
	}
	footer := getFooter(buf)
	// checks magic
	if footer&footerMagicMask != footerMagic {
		return
	}
	// checks index
	i := int(footer & footerIndexMask)
	if i < len(pools) {
		if p := pools[i]; p.Size == c {
			p.Put(&buf[0])
		}
	}
}

func getFooter(buf []byte) uint64 {
	h := (*sliceHeader)(unsafe.Pointer(&buf))
	return *(*uint64)(unsafe.Add(h.Data, h.Cap-footerLen))
}
