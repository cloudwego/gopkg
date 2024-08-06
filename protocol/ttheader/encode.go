package ttheader

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/gopkg/kio"
)

/**
 *  TTHeader Protocol
 *  +-------------2Byte--------------|-------------2Byte-------------+
 *	+----------------------------------------------------------------+
 *	| 0|                          LENGTH                             |
 *	+----------------------------------------------------------------+
 *	| 0|       HEADER MAGIC          |            FLAGS              |
 *	+----------------------------------------------------------------+
 *	|                         SEQUENCE NUMBER                        |
 *	+----------------------------------------------------------------+
 *	| 0|     Header Size(/32)        | ...
 *	+---------------------------------
 *
 *	Header is of variable size:
 *	(and starts at offset 14)
 *
 *	+----------------------------------------------------------------+
 *	| PROTOCOL ID  |NUM TRANSFORMS . |TRANSFORM 0 ID (uint8)|
 *	+----------------------------------------------------------------+
 *	|  TRANSFORM 0 DATA ...
 *	+----------------------------------------------------------------+
 *	|         ...                              ...                   |
 *	+----------------------------------------------------------------+
 *	|        INFO 0 ID (uint8)      |       INFO 0  DATA ...
 *	+----------------------------------------------------------------+
 *	|         ...                              ...                   |
 *	+----------------------------------------------------------------+
 *	|                                                                |
 *	|                              PAYLOAD                           |
 *	|                                                                |
 *	+----------------------------------------------------------------+
 */

// Header keys
const (
	// Header Magics
	// 0 and 16th bits must be 0 to differentiate from framed & unframed
	TTHeaderMagic     uint32 = 0x10000000
	MeshHeaderMagic   uint32 = 0xFFAF0000
	MeshHeaderLenMask uint32 = 0x0000FFFF

	// HeaderMask        uint32 = 0xFFFF0000
	FlagsMask     uint32 = 0x0000FFFF
	MethodMask    uint32 = 0x41000000 // method first byte [A-Za-z_]
	MaxFrameSize  uint32 = 0x3FFFFFFF
	MaxHeaderSize uint32 = 65536

	initialBufferSize = 256
)

type HeaderFlags uint16

const (
	HeaderFlagsKey              string      = "HeaderFlags"
	HeaderFlagSupportOutOfOrder HeaderFlags = 0x01
	HeaderFlagDuplexReverse     HeaderFlags = 0x08
	HeaderFlagSASL              HeaderFlags = 0x10
)

const (
	TTHeaderMetaSize = 14
)

// ProtocolID is the wrapped protocol id used in THeader.
type ProtocolID uint8

// Supported ProtocolID values.
const (
	ProtocolIDThriftBinary    ProtocolID = 0x00
	ProtocolIDThriftCompact   ProtocolID = 0x02 // Kitex not support
	ProtocolIDThriftCompactV2 ProtocolID = 0x03 // Kitex not support
	ProtocolIDKitexProtobuf   ProtocolID = 0x04
	ProtocolIDDefault                    = ProtocolIDThriftBinary
)

type InfoIDType uint8 // uint8

const (
	InfoIDPadding     InfoIDType = 0
	InfoIDKeyValue    InfoIDType = 0x01
	InfoIDIntKeyValue InfoIDType = 0x10
	InfoIDACLToken    InfoIDType = 0x11
)

// key of acl token
// You can set up acl token through metainfo.
// eg:
//
//	ctx = metainfo.WithValue(ctx, "gdpr-token", "your token")
const (
	// GDPRToken is used to set up gdpr token into InfoIDACLToken
	GDPRToken = metainfo.PrefixTransient + "gdpr-token"
)

type EncodeParam struct {
	Flags HeaderFlags

	SeqID int32

	ProtocolID ProtocolID

	// IntInfo is used to set up int key-value info into InfoIDIntKeyValue
	IntInfo map[uint16]string

	// StrInfo is used to set up string key-value info into InfoIDKeyValue
	StrInfo map[string]string
}

// EncodeToBytes encode ttheader to bytes.
// NOTICE: Must call
//
//	`binary.BigEndian.PutUint32(buf, uint32(totalLen))`
//
// after encoding both header and payload data to set total length of a request/response.
// And `totalLen` should be the length of header + payload - 4.
// You may refer to unit tests for examples.
func EncodeToBytes(ctx context.Context, param EncodeParam) (buf []byte, err error) {
	out := kio.NewReaderWriterBuffer(0)
	if _, err = Encode(ctx, param, out); err != nil {
		return
	}
	if err = out.Flush(); err != nil {
		return
	}
	buf, err = out.Next(out.ReadableLen())
	_ = out.Release(nil)
	return
}

