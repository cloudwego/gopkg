package shmipc

import "runtime"

// Architecture detection constants
const (
	// IsARM indicates whether the runtime architecture is ARM-based (ARM or ARM64)
	IsARM = runtime.GOARCH == "arm64" || runtime.GOARCH == "arm"
)
