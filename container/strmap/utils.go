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

package strmap

import "math/bits"

var bits2primes = []int32{
	0:  1,          // 1
	1:  7,          // 2
	2:  7,          // 4
	3:  17,         // 8
	4:  17,         // 16
	5:  31,         // 32
	6:  61,         // 64
	7:  127,        // 128
	8:  251,        // 256
	9:  509,        // 512
	10: 1021,       // 1024
	11: 2039,       // 2048
	12: 4093,       // 4096
	13: 8191,       // 8192
	14: 16381,      // 16384
	15: 32749,      // 32768
	16: 65521,      // 65536
	17: 131071,     // 131072
	18: 262139,     // 262144
	19: 524287,     // 524288
	20: 1048573,    // 1048576
	21: 2097143,    // 2097152
	22: 4194301,    // 4194304
	23: 8388593,    // 8388608
	24: 16777213,   // 16777216
	25: 33554393,   // 33554432
	26: 67108859,   // 67108864
	27: 134217689,  // 134217728
	28: 268435399,  // 268435456
	29: 536870909,  // 536870912
	30: 1073741789, // 1073741824
	31: 2147483647, // 2147483648
}

const loadfactor = float64(0.75) // always < 1, then len(hashtable) > n

func calcHashtableSlots(n int) int32 {
	// count bits to decide which prime number to use
	bits := bits.Len64(uint64(float64(n) / loadfactor))
	if bits >= len(bits2primes) {
		// ???? are you sure we need to hold so many items? ~ 2B items for 31 bits
		panic("too many items")
	}
	return bits2primes[bits] // a prime bigger than n
}
