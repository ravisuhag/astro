package pus

import (
	"fmt"
	"sync"
)

// MessageKey names one PUS message type: a service type ID and a message
// subtype ID, the pair that clause 5.3.3.1c calls the message type identifier.
//
// It is written TC[service,subtype] for requests and TM[service,subtype] for
// reports, which is how the standard refers to them throughout.
type MessageKey struct {
	Service uint8
	Subtype uint8
}

// String renders the key the way the standard writes it.
func (k MessageKey) String() string {
	return fmt.Sprintf("[%d,%d]", k.Service, k.Subtype)
}

// Request is the application data of a telecommand: the body that follows the
// TC secondary header. Clause 6 specifies the structure for each request type.
type Request interface {
	// Key returns the message type this request carries.
	Key() MessageKey
	// Encode serializes the application data field.
	Encode() ([]byte, error)
}

// Report is the source data of a telemetry packet: the body that follows the
// TM secondary header. Clause 8 specifies the structure for each report type.
type Report interface {
	// Key returns the message type this report carries.
	Key() MessageKey
	// Encode serializes the source data field.
	Encode() ([]byte, error)
}

// RequestDecoder parses the application data of one request type.
type RequestDecoder func(profile MissionProfile, data []byte) (Request, error)

// ReportDecoder parses the source data of one report type.
type ReportDecoder func(profile MissionProfile, data []byte) (Report, error)

// Registry maps message types to the codecs that handle them.
//
// A mission registers the services it supports; anything unregistered decodes
// to ErrUnknownMessageType rather than being guessed at. That matters because
// PUS lets missions define their own service types in the ranges the standard
// leaves open.
//
// A Registry is safe for concurrent use once built.
type Registry struct {
	mu       sync.RWMutex
	profile  MissionProfile
	requests map[MessageKey]RequestDecoder
	reports  map[MessageKey]ReportDecoder
}

// NewRegistry returns an empty registry bound to a mission profile.
func NewRegistry(profile MissionProfile) (*Registry, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	return &Registry{
		profile:  profile,
		requests: make(map[MessageKey]RequestDecoder),
		reports:  make(map[MessageKey]ReportDecoder),
	}, nil
}

// Profile returns the mission profile this registry decodes against.
func (r *Registry) Profile() MissionProfile { return r.profile }

// RegisterRequest adds a request decoder. Registering a key twice is an error
// rather than a silent overwrite.
func (r *Registry) RegisterRequest(key MessageKey, decoder RequestDecoder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.requests[key]; exists {
		return ErrDuplicateMessageType
	}
	r.requests[key] = decoder
	return nil
}

// RegisterReport adds a report decoder.
func (r *Registry) RegisterReport(key MessageKey, decoder ReportDecoder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.reports[key]; exists {
		return ErrDuplicateMessageType
	}
	r.reports[key] = decoder
	return nil
}

// DecodeRequest parses the application data of a telecommand.
func (r *Registry) DecodeRequest(key MessageKey, data []byte) (Request, error) {
	r.mu.RLock()
	decoder, ok := r.requests[key]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrUnknownMessageType
	}
	return decoder(r.profile, data)
}

// DecodeReport parses the source data of a telemetry packet.
func (r *Registry) DecodeReport(key MessageKey, data []byte) (Report, error) {
	r.mu.RLock()
	decoder, ok := r.reports[key]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrUnknownMessageType
	}
	return decoder(r.profile, data)
}

// KnownRequests lists the registered request types.
func (r *Registry) KnownRequests() []MessageKey {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MessageKey, 0, len(r.requests))
	for k := range r.requests {
		out = append(out, k)
	}
	return out
}

// KnownReports lists the registered report types.
func (r *Registry) KnownReports() []MessageKey {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MessageKey, 0, len(r.reports))
	for k := range r.reports {
		out = append(out, k)
	}
	return out
}

// NewDefaultRegistry returns a registry with every service this package
// implements already registered: ST[01] request verification, ST[03]
// housekeeping, ST[05] event reporting, and ST[17] test.
func NewDefaultRegistry(profile MissionProfile) (*Registry, error) {
	r, err := NewRegistry(profile)
	if err != nil {
		return nil, err
	}
	for _, register := range []func(*Registry) error{
		registerST01, registerST03, registerST05, registerST17,
	} {
		if err := register(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}
