package ttheader

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
)

type entriesTestSink struct {
	ints  map[uint16]string
	strs  map[string]string
	calls []string
}

func (s *entriesTestSink) SetInt(k uint16, v string) {
	if s.ints == nil {
		s.ints = make(map[uint16]string)
	}
	s.ints[k] = v
	s.calls = append(s.calls, fmt.Sprintf("int:%d=%s", k, v))
}

func (s *entriesTestSink) SetString(k, v string) {
	if s.strs == nil {
		s.strs = make(map[string]string)
	}
	s.strs[k] = v
	s.calls = append(s.calls, "str:"+k+"="+v)
}

func entriesTestString(s string) []byte {
	b := make([]byte, 2+len(s))
	binary.BigEndian.PutUint16(b, uint16(len(s)))
	copy(b[2:], s)
	return b
}

func entriesTestStr(k, v string) []byte {
	b := append([]byte{byte(InfoIDKeyValue), 0, 1}, entriesTestString(k)...)
	return append(b, entriesTestString(v)...)
}

func entriesTestInt(k uint16, v string) []byte {
	b := []byte{byte(InfoIDIntKeyValue), 0, 1, byte(k >> 8), byte(k)}
	return append(b, entriesTestString(v)...)
}

func entriesTestACL(v string) []byte {
	return append([]byte{byte(InfoIDACLToken)}, entriesTestString(v)...)
}

func entriesTestFrame(prefix []byte, blocks ...[]byte) []byte {
	info := append([]byte(nil), prefix...)
	for _, block := range blocks {
		info = append(info, block...)
	}
	for len(info)%4 != 0 {
		info = append(info, 0)
	}
	b := make([]byte, TTHeaderMetaSize)
	binary.BigEndian.PutUint32(b, uint32(TTHeaderMetaSize+len(info)-4))
	binary.BigEndian.PutUint32(b[4:], TTHeaderMagic|uint32(HeaderFlagSupportOutOfOrder))
	binary.BigEndian.PutUint32(b[8:], 37)
	binary.BigEndian.PutUint16(b[12:], uint16(len(info)/4))
	return append(b, info...)
}

func TestDecodeToMatchesMapOrderAndOwnership(t *testing.T) {
	for _, aclLast := range []bool{false, true} {
		blocks := [][]byte{
			entriesTestInt(3, "old"), entriesTestInt(65535, "unknown"),
			entriesTestStr("", "empty-key"), entriesTestStr("x", "old"),
			entriesTestInt(3, "new"), entriesTestStr("x", "new\x00value"),
			entriesTestInt(32, "independent"), entriesTestStr("empty-value", ""),
		}
		wantToken := "string"
		if aclLast {
			blocks = append(blocks, entriesTestStr(GDPRToken, "string"), entriesTestACL("special"))
			wantToken = "special"
		} else {
			blocks = append(blocks, entriesTestACL("special"), entriesTestStr(GDPRToken, "string"))
		}
		frame := entriesTestFrame([]byte{0, 0}, blocks...)
		want, err := Decode(context.Background(), bufiox.NewBytesReader(frame))
		if err != nil {
			t.Fatal(err)
		}
		sink := new(entriesTestSink)
		in := bufiox.NewBytesReader(frame)
		got, err := DecodeTo(context.Background(), in, sink)
		if err != nil || got.IntInfo != nil || got.StrInfo != nil {
			t.Fatalf("got %+v, %v", got, err)
		}
		got.IntInfo, got.StrInfo = sink.ints, sink.strs
		if !reflect.DeepEqual(got, want) || sink.strs[GDPRToken] != wantToken || sink.ints[32] != "independent" || len(sink.calls) != len(blocks) {
			t.Fatalf("sink differs: %+v / %+v, calls=%v", got, want, sink.calls)
		}
		if err := in.Release(nil); err != nil {
			t.Fatal(err)
		}
		for i := range frame {
			frame[i] = 0xff
		}
		if !reflect.DeepEqual(sink.ints, want.IntInfo) || !reflect.DeepEqual(sink.strs, want.StrInfo) {
			t.Fatal("sink retained reader bytes")
		}
	}
}

