package sle_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/sle"
)

// TestDeliveryModeValues pins the enum to the DeliveryMode INTEGER of the
// common types module.
func TestDeliveryModeValues(t *testing.T) {
	tests := []struct {
		mode sle.DeliveryMode
		want int
		name string
	}{
		{sle.DeliveryReturnTimelyOnline, 0, "return timely online"},
		{sle.DeliveryReturnCompleteOnline, 1, "return complete online"},
		{sle.DeliveryReturnOffline, 2, "return offline"},
		{sle.DeliveryForwardOnline, 3, "forward online"},
		{sle.DeliveryForwardOffline, 4, "forward offline"},
	}

	for _, test := range tests {
		if int(test.mode) != test.want {
			t.Errorf("%s = %d, want %d", test.name, int(test.mode), test.want)
		}
		if test.mode.String() != test.name {
			t.Errorf("String() = %q, want %q", test.mode.String(), test.name)
		}
		if !test.mode.Valid() {
			t.Errorf("%s reported invalid", test.name)
		}
	}
	if sle.DeliveryMode(5).Valid() {
		t.Error("mode 5 reported valid; there are only five")
	}
}

// TestDeliveryModeBehaviourTable is the table the guide describes: which mode
// asks what of the caller.
func TestDeliveryModeBehaviourTable(t *testing.T) {
	tests := []struct {
		mode            sle.DeliveryMode
		isReturn        bool
		isOnline        bool
		allowsDiscard   bool
		needsBackoff    bool
		allowsPastStart bool
		allowsPeriodic  bool
	}{
		{sle.DeliveryReturnTimelyOnline, true, true, true, false, false, true},
		{sle.DeliveryReturnCompleteOnline, true, true, false, true, false, true},
		{sle.DeliveryReturnOffline, true, false, false, true, true, false},
		{sle.DeliveryForwardOnline, false, true, false, false, false, true},
		{sle.DeliveryForwardOffline, false, false, false, false, true, false},
	}

	for _, test := range tests {
		t.Run(test.mode.String(), func(t *testing.T) {
			if test.mode.IsReturn() != test.isReturn {
				t.Errorf("IsReturn() = %v, want %v", test.mode.IsReturn(), test.isReturn)
			}
			if test.mode.IsForward() == test.isReturn {
				t.Errorf("IsForward() and IsReturn() agree, which cannot be right")
			}
			if test.mode.IsOnline() != test.isOnline {
				t.Errorf("IsOnline() = %v, want %v", test.mode.IsOnline(), test.isOnline)
			}
			if test.mode.AllowsDiscard() != test.allowsDiscard {
				t.Errorf("AllowsDiscard() = %v, want %v", test.mode.AllowsDiscard(), test.allowsDiscard)
			}
			if test.mode.RequiresBackpressure() != test.needsBackoff {
				t.Errorf("RequiresBackpressure() = %v, want %v",
					test.mode.RequiresBackpressure(), test.needsBackoff)
			}
			if test.mode.AllowsPastStartTime() != test.allowsPastStart {
				t.Errorf("AllowsPastStartTime() = %v, want %v",
					test.mode.AllowsPastStartTime(), test.allowsPastStart)
			}
			if test.mode.AllowsPeriodicStatusReport() != test.allowsPeriodic {
				t.Errorf("AllowsPeriodicStatusReport() = %v, want %v",
					test.mode.AllowsPeriodicStatusReport(), test.allowsPeriodic)
			}
		})
	}
}

// TestOnlyTimelyOnlineMayDiscard is the one behavioural difference the state
// table encodes directly: row 14 is the only cell that discards data, and it
// is the timely-online row.
func TestOnlyTimelyOnlineMayDiscard(t *testing.T) {
	discarding := 0
	for mode := sle.DeliveryReturnTimelyOnline; mode <= sle.DeliveryForwardOffline; mode++ {
		if mode.AllowsDiscard() {
			discarding++
		}
	}
	if discarding != 1 {
		t.Errorf("%d modes allow discarding, want exactly 1", discarding)
	}
}

// TestOfflineModeRefusesPeriodicReports checks the machine acts on the mode
// rather than only describing it: an offline instance will not ask for
// periodic status reports, because the provider would refuse.
func TestOfflineModeRefusesPeriodicReports(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnOffline)
	bindTo(t, user, provider, now)

	if _, err := user.ScheduleStatusReport(sle.ReportPeriodically, 30, now, 1); err == nil {
		t.Error("an offline instance asked for periodic reports")
	}
	// A one-off report is still fine.
	if _, err := user.ScheduleStatusReport(sle.ReportImmediately, 0, now, 2); err != nil {
		t.Errorf("ScheduleStatusReport(immediately) = %v", err)
	}
}

func TestScheduleStatusReportRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		kind  sle.ReportRequestKind
		cycle uint16
	}{
		{"immediately", sle.ReportImmediately, 0},
		{"periodically", sle.ReportPeriodically, 60},
		{"stop", sle.ReportStop, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := &sle.ScheduleStatusReportInvocation{
				InvokeId:       3,
				Kind:           test.kind,
				ReportingCycle: test.cycle,
			}
			encoded, err := want.Encode()
			if err != nil {
				t.Fatalf("Encode() = %v", err)
			}
			got, err := sle.DecodeScheduleStatusReportInvocation(encoded)
			if err != nil {
				t.Fatalf("DecodeScheduleStatusReportInvocation() = %v", err)
			}
			if got.Kind != test.kind || got.ReportingCycle != test.cycle {
				t.Errorf("request = %v/%d, want %v/%d", got.Kind, got.ReportingCycle, test.kind, test.cycle)
			}
		})
	}
}

// TestReportingCycleBounds pins ReportingCycle ::= INTEGER (2 .. 600).
func TestReportingCycleBounds(t *testing.T) {
	for _, cycle := range []uint16{0, 1, 601, 65535} {
		invocation := &sle.ScheduleStatusReportInvocation{Kind: sle.ReportPeriodically, ReportingCycle: cycle}
		if _, err := invocation.Encode(); !errors.Is(err, sle.ErrInvalidReportingCycle) {
			t.Errorf("cycle %d encoded with err = %v, want ErrInvalidReportingCycle", cycle, err)
		}
	}
	for _, cycle := range []uint16{2, 60, 600} {
		invocation := &sle.ScheduleStatusReportInvocation{Kind: sle.ReportPeriodically, ReportingCycle: cycle}
		if _, err := invocation.Encode(); err != nil {
			t.Errorf("cycle %d rejected: %v", cycle, err)
		}
	}
}
