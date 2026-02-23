package spp

import (
	"sync"
)

// APIDManager manages the allocation and validation of APIDs.
type APIDManager struct {
	reserved map[uint16]bool
	mutex    sync.RWMutex
}

// NewAPIDManager creates a new APIDManager instance.
func NewAPIDManager() *APIDManager {
	return &APIDManager{
		reserved: make(map[uint16]bool),
	}
}

// ReserveAPID reserves an APID.
func (m *APIDManager) ReserveAPID(apid uint16) error {
	if apid > 2047 {
		return ErrInvalidAPID
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.reserved[apid] {
		return ErrAPIDAlreadyReserved
	}

	m.reserved[apid] = true
	return nil
}

// ReleaseAPID releases a reserved APID.
func (m *APIDManager) ReleaseAPID(apid uint16) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.reserved, apid)
}

// IsAPIDReserved checks if an APID is reserved.
func (m *APIDManager) IsAPIDReserved(apid uint16) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.reserved[apid]
}

// DescribeAPID provides a description for a given APID.
func DescribeAPID(apid uint16) string {
	switch apid {
	case 0:
		return "Time Code Packet"
	case 1:
		return "Telemetry Packet"
	case 2:
		return "Command Packet"
	case 0x7FF:
		return "Idle Packet"
	default:
		return "Unknown or Custom Packet"
	}
}
