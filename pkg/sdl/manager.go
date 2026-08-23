package sdl

import "sync"

// MasterChanneler is the interface that master channels must implement
// to be used with ServiceManager. F is the frame type.
type MasterChanneler[F any] interface {
	AddFrame(frame F) error
	GetNextFrame() (F, error)
	HasPendingFrames() bool
}

// ServiceManager manages services and master channels generically.
// S is the service type key, F is the frame type.
// A ServiceManager is safe for concurrent use.
type ServiceManager[S comparable, F any] struct {
	mu sync.Mutex

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
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.virtualServices[vcid]; !exists {
		m.virtualServices[vcid] = make(map[S]Service)
	}
	m.virtualServices[vcid][serviceType] = service
}

// RegisterMasterChannel registers a Master Channel.
func (m *ServiceManager[S, F]) RegisterMasterChannel(scid uint16, mc MasterChanneler[F]) {
	m.mu.Lock()
	defer m.mu.Unlock()

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
	mc, err := m.masterChannel(scid)
	if err != nil {
		return err
	}
	return mc.AddFrame(frame)
}

// GetNextFrameFromMasterChannel retrieves the next frame from the
// Master Channel's multiplexer.
func (m *ServiceManager[S, F]) GetNextFrameFromMasterChannel(scid uint16) (F, error) {
	mc, err := m.masterChannel(scid)
	if err != nil {
		var zero F
		return zero, err
	}
	return mc.GetNextFrame()
}

// HasPendingFramesInMasterChannel checks if a Master Channel has pending frames.
func (m *ServiceManager[S, F]) HasPendingFramesInMasterChannel(scid uint16) bool {
	mc, err := m.masterChannel(scid)
	return err == nil && mc.HasPendingFrames()
}

// masterChannel looks a master channel up under the lock and returns it, so
// the caller can use it without holding the manager's lock. Calling into a
// channel while holding it would serialise every service on the manager
// behind one slow send.
func (m *ServiceManager[S, F]) masterChannel(scid uint16) (MasterChanneler[F], error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mc, exists := m.masterChannels[scid]
	if !exists {
		var zero MasterChanneler[F]
		return zero, ErrMasterChannelNotFound
	}
	return mc, nil
}

// getVirtualService looks a service up under the lock and returns it. As with
// masterChannel, the caller invokes the service outside the lock.
func (m *ServiceManager[S, F]) getVirtualService(vcid uint8, serviceType S) (Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if vcServices, exists := m.virtualServices[vcid]; exists {
		if service, exists := vcServices[serviceType]; exists {
			return service, nil
		}
	}
	return nil, ErrServiceNotFound
}
