package netx

import (
	"net"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/connstate"
)

var _ Conn = &conn{}

type Conn interface {
	// Conn is extended to provide the native interfaces of net.Conn.
	// NOT recommended to directly call the Write/Read interface.
	// Instead, calling the Reader and Writer to implement higher-performance
	// user mode zero-copy read/writes.
	net.Conn

	// Reader returns bufiox.Reader for nocopy reading.
	Reader() bufiox.Reader
	// Writer returns bufiox.Writer for nocopy writing.
	Writer() bufiox.Writer

	// State returns the state of a connection.
	State() connstate.ConnState
}

type conn struct {
	net.Conn
	stater connstate.ConnStater

	reader bufiox.Reader
	writer bufiox.Writer
}

func (c *conn) Reader() bufiox.Reader {
	return c.reader
}

func (c *conn) Writer() bufiox.Writer {
	return c.writer
}

func (c *conn) State() connstate.ConnState {
	return c.stater.State()
}

func (c *conn) Close() error {
	_ = c.stater.Close()
	return c.Conn.Close()
}

func Wrap(cn net.Conn) (Conn, error) {
	stater, err := connstate.ListenConnState(cn)
	if err != nil {
		return nil, err
	}
	return &conn{
		Conn:   cn,
		stater: stater,
		reader: bufiox.NewDefaultReader(cn),
		writer: bufiox.NewDefaultWriter(cn),
	}, nil
}