func TestDecodeToRetainsExistingAndPartialValues(t *testing.T) {
	frame := entriesTestFrame([]byte{0, 0}, entriesTestStr("first", "value"), entriesTestInt(3, "caller"), []byte{255})
	want, wantErr := DecodeInto(context.Background(), bufiox.NewBytesReader(frame), map[uint16]string{9: "retained"}, map[string]string{"retained": "yes"})
	sink := &entriesTestSink{ints: map[uint16]string{9: "retained"}, strs: map[string]string{"retained": "yes"}}
	got, err := DecodeTo(context.Background(), bufiox.NewBytesReader(frame), sink)
	if err == nil || wantErr == nil || err.Error() != wantErr.Error() || got.IntInfo != nil || got.StrInfo != nil {
		t.Fatalf("partial result %+v, %v; want %v", got, err, wantErr)
	}
	got.IntInfo, got.StrInfo = sink.ints, sink.strs
	if !reflect.DeepEqual(got, want) || len(sink.calls) != 2 {
		t.Fatalf("partial writes changed: %+v / %+v", got, want)
	}
}

func TestDecodeToProtocolAndMalformedParity(t *testing.T) {
	valid := entriesTestFrame([]byte{0, 0}, entriesTestInt(3, "ok"))
	badMagic := append([]byte(nil), valid...)
	badMagic[4] = 0
	badLength := append([]byte(nil), valid...)
	badLength[12], badLength[13] = 0, 0
	frames := [][]byte{
		valid, valid[:13], valid[:len(valid)-1], badMagic, badLength,
		entriesTestFrame([]byte{2, 0}), entriesTestFrame([]byte{255, 0}),
		entriesTestFrame([]byte{0, 255}), entriesTestFrame([]byte{0, 1, 254}),
		entriesTestFrame([]byte{0, 0}, []byte{255}),
		entriesTestFrame([]byte{0, 0}, []byte{byte(InfoIDKeyValue), 0, 1, 255, 255}),
		entriesTestFrame([]byte{0, 0}, []byte{byte(InfoIDIntKeyValue), 0, 2}),
		entriesTestFrame([]byte{0, 0}, []byte{byte(InfoIDACLToken), 255, 255}),
	}
	for _, protocol := range []byte{0, 3, 4, 0x10, 0x11} {
		frames = append(frames, entriesTestFrame([]byte{protocol, 0}))
	}
	for i, frame := range frames {
		want, wantErr := Decode(context.Background(), bufiox.NewBytesReader(frame))
		sink := new(entriesTestSink)
		got, err := DecodeTo(context.Background(), bufiox.NewBytesReader(frame), sink)
		if (err == nil) != (wantErr == nil) || (err != nil && err.Error() != wantErr.Error()) {
			t.Fatalf("case %d errors %v / %v", i, err, wantErr)
		}
		if got.IntInfo != nil || got.StrInfo != nil {
			t.Fatalf("case %d populated maps", i)
		}
		got.IntInfo, got.StrInfo = nil, nil
		want.IntInfo, want.StrInfo = nil, nil
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d facts %+v / %+v", i, got, want)
		}
	}
	if _, err := DecodeTo(context.Background(), bufiox.NewBytesReader(valid), nil); err == nil {
		t.Fatal("nil sink accepted")
	}
}

func TestDecodeToDoesNotConsumePayload(t *testing.T) {
	frame := entriesTestFrame([]byte{0, 0}, entriesTestStr("k", "v"))
	payload := []byte("untouched-payload")
	frame = append(frame, payload...)
	binary.BigEndian.PutUint32(frame, uint32(len(frame)-4))
	in := bufiox.NewBytesReader(frame)
	param, err := DecodeTo(context.Background(), in, new(entriesTestSink))
	if err != nil || param.PayloadLen != len(payload) {
		t.Fatalf("payload facts: %+v %v", param, err)
	}
	got, err := in.Next(len(payload))
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("payload consumed: %q %v", got, err)
	}
}

