package sdl

// MasterChanneler is the interface that master channels must implement
// to be used with ServiceManager. F is the frame type.
type MasterChanneler[F any] interface {
	AddFrame(frame F) error
	GetNextFrame() (F, error)
	HasPendingFrames() bool
}

// ServiceManager manages services and master channels generically.
// S is the service type key, F is the frame type.
type ServiceManager[S comparable, F any] struct {
	virtualServices map[uint8]map[S]Service
	masterChannels  map[uint16]MasterChanneler[F]
}

// NewServiceManager creates a new ServiceManager.
func NewServiceManager[S comparable, F any]() *ServiceManager[S, F] {
	return &ServiceManager[S, F]{
		virtualServices: make(map[uint8]map[S]Service),
		masterChannels:  make(map[uint16]MasterChanneler[F]),
	}
}

// RegisterVirtualService registers a service for a specific VCID and service type.
func (m *ServiceManager[S, F]) RegisterVirtualService(vcid uint8, serviceType S, service Service) {
	if _, exists := m.virtualServices[vcid]; !exists {
		m.virtualServices[vcid] = make(map[S]Service)
	}
	m.virtualServices[vcid][serviceType] = service
}

// RegisterMasterChannel registers a Master Channel.
func (m *ServiceManager[S, F]) RegisterMasterChannel(scid uint16, mc MasterChanneler[F]) {
	m.masterChannels[scid] = mc
}

// SendData sends data using the specified service type for a given VCID.
func (m *ServiceManager[S, F]) SendData(vcid uint8, serviceType S, data []byte) error {
	service, err := m.getVirtualService(vcid, serviceType)
	if err != nil {
		return err
	}
	return service.Send(data)
}

// ReceiveData receives data from the specified service type for a given VCID.
func (m *ServiceManager[S, F]) ReceiveData(vcid uint8, serviceType S) ([]byte, error) {
	service, err := m.getVirtualService(vcid, serviceType)
	if err != nil {
		return nil, err
	}
	return service.Receive()
}

// FlushService flushes the specified service.
func (m *ServiceManager[S, F]) FlushService(vcid uint8, serviceType S) error {
	service, err := m.getVirtualService(vcid, serviceType)
	if err != nil {
		return err
	}
	return service.Flush()
}

// AddFrameToMasterChannel routes a frame to the specified Master Channel.
func (m *ServiceManager[S, F]) AddFrameToMasterChannel(scid uint16, frame F) error {
	mc, exists := m.masterChannels[scid]
	if !exists {
		return ErrMasterChannelNotFound
	}
	return mc.AddFrame(frame)
}

// GetNextFrameFromMasterChannel retrieves the next frame from the
// Master Channel's multiplexer.
func (m *ServiceManager[S, F]) GetNextFrameFromMasterChannel(scid uint16) (F, error) {
	mc, exists := m.masterChannels[scid]
	if !exists {
		var zero F
		return zero, ErrMasterChannelNotFound
	}
	return mc.GetNextFrame()
}

// HasPendingFramesInMasterChannel checks if a Master Channel has pending frames.
func (m *ServiceManager[S, F]) HasPendingFramesInMasterChannel(scid uint16) bool {
	mc, exists := m.masterChannels[scid]
	return exists && mc.HasPendingFrames()
}

func (m *ServiceManager[S, F]) getVirtualService(vcid uint8, serviceType S) (Service, error) {
	if vcServices, exists := m.virtualServices[vcid]; exists {
		if service, exists := vcServices[serviceType]; exists {
			return service, nil
		}
	}
	return nil, ErrServiceNotFound
}
