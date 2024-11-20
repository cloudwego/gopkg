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
		br.Release(nil)
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
