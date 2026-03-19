package tcdl

import "github.com/ravisuhag/astro/pkg/sdl"

// VirtualChannel is a frame buffer for a single TC virtual channel.
type VirtualChannel = sdl.Channel[*TCTransferFrame]

// NewVirtualChannel creates a new TC Virtual Channel with the given VCID and buffer capacity.
func NewVirtualChannel(vcid uint8, bufferSize int) *VirtualChannel {
	return sdl.NewChannel[*TCTransferFrame](vcid, bufferSize)
}
