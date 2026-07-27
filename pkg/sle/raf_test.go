package sle_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/sle"
)

// testTime is a fixed moment used across the service tests so encodings are
// reproducible.
var testTime = time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

// mustTime converts a Go time or fails the test.
func mustTime(t *testing.T, at time.Time) sle.Time {
	t.Helper()
	converted, err := sle.NewTime(at)
	if err != nil {
		t.Fatalf("NewTime(%v) = %v", at, err)
	}
	return converted
}

func TestRAFStartInvocationRoundTrip(t *testing.T) {
	start := sle.ConditionalTime{Known: true, Time: mustTime(t, testTime)}
	stop := sle.ConditionalTime{Known: true, Time: mustTime(t, testTime.Add(time.Hour))}

	want := &sle.RAFStartInvocation{
		InvokeId:              7,
		StartTime:             start,
		StopTime:              stop,
		RequestedFrameQuality: sle.FrameQualityAll,
	}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeRAFStartInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeRAFStartInvocation() = %v", err)
	}

	if got.InvokeId != want.InvokeId {
		t.Errorf("InvokeId = %d, want %d", got.InvokeId, want.InvokeId)
	}
	if got.RequestedFrameQuality != want.RequestedFrameQuality {
		t.Errorf("RequestedFrameQuality = %v, want %v", got.RequestedFrameQuality, want.RequestedFrameQuality)
	}
	if got.StartTime != want.StartTime || got.StopTime != want.StopTime {
		t.Errorf("times = %+v/%+v, want %+v/%+v", got.StartTime, got.StopTime, want.StartTime, want.StopTime)
	}
}

func TestRAFStartInvocationUndefinedTimes(t *testing.T) {
	// "From now until further notice" is both times undefined, which is the
	// common case for a live pass.
	want := &sle.RAFStartInvocation{InvokeId: 1, RequestedFrameQuality: sle.FrameQualityGoodOnly}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeRAFStartInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeRAFStartInvocation() = %v", err)
	}
	if got.StartTime.Known || got.StopTime.Known {
		t.Errorf("times decoded as known: %+v %+v", got.StartTime, got.StopTime)
	}
}

func TestRAFStartReturnDiagnostics(t *testing.T) {
	tests := []struct {
		name       string
		usedCommon bool
		common     sle.Diagnostics
		specific   sle.RAFStartDiagnostic
	}{
		{"common", true, sle.DiagDuplicateInvokeId, 0},
		{"specific out of service", false, 0, sle.RAFStartOutOfService},
		{"specific invalid stop time", false, 0, sle.RAFStartInvalidStopTime},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := &sle.RAFStartReturn{
				InvokeId:           3,
				Positive:           false,
				UsedCommon:         test.usedCommon,
				CommonDiagnostic:   test.common,
				SpecificDiagnostic: test.specific,
			}
			encoded, err := want.Encode()
			if err != nil {
				t.Fatalf("Encode() = %v", err)
			}
			got, err := sle.DecodeRAFStartReturn(encoded)
			if err != nil {
				t.Fatalf("DecodeRAFStartReturn() = %v", err)
			}
			if got.Positive {
				t.Fatal("decoded as positive")
			}
			if got.UsedCommon != test.usedCommon {
				t.Errorf("UsedCommon = %v, want %v", got.UsedCommon, test.usedCommon)
			}
			if got.CommonDiagnostic != test.common || got.SpecificDiagnostic != test.specific {
				t.Errorf("diagnostics = %v/%v, want %v/%v",
					got.CommonDiagnostic, got.SpecificDiagnostic, test.common, test.specific)
			}
		})
	}
}

func TestRAFTransferDataRoundTrip(t *testing.T) {
	frame := bytes.Repeat([]byte{0xA5}, 1115)

	want := &sle.RAFTransferDataInvocation{
		EarthReceiveTime:      mustTime(t, testTime),
		AntennaId:             sle.AntennaId{Local: []byte("DSS-25")},
		DataLinkContinuity:    -1,
		DeliveredFrameQuality: sle.FrameGood,
		Data:                  frame,
	}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeRAFTransferDataInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeRAFTransferDataInvocation() = %v", err)
	}
	if !bytes.Equal(got.Data, frame) {
		t.Errorf("frame did not survive: %d octets back, want %d", len(got.Data), len(frame))
	}
	if got.DataLinkContinuity != -1 {
		t.Errorf("DataLinkContinuity = %d, want -1", got.DataLinkContinuity)
	}
	if got.AntennaId.String() != "DSS-25" {
		t.Errorf("AntennaId = %q, want %q", got.AntennaId, "DSS-25")
	}
}

func TestRAFTransferDataRejectsOversizedFrame(t *testing.T) {
	// SpaceLinkDataUnit is OCTET STRING (SIZE (1 .. 65536)).
	invocation := &sle.RAFTransferDataInvocation{
		Data: bytes.Repeat([]byte{0}, sle.MaxSpaceLinkDataUnit+1),
	}
	if _, err := invocation.Encode(); err == nil {
		t.Fatal("Encode() accepted a frame past the 65536-octet limit")
	}
}

