package tmdl

import "errors"

// ServiceType defines the types of TM services available.
type ServiceType int

const (
	VCP ServiceType = iota // Virtual Channel Packet Service
	VCA                    // Virtual Channel Access Service
	VCF                    // Virtual Channel Frame Service
)

// TMServiceManager manages multiple TM services, including both Virtual and Master Channel services.
type TMServiceManager struct {
	virtualServices map[uint8]map[ServiceType]Service // Virtual Channel services (VCID -> ServiceType -> Service)
	masterServices  map[uint16]*MasterChannelService  // Master Channel services (SCID -> Service)
}

// NewTMServiceManager creates a new TM Service Manager.
func NewTMServiceManager() *TMServiceManager {
	return &TMServiceManager{
		virtualServices: make(map[uint8]map[ServiceType]Service),
		masterServices:  make(map[uint16]*MasterChannelService),
	}
}

// RegisterVirtualService registers a new Virtual Channel service for a specific VCID and service type.
func (m *TMServiceManager) RegisterVirtualService(vcid uint8, serviceType ServiceType, service Service) {
	if _, exists := m.virtualServices[vcid]; !exists {
		m.virtualServices[vcid] = make(map[ServiceType]Service)
	}
	m.virtualServices[vcid][serviceType] = service
}

// RegisterMasterChannelService registers a new Master Channel service for a specific SCID.
func (m *TMServiceManager) RegisterMasterChannelService(scid uint16) {
	if _, exists := m.masterServices[scid]; !exists {
		m.masterServices[scid] = NewMasterChannelService(scid)
	}
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

// AddFrameToMasterChannel adds a full TM Transfer Frame to the specified Master Channel.
func (m *TMServiceManager) AddFrameToMasterChannel(scid uint16, frame *TMTransferFrame) error {
	masterService, exists := m.masterServices[scid]
	if !exists {
		return errors.New("master channel service not found for the specified SCID")
	}
	return masterService.AddFrame(frame)
}

// GetNextFrameFromMasterChannel retrieves the next frame from the specified Master Channel.
func (m *TMServiceManager) GetNextFrameFromMasterChannel(scid uint16) (*TMTransferFrame, error) {
	masterService, exists := m.masterServices[scid]
	if !exists {
		return nil, errors.New("master channel service not found for the specified SCID")
	}
	return masterService.GetNextFrame()
}

// HasPendingFramesInMasterChannel checks if a Master Channel has pending frames.
func (m *TMServiceManager) HasPendingFramesInMasterChannel(scid uint16) bool {
	masterService, exists := m.masterServices[scid]
	return exists && masterService.HasFrames()
}

// getVirtualService retrieves the virtual service for a given VCID and service type.
func (m *TMServiceManager) getVirtualService(vcid uint8, serviceType ServiceType) (Service, error) {
	if vcServices, exists := m.virtualServices[vcid]; exists {
		if service, exists := vcServices[serviceType]; exists {
			return service, nil
		}
	}
	return nil, errors.New("virtual service not found for specified VCID and service type")
}
