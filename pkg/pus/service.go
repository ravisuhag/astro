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

// RegistryOption configures a Registry at construction.
type RegistryOption func(*Registry)

// WithParameterResolver supplies the mission's on-board parameter layouts.
//
// ST[12] needs them: seven of its twenty-eight message types carry limits,
// thresholds, expected values and masks whose widths come from the monitored
// parameter's own definition, and those fields sit in the middle of a repeated
// group rather than at the end of the message. Without the widths there is no
// way to find where the next definition starts.
//
// A registry built without one still decodes the other twenty-one ST[12]
// types. The seven return ErrNoParameterResolver rather than guessing.
func WithParameterResolver(resolve ParameterResolver) RegistryOption {
	return func(r *Registry) { r.parameters = resolve }
}

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

	// parameters resolves on-board parameter layouts for ST[12]. It is set
	// at construction and read, never written, afterwards.
	parameters ParameterResolver
}

// NewRegistry returns an empty registry bound to a mission profile.
func NewRegistry(profile MissionProfile, opts ...RegistryOption) (*Registry, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	r := &Registry{
		profile:  profile,
		requests: make(map[MessageKey]RequestDecoder),
		reports:  make(map[MessageKey]ReportDecoder),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// ParameterResolver returns the resolver this registry decodes ST[12] against,
// or nil if it was built without one.
func (r *Registry) ParameterResolver() ParameterResolver { return r.parameters }

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
// housekeeping, ST[05] event reporting, ST[08] function management, ST[11]
// time-based scheduling, ST[12] on-board monitoring, and ST[17] test.
//
// Pass WithParameterResolver to decode the seven ST[12] message types that
// carry fields sized by the mission's parameter definitions.
func NewDefaultRegistry(profile MissionProfile, opts ...RegistryOption) (*Registry, error) {
	r, err := NewRegistry(profile, opts...)
	if err != nil {
		return nil, err
	}
	for _, register := range []func(*Registry) error{
		registerST01, registerST03, registerST05, registerST08, registerST11,
		registerST12, registerST17,
	} {
		if err := register(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}
