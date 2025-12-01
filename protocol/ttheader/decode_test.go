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
