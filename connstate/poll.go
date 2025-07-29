package connstate

import (
	"sync"
)

type op int

const (
	opAdd op = iota
	opDel
)

var (
	pollInitOnce sync.Once
	poll         poller
)

type poller interface {
	wait() error
	control(fd *fdOperator, op op) error
}
