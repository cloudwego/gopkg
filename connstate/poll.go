package connstate

import (
	"fmt"
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

func createPoller() {
	var err error
	poll, err = openpoll()
	if err != nil {
		panic(fmt.Sprintf("gopkg.connstate openpoll failed, err: %v", err))
	}
	go poll.wait()
}
