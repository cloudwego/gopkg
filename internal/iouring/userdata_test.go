package iouring

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

// helper function to get userData for testing
func newTestUserData() *userData {
	return userDataPoolGet()
}

func TestUserData_SetReadOp(t *testing.T) {
	u := newTestUserData()
	defer userDataPoolPut(u)

	fd := int32(3)
	buf1 := []byte("hello")
	buf2 := []byte("world")
	emptyBuf := []byte{}

	u.SetReadOp(fd, buf1, buf2, emptyBuf)

	// Verify SQE configuration
	assert.Equal(t, uint8(IORING_OP_READV), u.sqe.Opcode)
	assert.Equal(t, fd, u.sqe.Fd)
	assert.Equal(t, uint32(2), u.sqe.Len)

	// Verify iovecs - empty buf should be skipped
	assert.Len(t, u.ivs, 2)
	assert.Equal(t, uint64(5), u.ivs[0].Len)
	assert.Equal(t, uint64(5), u.ivs[1].Len)
}

func TestUserData_SetWriteOp(t *testing.T) {
	u := newTestUserData()
	defer userDataPoolPut(u)

	fd := int32(4)
	buf1 := []byte("test")
	buf2 := []byte("data")

	u.SetWriteOp(fd, buf1, buf2)

	// Verify SQE configuration
	assert.Equal(t, uint8(IORING_OP_WRITEV), u.sqe.Opcode)
	assert.Equal(t, fd, u.sqe.Fd)
	assert.Equal(t, uint32(2), u.sqe.Len)

	// Verify iovecs
	assert.Len(t, u.ivs, 2)
	assert.Equal(t, uint64(4), u.ivs[0].Len)
	assert.Equal(t, uint64(4), u.ivs[1].Len)
}

func TestUserData_SetWriteOpEmptyBuffers(t *testing.T) {
	u := newTestUserData()
	defer userDataPoolPut(u)

	u.SetWriteOp(1, []byte{}, []byte{})

	assert.Empty(t, u.ivs)
}

func TestUserData_AdvanceWrite_WRITE(t *testing.T) {
	u := newTestUserData()
	defer userDataPoolPut(u)

	// Setup for IORING_OP_WRITE
	buf := []byte("hello")
	u.sqe.Opcode = IORING_OP_WRITE
	u.sqe.Addr = uint64(uintptr(unsafe.Pointer(&buf[0])))
	u.sqe.Len = uint32(len(buf))

	// Advance by 2 bytes
	total, done := u.AdvanceWrite(2)

	assert.Equal(t, int32(2), total)
	assert.False(t, done)
	assert.Equal(t, uint32(3), u.sqe.Len)

	// Advance remaining bytes
	total, done = u.AdvanceWrite(3)

	assert.Equal(t, int32(5), total)
	assert.True(t, done)
}

func TestUserData_AdvanceWrite_WRITEV(t *testing.T) {
	u := newTestUserData()
	defer userDataPoolPut(u)

	// Setup for IORING_OP_WRITEV with multiple buffers
	buf1 := []byte("hello")
	buf2 := []byte(" ")
	buf3 := []byte("world")
	u.sqe.Opcode = IORING_OP_WRITEV
	u.ivs = []Iovec{
		{Base: uintptr(unsafe.Pointer(&buf1[0])), Len: 5},
		{Base: uintptr(unsafe.Pointer(&buf2[0])), Len: 1},
		{Base: uintptr(unsafe.Pointer(&buf3[0])), Len: 5},
	}

	// Advance by 3 bytes (should consume first buffer partially)
	total, done := u.AdvanceWrite(3)

	assert.Equal(t, int32(3), total)
	assert.False(t, done)
	assert.Len(t, u.ivs, 3)
	assert.Equal(t, uint64(2), u.ivs[0].Len)

	// Advance by 4 bytes (should finish first buffer and consume second)
	total, done = u.AdvanceWrite(4)

	assert.Equal(t, int32(7), total)
	assert.False(t, done)
	assert.Len(t, u.ivs, 1)

	// Advance remaining bytes
	total, done = u.AdvanceWrite(5)

	assert.Equal(t, int32(12), total)
	assert.True(t, done)
	assert.Empty(t, u.ivs)
}

func TestUserData_AdvanceWritePanic(t *testing.T) {
	u := newTestUserData()
	defer userDataPoolPut(u)

	u.sqe.Opcode = IORING_OP_READ // Invalid opcode for AdvanceWrite

	assert.Panics(t, func() {
		u.AdvanceWrite(1)
	})
}