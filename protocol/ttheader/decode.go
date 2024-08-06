package ttheader

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/gopkg/kio"
	"github.com/cloudwego/gopkg/protocol/util"
)

const (
	// MagicMask is bit mask for checking version.
	MagicMask = 0xffff0000
)

type DecodeParam struct {
	Flags HeaderFlags

	SeqID int32

	ProtocolID ProtocolID

	// IntInfo is used to set up int key-value info into InfoIDIntKeyValue
	IntInfo map[uint16]string

	// StrInfo is used to set up string key-value info into InfoIDKeyValue
	StrInfo map[string]string

	PayloadLen int
}

func DecodeFromBytes(ctx context.Context, bs []byte) (param DecodeParam, err error) {
	in := kio.NewReaderBuffer(bs)
	param, err = Decode(ctx, in)
	_ = in.Release(nil)
	return
}

func Decode(ctx context.Context, in kio.ByteBuffer) (param DecodeParam, err error) {
	var headerMeta []byte
	headerMeta, err = in.Next(TTHeaderMetaSize)
	if err != nil {
		return
	}
	if !IsTTHeader(headerMeta) {
		err = errors.New("not TTHeader protocol")
		return
	}
	totalLen := util.Bytes2Uint32NoCheck(headerMeta[:kio.Size32])

	flags := util.Bytes2Uint16NoCheck(headerMeta[kio.Size16*3:])
	param.Flags = HeaderFlags(flags)

	seqID := util.Bytes2Uint32NoCheck(headerMeta[kio.Size32*2 : kio.Size32*3])
	param.SeqID = int32(seqID)

	headerInfoSize := util.Bytes2Uint16NoCheck(headerMeta[kio.Size32*3:TTHeaderMetaSize]) * 4
	if uint32(headerInfoSize) > MaxHeaderSize || headerInfoSize < 2 {
		err = fmt.Errorf("invalid header length[%d]", headerInfoSize)
		return
	}

	var headerInfo []byte
	if headerInfo, err = in.Next(int(headerInfoSize)); err != nil {
		return
	}
	if err = checkProtocolID(headerInfo[0]); err != nil {
		return
	}
	hdIdx := 2
	transformIDNum := int(headerInfo[1])
	if int(headerInfoSize)-hdIdx < transformIDNum {
		err = fmt.Errorf("need read %d transformIDs, but not enough", transformIDNum)
		return
	}
	transformIDs := make([]uint8, transformIDNum)
	for i := 0; i < transformIDNum; i++ {
		transformIDs[i] = headerInfo[hdIdx]
		hdIdx++
	}

	param.IntInfo, param.StrInfo, err = readKVInfo(hdIdx, headerInfo)
	if err != nil {
		err = fmt.Errorf("ttHeader read kv info failed, %s, headerInfo=%#x", err.Error(), headerInfo)
		return
	}

	param.PayloadLen = int(totalLen - uint32(headerInfoSize) + kio.Size32 - TTHeaderMetaSize)
	return
}

/**
 * +------------------------------------------------------------+
 * |                  4Byte                 |       2Byte       |
 * +------------------------------------------------------------+
 * |   			     Length			    	|   HEADER MAGIC    |
 * +------------------------------------------------------------+
 */
func IsTTHeader(flagBuf []byte) bool {
	return binary.BigEndian.Uint32(flagBuf[kio.Size32:])&MagicMask == TTHeaderMagic
}

func readKVInfo(idx int, buf []byte) (intKVMap map[uint16]string, strKVMap map[string]string, err error) {
	for {
		var infoID uint8
		infoID, err = util.Bytes2Uint8(buf, idx)
		idx++
		if err != nil {
			// this is the last field, read until there is no more padding
			if err == io.EOF {
				break
			} else {
				return
			}
		}
		switch InfoIDType(infoID) {
		case InfoIDPadding:
			continue
		case InfoIDKeyValue:
			if strKVMap == nil {
				strKVMap = make(map[string]string)
			}
			_, err = readStrKVInfo(&idx, buf, strKVMap)
			if err != nil {
				return
			}
		case InfoIDIntKeyValue:
			if intKVMap == nil {
				intKVMap = make(map[uint16]string)
			}
			_, err = readIntKVInfo(&idx, buf, intKVMap)
			if err != nil {
				return
			}
		case InfoIDACLToken:
			if strKVMap == nil {
				strKVMap = make(map[string]string)
			}
			if err = readACLToken(&idx, buf, strKVMap); err != nil {
				return
			}
		default:
			err = fmt.Errorf("invalid infoIDType[%#x]", infoID)
			return
		}
	}
	return
}

func readIntKVInfo(idx *int, buf []byte, info map[uint16]string) (has bool, err error) {
	kvSize, err := util.Bytes2Uint16(buf, *idx)
	*idx += 2
	if err != nil {
		return false, fmt.Errorf("error reading int kv info size: %s", err.Error())
	}
	if kvSize <= 0 {
		return false, nil
	}
	for i := uint16(0); i < kvSize; i++ {
		key, err := util.Bytes2Uint16(buf, *idx)
		*idx += 2
		if err != nil {
			return false, fmt.Errorf("error reading int kv info: %s", err.Error())
		}
		val, n, err := util.ReadString2BLen(buf, *idx)
		*idx += n
		if err != nil {
			return false, fmt.Errorf("error reading int kv info: %s", err.Error())
		}
		info[key] = val
	}
	return true, nil
}

func readStrKVInfo(idx *int, buf []byte, info map[string]string) (has bool, err error) {
	kvSize, err := util.Bytes2Uint16(buf, *idx)
	*idx += 2
	if err != nil {
		return false, fmt.Errorf("error reading str kv info size: %s", err.Error())
	}
	if kvSize <= 0 {
		return false, nil
	}
	for i := uint16(0); i < kvSize; i++ {
		key, n, err := util.ReadString2BLen(buf, *idx)
		*idx += n
		if err != nil {
			return false, fmt.Errorf("error reading str kv info: %s", err.Error())
		}
		val, n, err := util.ReadString2BLen(buf, *idx)
		*idx += n
		if err != nil {
			return false, fmt.Errorf("error reading str kv info: %s", err.Error())
		}
		info[key] = val
	}
	return true, nil
}

// readACLToken reads acl token
func readACLToken(idx *int, buf []byte, info map[string]string) error {
	val, n, err := util.ReadString2BLen(buf, *idx)
	*idx += n
	if err != nil {
		return fmt.Errorf("error reading acl token: %s", err.Error())
	}
	info[GDPRToken] = val
	return nil
}

// protoID just for ttheader
func checkProtocolID(protoID uint8) error {
	switch protoID {
	case uint8(ProtocolIDThriftBinary):
	case uint8(ProtocolIDKitexProtobuf):
	case uint8(ProtocolIDThriftCompactV2):
		// just for compatibility
	default:
		return fmt.Errorf("unsupported ProtocolID[%d]", protoID)
	}
	return nil
}
