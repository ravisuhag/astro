package sdl

import (
	"errors"
	"slices"
	"sync"
)

// MCSource is the interface that master channels must implement
// to participate in physical-channel multiplexing.
type MCSource[F any] interface {
	SCID() uint16
	GetNextFrame() (F, error)
	HasPendingFrames() bool
}

// MCMultiplexer handles weighted round-robin scheduling across
// master channels, keyed by Spacecraft ID (uint16).
// A MCMultiplexer is safe for concurrent use.
type MCMultiplexer[F any] struct {
	mu              sync.Mutex
	channels        map[uint16]MCSource[F]
	priority        map[uint16]int
	sortedSCIDs     []uint16
	currentIndex    int
	remainingWeight int
}

// NewMCMultiplexer creates a new master channel multiplexer.
func NewMCMultiplexer[F any]() *MCMultiplexer[F] {
	return &MCMultiplexer[F]{
		channels: make(map[uint16]MCSource[F]),
		priority: make(map[uint16]int),
	}
}

// Add registers a master channel with a priority weight.
// Priority must be at least 1; values below 1 are clamped to 1.
func (m *MCMultiplexer[F]) Add(mc MCSource[F], priority int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if priority < 1 {
		priority = 1
	}
	scid := mc.SCID()
	m.channels[scid] = mc
	m.priority[scid] = priority

	m.sortedSCIDs = make([]uint16, 0, len(m.channels))
	for s := range m.channels {
		m.sortedSCIDs = append(m.sortedSCIDs, s)
	}
	slices.Sort(m.sortedSCIDs)

	m.currentIndex = 0
	if len(m.sortedSCIDs) > 0 {
		m.remainingWeight = m.priority[m.sortedSCIDs[0]]
	}
}

// Next selects the next frame for transmission using weighted round-robin.
func (m *MCMultiplexer[F]) Next() (F, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sortedSCIDs) == 0 {
		var zero F
		return zero, ErrNoMasterChannels
	}

	for range len(m.sortedSCIDs) {
		scid := m.sortedSCIDs[m.currentIndex]
		mc := m.channels[scid]

		// Take the frame directly rather than asking HasPendingFrames first:
		// the two calls lock the master channel separately, so it could empty
		// in between and this would fail instead of trying the next one.
		frame, err := mc.GetNextFrame()
		if err == nil {
			m.remainingWeight--
			if m.remainingWeight <= 0 {
				m.advanceToNext()
			}
			return frame, nil
		}
		if !errors.Is(err, ErrNoFramesAvailable) {
			var zero F
			return zero, err
		}

		m.advanceToNext()
	}

	var zero F
	return zero, ErrNoFramesAvailable
}

// HasPending checks if any master channel has pending frames.
func (m *MCMultiplexer[F]) HasPending() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, mc := range m.channels {
		if mc.HasPendingFrames() {
			return true
		}
	}
	return false
}

// Len returns the number of registered master channels.
func (m *MCMultiplexer[F]) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.channels)
}

// advanceToNext moves the round-robin cursor on. The caller holds m.mu.
func (m *MCMultiplexer[F]) advanceToNext() {
	m.currentIndex = (m.currentIndex + 1) % len(m.sortedSCIDs)
	scid := m.sortedSCIDs[m.currentIndex]
	m.remainingWeight = m.priority[scid]
}
