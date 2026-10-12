package ttheader

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
)

func BenchmarkPrimaryOrdinaryMap(b *testing.B) {
	ctx := context.Background()
	for _, count := range []int{8, 32} {
		param := EncodeParam{IntInfo: make(map[uint16]string), StrInfo: make(map[string]string)}
		for i := 0; i < count; i++ {
			param.IntInfo[uint16(i)] = strings.Repeat("v", 16)
			param.StrInfo["k"+strconv.Itoa(i)] = strings.Repeat("v", 32)
		}
		param.StrInfo[GDPRToken] = "token"
		frame, err := EncodeToBytes(ctx, param)
		if err != nil {
			b.Fatal(err)
		}
		binary.BigEndian.PutUint32(frame, uint32(len(frame)-4))
		b.Run(fmt.Sprintf("decode/%d", count), func(b *testing.B) {
			ints, strs := make(map[uint16]string), make(map[string]string)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				in := bufiox.NewBytesReader(frame)
				if _, err := DecodeInto(ctx, in, ints, strs); err != nil {
					b.Fatal(err)
				}
				if err := in.Release(nil); err != nil {
					b.Fatal(err)
				}
				for key := range ints {
					delete(ints, key)
				}
				for key := range strs {
					delete(strs, key)
				}
			}
		})
		b.Run(fmt.Sprintf("encode/%d", count), func(b *testing.B) {
			buf := make([]byte, len(frame))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf = buf[:0]
				out := bufiox.NewBytesWriter(&buf)
				if _, err := Encode(ctx, param, out); err != nil {
					b.Fatal(err)
				}
				if err := out.Flush(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
