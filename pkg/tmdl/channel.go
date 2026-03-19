package tmdl

import "github.com/ravisuhag/astro/pkg/sdl"

// VirtualChannel is a frame buffer for a single TM virtual channel.
type VirtualChannel = sdl.Channel[*TMTransferFrame]

// NewVirtualChannel creates a new TM Virtual Channel with the given VCID and buffer capacity.
func NewVirtualChannel(vcid uint8, bufferSize int) *VirtualChannel {
	return sdl.NewChannel[*TMTransferFrame](vcid, bufferSize)
}
