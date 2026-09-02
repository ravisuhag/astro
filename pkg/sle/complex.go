package sle

import (
	"fmt"
	"sync"
	"time"
)

// Serving many service instances at once.
//
// One ServiceProvider is one service instance on one association. A real
// ground station runs several: a RAF and an ROCF over the same pass, several
// spacecraft in a row, a user that binds and unbinds while another stays up.
//
// A BIND arrives naming the instance it wants, and the station has to route
// it. That is what this does: hold the configured instances, hand an inbound
// BIND to the right one, and refuse the ones that should be refused —
// unknown instance, already bound, wrong version.
//
// The name follows the standard's. CCSDS 910.4-B-2 §4.4.2.1b defines an SLE
// Complex as "a set of SLE-FGs under a single management authority", and its
// transfer ports (§4.4.2.4, §4.4.2.5) are where a user's BIND lands. So the
// Complex, not any one instance, is what decides whether a BIND is
// acceptable. The set of instances provided to one user is a Service Package
// (§4.4.1.2.2), which is negotiated rather than configured, so that is not
// what this type is.
//
// 910.4-B-2 was retired by CMC resolution CMC-R-2023-12-001 on 29 December
// 2023, with nothing named in its place. It is still reference [1] of
// 911.1-B-5, which imports its terms rather than restating them, so it remains
// where the vocabulary comes from.
//
// What this is not is a service agreement. The provision periods, the
// permitted parameter ranges and the scheduling that service management hands
// down are configuration a mission supplies, and modelling them would be
// modelling a mission rather than a protocol. What is here is the routing and
// the admission checks that follow from the instance set alone.

// InstanceConfig describes one service instance the complex will serve.
type InstanceConfig struct {
	// Service is the instance's own configuration, as NewServiceProvider
	// takes it.
	Service ServiceConfig

	// Production configures the transfer buffer and production status. A
	// zero value leaves the instance without production, which is what a
	// forward service like FCLTU wants: it has no frames to buffer.
	Production *ProductionConfig
}

// Instance is one service instance in the complex: its provider, and its
// production when it has any.
type Instance struct {
	// Name is the service instance identifier, rendered the way operators
	// write it.
	Name string

	// Provider answers the operations a user drives.
	Provider *ServiceProvider

	// Production runs the transfer buffer, or nil for an instance configured
	// without one.
	Production *Production

	config InstanceConfig
}

// Kind names the service this instance provides.
func (i *Instance) Kind() ServiceKind { return i.config.Service.Kind }

// Complex holds the service instances a provider serves and routes inbound
// BINDs to them.
//
// It is safe for concurrent use: instances are naturally driven by different
// connections.
type Complex struct {
	mu        sync.Mutex
	instances map[string]*Instance
	// order preserves configuration order, so listing and draining are
	// repeatable rather than map-random.
	order []string
}

// NewComplex prepares a complex with no instances.
func NewComplex() *Complex {
	return &Complex{instances: make(map[string]*Instance)}
}

// Add configures one service instance.
//
// The instance identifier is the key, because that is what a BIND names. Two
// instances cannot share one: a BIND would be ambiguous, and the standard
// gives no way to disambiguate.
func (c *Complex) Add(config InstanceConfig) (*Instance, error) {
	name := config.Service.Instance.String()
	if name == "" {
		return nil, fmt.Errorf("%w: the service instance identifier is empty", ErrInvalidIdentifier)
	}

	provider, err := NewServiceProvider(config.Service)
	if err != nil {
		return nil, err
	}

	var production *Production
	if config.Production != nil {
		if production, err = NewProduction(*config.Production); err != nil {
			return nil, err
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.instances[name]; exists {
		return nil, fmt.Errorf("%w: service instance %q is already configured",
			ErrDuplicateInstance, name)
	}

	instance := &Instance{
		Name:       name,
		Provider:   provider,
		Production: production,
		config:     config,
	}
	c.instances[name] = instance
	c.order = append(c.order, name)

	return instance, nil
}

// Instance returns the instance with this identifier.
func (c *Complex) Instance(name string) (*Instance, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	instance, ok := c.instances[name]
	if !ok {
		return nil, fmt.Errorf("%w: no service instance %q", ErrUnknownInstance, name)
	}
	return instance, nil
}

// Instances lists the configured instances in configuration order.
func (c *Complex) Instances() []*Instance {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]*Instance, 0, len(c.order))
	for _, name := range c.order {
		out = append(out, c.instances[name])
	}
	return out
}

