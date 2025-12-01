// Copyright 2024 CloudWeGo Authors
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
	"context"
	"encoding/binary"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
)

var seqId int32

func TestEncodeToBytes(t *testing.T) {
	seqId++
	encodeParam := EncodeParam{
		Flags:      0,
		SeqID:      seqId,
		ProtocolID: 0,
		IntInfo: map[uint16]string{
			ToService: "to.service",
			ToCluster: "to.cluster",
			ToMethod:  "method",
			LogID:     "xxxxxxxxx",
		},
		StrInfo: map[string]string{
			GDPRToken:            "gdpr_token_xxxxxxxx",
			HeaderIDLServiceName: "a.b.c",
			HeaderTransToIDC:     "to_idc",
		},
	}
	buf, err := EncodeToBytes(context.Background(), encodeParam)
	if err != nil {
		t.Fatalf("encode to bytes failed, %s", err.Error())
	}
	binary.BigEndian.PutUint32(buf, uint32(len(buf)-4))
	decodeParam, err := DecodeFromBytes(context.Background(), buf)
	if err != nil {
		t.Fatalf("encode to bytes failed, %s", err.Error())
	}
	checkParamEqual(t, encodeParam, decodeParam, len(buf), 0)
}

func TestEncode(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:14089")
	if err != nil {
		t.Fatalf("listen failed, %s", err.Error())
	}
	var decodeParam DecodeParam
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := l.Accept()
		if err != nil {
			t.Errorf("accept failed, %s", err.Error())
			return
		}
		br := bufiox.NewDefaultReader(conn)
		decodeParam, err = Decode(context.Background(), br)
		if err != nil {
			t.Errorf("decode failed, %s", err.Error())
			return
		}
		_ = br.Release(nil)
		l.Close()
		conn.Close()
	}()

	seqId++
	encodeParam := EncodeParam{
		Flags:      0,
		SeqID:      seqId,
		ProtocolID: 0,
		IntInfo: map[uint16]string{
			ToService: "to.service",
			ToCluster: "to.cluster",
			ToMethod:  "method",
			LogID:     "xxxxxxxxx",
		},
		StrInfo: map[string]string{
			GDPRToken:            "gdpr_token_xxxxxxxx",
			HeaderIDLServiceName: "a.b.c",
			HeaderTransToIDC:     "to_idc",
		},
	}
	conn, err := net.Dial("tcp", "127.0.0.1:14089")
	if err != nil {
		t.Fatalf("dial failed, %s", err.Error())
	}
	bw := bufiox.NewDefaultWriter(conn)
	totalLenField, err := Encode(context.Background(), encodeParam, bw)
	if err != nil {
		t.Fatalf("encode failed, %s", err.Error())
	}
	binary.BigEndian.PutUint32(totalLenField, uint32(bw.WrittenLen()-4))
	headerLen := bw.WrittenLen()
	bw.Flush()
	wg.Wait()
	checkParamEqual(t, encodeParam, decodeParam, headerLen, 0)
}

func TestEncodeStreamingFrame(t *testing.T) {
	seqId++
	encodeParam := EncodeParam{
		Flags:      HeaderFlagsStreaming,
		SeqID:      seqId,
		ProtocolID: ProtocolIDThriftStruct,
		IntInfo: map[uint16]string{
			ToService: "to.service",
			ToCluster: "to.cluster",
			ToMethod:  "method",
			LogID:     "xxxxxxxxx",
		},
		StrInfo: map[string]string{
			HeaderIDLServiceName: "a.b.c",
			HeaderTransToIDC:     "to_idc",
		},
	}
	buf, err := EncodeToBytes(context.Background(), encodeParam)
	if err != nil {
		t.Fatalf("encode to bytes failed, %s", err.Error())
	}
	binary.BigEndian.PutUint32(buf, uint32(len(buf)-4))
	decodeParam, err := DecodeFromBytes(context.Background(), buf)
	if err != nil {
		t.Fatalf("encode to bytes failed, %s", err.Error())
	}
	checkParamEqual(t, encodeParam, decodeParam, len(buf), 0)
}

func checkParamEqual(t *testing.T, encodeParam EncodeParam, decodeParam DecodeParam, headerLen, payloadLen int) {
	if decodeParam.Flags != encodeParam.Flags {
		t.Fatalf("encode to bytes failed, flags not equal")
	}
	if decodeParam.SeqID != encodeParam.SeqID {
		t.Fatalf("encode to bytes failed, seq id not equal")
	}
	if decodeParam.ProtocolID != encodeParam.ProtocolID {
		t.Fatalf("encode to bytes failed, protocol id not equal")
	}
	if !reflect.DeepEqual(decodeParam.IntInfo, encodeParam.IntInfo) {
		t.Fatalf("encode to bytes failed, int info not equal")
	}
	if !reflect.DeepEqual(decodeParam.StrInfo, encodeParam.StrInfo) {
		t.Fatalf("encode to bytes failed, str info not equal")
	}
	if decodeParam.HeaderLen != headerLen {
		t.Fatalf("encode to bytes failed, header len not equal")
	}
	if decodeParam.PayloadLen != payloadLen {
		t.Fatalf("encode to bytes failed, payload len not equal")
	}
}

