package sdl

import "sync"

// MasterChannelSink is a master channel a set can route frames to.
//
// It extends MCSource with the receiving half: a set both pulls frames out of
// its channels for transmission and pushes frames into them on the way in, and
// a physical channel needs both directions.
type MasterChannelSink[F any] interface {
	MCSource[F]
	AddFrame(F) error
}

// MasterChannelSet holds the master channels of one physical channel: the
// registry that maps a Spacecraft ID to its channel, and the weighted
// round-robin that schedules between them.
//
// It exists because the four data link protocols were each carrying their own
// copy of it. The copies differed in the frame type and in nothing else, which
// is the shape a generic is for — and four copies of the same routing rule is
// four places for it to drift apart.
//
// What stays per protocol is everything that genuinely differs: the channel
// configuration type, the master channel type, where the Spacecraft ID sits in
// a frame header, and the extras one protocol has and another does not, such as
// AOS's idle frame counter or TM's OID fill generator. This holds only the part
// that was identical.
//
// A MasterChannelSet is safe for concurrent use: MCMultiplexer guards its own
// state, and mu guards the registry.
type MasterChannelSet[F any, MC MasterChannelSink[F]] struct {
	mux      *MCMultiplexer[F]
	mu       sync.RWMutex
	channels map[uint16]MC
}

// NewMasterChannelSet returns an empty set.
func NewMasterChannelSet[F any, MC MasterChannelSink[F]]() *MasterChannelSet[F, MC] {
	return &MasterChannelSet[F, MC]{
		mux:      NewMCMultiplexer[F](),
		channels: make(map[uint16]MC),
	}
}

// Add registers a master channel with a scheduling weight.
//
// The weight is a share of the link rather than a rank: a channel weighted 3
// gets three consecutive turns for every one a channel weighted 1 gets, and no
// channel starves another. See MCMultiplexer.
func (s *MasterChannelSet[F, MC]) Add(mc MC, priority int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.channels[mc.SCID()] = mc
	s.mux.Add(mc, priority)
}

// Get returns the master channel for a Spacecraft ID.
func (s *MasterChannelSet[F, MC]) Get(scid uint16) (MC, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mc, ok := s.channels[scid]
	return mc, ok
}

// Route hands a frame to the master channel for the given Spacecraft ID.
//
// A frame for a spacecraft this physical channel does not carry is refused with
// ErrMasterChannelNotFound rather than delivered to whichever channel happens
// to be registered.
func (s *MasterChannelSet[F, MC]) Route(scid uint16, frame F) error {
	s.mu.RLock()
	mc, ok := s.channels[scid]
	s.mu.RUnlock()

	if !ok {
		return ErrMasterChannelNotFound
	}
	return mc.AddFrame(frame)
}

// Lowest returns the master channel with the lowest Spacecraft ID, and false
// when the set is empty.
//
// It exists for idle frames. A physical channel with nothing to send still has
// to fill the link, and the frame it invents needs a Spacecraft ID from
// somewhere. Ranging the registry to pick one looks equivalent and is not: Go
// randomises map iteration, so a channel carrying two spacecraft would stamp
// its idle frames with whichever ID came up that run. Taking the lowest makes
// the fill deterministic, which is what a receiver counting frames on a
// channel needs.
func (s *MasterChannelSet[F, MC]) Lowest() (MC, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var (
		chosen MC
		found  bool
		best   uint16
	)
	for scid, mc := range s.channels {
		if !found || scid < best {
			chosen, best, found = mc, scid, true
		}
	}
	return chosen, found
}

// Next returns the next frame to transmit, chosen by the round-robin.
func (s *MasterChannelSet[F, MC]) Next() (F, error) { return s.mux.Next() }

// HasPending reports whether any master channel has a frame ready.
func (s *MasterChannelSet[F, MC]) HasPending() bool { return s.mux.HasPending() }

// Len returns the number of registered master channels.
func (s *MasterChannelSet[F, MC]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.mux.Len()
}
