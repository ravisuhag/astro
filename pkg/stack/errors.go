package stack

import "errors"

var (
	// ErrInvalidConfig indicates a configuration that cannot be built: a
	// frame length that is not positive, no virtual channels, a duplicate or
	// out-of-range channel identifier, or a value the underlying channel
	// profile rejects.
	ErrInvalidConfig = errors.New("downlink configuration is not valid")

	// ErrUnknownChannel indicates a virtual channel the configuration does
	// not name. It is reported rather than ignored: sending to a channel that
	// does not exist would otherwise lose the packet silently.
	ErrUnknownChannel = errors.New("virtual channel is not configured")
)