func TestEncodeHeaderSizeCheck(t *testing.T) {
	t.Run("encode header below 64KB", func(t *testing.T) {
		strInfo := make(map[string]string)
		targetSize := 60000
		currentSize := 0
		i := 0
		for currentSize < targetSize {
			key := strings.Repeat("k", 10+i%10)
			value := strings.Repeat("v", 100)
			strInfo[key] = value
			currentSize += 2 + len(key) + 2 + len(value)
			i++
		}

		encodeParam := EncodeParam{
			SeqID:      1,
			ProtocolID: ProtocolIDThriftBinary,
			StrInfo:    strInfo,
		}

		buf, err := EncodeToBytes(context.Background(), encodeParam)
		if err != nil {
			t.Fatalf("unexpected error when encoding header below MaxHeaderSize: %s", err.Error())
		}
		binary.BigEndian.PutUint32(buf, uint32(len(buf)-4))

		decodeParam, err := DecodeFromBytes(context.Background(), buf)
		if err != nil {
			t.Fatalf("failed to decode: %s", err.Error())
		}
		checkParamEqual(t, encodeParam, decodeParam, len(buf), 0)
	})
	t.Run("encode header equals 64KB", func(t *testing.T) {
		strInfo := make(map[string]string)
		// PROTOCOL ID(1) + NUM TRANSFORMS(1) + INFO ID TYPE(1) + NUM HEADERS(2) +
		// 3 + 2 + 65524 + 2 = 65536
		strInfo["key"] = strings.Repeat("v", 65524)
		encodeParam := EncodeParam{
			SeqID:      1,
			ProtocolID: ProtocolIDThriftBinary,
			StrInfo:    strInfo,
		}

		buf, err := EncodeToBytes(context.Background(), encodeParam)
		if err != nil {
			t.Fatalf("unexpected error when encoding header below MaxHeaderSize: %s", err.Error())
		}
		binary.BigEndian.PutUint32(buf, uint32(len(buf)-4))

		decodeParam, err := DecodeFromBytes(context.Background(), buf)
		if err != nil {
			t.Fatalf("failed to decode: %s", err.Error())
		}
		checkParamEqual(t, encodeParam, decodeParam, len(buf), 0)
	})
	t.Run("encode header equals MaxHeaderSize", func(t *testing.T) {
		strInfo := make(map[string]string)
		// PROTOCOL ID(1) + NUM TRANSFORMS(1) + INFO ID TYPE(1) + NUM HEADERS(2) +
		// 2 + 65522 + 2 + 65535 + 2 + 65535 + 2 + 65535 = 262140
		strInfo[strings.Repeat("k", 65522)] = strings.Repeat("v", 65535)
		strInfo[strings.Repeat("x", 65535)] = strings.Repeat("y", 65535)
		encodeParam := EncodeParam{
			SeqID:      1,
			ProtocolID: ProtocolIDThriftBinary,
			StrInfo:    strInfo,
		}

		buf, err := EncodeToBytes(context.Background(), encodeParam)
		if err != nil {
			t.Fatalf("unexpected error when encoding header below MaxHeaderSize: %s", err.Error())
		}
		binary.BigEndian.PutUint32(buf, uint32(len(buf)-4))

		decodeParam, err := DecodeFromBytes(context.Background(), buf)
		if err != nil {
			t.Fatalf("failed to decode: %s", err.Error())
		}
		checkParamEqual(t, encodeParam, decodeParam, len(buf), 0)
	})
	t.Run("encode header exceeding MaxHeaderSize", func(t *testing.T) {
		strInfo := make(map[string]string)
		for i := 0; i < 5000; i++ {
			key := strconv.Itoa(i)
			value := strings.Repeat("v", 60)
			strInfo[key] = value
		}

		encodeParam := EncodeParam{
			SeqID:      1,
			ProtocolID: ProtocolIDThriftBinary,
			StrInfo:    strInfo,
		}

		_, err := EncodeToBytes(context.Background(), encodeParam)
		if err == nil {
			t.Fatal("expected error when encoding header exceeding MaxHeaderSize, but got nil")
		}
		if !strings.Contains(err.Error(), "invalid header length") {
			t.Fatalf("expected 'invalid header length' error, but got: %s", err.Error())
		}
	})
}

