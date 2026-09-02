package sle_test

import (
	"errors"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/sle"
)

// instanceID builds a service instance identifier the way operators write
// them: name=value pairs, most significant first.
func instanceID(spacecraft, service string) sle.ServiceInstanceIdentifier {
	return sle.ServiceInstanceIdentifier{
		{Identifier: "sagr", Value: spacecraft},
		{Identifier: "spack", Value: "pass1"},
		{Identifier: "rsl-fg", Value: "1"},
		{Identifier: "raf", Value: service},
	}
}

func complexInstance(t *testing.T, spacecraft, service string, kind sle.ServiceKind, production bool) sle.InstanceConfig {
	t.Helper()

	association, err := sle.NewAssociation(sle.AssociationConfig{
		LocalIdentifier: "GROUND-STN",
		Role:            sle.RoleProvider,
	})
	if err != nil {
		t.Fatalf("NewAssociation: %v", err)
	}

	config := sle.InstanceConfig{
		Service: sle.ServiceConfig{
			Association:   association,
			Kind:          kind,
			DeliveryMode:  sle.DeliveryReturnTimelyOnline,
			Version:       5,
			ResponderPort: "PORT-1",
			Instance:      instanceID(spacecraft, service),
		},
	}
	if production {
		config.Production = &sle.ProductionConfig{BufferSize: 4, LatencyLimit: time.Second}
	}
	return config
}

// A station runs several instances at once, and each is separate: its own
// provider, its own production, its own state.
func TestComplexHoldsManyInstances(t *testing.T) {
	complex := sle.NewComplex()

	raf, err := complex.Add(complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true))
	if err != nil {
		t.Fatalf("Add RAF: %v", err)
	}
	rocf, err := complex.Add(complexInstance(t, "SAT1", "onlc2", sle.ServiceROCF, true))
	if err != nil {
		t.Fatalf("Add ROCF: %v", err)
	}
	// A forward service has no frames to buffer, so no production.
	cltu, err := complex.Add(complexInstance(t, "SAT2", "onlc1", sle.ServiceFCLTU, false))
	if err != nil {
		t.Fatalf("Add FCLTU: %v", err)
	}

	if got := complex.Len(); got != 3 {
		t.Fatalf("complex holds %d instances, want 3", got)
	}
	if raf.Kind() != sle.ServiceRAF || rocf.Kind() != sle.ServiceROCF || cltu.Kind() != sle.ServiceFCLTU {
		t.Error("an instance reports the wrong service kind")
	}
	if cltu.Production != nil {
		t.Error("an instance configured without production has one")
	}
	if raf.Production == nil {
		t.Error("an instance configured with production has none")
	}

	// Configuration order, not map order, so a drain is repeatable.
	instances := complex.Instances()
	if instances[0] != raf || instances[1] != rocf || instances[2] != cltu {
		t.Error("Instances() did not preserve configuration order")
	}
}

// Two instances cannot share an identifier: a BIND naming it would be
// ambiguous, and the standard gives no way to disambiguate.
func TestComplexRefusesDuplicateInstance(t *testing.T) {
	complex := sle.NewComplex()

	if _, err := complex.Add(complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	_, err := complex.Add(complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true))
	if !errors.Is(err, sle.ErrDuplicateInstance) {
		t.Errorf("err = %v, want ErrDuplicateInstance", err)
	}
}

