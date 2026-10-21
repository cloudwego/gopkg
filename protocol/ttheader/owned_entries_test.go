package ttheader

import (
	"bytes"
	"context"
	"reflect"
	"runtime"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
)

func ownedHeaderParity(t *testing.T, wire []byte) {
	t.Helper()
	old, fresh := new(entriesTestSink), new(entriesTestSink)
	source := bytes.Clone(wire)
	oldReader, reader := bufiox.NewBytesReader(wire), bufiox.NewBytesReader(source)
	want, wantErr := DecodeTo(context.Background(), oldReader, old)
	got, err := DecodeToOwned(context.Background(), reader, fresh)
	if (err == nil) != (wantErr == nil) || (err != nil && err.Error() != wantErr.Error()) || !reflect.DeepEqual(got, want) || reader.ReadLen() != oldReader.ReadLen() || !reflect.DeepEqual(old, fresh) {
		t.Fatalf("decoder mismatch: got=%+v err=%v want=%+v err=%v read=%d/%d sinks=%+v/%+v", got, err, want, wantErr, reader.ReadLen(), oldReader.ReadLen(), fresh, old)
	}
	if err := reader.Release(nil); err != nil {
		t.Fatal(err)
	}
	for i := range source {
		source[i] = 0xcc
	}
	if !reflect.DeepEqual(old, fresh) {
		t.Fatal("owned decoder retained mutable input")
	}
}

func TestDecodeToOwnedParityAndOwnership(t *testing.T) {
	frames := [][]byte{
		entriesTestFrame([]byte{0, 0}),
		entriesTestFrame([]byte{0, 0}, entriesTestStr("dup", "old"), entriesTestInt(3, "caller"), entriesTestStr("dup", "new"), entriesTestACL("old-token"), entriesTestACL("new-token"), entriesTestStr("\x00二进制", "\xff\x00value")),
		entriesTestFrame([]byte{byte(ProtocolIDThriftStruct), 0}, entriesTestStr("", ""), entriesTestInt(65535, "last")),
		entriesTestFrame([]byte{0xff, 0}),
		entriesTestFrame([]byte{0, 0}, []byte{0xfe}),
		entriesTestFrame([]byte{0, 1}),
	}
	for _, wire := range frames {
		for n := 0; n <= len(wire); n++ {
			ownedHeaderParity(t, wire[:n])
		}
	}
	if _, err := DecodeToOwned(context.Background(), bufiox.NewBytesReader(frames[0]), nil); err == nil {
		t.Fatal("nil sink accepted")
	}
	runtime.GC()
}

func FuzzDecodeToOwnedParity(f *testing.F) {
	f.Add(entriesTestFrame([]byte{0, 0}, entriesTestStr("key", "value"), entriesTestInt(3, "caller"), entriesTestACL("token")))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, wire []byte) {
		if len(wire) > 1<<20 {
			return
		}
		ownedHeaderParity(t, wire)
	})
}
