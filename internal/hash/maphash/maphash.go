//go:build go1.19

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

// maphash wraps std hash/maphash for working with go1.18 which doesn't has Bytes, String functions
// TODO: use hash/maphash directly if we no longer support go1.18
package maphash

import "hash/maphash"

// Seed ...
type Seed = maphash.Seed

// MakeSeed ...
func MakeSeed() maphash.Seed { return maphash.MakeSeed() }

// Bytes ...
func Bytes(seed maphash.Seed, b []byte) uint64 {
	return maphash.Bytes(seed, b)
}

// String ...
func String(seed maphash.Seed, s string) uint64 {
	return maphash.String(seed, s)
}
