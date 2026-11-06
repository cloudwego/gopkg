package ttheader

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/cloudwego/gopkg/bufiox"
)

type HeaderEntry = struct {
	Kind, ID   uint16
	Op         uint32
	Key, Value string
}

type HeaderEntries struct {
	Entries             []HeaderEntry
	IntCount, StrCount  int
	GDPRIndex           uint32
	StringKind, IntKind uint16
}

type HeaderSink interface {
	SetInt(uint16, string)
	SetString(string, string)
}

func DecodeTo(ctx context.Context, in bufiox.Reader, sink HeaderSink) (DecodeParam, error) {
	if sink == nil {
		return DecodeParam{}, fmt.Errorf("nil TTHeader sink")
	}
	return decodeInto(ctx, in, nil, nil, sink)
}

func EncodeEntries(ctx context.Context, param EncodeParam, headers HeaderEntries, out bufiox.Writer) ([]byte, error) {
	if headers.StringKind == headers.IntKind {
		return nil, fmt.Errorf("identical TTHeader entry kinds")
	}
	if headers.IntCount < 0 || headers.IntCount > math.MaxUint16 || headers.StrCount < 0 || headers.StrCount > math.MaxUint16 {
		return nil, fmt.Errorf("invalid TTHeader entry counts[%d,%d]", headers.IntCount, headers.StrCount)
	}
	if headers.GDPRIndex != 0 {
		if uint64(headers.GDPRIndex) > uint64(len(headers.Entries)) || headers.StrCount == 0 {
			return nil, fmt.Errorf("invalid TTHeader GDPR index[%d]", headers.GDPRIndex)
		}
		entry := headers.Entries[headers.GDPRIndex-1]
		if entry.Op != 0 || entry.Kind != headers.StringKind || entry.Key != GDPRToken {
			return nil, fmt.Errorf("invalid TTHeader GDPR entry[%d]", headers.GDPRIndex)
		}
	}
	return encodeEntriesBlock(param, headers, out)
}

func encodeEntriesBlock(param EncodeParam, headers HeaderEntries, out bufiox.Writer) ([]byte, error) {
	stringCount := headers.StrCount
	size := 2
	if headers.GDPRIndex != 0 {
		stringCount--
		size += 3 + len(headers.Entries[headers.GDPRIndex-1].Value)
	}
	if stringCount != 0 {
		size += 3
	}
	if headers.IntCount != 0 {
		size += 3
	}
	stringsWritten, intsWritten := 0, 0
	for i := range headers.Entries {
		entry := &headers.Entries[i]
		if entry.Op > 1 || (entry.Kind != headers.StringKind && entry.Kind != headers.IntKind) {
			return nil, fmt.Errorf("invalid TTHeader entry[%d] kind[%d] op[%d]", i, entry.Kind, entry.Op)
		}
		if entry.Op == 1 {
			continue
		}
		if len(entry.Value) > math.MaxUint16 {
			return nil, fmt.Errorf("TTHeader entry[%d] value too long", i)
		}
		if entry.Kind == headers.StringKind {
			stringsWritten++
			if entry.Key == GDPRToken {
				if uint64(i)+1 != uint64(headers.GDPRIndex) {
					return nil, fmt.Errorf("unindexed TTHeader GDPR entry[%d]", i)
				}
				continue
			}
			if len(entry.Key) > math.MaxUint16 {
				return nil, fmt.Errorf("TTHeader entry[%d] key too long", i)
			}
			size += 4 + len(entry.Key) + len(entry.Value)
		} else {
			intsWritten++
			size += 4 + len(entry.Value)
		}
	}
	if stringsWritten != headers.StrCount || intsWritten != headers.IntCount {
		return nil, fmt.Errorf("TTHeader entry count mismatch[%d,%d/%d,%d]", stringsWritten, intsWritten, headers.StrCount, headers.IntCount)
	}
	padding := (4 - size%4) % 4
	size += padding
	if size > int(MaxHeaderSize) {
		return nil, fmt.Errorf("invalid header length[%d]", size)
	}
	buf, err := out.Malloc(TTHeaderMetaSize + size)
	if err != nil {
		return nil, fmt.Errorf("ttHeader malloc header meta failed, %s", err)
	}
	for i := range buf[:TTHeaderMetaSize] {
		buf[i] = 0
	}
	binary.BigEndian.PutUint32(buf[4:8], TTHeaderMagic+uint32(param.Flags))
	binary.BigEndian.PutUint32(buf[8:12], uint32(param.SeqID))
	binary.BigEndian.PutUint16(buf[12:14], uint16(size/4))
	offset := TTHeaderMetaSize
	buf[offset], buf[offset+1] = byte(param.ProtocolID), 0
	offset += 2
	if headers.GDPRIndex != 0 {
		buf[offset] = byte(InfoIDACLToken)
		offset = writeEntryString(buf, offset+1, headers.Entries[headers.GDPRIndex-1].Value)
	}
	if stringCount > 0 {
		buf[offset] = byte(InfoIDKeyValue)
		binary.BigEndian.PutUint16(buf[offset+1:], uint16(stringCount))
		offset += 3
		for i := range headers.Entries {
			entry := &headers.Entries[i]
			if entry.Op == 0 && entry.Kind == headers.StringKind && entry.Key != GDPRToken {
				offset = writeEntryString(buf, offset, entry.Key)
				offset = writeEntryString(buf, offset, entry.Value)
			}
		}
	}
	if headers.IntCount > 0 {
		buf[offset] = byte(InfoIDIntKeyValue)
		binary.BigEndian.PutUint16(buf[offset+1:], uint16(headers.IntCount))
		offset += 3
		for i := range headers.Entries {
			entry := &headers.Entries[i]
			if entry.Op == 0 && entry.Kind == headers.IntKind {
				binary.BigEndian.PutUint16(buf[offset:], entry.ID)
				offset = writeEntryString(buf, offset+2, entry.Value)
			}
		}
	}
	for i := range buf[offset:] {
		buf[offset+i] = 0
	}
	return buf[:4], nil
}

func writeEntryString(buf []byte, offset int, value string) int {
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(value)))
	offset += 2
	return offset + copy(buf[offset:], value)
}
