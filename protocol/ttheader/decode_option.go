// Copyright 2026 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ttheader

// DecodeConfig controls TTHeader decode behavior.
// Zero value keeps the historical safe path (per-string copy).
type DecodeConfig struct {
	// BulkStringAlloc, when true, copies the remaining KV header bytes into one
	// owned buffer and builds map strings with unsafex.BinaryToString against
	// that buffer. This cuts per-field string allocations. Strings then share
	// the bulk buffer's backing array (immutable use only).
	//
	// Default false: each field uses a safe string([]byte) copy.
	BulkStringAlloc bool
}

// DecodeOption mutates DecodeConfig for Decode / DecodeFromBytes.
type DecodeOption func(*DecodeConfig)

// WithBulkStringAlloc enables or disables bulk+unsafe string materialization.
// Pass true only when callers treat returned StrInfo/IntInfo values as immutable
// and do not retain them past ownership of the DecodeParam maps themselves
// (the bulk backing buffer is owned by those strings via unsafe).
func WithBulkStringAlloc(enable bool) DecodeOption {
	return func(cfg *DecodeConfig) {
		cfg.BulkStringAlloc = enable
	}
}

func applyDecodeOptions(opts []DecodeOption) DecodeConfig {
	var cfg DecodeConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}