func Encode(ctx context.Context, param EncodeParam, out kio.ByteBuffer) (totalLenField []byte, err error) {
	// 1. header meta
	var headerMeta []byte
	headerMeta, err = out.Malloc(TTHeaderMetaSize)
	if err != nil {
		return nil, fmt.Errorf("ttHeader malloc header meta failed, %s", err.Error())
	}

	totalLenField = headerMeta[0:4]
	headerInfoSizeField := headerMeta[12:14]
	binary.BigEndian.PutUint32(headerMeta[4:8], TTHeaderMagic+uint32(param.Flags))
	binary.BigEndian.PutUint32(headerMeta[8:12], uint32(param.Flags))

	var transformIDs []uint8 // transformIDs not support TODO compress
	// 2.  header info, malloc and write
	if err = kio.WriteByte(byte(param.ProtocolID), out); err != nil {
		return nil, fmt.Errorf("ttHeader write protocol id failed, %s", err.Error())
	}
	if err = kio.WriteByte(byte(len(transformIDs)), out); err != nil {
		return nil, fmt.Errorf("ttHeader write transformIDs length failed, %s", err.Error())
	}
	for tid := range transformIDs {
		if err = kio.WriteByte(byte(tid), out); err != nil {
			return nil, fmt.Errorf("ttHeader write transformIDs failed, %s", err.Error())
		}
	}
	// PROTOCOL ID(u8) + NUM TRANSFORMS(always 0)(u8) + TRANSFORM IDs([]u8)
	headerInfoSize := 1 + 1 + len(transformIDs)
	headerInfoSize, err = writeKVInfo(headerInfoSize, param.IntInfo, param.StrInfo, out)
	if err != nil {
		return nil, fmt.Errorf("ttHeader write kv info failed, %s", err.Error())
	}

	if uint32(headerInfoSize) > MaxHeaderSize {
		return nil, fmt.Errorf("invalid header length[%d]", headerInfoSize)
	}
	binary.BigEndian.PutUint16(headerInfoSizeField, uint16(headerInfoSize/4))
	return totalLenField, nil
}

func writeKVInfo(writtenSize int, intKVMap map[uint16]string, strKVMap map[string]string, out kio.ByteBuffer) (writeSize int, err error) {
	writeSize = writtenSize
	// str kv info
	strKVSize := len(strKVMap)
	// write gdpr token into InfoIDACLToken
	// supplementary doc: https://www.cloudwego.io/docs/kitex/reference/transport_protocol_ttheader/
	if gdprToken, ok := strKVMap[GDPRToken]; ok {
		strKVSize--
		// INFO ID TYPE(u8)
		if err = kio.WriteByte(byte(InfoIDACLToken), out); err != nil {
			return writeSize, err
		}
		writeSize += 1

		wLen, err := kio.WriteString2BLen(gdprToken, out)
		if err != nil {
			return writeSize, err
		}
		writeSize += wLen
	}

	if strKVSize > 0 {
		// INFO ID TYPE(u8) + NUM HEADERS(u16)
		if err = kio.WriteByte(byte(InfoIDKeyValue), out); err != nil {
			return writeSize, err
		}
		if err = kio.WriteUint16(uint16(strKVSize), out); err != nil {
			return writeSize, err
		}
		writeSize += 3
		for key, val := range strKVMap {
			if key == GDPRToken {
				continue
			}
			keyWLen, err := kio.WriteString2BLen(key, out)
			if err != nil {
				return writeSize, err
			}
			valWLen, err := kio.WriteString2BLen(val, out)
			if err != nil {
				return writeSize, err
			}
			writeSize = writeSize + keyWLen + valWLen
		}
	}

	// int kv info
	intKVSize := len(intKVMap)
	if intKVSize > 0 {
		// INFO ID TYPE(u8) + NUM HEADERS(u16)
		if err = kio.WriteByte(byte(InfoIDIntKeyValue), out); err != nil {
			return writeSize, err
		}
		if err = kio.WriteUint16(uint16(intKVSize), out); err != nil {
			return writeSize, err
		}
		writeSize += 3
		for key, val := range intKVMap {
			if err = kio.WriteUint16(key, out); err != nil {
				return writeSize, err
			}
			valWLen, err := kio.WriteString2BLen(val, out)
			if err != nil {
				return writeSize, err
			}
			writeSize = writeSize + 2 + valWLen
		}
	}

	// padding = (4 - headerInfoSize%4) % 4
	padding := (4 - writeSize%4) % 4
	paddingBuf, err := out.Malloc(padding)
	if err != nil {
		return writeSize, err
	}
	for i := 0; i < len(paddingBuf); i++ {
		paddingBuf[i] = byte(0)
	}
	writeSize += padding
	return
}
