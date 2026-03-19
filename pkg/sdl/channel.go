package sdl

import "sync"

// Channel is a generic thread-safe FIFO frame buffer.
// F is the frame type (e.g., *TMTransferFrame or *TCTransferFrame).
type Channel[F any] struct {
	ID      uint8
	mu      sync.Mutex
	buffer  []F
	maxSize int
}

// NewChannel creates a new channel with the given ID and buffer capacity.
func NewChannel[F any](id uint8, bufferSize int) *Channel[F] {
	return &Channel[F]{
		ID:      id,
		buffer:  make([]F, 0, bufferSize),
		maxSize: bufferSize,
	}
}

// Add stores a new frame in the channel buffer.
func (ch *Channel[F]) Add(f F) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if len(ch.buffer) >= ch.maxSize {
		return ErrBufferFull
	}
	ch.buffer = append(ch.buffer, f)
	return nil
}

// Next retrieves and removes the oldest frame from the buffer.
func (ch *Channel[F]) Next() (F, error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if len(ch.buffer) == 0 {
		var zero F
		return zero, ErrNoFramesAvailable
	}
	f := ch.buffer[0]
	var zero F
	ch.buffer[0] = zero // allow GC
	ch.buffer = ch.buffer[1:]
	if len(ch.buffer) == 0 {
		ch.buffer = make([]F, 0, ch.maxSize)
	}
	return f, nil
}

// HasFrames checks if there are frames available.
func (ch *Channel[F]) HasFrames() bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	return len(ch.buffer) > 0
}

// Len returns the number of frames currently buffered.
func (ch *Channel[F]) Len() int {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	return len(ch.buffer)
}
