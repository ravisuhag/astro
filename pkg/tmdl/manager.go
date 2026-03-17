package tmdl

// TMServiceManager manages multiple TM services and Master Channels,
// wiring the pipeline: Service → VirtualChannel → Mux → MasterChannel.
type TMServiceManager struct {
	virtualServices map[uint8]map[ServiceType]Service
	masterChannels  map[uint16]*MasterChannel
}

// NewTMServiceManager creates a new TM Service Manager.
func NewTMServiceManager() *TMServiceManager {
	return &TMServiceManager{
		virtualServices: make(map[uint8]map[ServiceType]Service),
		masterChannels:  make(map[uint16]*MasterChannel),
	}
}

// RegisterVirtualService registers a Virtual Channel service for a specific VCID and service type.
func (m *TMServiceManager) RegisterVirtualService(vcid uint8, serviceType ServiceType, service Service) {
	if _, exists := m.virtualServices[vcid]; !exists {
		m.virtualServices[vcid] = make(map[ServiceType]Service)
	}
	m.virtualServices[vcid][serviceType] = service
}

// RegisterMasterChannel registers a Master Channel.
func (m *TMServiceManager) RegisterMasterChannel(scid uint16, mc *MasterChannel) {
	m.masterChannels[scid] = mc
}

// SendData sends data using the specified Virtual Channel service type for a given VCID.
func (m *TMServiceManager) SendData(vcid uint8, serviceType ServiceType, data []byte) error {
	service, err := m.getVirtualService(vcid, serviceType)
	if err != nil {
		return err
	}
	return service.Send(data)
}

// ReceiveData receives data from the specified Virtual Channel service type for a given VCID.
func (m *TMServiceManager) ReceiveData(vcid uint8, serviceType ServiceType) ([]byte, error) {
	service, err := m.getVirtualService(vcid, serviceType)
	if err != nil {
		return nil, err
	}
	return service.Receive()
}

// AddFrameToMasterChannel routes a frame to the appropriate Virtual Channel
// within the specified Master Channel.
func (m *TMServiceManager) AddFrameToMasterChannel(scid uint16, frame *TMTransferFrame) error {
	mc, exists := m.masterChannels[scid]
	if !exists {
		return ErrMasterChannelNotFound
	}
	return mc.AddFrame(frame)
}

// GetNextFrameFromMasterChannel retrieves the next frame from the
// Master Channel's multiplexer.
func (m *TMServiceManager) GetNextFrameFromMasterChannel(scid uint16) (*TMTransferFrame, error) {
	mc, exists := m.masterChannels[scid]
	if !exists {
		return nil, ErrMasterChannelNotFound
	}
	return mc.GetNextFrame()
}

// HasPendingFramesInMasterChannel checks if a Master Channel has pending frames.
func (m *TMServiceManager) HasPendingFramesInMasterChannel(scid uint16) bool {
	mc, exists := m.masterChannels[scid]
	return exists && mc.HasPendingFrames()
}

// getVirtualService retrieves the virtual service for a given VCID and service type.
func (m *TMServiceManager) getVirtualService(vcid uint8, serviceType ServiceType) (Service, error) {
	if vcServices, exists := m.virtualServices[vcid]; exists {
		if service, exists := vcServices[serviceType]; exists {
			return service, nil
		}
	}
	return nil, ErrServiceNotFound
}
