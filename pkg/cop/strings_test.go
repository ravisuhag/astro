package cop_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/cop"
)

// Every state and alert reason has a name. The loops run past the last
// constant on purpose: a value added to the type without a case in String
// falls through to "unknown", and that is what these catch.

func TestFOPStateNames(t *testing.T) {
	want := map[cop.FOPState]string{
		cop.FOPActive:                "active",
		cop.FOPRetransmitWithoutWait: "retransmit without wait",
		cop.FOPRetransmitWithWait:    "retransmit with wait",
		cop.FOPInitialisingWithoutBC: "initialising without BC frame",
		cop.FOPInitialisingWithBC:    "initialising with BC frame",
		cop.FOPInitial:               "initial",
	}

	for state, name := range want {
		if got := state.String(); got != name {
			t.Errorf("FOPState(%d).String() = %q, want %q", int(state), got, name)
		}
	}

	// Every value from the first constant to the last must be named, so a
	// state inserted in the middle of the iota run is caught.
	for state := cop.FOPActive; state <= cop.FOPInitial; state++ {
		if _, named := want[state]; !named {
			t.Errorf("FOPState(%d) is a declared state with no case in the test",
				int(state))
		}
		if got := state.String(); got == "unknown" {
			t.Errorf("FOPState(%d).String() = %q: a declared state has no name",
				int(state), got)
		}
	}

	if got := cop.FOPState(99).String(); got != "unknown" {
		t.Errorf("FOPState(99).String() = %q, want %q", got, "unknown")
	}
}

func TestFARMStateNames(t *testing.T) {
	want := map[cop.FARMState]string{
		cop.FARMOpen:    "open",
		cop.FARMWait:    "wait",
		cop.FARMLockout: "lockout",
	}

	for state, name := range want {
		if got := state.String(); got != name {
			t.Errorf("FARMState(%d).String() = %q, want %q", int(state), got, name)
		}
	}
	for state := cop.FARMOpen; state <= cop.FARMLockout; state++ {
		if got := state.String(); got == "unknown" {
			t.Errorf("FARMState(%d).String() = %q: a declared state has no name",
				int(state), got)
		}
	}

	if got := cop.FARMState(99).String(); got != "unknown" {
		t.Errorf("FARMState(99).String() = %q, want %q", got, "unknown")
	}
}

func TestAlertReasonNames(t *testing.T) {
	// AlertNone reads as "none" rather than as an empty string, so a log line
	// reporting the pending alert never looks like a missing field.
	if got := cop.AlertNone.String(); got != "none" {
		t.Errorf("AlertNone.String() = %q, want %q", got, "none")
	}

	for reason := cop.AlertNone; reason <= cop.AlertTerminate; reason++ {
		got := reason.String()
		if got == "unknown" {
			t.Errorf("AlertReason(%d).String() = %q: a declared reason has no name",
				int(reason), got)
		}
		if strings.TrimSpace(got) == "" {
			t.Errorf("AlertReason(%d).String() is blank", int(reason))
		}
	}

	if got := cop.AlertReason(99).String(); got != "unknown" {
		t.Errorf("AlertReason(99).String() = %q, want %q", got, "unknown")
	}
}

// The three types satisfy fmt.Stringer, which is what makes them printable
// with %s and %v. Before String existed, %v gave an integer and %s was a vet
// error, so a caller could not log the state a machine reported.
var (
	_ fmt.Stringer = cop.FOPActive
	_ fmt.Stringer = cop.FARMOpen
	_ fmt.Stringer = cop.AlertNone
)

// TestStatesFormatAsText reads the states back off real machines and formats
// them the way a log line would, rather than calling String directly.
func TestStatesFormatAsText(t *testing.T) {
	fop := cop.NewFOP(42, 0, 10)
	fop.Initialize(0)

	got := fmt.Sprintf("FOP-1 is %s, pending alert %s", fop.State(), fop.LastAlert())
	if want := "FOP-1 is active, pending alert none"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	farm := cop.NewFARM(0, 10)
	if got := fmt.Sprintf("FARM-1 is %s", farm.State()); got != "FARM-1 is open" {
		t.Errorf("got %q, want %q", got, "FARM-1 is open")
	}
}
