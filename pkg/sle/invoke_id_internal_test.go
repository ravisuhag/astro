package sle

import "testing"

// newTestServiceProvider builds a bare *ServiceProvider over a fresh provider
// association, with no service-specific (RAF/RCF/ROCF/FCLTU) behaviour
// attached. That is enough to drive registerInvokeId and settleInvokeId
// directly: this file is a white-box test of the invoke identifier tracking
// itself, not of any one service's state machine.
func newTestServiceProvider(t *testing.T) *ServiceProvider {
	t.Helper()

	assoc, err := NewAssociation(AssociationConfig{
		Role:            RoleProvider,
		LocalIdentifier: "GROUND-STN",
	})
	if err != nil {
		t.Fatalf("NewAssociation() = %v", err)
	}
	provider, err := NewServiceProvider(ServiceConfig{
		Association:   assoc,
		Version:       5,
		ResponderPort: "PORT",
	})
	if err != nil {
		t.Fatalf("NewServiceProvider() = %v", err)
	}
	return provider
}

// settle is a test helper for the locked settleInvokeId, since the real
// callers (via respond) already hold p.mu across the whole step.
func settle(p *ServiceProvider, id InvokeId) {
	p.mu.Lock()
	p.settleInvokeId(id)
	p.mu.Unlock()
}

// TestProviderInvokeIdRecyclesAfterWrap is the important case B5 exists for:
// InvokeId is 16 bits, and a confirmed operation such as FCLTU TRANSFER-DATA
// burns one per CLTU, so an ordinary long pass wraps the identifier space
// well within one association. Before this fix, seenInvokeIds remembered
// every identifier ever seen since BIND, so operation 65,537 — which reuses
// identifier 0 — was answered ErrDuplicateInvokeId forever, wedging the
// association. This test drives exactly that many register+settle cycles
// directly, rather than pumping 65k real CLTUs, so it runs in milliseconds.
func TestProviderInvokeIdRecyclesAfterWrap(t *testing.T) {
	p := newTestServiceProvider(t)

	// One full trip around the 16-bit identifier space: register then settle
	// every identifier from 0 to 65535, exactly as the provider does for each
	// confirmed operation it answers.
	for i := 0; i < 65536; i++ {
		id := InvokeId(uint16(i))
		if !p.registerInvokeId(id) {
			t.Fatalf("registerInvokeId(%d) at iteration %d = false, want true", id, i)
		}
		settle(p, id)
	}

	// Operation 65,537 reuses identifier 0. It must be accepted — the
	// identifier space recycles — not answered as a duplicate.
	if !p.registerInvokeId(0) {
		t.Fatal("registerInvokeId(0) after a full wrap = false, want true: invoke identifiers must recycle")
	}
	settle(p, 0)
}

// TestProviderInvokeIdRejectsGenuineDuplicate is the property Step 1 must not
// lose: an invocation still outstanding, or one just answered, still draws
// the duplicate diagnostic on a second try with the same identifier.
func TestProviderInvokeIdRejectsGenuineDuplicate(t *testing.T) {
	p := newTestServiceProvider(t)

	// Still outstanding: the first invocation has not been answered yet.
	if !p.registerInvokeId(7) {
		t.Fatal("first registerInvokeId(7) = false, want true")
	}
	if p.registerInvokeId(7) {
		t.Fatal("registerInvokeId(7) while outstanding = true, want false (duplicate)")
	}

	// Just answered: a retransmission of the same invocation right after its
	// return was queued must still be refused.
	settle(p, 7)
	if p.registerInvokeId(7) {
		t.Fatal("registerInvokeId(7) immediately after being answered = true, want false (duplicate)")
	}
}

// TestProviderAnsweredInvokeIdWindowStaysBounded asserts the
// recently-answered window does not grow linearly with the number of
// operations the provider has handled — it is what lets the identifier
// space recycle instead of being remembered forever.
func TestProviderAnsweredInvokeIdWindowStaysBounded(t *testing.T) {
	p := newTestServiceProvider(t)

	const operations = 10_000
	for i := 0; i < operations; i++ {
		id := InvokeId(uint16(i))
		if !p.registerInvokeId(id) {
			t.Fatalf("registerInvokeId(%d) = false, want true", id)
		}
		settle(p, id)
	}

	p.mu.Lock()
	got := len(p.answeredInvokeIdOrder)
	p.mu.Unlock()

	if got > answeredInvokeIdWindow {
		t.Fatalf("recently-answered window has %d entries after %d operations, want at most %d",
			got, operations, answeredInvokeIdWindow)
	}
}

// TestServiceProviderResetInvokeIdTrackingClearsWindow guards the BIND/UNBIND
// reset path: resetInvokeIdTracking must drop both the outstanding set and
// the recently-answered window, so a fresh association starts with a fresh
// identifier space rather than inheriting the previous one's tail.
func TestServiceProviderResetInvokeIdTrackingClearsWindow(t *testing.T) {
	p := newTestServiceProvider(t)

	if !p.registerInvokeId(42) {
		t.Fatal("registerInvokeId(42) = false, want true")
	}
	settle(p, 42)

	p.mu.Lock()
	p.resetInvokeIdTracking()
	p.mu.Unlock()

	if !p.registerInvokeId(42) {
		t.Fatal("registerInvokeId(42) after reset = false, want true")
	}
}