func entriesTestEncode(t *testing.T, h HeaderEntries) []byte {
	t.Helper()
	var buf []byte
	w := bufiox.NewBytesWriter(&buf)
	param := EncodeParam{Flags: HeaderFlagDuplexReverse, SeqID: 73, ProtocolID: ProtocolIDKitexProtobuf, IntInfo: map[uint16]string{42: "ignored"}, StrInfo: map[string]string{"ignored": strings.Repeat("x", 65536)}}
	length, err := EncodeEntries(context.Background(), param, h, w)
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(length, uint32(w.WrittenLen()-4))
	if err = w.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf
}

func TestEncodedEntriesSizeMatchesMapEncoder(t *testing.T) {
	for count := 0; count <= 4; count++ {
		for _, length := range []int{0, 1, 2, 3, 4, 127, 65535, 65536} {
			for _, acl := range []bool{false, true} {
				h := HeaderEntries{StringKind: 123, IntKind: 456}
				param := EncodeParam{IntInfo: make(map[uint16]string), StrInfo: make(map[string]string)}
				value := strings.Repeat("v", length)
				for i := 0; i < count; i++ {
					key := fmt.Sprintf("key-%d", i)
					h.Entries = append(h.Entries, HeaderEntry{Kind: h.StringKind, Key: key, Value: value}, HeaderEntry{Kind: h.IntKind, ID: uint16(i), Value: value})
					h.StrCount++
					h.IntCount++
					param.StrInfo[key], param.IntInfo[uint16(i)] = value, value
				}
				if acl {
					h.Entries = append(h.Entries, HeaderEntry{Kind: h.StringKind, Key: GDPRToken, Value: value})
					h.GDPRIndex = uint32(len(h.Entries))
					h.StrCount++
					param.StrInfo[GDPRToken] = value
				}
				h.Entries = append(h.Entries, HeaderEntry{Kind: h.StringKind, Key: "deleted", Value: strings.Repeat("x", 65536), Op: 1})
				wire, wantErr := EncodeToBytes(context.Background(), param)
				got, err := EncodedEntriesSize(h)
				if (err == nil) != (wantErr == nil) {
					t.Fatalf("count=%d length=%d acl=%v: size error=%v map error=%v", count, length, acl, err, wantErr)
				}
				if err == nil && (got != len(wire) || got != len(entriesTestEncode(t, h))) {
					t.Fatalf("count=%d length=%d acl=%v: size=%d map bytes=%d", count, length, acl, got, len(wire))
				}
			}
		}
	}
}

