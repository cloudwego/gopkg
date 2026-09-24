// Copyright 2025 CloudWeGo Authors
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

import (
	"bytes"
	"context"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
)

func TestDecodeNonTTHeader(t *testing.T) {
	reader := bytes.NewReader([]byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06,
		0x07, 0x08, 0x09, 0x10, 0x11, 0x12, 0x13,
	})
	defReader := bufiox.NewDefaultReader(reader)
	_, err := Decode(context.Background(), defReader)
	if err == nil {
		t.Fatal("err should be non-nil")
	}
	expectStr := "not TTHeader protocol"
	if !strings.Contains(err.Error(), expectStr) {
		t.Fatalf("expect %s but got %s", expectStr, err.Error())
	}
}

func TestDecodeHeaderSizeCheck(t *testing.T) {
	testcases := []struct {
		desc               string
		headerSizeField    uint16
		expectedActualSize uint32
		expectErr          string
	}{
		{
			desc:               "normal size within 64KB",
			headerSizeField:    16,
			expectedActualSize: 64,
		},
		{
			desc:               "size exactly at 64KB boundary",
			headerSizeField:    16384,
			expectedActualSize: 65536,
		},
		{
			desc:               "size slightly above 64KB (would overflow if using uint16 * 4 to calculate header size)",
			headerSizeField:    16385,
			expectedActualSize: 65540,
		},
		{
			desc:               "size at 128KB",
			headerSizeField:    32768,
			expectedActualSize: 131072,
		},
		{
			desc:               "size at max value 262140B",
			headerSizeField:    65535,
			expectedActualSize: 262140,
		},
		{
			desc:               "size less than minimum (0 bytes)",
			headerSizeField:    0,
			expectedActualSize: 0,
			expectErr:          "invalid header length",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			buf := make([]byte, TTHeaderMetaSize+int(tc.expectedActualSize))
			shouldFail := tc.expectErr != ""

			totalLen := uint32(len(buf) - 4)
			binary.BigEndian.PutUint32(buf[0:4], totalLen)
			binary.BigEndian.PutUint32(buf[4:8], TTHeaderMagic)
			seqID := uint32(1)
			binary.BigEndian.PutUint32(buf[8:12], seqID)
			binary.BigEndian.PutUint16(buf[12:14], tc.headerSizeField)

			if int(tc.expectedActualSize) >= 2 && !shouldFail {
				buf[14] = byte(ProtocolIDThriftBinary)
				buf[15] = 0
			}

			reader := bufiox.NewBytesReader(buf)
			param, err := Decode(context.Background(), reader)

			if shouldFail {
				if err == nil {
					t.Fatalf("expected error containing '%s', but got nil", tc.expectErr)
				}
				if !strings.Contains(err.Error(), tc.expectErr) {
					t.Fatalf("expected error containing '%s', but got: %s", tc.expectErr, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %s", err.Error())
				}
				if param.SeqID != int32(seqID) {
					t.Fatalf("expected seqID=%d, got=%d", seqID, param.SeqID)
				}
				expectedHeaderLen := int(tc.expectedActualSize) + TTHeaderMetaSize
				if param.HeaderLen != expectedHeaderLen {
					t.Fatalf("expected HeaderLen=%d, got=%d", expectedHeaderLen, param.HeaderLen)
				}
			}
		})
	}
}

