package ttheader

import (
	"context"
	"encoding/binary"
	"reflect"
	"strconv"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
)

func reuseStringBlock(kv ...string) []byte {
	b := []byte{byte(InfoIDKeyValue)}
	b = binary.BigEndian.AppendUint16(b, uint16(len(kv)/2))
	for _, s := range kv {
		b = binary.BigEndian.AppendUint16(b, uint16(len(s)))
		b = append(b, s...)
	}
	return b
}

func reuseIntBlock(key uint16, value string) []byte {
	b := []byte{byte(InfoIDIntKeyValue), 0, 1}
	b = binary.BigEndian.AppendUint16(b, key)
	b = binary.BigEndian.AppendUint16(b, uint16(len(value)))
	return append(b, value...)
}

func reuseACLBlock(value string) []byte {
	b := []byte{byte(InfoIDACLToken)}
	b = binary.BigEndian.AppendUint16(b, uint16(len(value)))
	return append(b, value...)
}

func reuseFrame(blocks ...[]byte) []byte {
	info := []byte{byte(ProtocolIDThriftBinary), 0}
	for _, block := range blocks {
		info = append(info, block...)
	}
	for len(info)%4 != 0 {
		info = append(info, 0)
	}
	frame := make([]byte, TTHeaderMetaSize)
	binary.BigEndian.PutUint32(frame[:4], uint32(TTHeaderMetaSize+len(info)-4))
	binary.BigEndian.PutUint32(frame[4:8], TTHeaderMagic)
	binary.BigEndian.PutUint32(frame[8:12], 7)
	binary.BigEndian.PutUint16(frame[12:14], uint16(len(info)/4))
	return append(frame, info...)
}

func TestDecodeIntoReuseAndBlockOrder(t *testing.T) {
	ctx := context.Background()
	for _, tokenLast := range []bool{false, true} {
		blocks := [][]byte{
			reuseStringBlock("first-only", "a", "duplicate", "old"),
			reuseIntBlock(3, "caller"), reuseIntBlock(9, "old-method"),
			reuseStringBlock("second-only", "b", "duplicate", "new"),
			reuseIntBlock(6, "target"), reuseIntBlock(9, "new-method"),
		}
		token, strToken := reuseACLBlock("special"), reuseStringBlock(GDPRToken, "string")
		wantToken := "string"
		if tokenLast {
			blocks = append(blocks, strToken, token)
			wantToken = "special"
		} else {
			blocks = append(blocks, token, strToken)
		}
		frame := reuseFrame(blocks...)
		want, err := Decode(ctx, bufiox.NewBytesReader(frame))
		if err != nil {
			t.Fatal(err)
		}
		ints, strs := make(map[uint16]string), make(map[string]string)
		for round := 0; round < 3; round++ {
			got, err := DecodeInto(ctx, bufiox.NewBytesReader(frame), ints, strs)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("round %d: got %+v, %v; want %+v", round, got, err, want)
			}
			if strs["first-only"] != "a" || strs["second-only"] != "b" || strs["duplicate"] != "new" || strs[GDPRToken] != wantToken || ints[3] != "caller" || ints[6] != "target" || ints[9] != "new-method" {
				t.Fatal("multiple info blocks changed merge order")
			}
			ints[60000], strs["retained-map"] = "int", "string"
			if got.IntInfo[60000] != "int" || got.StrInfo["retained-map"] != "string" {
				t.Fatal("supplied maps were replaced")
			}
			for i := 0; i < 64; i++ {
				strs["mesh-"+strconv.Itoa(i)] = "value"
			}
			for key := range ints {
				delete(ints, key)
			}
			for key := range strs {
				delete(strs, key)
			}
		}
	}
}

func TestDecodeIntoErrorsAndNilMaps(t *testing.T) {
	valid := reuseFrame(reuseStringBlock("first", "value"), reuseIntBlock(3, "caller"))
	unknown := reuseFrame(reuseStringBlock("first", "value"), reuseIntBlock(3, "caller"), []byte{255})
	for _, frame := range [][]byte{valid, valid[:13], valid[:len(valid)-1], unknown} {
		want, wantErr := Decode(context.Background(), bufiox.NewBytesReader(frame))
		got, gotErr := DecodeInto(context.Background(), bufiox.NewBytesReader(frame), nil, nil)
		if (wantErr == nil) != (gotErr == nil) || (wantErr != nil && wantErr.Error() != gotErr.Error()) || !reflect.DeepEqual(want, got) {
			t.Fatalf("nil maps changed Decode result: got %+v, %v; want %+v, %v", got, gotErr, want, wantErr)
		}
	}
}

func TestDecodeIntoReusesMapAllocations(t *testing.T) {
	var blocks [][]byte
	for i := 0; i < 32; i++ {
		blocks = append(blocks, reuseIntBlock(uint16(i), "value"), reuseStringBlock(strconv.Itoa(i), "value"))
	}
	frame := reuseFrame(blocks...)
	ints, strs := make(map[uint16]string), make(map[string]string)
	decode := func(reuse bool) {
		var err error
		if reuse {
			_, err = DecodeInto(context.Background(), bufiox.NewBytesReader(frame), ints, strs)
			for key := range ints {
				delete(ints, key)
			}
			for key := range strs {
				delete(strs, key)
			}
		} else {
			_, err = Decode(context.Background(), bufiox.NewBytesReader(frame))
		}
		if err != nil {
			panic(err)
		}
	}
	decode(true)
	fresh := testing.AllocsPerRun(100, func() { decode(false) })
	reused := testing.AllocsPerRun(100, func() { decode(true) })
	if reused >= fresh {
		t.Fatalf("map reuse allocated %v, fresh decode %v", reused, fresh)
	}
}