func TestEncodeEntriesRoundTripSelectorsAndPadding(t *testing.T) {
	h := HeaderEntries{StringKind: 123, IntKind: 456, StrCount: 3, IntCount: 2, GDPRIndex: 3,
		Entries: []HeaderEntry{
			{Kind: 456, ID: 65535, Value: "unknown"},
			{Kind: 123, Key: "", Value: "empty-key"},
			{Kind: 123, Key: GDPRToken, Value: "token"},
			{Kind: 456, ID: 32, Value: "independent"},
			{Kind: 123, Key: "delete", Value: "deleted", Op: 1},
			{Kind: 456, ID: 3, Value: "deleted", Op: 1},
			{Kind: 123, Key: "last", Value: "\x00"},
		}}
	frame := entriesTestEncode(t, h)
	got, err := Decode(context.Background(), bufiox.NewBytesReader(frame))
	if err != nil || got.Flags != HeaderFlagDuplexReverse || got.SeqID != 73 || got.ProtocolID != ProtocolIDKitexProtobuf || got.PayloadLen != 0 {
		t.Fatalf("roundtrip %+v %v", got, err)
	}
	if !reflect.DeepEqual(got.IntInfo, map[uint16]string{65535: "unknown", 32: "independent"}) || !reflect.DeepEqual(got.StrInfo, map[string]string{"": "empty-key", GDPRToken: "token", "last": "\x00"}) {
		t.Fatalf("headers %+v", got)
	}
	if frame[16] != byte(InfoIDACLToken) || frame[24] != byte(InfoIDKeyValue) || binary.BigEndian.Uint16(frame[25:]) != 2 {
		t.Fatalf("special token/string count: %x", frame)
	}
	for n := 0; n < 8; n++ {
		h := HeaderEntries{StringKind: 5, IntKind: 9, StrCount: 1, Entries: []HeaderEntry{{Kind: 5, Key: "k", Value: strings.Repeat("v", n)}}}
		frame := entriesTestEncode(t, h)
		unpadded := 14 + 2 + 3 + 2 + 1 + 2 + n
		if (len(frame)-14)%4 != 0 || !bytes.Equal(frame[unpadded:], make([]byte, len(frame)-unpadded)) {
			t.Fatalf("padding n=%d: %x", n, frame)
		}
	}
	empty := entriesTestEncode(t, HeaderEntries{StringKind: 1, IntKind: 2, Entries: []HeaderEntry{{Kind: 1, Op: 1}}})
	if len(empty) != 18 {
		t.Fatalf("empty header size %d", len(empty))
	}
	largeValue := strings.Repeat("z", 65535)
	large := entriesTestEncode(t, HeaderEntries{StringKind: 1, IntKind: 2, StrCount: 1, Entries: []HeaderEntry{{Kind: 1, Key: "large", Value: largeValue}}})
	decoded, err := Decode(context.Background(), bufiox.NewBytesReader(large))
	if err != nil || decoded.StrInfo["large"] != largeValue || decoded.HeaderLen != len(large) || decoded.PayloadLen != 0 {
		t.Fatalf("maximum string boundary: len=%d, err=%v", len(large), err)
	}
}

func TestDecodeToReadsMapEncoderOutput(t *testing.T) {
	param := EncodeParam{Flags: HeaderFlagSupportOutOfOrder, SeqID: -7, ProtocolID: ProtocolIDThriftCompactV2, IntInfo: map[uint16]string{0: "zero", 65535: "unknown"}, StrInfo: map[string]string{GDPRToken: "token", "": "empty", "x": "\x00"}}
	frame, err := EncodeToBytes(context.Background(), param)
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(frame, uint32(len(frame)-4))
	sink := new(entriesTestSink)
	got, err := DecodeTo(context.Background(), bufiox.NewBytesReader(frame), sink)
	if err != nil || got.Flags != param.Flags || got.SeqID != param.SeqID || got.ProtocolID != param.ProtocolID || !reflect.DeepEqual(sink.ints, param.IntInfo) || !reflect.DeepEqual(sink.strs, param.StrInfo) {
		t.Fatalf("map encoder parity: %+v %v", got, err)
	}
}

