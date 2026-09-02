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

	// ErrMissingOCF indicates a downlink carrying an operational control
	// field with nothing to put in it. Four zero octets decode as a valid
	// CLCW reporting V(R)=0, so a sender that invented them would have the
	// ground believe a spacecraft was acknowledging nothing, and FOP-1 would
	// never advance its window. Pass WithOCF instead.
	ErrMissingOCF = errors.New("downlink carries an operational control field but no supplier")
)
