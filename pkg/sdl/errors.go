package sdl

import "errors"

var (
	// ErrNoFramesAvailable indicates there are no frames to retrieve.
	ErrNoFramesAvailable = errors.New("no frames available")

	// ErrBufferFull indicates the channel buffer is full.
	ErrBufferFull = errors.New("channel buffer full")

	// ErrNoChannels indicates no channels are registered on the multiplexer.
	ErrNoChannels = errors.New("no channels registered")

	// ErrServiceNotFound indicates the requested service was not found.
	ErrServiceNotFound = errors.New("service not found for specified key")

	// ErrMasterChannelNotFound indicates the requested master channel was not found.
	ErrMasterChannelNotFound = errors.New("master channel not found for specified SCID")

	// ErrNoMasterChannels indicates no master channels are registered.
	ErrNoMasterChannels = errors.New("no master channels registered")
)
