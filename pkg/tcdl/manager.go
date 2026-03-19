package tcdl

// TCServiceManager manages multiple TC services and Master Channels.
type TCServiceManager struct {
	virtualServices map[uint8]map[ServiceType]Service
	masterChannels  map[uint16]*MasterChannel
}

// NewTCServiceManager creates a new TC Service Manager.
func NewTCServiceManager() *TCServiceManager {
	return &TCServiceManager{
		virtualServices: make(map[uint8]map[ServiceType]Service),
		masterChannels:  make(map[uint16]*MasterChannel),
	}
}

// RegisterVirtualService registers a service for a specific VCID and service type.
func (m *TCServiceManager) RegisterVirtualService(vcid uint8, serviceType ServiceType, service Service) {
	if _, exists := m.virtualServices[vcid]; !exists {
		m.virtualServices[vcid] = make(map[ServiceType]Service)
	}
	m.virtualServices[vcid][serviceType] = service
}

// RegisterMasterChannel registers a Master Channel.
func (m *TCServiceManager) RegisterMasterChannel(scid uint16, mc *MasterChannel) {
	m.masterChannels[scid] = mc
}

// SendData sends data using the specified service type for a given VCID.
func (m *TCServiceManager) SendData(vcid uint8, serviceType ServiceType, data []byte) error {
	service, err := m.getVirtualService(vcid, serviceType)
	if err != nil {
		return err
	}
	return service.Send(data)
}

// ReceiveData receives data from the specified service type for a given VCID.
func (m *TCServiceManager) ReceiveData(vcid uint8, serviceType ServiceType) ([]byte, error) {
	service, err := m.getVirtualService(vcid, serviceType)
	if err != nil {
		return nil, err
	}
	return service.Receive()
}

// FlushService flushes the specified service.
func (m *TCServiceManager) FlushService(vcid uint8, serviceType ServiceType) error {
	service, err := m.getVirtualService(vcid, serviceType)
	if err != nil {
		return err
	}
	return service.Flush()
}

// AddFrameToMasterChannel routes a frame to the appropriate VC within the MC.
func (m *TCServiceManager) AddFrameToMasterChannel(scid uint16, frame *TCTransferFrame) error {
	mc, exists := m.masterChannels[scid]
	if !exists {
		return ErrMasterChannelNotFound
	}
	return mc.AddFrame(frame)
}

// GetNextFrameFromMasterChannel retrieves the next frame from the MC multiplexer.
func (m *TCServiceManager) GetNextFrameFromMasterChannel(scid uint16) (*TCTransferFrame, error) {
	mc, exists := m.masterChannels[scid]
	if !exists {
		return nil, ErrMasterChannelNotFound
	}
	return mc.GetNextFrame()
}

// HasPendingFramesInMasterChannel checks if a MC has pending frames.
func (m *TCServiceManager) HasPendingFramesInMasterChannel(scid uint16) bool {
	mc, exists := m.masterChannels[scid]
	return exists && mc.HasPendingFrames()
}

func (m *TCServiceManager) getVirtualService(vcid uint8, serviceType ServiceType) (Service, error) {
	if vcServices, exists := m.virtualServices[vcid]; exists {
		if service, exists := vcServices[serviceType]; exists {
			return service, nil
		}
	}
	return nil, ErrServiceNotFound
}