// Routing is the point: a BIND names an instance and the station has to find
// it among several.
func TestComplexRoutesBindToTheRightInstance(t *testing.T) {
	complex := sle.NewComplex()

	first, err := complex.Add(complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	second, err := complex.Add(complexInstance(t, "SAT2", "onlc1", sle.ServiceRAF, true))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	bind := &sle.BindInvocation{
		InitiatorIdentifier:       "MISSION-1",
		ResponderPortIdentifier:   "PORT-1",
		ServiceType:               sle.AppReturnAllFrames,
		VersionNumber:             5,
		ServiceInstanceIdentifier: instanceID("SAT2", "onlc1"),
	}

	instance, _, err := complex.Route(bind)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if instance != second {
		t.Error("the BIND routed to the wrong instance")
	}
	if instance == first {
		t.Error("the BIND routed to the first instance rather than the one it named")
	}
}

// An identifier the complex does not serve draws 'no such service instance',
// which is the diagnostic annex A defines for it.
func TestComplexRoutesUnknownInstance(t *testing.T) {
	complex := sle.NewComplex()

	if _, err := complex.Add(complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true)); err != nil {
		t.Fatalf("Add: %v", err)
	}

	bind := &sle.BindInvocation{
		VersionNumber:             5,
		ServiceInstanceIdentifier: instanceID("SAT9", "onlc1"),
	}

	instance, diagnostic, err := complex.Route(bind)
	if !errors.Is(err, sle.ErrUnknownInstance) {
		t.Errorf("err = %v, want ErrUnknownInstance", err)
	}
	if diagnostic != sle.BindNoSuchServiceInstance {
		t.Errorf("diagnostic = %s, want no such service instance", diagnostic)
	}
	if instance != nil {
		t.Error("an unknown instance came back with an instance anyway")
	}
}

// A second BIND to a bound instance is the classic collision: two users
// scheduled over each other, or one that never unbound.
func TestComplexRoutesAlreadyBoundInstance(t *testing.T) {
	complex := sle.NewComplex()

	instance, err := complex.Add(complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	bind := &sle.BindInvocation{
		InitiatorIdentifier:       "MISSION-1",
		ResponderPortIdentifier:   "PORT-1",
		ServiceType:               sle.AppReturnAllFrames,
		VersionNumber:             5,
		ServiceInstanceIdentifier: instanceID("SAT1", "onlc1"),
	}

	// The first one is fine.
	if _, _, err := complex.Route(bind); err != nil {
		t.Fatalf("the first Route: %v", err)
	}

	// Bind it for real, so the provider leaves the unbound state.
	if err := instance.Provider.HandleBindInvocation(bind, epoch, 1); err != nil {
		t.Fatalf("HandleBindInvocation: %v", err)
	}

	// Now the same BIND collides.
	got, diagnostic, err := complex.Route(bind)
	if !errors.Is(err, sle.ErrInstanceInUse) {
		t.Errorf("err = %v, want ErrInstanceInUse", err)
	}
	if diagnostic != sle.BindAlreadyBound {
		t.Errorf("diagnostic = %s, want already bound", diagnostic)
	}
	// The instance still comes back, so a caller can answer on its
	// association rather than having to look it up again.
	if got != instance {
		t.Error("a refused BIND did not name the instance it collided with")
	}
}

// A version the instance was not configured for draws 'version not
// supported'.
func TestComplexRoutesWrongVersion(t *testing.T) {
	complex := sle.NewComplex()

	if _, err := complex.Add(complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true)); err != nil {
		t.Fatalf("Add: %v", err)
	}

	bind := &sle.BindInvocation{
		VersionNumber:             2,
		ServiceInstanceIdentifier: instanceID("SAT1", "onlc1"),
	}

	_, diagnostic, err := complex.Route(bind)
	if !errors.Is(err, sle.ErrVersionNotSupported) {
		t.Errorf("err = %v, want ErrVersionNotSupported", err)
	}
	if diagnostic != sle.BindVersionNotSupported {
		t.Errorf("diagnostic = %s, want version not supported", diagnostic)
	}
}

// A station serving several instances drives one loop over them rather than
// polling each buffer.
func TestComplexDueInstances(t *testing.T) {
	complex := sle.NewComplex()

	// Buffer size two, so one frame does not fill it.
	first := complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true)
	first.Production = &sle.ProductionConfig{BufferSize: 2, LatencyLimit: time.Second}
	one, err := complex.Add(first)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	second := complexInstance(t, "SAT2", "onlc1", sle.ServiceRAF, true)
	second.Production = &sle.ProductionConfig{BufferSize: 2, LatencyLimit: time.Hour}
	two, err := complex.Add(second)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	for _, instance := range []*sle.Instance{one, two} {
		if _, ok := instance.Production.SetRunning(); !ok {
			t.Fatal("SetRunning was refused")
		}
		if _, err := instance.Production.Insert(frame("one"), epoch); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	// Neither is full, and neither timer has run out yet.
	if due := complex.DueInstances(epoch); len(due) != 0 {
		t.Errorf("%d instances due immediately, want 0", len(due))
	}

	// The first instance's second-long timer expires; the second's hour-long
	// one does not.
	due := complex.DueInstances(epoch.Add(time.Second))
	if len(due) != 1 {
		t.Fatalf("%d instances due after a second, want 1", len(due))
	}
	if due[0] != one {
		t.Error("the wrong instance came back due")
	}

	// The earliest deadline across the complex is the first instance's.
	deadline, running := complex.NextDeadline()
	if !running {
		t.Fatal("no release timer is running across the complex")
	}
	if want := epoch.Add(time.Second); !deadline.Equal(want) {
		t.Errorf("next deadline = %s, want the earlier %s", deadline, want)
	}
}