// Len reports how many instances are configured.
func (c *Complex) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.instances)
}

// Route finds the instance a BIND invocation names and checks it can be
// bound.
//
// It is the admission decision, and it returns the diagnostic to answer with
// when the answer is no, so a caller does not have to work out which of the
// BIND diagnostics applies. The diagnostics are the ones annex A defines for
// SleBindReturn:
//
//   - an identifier the complex does not know is 'no such service instance'
//   - one already bound is 'already bound'
//   - a version the instance was not configured for is 'version not supported'
//
// A caller that wants to refuse for a reason outside the instance set — the
// initiator not being the one the agreement names, or the request arriving
// outside a provision period — checks that itself and answers with the
// diagnostic that fits. Those are service-agreement matters, and the complex
// does not hold an agreement.
func (c *Complex) Route(bind *BindInvocation) (*Instance, BindDiagnostic, error) {
	if bind == nil {
		return nil, BindOtherReason, fmt.Errorf("%w: no BIND invocation", ErrInvalidTag)
	}

	name := bind.ServiceInstanceIdentifier.String()

	c.mu.Lock()
	instance, ok := c.instances[name]
	c.mu.Unlock()

	if !ok {
		return nil, BindNoSuchServiceInstance,
			fmt.Errorf("%w: no service instance %q", ErrUnknownInstance, name)
	}

	// A second BIND to a bound instance is the classic collision: two users
	// scheduled over each other, or one that never unbound.
	if state := instance.Provider.State(); state != ServiceUnbound {
		return instance, BindAlreadyBound,
			fmt.Errorf("%w: service instance %q is %s", ErrInstanceInUse, name, state)
	}

	if bind.VersionNumber != instance.config.Service.Version {
		return instance, BindVersionNotSupported,
			fmt.Errorf("%w: service instance %q is version %d, the BIND asks for %d",
				ErrVersionNotSupported, name, instance.config.Service.Version, bind.VersionNumber)
	}

	return instance, BindOtherReason, nil
}

// Abort clears every instance's transfer buffer, for a complex whose
// underlying connection has gone.
//
// §3.1.9.1.12 requires an aborted association to clear its buffer. An abort
// usually takes one association, so a caller with a live complex aborts the
// instance rather than the whole thing; this is for shutdown.
func (c *Complex) Abort() {
	for _, instance := range c.Instances() {
		if instance.Production != nil {
			instance.Production.Abort()
		}
	}
}

// DueInstances lists the instances whose transfer buffer should be released
// now, in configuration order.
//
// A station serving several instances polls this rather than each buffer, so
// one loop drives them all.
func (c *Complex) DueInstances(now time.Time) []*Instance {
	var due []*Instance
	for _, instance := range c.Instances() {
		if instance.Production != nil && instance.Production.Due(now) {
			due = append(due, instance)
		}
	}
	return due
}

// NextDeadline reports the earliest release timer across every instance, and
// whether any is running.
//
// It is what a single-threaded station waits on: the next moment any buffer
// needs attention.
func (c *Complex) NextDeadline() (time.Time, bool) {
	var earliest time.Time
	found := false

	for _, instance := range c.Instances() {
		if instance.Production == nil {
			continue
		}
		deadline, running := instance.Production.Deadline()
		if !running {
			continue
		}
		if !found || deadline.Before(earliest) {
			earliest = deadline
			found = true
		}
	}
	return earliest, found
}

// Humanize returns a human-readable summary of the complex.
func (c *Complex) Humanize() string {
	instances := c.Instances()

	out := fmt.Sprintf("SLE Complex: %d instance(s)", len(instances))
	for _, instance := range instances {
		out += fmt.Sprintf("\n  %s (%s) %s",
			instance.Name, instance.Kind(), instance.Provider.State())
		if instance.Production != nil {
			out += fmt.Sprintf(", production %s, %d record(s) buffered",
				instance.Production.Status(), instance.Production.Pending())
		}
	}
	return out
}
