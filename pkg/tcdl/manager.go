package tcdl

import "github.com/ravisuhag/astro/pkg/sdl"

// TCServiceManager manages multiple TC services and Master Channels.
type TCServiceManager = sdl.ServiceManager[ServiceType, *TCTransferFrame]

// NewTCServiceManager creates a new TC Service Manager.
func NewTCServiceManager() *TCServiceManager {
	return sdl.NewServiceManager[ServiceType, *TCTransferFrame]()
}