func TestRAFTransferDataRejectsEmptyFrame(t *testing.T) {
	invocation := &sle.RAFTransferDataInvocation{Data: nil}
	if _, err := invocation.Encode(); err == nil {
		t.Fatal("Encode() accepted an empty frame")
	}
}

func TestSyncNotifyRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		event *sle.SyncNotifyInvocation
	}{
		{"loss of frame sync", &sle.SyncNotifyInvocation{
			Kind: sle.NotifyLossFrameSync,
			LockStatus: &sle.LockStatusReport{
				Time:                 mustTime(t, testTime),
				CarrierLockStatus:    sle.LockInLock,
				SubcarrierLockStatus: sle.LockNotInUse,
				SymbolSyncLockStatus: sle.LockOutOfLock,
			},
		}},
		{"production status change", &sle.SyncNotifyInvocation{
			Kind:             sle.NotifyProductionStatusChange,
			ProductionStatus: sle.ProductionInterrupted,
		}},
		{"excessive backlog", &sle.SyncNotifyInvocation{Kind: sle.NotifyExcessiveDataBacklog}},
		{"end of data", &sle.SyncNotifyInvocation{Kind: sle.NotifyEndOfData}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := test.event.Encode()
			if err != nil {
				t.Fatalf("Encode() = %v", err)
			}
			got, err := sle.DecodeSyncNotifyInvocation(encoded)
			if err != nil {
				t.Fatalf("DecodeSyncNotifyInvocation() = %v", err)
			}
			if got.Kind != test.event.Kind {
				t.Fatalf("Kind = %v, want %v", got.Kind, test.event.Kind)
			}
			if test.event.Kind == sle.NotifyLossFrameSync {
				if got.LockStatus == nil {
					t.Fatal("lock status report went missing")
				}
				if *got.LockStatus != *test.event.LockStatus {
					t.Errorf("lock status = %+v, want %+v", *got.LockStatus, *test.event.LockStatus)
				}
			}
			if test.event.Kind == sle.NotifyProductionStatusChange &&
				got.ProductionStatus != test.event.ProductionStatus {
				t.Errorf("ProductionStatus = %v, want %v", got.ProductionStatus, test.event.ProductionStatus)
			}
		})
	}
}

func TestRAFStatusReportRoundTrip(t *testing.T) {
	want := &sle.RAFStatusReportInvocation{
		ErrorFreeFrameNumber: 1200,
		DeliveredFrameNumber: 1234,
		FrameSyncLockStatus:  sle.LockInLock,
		SymbolSyncLockStatus: sle.LockInLock,
		SubcarrierLockStatus: sle.LockNotInUse,
		CarrierLockStatus:    sle.LockInLock,
		ProductionStatus:     sle.ProductionRunning,
	}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeRAFStatusReportInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeRAFStatusReportInvocation() = %v", err)
	}
	if *got != *want {
		t.Errorf("status report = %+v, want %+v", *got, *want)
	}
}

func TestRAFTransferBufferMixesFramesAndNotifications(t *testing.T) {
	buffer := sle.RAFTransferBuffer{
		{Frame: &sle.RAFTransferDataInvocation{
			EarthReceiveTime: mustTime(t, testTime),
			AntennaId:        sle.AntennaId{Local: []byte("A")},
			Data:             []byte{1, 2, 3, 4},
		}},
		{Notification: &sle.SyncNotifyInvocation{Kind: sle.NotifyEndOfData}},
		{Frame: &sle.RAFTransferDataInvocation{
			EarthReceiveTime: mustTime(t, testTime),
			AntennaId:        sle.AntennaId{Local: []byte("A")},
			Data:             []byte{5, 6, 7, 8},
		}},
	}

	encoded, err := buffer.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeRAFTransferBuffer(encoded)
	if err != nil {
		t.Fatalf("DecodeRAFTransferBuffer() = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("buffer has %d entries, want 3", len(got))
	}
	frames := got.Frames()
	if len(frames) != 2 {
		t.Fatalf("Frames() returned %d, want 2", len(frames))
	}
	if !bytes.Equal(frames[1].Data, []byte{5, 6, 7, 8}) {
		t.Errorf("second frame = % X, want 05 06 07 08", frames[1].Data)
	}
	if got[1].Notification == nil || got[1].Notification.Kind != sle.NotifyEndOfData {
		t.Error("the notification did not survive in the middle of the buffer")
	}
}

func TestRAFDecodersRejectTruncation(t *testing.T) {
	start := &sle.RAFStartInvocation{
		InvokeId:  1,
		StartTime: sle.ConditionalTime{Known: true, Time: mustTime(t, testTime)},
	}
	encoded, err := start.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}

	for n := range len(encoded) {
		if _, err := sle.DecodeRAFStartInvocation(encoded[:n]); err == nil {
			t.Errorf("DecodeRAFStartInvocation accepted %d of %d octets", n, len(encoded))
		}
	}
}
