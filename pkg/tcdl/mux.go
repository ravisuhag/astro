package tcdl

import "github.com/ravisuhag/astro/pkg/sdl"

// VirtualChannelMultiplexer is a weighted round-robin frame scheduler
// for TC Virtual Channels.
type VirtualChannelMultiplexer = sdl.Multiplexer[*TCTransferFrame]

// NewMultiplexer creates a new TC Virtual Channel multiplexer.
func NewMultiplexer() *VirtualChannelMultiplexer {
	return sdl.NewMultiplexer[*TCTransferFrame]()
}
