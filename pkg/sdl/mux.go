package sdl

import "slices"

// Multiplexer handles frame scheduling from multiple channels
// using weighted round-robin. F is the frame type.
type Multiplexer[F any] struct {
	channels        map[uint8]*Channel[F]
	priority        map[uint8]int
	sortedIDs       []uint8
	currentIndex    int
	remainingWeight int
}

// NewMultiplexer initializes a new multiplexer.
func NewMultiplexer[F any]() *Multiplexer[F] {
	return &Multiplexer[F]{
		channels: make(map[uint8]*Channel[F]),
		priority: make(map[uint8]int),
	}
}

// AddChannel registers a channel with a priority weight.
// Priority must be at least 1; values below 1 are clamped to 1.
func (mux *Multiplexer[F]) AddChannel(ch *Channel[F], priority int) {
	if priority < 1 {
		priority = 1
	}
	mux.channels[ch.ID] = ch
	mux.priority[ch.ID] = priority

	mux.sortedIDs = make([]uint8, 0, len(mux.channels))
	for id := range mux.channels {
		mux.sortedIDs = append(mux.sortedIDs, id)
	}
	slices.Sort(mux.sortedIDs)

	mux.currentIndex = 0
	if len(mux.sortedIDs) > 0 {
		mux.remainingWeight = mux.priority[mux.sortedIDs[0]]
	}
}

// Next selects the next frame for transmission based on weighted round-robin.
func (mux *Multiplexer[F]) Next() (F, error) {
	if len(mux.sortedIDs) == 0 {
		var zero F
		return zero, ErrNoChannels
	}

	for range len(mux.sortedIDs) {
		id := mux.sortedIDs[mux.currentIndex]
		ch := mux.channels[id]

		if ch.HasFrames() {
			frame, err := ch.Next()
			if err != nil {
				var zero F
				return zero, err
			}
			mux.remainingWeight--
			if mux.remainingWeight <= 0 {
				mux.advanceToNext()
			}
			return frame, nil
		}

		mux.advanceToNext()
	}

	var zero F
	return zero, ErrNoFramesAvailable
}

// HasPending checks if any channel has pending frames.
func (mux *Multiplexer[F]) HasPending() bool {
	for _, ch := range mux.channels {
		if ch.HasFrames() {
			return true
		}
	}
	return false
}

// Len returns the number of registered channels.
func (mux *Multiplexer[F]) Len() int {
	return len(mux.channels)
}

func (mux *Multiplexer[F]) advanceToNext() {
	mux.currentIndex = (mux.currentIndex + 1) % len(mux.sortedIDs)
	id := mux.sortedIDs[mux.currentIndex]
	mux.remainingWeight = mux.priority[id]
}
