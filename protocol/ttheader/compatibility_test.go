package ttheader_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/ttheader"
)

// Old consumers can still bind every original public codec entry point.
var (
	_ func(context.Context, bufiox.Reader) (ttheader.DecodeParam, error)         = ttheader.Decode
	_ func(context.Context, []byte) (ttheader.DecodeParam, error)                = ttheader.DecodeFromBytes
	_ func(context.Context, ttheader.EncodeParam, bufiox.Writer) ([]byte, error) = ttheader.Encode
	_ func(context.Context, ttheader.EncodeParam) ([]byte, error)                = ttheader.EncodeToBytes
)

func TestLegacyEntryPointsWithNewHeaderEntries(t *testing.T) {
	for _, value := range []string{"", "a", "ab", "abc", "abcd", "中文"} {
		t.Run(value, func(t *testing.T) {
			ctx := context.Background()
			param := ttheader.EncodeParam{
				SeqID:   17,
				IntInfo: map[uint16]string{ttheader.LogID: value},
				StrInfo: map[string]string{"key": value, ttheader.GDPRToken: "token"},
			}
			legacy, err := ttheader.EncodeToBytes(ctx, param)
			if err != nil {
				t.Fatal(err)
			}
			entries := ttheader.HeaderEntries{
				Entries: []ttheader.HeaderEntry{
					{Kind: 1, ID: ttheader.LogID, Value: value},
					{Kind: 2, Key: "key", Value: value},
					{Kind: 2, Key: ttheader.GDPRToken, Value: "token"},
				},
				IntCount: 1, StrCount: 2, GDPRIndex: 3, IntKind: 1, StringKind: 2,
			}
			var wire []byte
			writer := bufiox.NewBytesWriter(&wire)
			if _, err = ttheader.EncodeEntries(ctx, param, entries, writer); err != nil {
				t.Fatal(err)
			}
			if err = writer.Flush(); err != nil {
				t.Fatal(err)
			}
			// Both APIs reserve the length word for the transport to fill.
			binary.BigEndian.PutUint32(legacy, uint32(len(legacy)-4))
			binary.BigEndian.PutUint32(wire, uint32(len(wire)-4))
			if !bytes.Equal(legacy, wire) {
				t.Fatal("entries changed the legacy wire format")
			}
			binary.BigEndian.PutUint32(wire, uint32(len(wire)-4))
			decoded, err := ttheader.DecodeFromBytes(ctx, wire)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.SeqID != param.SeqID || !reflect.DeepEqual(decoded.IntInfo, param.IntInfo) || !reflect.DeepEqual(decoded.StrInfo, param.StrInfo) {
				t.Fatalf("legacy decoder lost metadata: %+v", decoded)
			}
		})
	}
}