func TestEncodeEntriesRejectsInvalidDescriptors(t *testing.T) {
	valid := HeaderEntries{StringKind: 5, IntKind: 9, StrCount: 1, IntCount: 1, Entries: []HeaderEntry{{Kind: 5, Key: "k", Value: "v"}, {Kind: 9, ID: 3, Value: "caller"}}}
	cases := map[string]func(*HeaderEntries){
		"same-kind":          func(h *HeaderEntries) { h.IntKind = h.StringKind },
		"negative-string":    func(h *HeaderEntries) { h.StrCount = -1 },
		"negative-int":       func(h *HeaderEntries) { h.IntCount = -1 },
		"large-count":        func(h *HeaderEntries) { h.IntCount = 65536 },
		"string-too-small":   func(h *HeaderEntries) { h.StrCount = 0 },
		"string-too-large":   func(h *HeaderEntries) { h.StrCount = 2 },
		"int-too-small":      func(h *HeaderEntries) { h.IntCount = 0 },
		"int-too-large":      func(h *HeaderEntries) { h.IntCount = 2 },
		"unknown-kind":       func(h *HeaderEntries) { h.Entries[0].Kind = 8 },
		"unknown-op":         func(h *HeaderEntries) { h.Entries[0].Op = 2 },
		"deleted-count":      func(h *HeaderEntries) { h.Entries[0].Op = 1 },
		"gdpr-outside":       func(h *HeaderEntries) { h.GDPRIndex = 3 },
		"gdpr-wrong-key":     func(h *HeaderEntries) { h.GDPRIndex = 1 },
		"gdpr-int":           func(h *HeaderEntries) { h.GDPRIndex = 2 },
		"gdpr-missing-index": func(h *HeaderEntries) { h.Entries[0].Key = GDPRToken },
		"gdpr-deleted":       func(h *HeaderEntries) { h.Entries[0].Key = GDPRToken; h.Entries[0].Op = 1; h.GDPRIndex = 1 },
		"gdpr-duplicate": func(h *HeaderEntries) {
			h.Entries[0].Key = GDPRToken
			h.GDPRIndex = 1
			h.StrCount++
			h.Entries = append(h.Entries, h.Entries[0])
		},
		"string-limit": func(h *HeaderEntries) { h.Entries[0].Value = strings.Repeat("v", 65536) },
		"header-limit": func(h *HeaderEntries) {
			h.IntCount = 0
			h.StrCount = 4
			h.Entries = nil
			for i := 0; i < 4; i++ {
				h.Entries = append(h.Entries, HeaderEntry{Kind: 5, Key: string(rune('a' + i)), Value: strings.Repeat("v", 65535)})
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h := valid
			h.Entries = append([]HeaderEntry(nil), valid.Entries...)
			mutate(&h)
			var buf []byte
			if _, err := EncodeEntries(context.Background(), EncodeParam{}, h, bufiox.NewBytesWriter(&buf)); err == nil {
				t.Fatal("invalid descriptor accepted")
			}
		})
	}
}

type entriesFailWriter struct {
	bufiox.Writer
	calls, fail int
}

func (w *entriesFailWriter) Malloc(n int) ([]byte, error) {
	w.calls++
	if w.calls == w.fail {
		return nil, errors.New("entries writer stopped")
	}
	return w.Writer.Malloc(n)
}

func (w *entriesFailWriter) WriteBinary(b []byte) (int, error) {
	w.calls++
	if w.calls == w.fail {
		return 0, errors.New("entries writer stopped")
	}
	return w.Writer.WriteBinary(b)
}

func TestEncodeEntriesWriterFailures(t *testing.T) {
	h := HeaderEntries{StringKind: 7, IntKind: 8, StrCount: 2, IntCount: 1, GDPRIndex: 1, Entries: []HeaderEntry{{Kind: 7, Key: GDPRToken, Value: "token"}, {Kind: 7, Key: "key", Value: "value"}, {Kind: 8, ID: 3, Value: "caller"}}}
	var buf []byte
	w := &entriesFailWriter{Writer: bufiox.NewBytesWriter(&buf)}
	if _, err := EncodeEntries(context.Background(), EncodeParam{}, h, w); err != nil {
		t.Fatal(err)
	}
	for fail := 1; fail <= w.calls; fail++ {
		var buf []byte
		out := &entriesFailWriter{Writer: bufiox.NewBytesWriter(&buf), fail: fail}
		if _, err := EncodeEntries(context.Background(), EncodeParam{}, h, out); err == nil || !strings.Contains(err.Error(), "entries writer stopped") {
			t.Fatalf("write %d: %v", fail, err)
		}
	}
}
