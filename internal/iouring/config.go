package iouring

import "time"

// Config holds the configuration for the IOUringEventLoop.
type Config struct {
	IOUringQueueSize  uint32
	SQEBatchSize       int
	SQESubmitInterval time.Duration
}

// DefaultConfig returns a new Config with default values.
func DefaultConfig() *Config {
	return &Config{
		IOUringQueueSize:  10000,
		SQEBatchSize:       256,
		SQESubmitInterval: 0, // 0 means disabled (submit only on batch size/channel empty)
	}
}
