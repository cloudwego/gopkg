package ttheader

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
)

type reservationWriter struct {
	bufiox.Writer
	reservations int
	failure      error
}

func (w *reservationWriter) Malloc(n int) ([]byte, error) {
	w.reservations++
	if w.failure != nil {
		return nil, w.failure
	}
	return w.Writer.Malloc(n)
}

func TestPackedEntriesReservationCount(t *testing.T) {
	var data []byte
	w := &reservationWriter{Writer: bufiox.NewBytesWriter(&data)}
	headers := HeaderEntries{StringKind: 2, IntKind: 3, StrCount: 2, IntCount: 2, GDPRIndex: 1,
		Entries: []HeaderEntry{{Kind: 2, Key: GDPRToken, Value: "token"}, {Kind: 2, Key: "meta", Value: "binary\x00value"}, {Kind: 3, ID: 3, Value: "from"}, {Kind: 3, ID: 9, Value: "method"}}}
	_, err := EncodeEntries(context.Background(), EncodeParam{}, headers, w)
	if err != nil || w.reservations != 1 {
		t.Fatalf("header reservations=%d, err=%v", w.reservations, err)
	}
}

func TestPackedEntriesReservationError(t *testing.T) {
	w := &reservationWriter{failure: errors.New("allocation failure")}
	_, err := EncodeEntries(context.Background(), EncodeParam{}, HeaderEntries{StringKind: 2, IntKind: 3}, w)
	if err == nil || w.reservations != 1 {
		t.Fatalf("reservation error not returned: %v", err)
	}
}