func TestEncodeHeaderStringLengthCheck(t *testing.T) {
	t.Run("StrInfo key length equals maxHeaderStringSize", func(t *testing.T) {
		key := strings.Repeat("k", maxHeaderStringSize)

		encodeParam := EncodeParam{
			SeqID:      1,
			ProtocolID: ProtocolIDThriftBinary,
			StrInfo: map[string]string{
				key: "val",
			},
		}

		buf, err := EncodeToBytes(context.Background(), encodeParam)
		if err != nil {
			t.Fatalf("unexpected error when encoding string at maxHeaderStringSize: %s", err.Error())
		}

		binary.BigEndian.PutUint32(buf, uint32(len(buf)-4))

		decodeParam, err := DecodeFromBytes(context.Background(), buf)
		if err != nil {
			t.Fatalf("failed to decode: %s", err.Error())
		}

		checkParamEqual(t, encodeParam, decodeParam, len(buf), 0)
	})
	t.Run("StrInfo key length exceeding maxHeaderStringSize", func(t *testing.T) {
		key := strings.Repeat("k", maxHeaderStringSize+1)

		encodeParam := EncodeParam{
			SeqID:      1,
			ProtocolID: ProtocolIDThriftBinary,
			StrInfo: map[string]string{
				key: "val",
			},
		}

		_, err := EncodeToBytes(context.Background(), encodeParam)
		if err == nil {
			t.Fatal("expected error when encoding string exceeding maxHeaderStringSize, but got nil")
		}
		t.Log(err)
		if !strings.Contains(err.Error(), "exceeded 65535B max size") {
			t.Fatalf("expected 'exceeded 65535B max size' error, but got: %s", err.Error())
		}
	})
	t.Run("StrInfo value length equals maxHeaderStringSize", func(t *testing.T) {
		val := strings.Repeat("v", maxHeaderStringSize)

		encodeParam := EncodeParam{
			SeqID:      1,
			ProtocolID: ProtocolIDThriftBinary,
			StrInfo: map[string]string{
				"key": val,
			},
		}

		buf, err := EncodeToBytes(context.Background(), encodeParam)
		if err != nil {
			t.Fatalf("unexpected error when encoding string at maxHeaderStringSize: %s", err.Error())
		}

		binary.BigEndian.PutUint32(buf, uint32(len(buf)-4))

		decodeParam, err := DecodeFromBytes(context.Background(), buf)
		if err != nil {
			t.Fatalf("failed to decode: %s", err.Error())
		}

		checkParamEqual(t, encodeParam, decodeParam, len(buf), 0)
	})
	t.Run("StrInfo value length exceeding maxHeaderStringSize", func(t *testing.T) {
		val := strings.Repeat("v", maxHeaderStringSize+1)

		encodeParam := EncodeParam{
			SeqID:      1,
			ProtocolID: ProtocolIDThriftBinary,
			StrInfo: map[string]string{
				"key": val,
			},
		}

		_, err := EncodeToBytes(context.Background(), encodeParam)
		if err == nil {
			t.Fatal("expected error when encoding string exceeding maxHeaderStringSize, but got nil")
		}
		t.Log(err)
		if !strings.Contains(err.Error(), "exceeded 65535B max size") {
			t.Fatalf("expected 'exceeded 65535B max size' error, but got: %s", err.Error())
		}
	})
	t.Run("IntInfo value length equals maxHeaderStringSize", func(t *testing.T) {
		val := strings.Repeat("v", maxHeaderStringSize)

		encodeParam := EncodeParam{
			SeqID:      1,
			ProtocolID: ProtocolIDThriftBinary,
			IntInfo: map[uint16]string{
				1: val,
			},
		}

		buf, err := EncodeToBytes(context.Background(), encodeParam)
		if err != nil {
			t.Fatalf("unexpected error when encoding string at maxHeaderStringSize: %s", err.Error())
		}

		binary.BigEndian.PutUint32(buf, uint32(len(buf)-4))

		decodeParam, err := DecodeFromBytes(context.Background(), buf)
		if err != nil {
			t.Fatalf("failed to decode: %s", err.Error())
		}

		checkParamEqual(t, encodeParam, decodeParam, len(buf), 0)
	})
	t.Run("IntInfo value length exceeding maxHeaderStringSize", func(t *testing.T) {
		val := strings.Repeat("v", maxHeaderStringSize+1)

		encodeParam := EncodeParam{
			SeqID:      1,
			ProtocolID: ProtocolIDThriftBinary,
			IntInfo: map[uint16]string{
				1: val,
			},
		}

		_, err := EncodeToBytes(context.Background(), encodeParam)
		if err == nil {
			t.Fatal("expected error when encoding int kv value exceeding maxHeaderStringSize, but got nil")
		}
		t.Log(err)
		if !strings.Contains(err.Error(), "exceeded 65535B max size") {
			t.Fatalf("expected 'exceeded 65535B max size' error, but got: %s", err.Error())
		}
	})
	t.Run("GDPR token exceeding maxHeaderStringSize should fail", func(t *testing.T) {
		veryLongToken := strings.Repeat("t", maxHeaderStringSize+1)

		encodeParam := EncodeParam{
			SeqID:      1,
			ProtocolID: ProtocolIDThriftBinary,
			StrInfo: map[string]string{
				GDPRToken: veryLongToken,
			},
		}

		_, err := EncodeToBytes(context.Background(), encodeParam)
		if err == nil {
			t.Fatal("expected error when encoding GDPR token exceeding maxHeaderStringSize, but got nil")
		}
		if !strings.Contains(err.Error(), "exceeded 65535B max size") {
			t.Fatalf("expected 'exceeded 65535B max size' error, but got: %s", err.Error())
		}
	})
}
