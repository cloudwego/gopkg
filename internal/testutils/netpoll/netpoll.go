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

package netpoll

// NetpollDirectWriter implements NocopyWriter for fastcodec.
//
// It's only used for testing purposes,
// see the WriteDirect implementation of netpoll.Writer for details.
type NetpollDirectWriter struct {
	data []byte

	// for WriteDirect
	wbuf [][]byte
	wend []int // remainCap
}

// Bytes returns the actual result of FastWriteNocopy
func (p *NetpollDirectWriter) Bytes() []byte {
	ret := make([]byte, 0, len(p.data))
	start := 0
	for i := 0; i < len(p.wend); i++ {
		end := len(p.data) - p.wend[i]
		ret = append(ret, p.data[start:end]...) // bytes from p.data
		ret = append(ret, p.wbuf[i]...)         // bytes from WriteDirect
		start = end
	}
	// copy left bytes
	ret = append(ret, p.data[start:start+len(p.data)-len(ret)]...)
	if len(ret) != len(p.data) {
		panic("size not match")
	}
	return ret
}

// Malloc creates a new buffer for FastWriteNocopy
func (p *NetpollDirectWriter) Malloc(n int) []byte {
	p.wbuf = p.wbuf[:0]
	p.wend = p.wend[:0]
	p.data = make([]byte, n)
	return p.data
}

// WriteDirect implements NocopyWriter for fastcodec
func (p *NetpollDirectWriter) WriteDirect(b []byte, remainCap int) error {
	if remainCap < len(b) {
		panic("buffer too small to fit the input")
	}
	p.wbuf = append(p.wbuf, b)
	p.wend = append(p.wend, remainCap)
	return nil
}

func (p *NetpollDirectWriter) WriteDirectN() int {
	return len(p.wbuf)
}