func BenchmarkDecodeFromBytes(b *testing.B) {
	sizes := []int{16, 64, 512}
	modes := []struct {
		name string
		opts []DecodeOption
	}{
		{name: "SafeCopy", opts: nil},
		{name: "BulkUnsafe", opts: []DecodeOption{WithBulkStringAlloc(true)}},
	}

	for _, mode := range modes {
		for _, n := range sizes {
			b.Run(mode.name+"/StrInfo_"+strconv.Itoa(n)+"Fields", func(b *testing.B) {
				benchmarkDecodeFromBytes(b, EncodeParam{
					SeqID:      1,
					ProtocolID: ProtocolIDThriftBinary,
					StrInfo:    makeStrInfo(n, false),
				}, mode.opts...)
			})
		}

		for _, n := range sizes {
			b.Run(mode.name+"/IntInfo_"+strconv.Itoa(n)+"Fields", func(b *testing.B) {
				benchmarkDecodeFromBytes(b, EncodeParam{
					SeqID:      1,
					ProtocolID: ProtocolIDThriftBinary,
					IntInfo:    makeIntInfo(n),
				}, mode.opts...)
			})
		}

		// Mixed Str + Int with equal field counts.
		for _, n := range sizes {
			b.Run(mode.name+"/StrAndIntInfo_"+strconv.Itoa(n)+"Fields", func(b *testing.B) {
				benchmarkDecodeFromBytes(b, EncodeParam{
					SeqID:      1,
					ProtocolID: ProtocolIDThriftBinary,
					StrInfo:    makeStrInfo(n, false),
					IntInfo:    makeIntInfo(n),
				}, mode.opts...)
			})
		}

		// Str + GDPR token: encoder writes a separate InfoIDACLToken segment that
		// shares strKVMap, exercising the +1 size hint path in readStrKVInfo.
		for _, n := range sizes {
			b.Run(mode.name+"/StrInfoWithGDPR_"+strconv.Itoa(n)+"Fields", func(b *testing.B) {
				benchmarkDecodeFromBytes(b, EncodeParam{
					SeqID:      1,
					ProtocolID: ProtocolIDThriftBinary,
					StrInfo:    makeStrInfo(n, true),
				}, mode.opts...)
			})
		}

		// Full mix: Str + Int + GDPR token.
		for _, n := range sizes {
			b.Run(mode.name+"/MixedAll_"+strconv.Itoa(n)+"Fields", func(b *testing.B) {
				benchmarkDecodeFromBytes(b, EncodeParam{
					SeqID:      1,
					ProtocolID: ProtocolIDThriftBinary,
					StrInfo:    makeStrInfo(n, true),
					IntInfo:    makeIntInfo(n),
				}, mode.opts...)
			})
		}
	}
}

func TestDecodeBulkStringAllocMatchesSafe(t *testing.T) {
	param := EncodeParam{
		SeqID:      42,
		ProtocolID: ProtocolIDThriftBinary,
		StrInfo:    makeStrInfo(8, true),
		IntInfo:    makeIntInfo(8),
	}
	buf, err := EncodeToBytes(context.Background(), param)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	binary.BigEndian.PutUint32(buf, uint32(len(buf)-4))

	safe, err := DecodeFromBytes(context.Background(), buf)
	if err != nil {
		t.Fatalf("safe decode: %v", err)
	}
	bulk, err := DecodeFromBytes(context.Background(), buf, WithBulkStringAlloc(true))
	if err != nil {
		t.Fatalf("bulk decode: %v", err)
	}
	if safe.SeqID != bulk.SeqID || safe.ProtocolID != bulk.ProtocolID {
		t.Fatalf("meta mismatch: safe=%+v bulk=%+v", safe, bulk)
	}
	if len(safe.StrInfo) != len(bulk.StrInfo) || len(safe.IntInfo) != len(bulk.IntInfo) {
		t.Fatalf("map size mismatch str %d/%d int %d/%d",
			len(safe.StrInfo), len(bulk.StrInfo), len(safe.IntInfo), len(bulk.IntInfo))
	}
	for k, v := range safe.StrInfo {
		if bulk.StrInfo[k] != v {
			t.Fatalf("StrInfo[%q]: safe=%q bulk=%q", k, v, bulk.StrInfo[k])
		}
	}
	for k, v := range safe.IntInfo {
		if bulk.IntInfo[k] != v {
			t.Fatalf("IntInfo[%d]: safe=%q bulk=%q", k, v, bulk.IntInfo[k])
		}
	}
}

func benchmarkDecodeFromBytes(b *testing.B, param EncodeParam, opts ...DecodeOption) {
	buf, err := EncodeToBytes(context.Background(), param)
	if err != nil {
		b.Fatalf("failed to encode: %v", err)
	}
	// EncodeToBytes leaves the first 4 bytes (total length) for the caller to fill.
	binary.BigEndian.PutUint32(buf, uint32(len(buf)-4))

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := DecodeFromBytes(ctx, buf, opts...); err != nil {
			b.Fatalf("failed to decode: %v", err)
		}
	}
}

func makeStrInfo(n int, withGDPRToken bool) map[string]string {
	size := n
	if withGDPRToken {
		size++
	}
	m := make(map[string]string, size)
	for i := 0; i < n; i++ {
		m["key"+strconv.Itoa(i)] = "value" + strconv.Itoa(i)
	}
	if withGDPRToken {
		m[GDPRToken] = "gdpr_token_xxxxxxxx"
	}
	return m
}

func makeIntInfo(n int) map[uint16]string {
	m := make(map[uint16]string, n)
	for i := 0; i < n; i++ {
		m[uint16(i)] = "value" + strconv.Itoa(i)
	}
	return m
}
