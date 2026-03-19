package tmdl

import "github.com/ravisuhag/astro/pkg/sdl"

// VirtualChannelMultiplexer is a weighted round-robin frame scheduler
// for TM Virtual Channels.
type VirtualChannelMultiplexer = sdl.Multiplexer[*TMTransferFrame]

// NewMultiplexer creates a new TM Virtual Channel multiplexer.
func NewMultiplexer() *VirtualChannelMultiplexer {
	return sdl.NewMultiplexer[*TMTransferFrame]()
}
