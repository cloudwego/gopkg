package shmipc

import (
	"fmt"
	"net"
	"syscall"
)

const (
	memfdDataLen = 4
	memfdCount   = 2
)

// unixWriteMsg sends out-of-band data (file descriptors) via Unix domain socket
func unixWriteMsg(conn net.Conn, oob []byte) error {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("conn is not a UnixConn")
	}
	_, _, err := unixConn.WriteMsgUnix(nil, oob, nil)
	return err
}

// unixReadMsg receives out-of-band data (file descriptors) via Unix domain socket
func unixReadMsg(conn net.Conn, oob []byte) (oobn int, err error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, fmt.Errorf("conn is not a UnixConn")
	}
	_, oobn, _, _, err = unixConn.ReadMsgUnix(nil, oob)
	return
}

// sendFileDescriptors sends file descriptors to peer via Unix domain socket
func sendFileDescriptors(conn net.Conn, fds ...int) error {
	oob := syscall.UnixRights(fds...)
	return unixWriteMsg(conn, oob)
}

// receiveFileDescriptors receives file descriptors from peer via Unix domain socket
func receiveFileDescriptors(conn net.Conn) ([]int, error) {
	oob := make([]byte, syscall.CmsgSpace(memfdCount*memfdDataLen))

	oobn, err := unixReadMsg(conn, oob)
	if err != nil {
		return nil, fmt.Errorf("failed to receive file descriptors: %w", err)
	}

	if oobn != len(oob) {
		return nil, fmt.Errorf("expected oob length %d, got %d", len(oob), oobn)
	}

	msgs, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return nil, fmt.Errorf("failed to parse socket control message: %w", err)
	}

	if len(msgs) == 0 {
		return nil, fmt.Errorf("no socket control messages received")
	}

	fds, err := syscall.ParseUnixRights(&msgs[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse unix rights: %w", err)
	}

	if len(fds) < memfdCount {
		return nil, fmt.Errorf("expected at least %d file descriptors, got %d", memfdCount, len(fds))
	}

	return fds, nil
}