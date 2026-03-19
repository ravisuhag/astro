// Package sdl provides shared data link primitives used by both
// TM (tmdl) and TC (tcdl) Space Data Link Protocol implementations.
//
// It contains generic channel buffers, multiplexers, service interfaces,
// and manager types that are parameterized by frame type, avoiding
// code duplication between the TM downlink and TC uplink packages.
package sdl

// Service defines the interface for all Space Data Link services.
type Service interface {
	Send(data []byte) error
	Receive() ([]byte, error)
	Flush() error
}

// PacketSizer returns the total length in bytes of the packet starting
// at data[0], or -1 if the data is too short to determine length.
// Used by packet services to find packet boundaries within frame data.
type PacketSizer func(data []byte) int
