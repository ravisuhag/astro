package tmdl

import "github.com/ravisuhag/astro/pkg/sdl"

// TMServiceManager manages multiple TM services and Master Channels,
// wiring the pipeline: Service → VirtualChannel → Mux → MasterChannel.
type TMServiceManager = sdl.ServiceManager[ServiceType, *TMTransferFrame]

// NewTMServiceManager creates a new TM Service Manager.
func NewTMServiceManager() *TMServiceManager {
	return sdl.NewServiceManager[ServiceType, *TMTransferFrame]()
}