// With nothing buffered anywhere there is no deadline to wait on.
func TestComplexNextDeadlineWithNothingBuffered(t *testing.T) {
	complex := sle.NewComplex()

	if _, err := complex.Add(complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// And an instance with no production at all, which must not confuse it.
	if _, err := complex.Add(complexInstance(t, "SAT2", "onlc1", sle.ServiceFCLTU, false)); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, running := complex.NextDeadline(); running {
		t.Error("a complex with nothing buffered reported a running timer")
	}
	if due := complex.DueInstances(epoch.Add(time.Hour)); len(due) != 0 {
		t.Errorf("%d instances due with nothing buffered", len(due))
	}
}

// Clause 3.1.9.1.12: an abort clears the buffers, because there is nowhere left to
// deliver them.
func TestComplexAbortClearsEveryBuffer(t *testing.T) {
	complex := sle.NewComplex()

	for _, spacecraft := range []string{"SAT1", "SAT2"} {
		instance, err := complex.Add(complexInstance(t, spacecraft, "onlc1", sle.ServiceRAF, true))
		if err != nil {
			t.Fatalf("Add: %v", err)
		}
		if _, ok := instance.Production.SetRunning(); !ok {
			t.Fatal("SetRunning was refused")
		}
		if _, err := instance.Production.Insert(frame("one"), epoch); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	complex.Abort()

	for _, instance := range complex.Instances() {
		if instance.Production == nil {
			continue
		}
		if got := instance.Production.Pending(); got != 0 {
			t.Errorf("instance %s still holds %d records after the abort", instance.Name, got)
		}
	}
}

func TestComplexInstanceLookup(t *testing.T) {
	complex := sle.NewComplex()

	added, err := complex.Add(complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	found, err := complex.Instance(added.Name)
	if err != nil {
		t.Fatalf("Instance: %v", err)
	}
	if found != added {
		t.Error("Instance returned a different instance")
	}

	if _, err := complex.Instance("sagr=nope"); !errors.Is(err, sle.ErrUnknownInstance) {
		t.Errorf("err = %v, want ErrUnknownInstance", err)
	}
}

func TestComplexRefusesEmptyIdentifier(t *testing.T) {
	complex := sle.NewComplex()

	config := complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true)
	config.Service.Instance = nil

	if _, err := complex.Add(config); err == nil {
		t.Error("an instance with no identifier was accepted")
	}
}

func TestComplexRefusesBadProductionConfig(t *testing.T) {
	complex := sle.NewComplex()

	config := complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true)
	config.Production = &sle.ProductionConfig{BufferSize: 0}

	if _, err := complex.Add(config); !errors.Is(err, sle.ErrInvalidProductionConfig) {
		t.Errorf("err = %v, want ErrInvalidProductionConfig", err)
	}
}

func TestComplexRouteRejectsNil(t *testing.T) {
	complex := sle.NewComplex()

	if _, _, err := complex.Route(nil); err == nil {
		t.Error("Route accepted a nil BIND")
	}
}

func TestComplexHumanize(t *testing.T) {
	complex := sle.NewComplex()

	if _, err := complex.Add(complexInstance(t, "SAT1", "onlc1", sle.ServiceRAF, true)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got := complex.Humanize(); got == "" {
		t.Error("Humanize returned nothing")
	}
}
