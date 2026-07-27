package sle_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/sle"
)

func TestFCLTUStartRoundTrip(t *testing.T) {
	want := &sle.FCLTUStartInvocation{InvokeId: 1, FirstCltuIdentification: 1000}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeFCLTUStartInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeFCLTUStartInvocation() = %v", err)
	}
	if got.FirstCltuIdentification != 1000 {
		t.Errorf("FirstCltuIdentification = %d, want 1000", got.FirstCltuIdentification)
	}
}

// TestFCLTUStartReturnCarriesTheRadiationWindow records what makes FCLTU's
// START return different from the return services': its positive result is a
// SEQUENCE holding the window the provider reserved, not an empty NULL.
func TestFCLTUStartReturnCarriesTheRadiationWindow(t *testing.T) {
	want := &sle.FCLTUStartReturn{
		InvokeId:           2,
		Positive:           true,
		StartRadiationTime: mustTime(t, testTime),
		StopRadiationTime:  sle.ConditionalTime{Known: true, Time: mustTime(t, testTime.Add(time.Hour))},
	}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeFCLTUStartReturn(encoded)
	if err != nil {
		t.Fatalf("DecodeFCLTUStartReturn() = %v", err)
	}
	if !got.Positive {
		t.Fatal("decoded as negative")
	}
	if got.StartRadiationTime != want.StartRadiationTime {
		t.Errorf("start radiation = %+v, want %+v", got.StartRadiationTime, want.StartRadiationTime)
	}
	if got.StopRadiationTime != want.StopRadiationTime {
		t.Errorf("stop radiation = %+v, want %+v", got.StopRadiationTime, want.StopRadiationTime)
	}
}

func TestFCLTUStartReturnOpenEndedWindow(t *testing.T) {
	want := &sle.FCLTUStartReturn{
		InvokeId:           3,
		Positive:           true,
		StartRadiationTime: mustTime(t, testTime),
	}
	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeFCLTUStartReturn(encoded)
	if err != nil {
		t.Fatalf("DecodeFCLTUStartReturn() = %v", err)
	}
	if got.StopRadiationTime.Known {
		t.Error("stop radiation time decoded as known; the window was open ended")
	}
}

func TestFCLTUTransferDataRoundTrip(t *testing.T) {
	cltu := bytes.Repeat([]byte{0xEB, 0x90}, 100)

	want := &sle.FCLTUTransferDataInvocation{
		InvokeId:                 5,
		CltuIdentification:       1001,
		EarliestTransmissionTime: sle.ConditionalTime{Known: true, Time: mustTime(t, testTime)},
		DelayTime:                250,
		RadiationNotification:    sle.ProduceNotification,
		Data:                     cltu,
	}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeFCLTUTransferDataInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeFCLTUTransferDataInvocation() = %v", err)
	}
	if !bytes.Equal(got.Data, cltu) {
		t.Errorf("CLTU did not survive: %d octets back, want %d", len(got.Data), len(cltu))
	}
	if got.CltuIdentification != 1001 {
		t.Errorf("CltuIdentification = %d, want 1001", got.CltuIdentification)
	}
	if got.DelayTime != 250 {
		t.Errorf("DelayTime = %d, want 250", got.DelayTime)
	}
	if got.RadiationNotification != sle.ProduceNotification {
		t.Errorf("RadiationNotification = %v, want produce", got.RadiationNotification)
	}
	if got.LatestTransmissionTime.Known {
		t.Error("latest transmission time decoded as known; it was left undefined")
	}
}

func TestFCLTUTransferDataReturnReportsBufferAndExpectedID(t *testing.T) {
	want := &sle.FCLTUTransferDataReturn{
		InvokeId:            5,
		CltuIdentification:  1002,
		CltuBufferAvailable: 40960,
		Positive:            true,
	}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeFCLTUTransferDataReturn(encoded)
	if err != nil {
		t.Fatalf("DecodeFCLTUTransferDataReturn() = %v", err)
	}
	if *got != *want {
		t.Errorf("return = %+v, want %+v", *got, *want)
	}
}

func TestFCLTUTransferDataReturnDiagnostics(t *testing.T) {
	want := &sle.FCLTUTransferDataReturn{
		InvokeId:           6,
		CltuIdentification: 1002,
		Positive:           false,
		SpecificDiagnostic: sle.FCLTUDataOutOfSequence,
	}
	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeFCLTUTransferDataReturn(encoded)
	if err != nil {
		t.Fatalf("DecodeFCLTUTransferDataReturn() = %v", err)
	}
	if got.Positive || got.SpecificDiagnostic != sle.FCLTUDataOutOfSequence {
		t.Errorf("return = %+v, want a refusal for out of sequence", *got)
	}
}

// TestCltuStatusValuesHaveAHole pins the ForwardDuStatus numbering. Value 3 is
// 'acknowledged', which belongs to the Forward Space Packet service, so FCLTU
// jumps from 2 to 4.
func TestCltuStatusValuesHaveAHole(t *testing.T) {
	if sle.CltuInterrupted != 2 {
		t.Errorf("interrupted = %d, want 2", sle.CltuInterrupted)
	}
	if sle.CltuProductionStarted != 4 {
		t.Errorf("radiation started = %d, want 4", sle.CltuProductionStarted)
	}
	if sle.CltuProductionNotStarted != 5 {
		t.Errorf("radiation not started = %d, want 5", sle.CltuProductionNotStarted)
	}
	if sle.CltuStatus(3).Valid() {
		t.Error("value 3 reported as valid; it is FSP's 'acknowledged'")
	}
}

// TestFCLTUProductionStatusDiffersFromTheReturnServices guards the trap that
// made these separate Go types: FCLTU has four states and the return services
// three, and the numbers disagree at 1.
func TestFCLTUProductionStatusDiffersFromTheReturnServices(t *testing.T) {
	if sle.FCLTUProductionConfigured != 1 {
		t.Errorf("FCLTU 'configured' = %d, want 1", sle.FCLTUProductionConfigured)
	}
	if sle.ProductionInterrupted != 1 {
		t.Errorf("return-service 'interrupted' = %d, want 1", sle.ProductionInterrupted)
	}
	if sle.FCLTUProductionHalted != 3 {
		t.Errorf("FCLTU 'halted' = %d, want 3", sle.FCLTUProductionHalted)
	}
	if sle.ProductionHalted != 2 {
		t.Errorf("return-service 'halted' = %d, want 2", sle.ProductionHalted)
	}
}

func TestFCLTUAsyncNotifyRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		notify *sle.FCLTUAsyncNotifyInvocation
	}{
		{"CLTU radiated", &sle.FCLTUAsyncNotifyInvocation{
			Kind: sle.NotifyCltuRadiated,
			LastProcessed: sle.CltuLastProcessed{
				Processed:          true,
				CltuIdentification: 1001,
				RadiationStartTime: sle.ConditionalTime{Known: true, Time: mustTime(t, testTime)},
				Status:             sle.CltuRadiated,
			},
			LastOk: sle.CltuLastOk{
				Ok:                 true,
				CltuIdentification: 1001,
				RadiationStopTime:  mustTime(t, testTime),
			},
			ProductionStatus: sle.FCLTUProductionOperational,
			UplinkStatus:     sle.UplinkNominal,
		}},
		{"buffer empty", &sle.FCLTUAsyncNotifyInvocation{
			Kind:             sle.NotifyBufferEmpty,
			ProductionStatus: sle.FCLTUProductionConfigured,
			UplinkStatus:     sle.UplinkNoRfAvailable,
		}},
		{"action list completed", &sle.FCLTUAsyncNotifyInvocation{
			Kind:              sle.NotifyActionListCompleted,
			EventInvocationId: 77,
			ProductionStatus:  sle.FCLTUProductionOperational,
			UplinkStatus:      sle.UplinkNominal,
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := test.notify.Encode()
			if err != nil {
				t.Fatalf("Encode() = %v", err)
			}
			got, err := sle.DecodeFCLTUAsyncNotifyInvocation(encoded)
			if err != nil {
				t.Fatalf("DecodeFCLTUAsyncNotifyInvocation() = %v", err)
			}
			if got.Kind != test.notify.Kind {
				t.Fatalf("Kind = %v, want %v", got.Kind, test.notify.Kind)
			}
			if got.EventInvocationId != test.notify.EventInvocationId {
				t.Errorf("EventInvocationId = %d, want %d",
					got.EventInvocationId, test.notify.EventInvocationId)
			}
			if got.LastProcessed != test.notify.LastProcessed {
				t.Errorf("LastProcessed = %+v, want %+v", got.LastProcessed, test.notify.LastProcessed)
			}
			if got.LastOk != test.notify.LastOk {
				t.Errorf("LastOk = %+v, want %+v", got.LastOk, test.notify.LastOk)
			}
			if got.ProductionStatus != test.notify.ProductionStatus {
				t.Errorf("ProductionStatus = %v, want %v", got.ProductionStatus, test.notify.ProductionStatus)
			}
			if got.UplinkStatus != test.notify.UplinkStatus {
				t.Errorf("UplinkStatus = %v, want %v", got.UplinkStatus, test.notify.UplinkStatus)
			}
		})
	}
}

func TestFCLTUThrowEventRoundTrip(t *testing.T) {
	want := &sle.FCLTUThrowEventInvocation{
		InvokeId:                      8,
		EventInvocationIdentification: 5,
		EventIdentifier:               42,
		EventQualifier:                []byte("switch-antenna"),
	}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeFCLTUThrowEventInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeFCLTUThrowEventInvocation() = %v", err)
	}
	if got.EventIdentifier != 42 {
		t.Errorf("EventIdentifier = %d, want 42", got.EventIdentifier)
	}
	if !bytes.Equal(got.EventQualifier, want.EventQualifier) {
		t.Errorf("EventQualifier = %q, want %q", got.EventQualifier, want.EventQualifier)
	}
}

func TestFCLTUThrowEventRejectsOutOfRangeFields(t *testing.T) {
	tests := []struct {
		name  string
		event *sle.FCLTUThrowEventInvocation
	}{
		{"zero event identifier", &sle.FCLTUThrowEventInvocation{
			EventIdentifier: 0, EventQualifier: []byte("x"),
		}},
		{"empty qualifier", &sle.FCLTUThrowEventInvocation{
			EventIdentifier: 1, EventQualifier: nil,
		}},
		{"oversized qualifier", &sle.FCLTUThrowEventInvocation{
			EventIdentifier: 1,
			EventQualifier:  bytes.Repeat([]byte{0}, sle.MaxEventQualifier+1),
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.event.Encode(); err == nil {
				t.Error("Encode() accepted an out-of-range THROW-EVENT")
			}
		})
	}
}

func TestFCLTUStatusReportRoundTrip(t *testing.T) {
	want := &sle.FCLTUStatusReportInvocation{
		LastProcessed: sle.CltuLastProcessed{
			Processed:          true,
			CltuIdentification: 9,
			Status:             sle.CltuRadiated,
		},
		LastOk:                 sle.CltuLastOk{Ok: true, CltuIdentification: 9, RadiationStopTime: mustTime(t, testTime)},
		ProductionStatus:       sle.FCLTUProductionOperational,
		UplinkStatus:           sle.UplinkNominal,
		NumberOfCltusReceived:  100,
		NumberOfCltusProcessed: 98,
		NumberOfCltusRadiated:  97,
		CltuBufferAvailable:    65536,
	}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeFCLTUStatusReportInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeFCLTUStatusReportInvocation() = %v", err)
	}
	if *got != *want {
		t.Errorf("report = %+v, want %+v", *got, *want)
	}
}

func TestCltuLastProcessedAndLastOkAbsentForms(t *testing.T) {
	notify := &sle.FCLTUAsyncNotifyInvocation{Kind: sle.NotifyProductionInterrupted}

	encoded, err := notify.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeFCLTUAsyncNotifyInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeFCLTUAsyncNotifyInvocation() = %v", err)
	}
	if got.LastProcessed.Processed {
		t.Error("LastProcessed decoded as present; nothing had been processed")
	}
	if got.LastOk.Ok {
		t.Error("LastOk decoded as present; nothing had been radiated")
	}
}

func TestFCLTUDecodersRejectTruncation(t *testing.T) {
	transfer := &sle.FCLTUTransferDataInvocation{
		InvokeId:           1,
		CltuIdentification: 1,
		Data:               []byte{0xEB, 0x90, 0x00, 0x01},
	}
	encoded, err := transfer.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	for n := range len(encoded) {
		if _, err := sle.DecodeFCLTUTransferDataInvocation(encoded[:n]); err == nil {
			t.Errorf("DecodeFCLTUTransferDataInvocation accepted %d of %d octets", n, len(encoded))
		}
	}
}

// TestFCLTUTagsDifferFromTheReturnServices guards the mapping that DecodePDU
// depends on. Tag 8 is THROW-EVENT here and TRANSFER-BUFFER in RAF, so
// decoding an FCLTU PDU with the wrong table would name the wrong operation.
func TestFCLTUTagsDifferFromTheReturnServices(t *testing.T) {
	content := []byte{0x05, 0x00}
	pdu := sle.AppendPDU(nil, 8, content)

	asFCLTU, err := sle.DecodePDU(pdu, sle.ServiceFCLTU)
	if err != nil {
		t.Fatalf("DecodePDU(FCLTU) = %v", err)
	}
	asRAF, err := sle.DecodePDU(pdu, sle.ServiceRAF)
	if err != nil {
		t.Fatalf("DecodePDU(RAF) = %v", err)
	}

	if asFCLTU.Operation != sle.OpThrowEventInvocation {
		t.Errorf("FCLTU tag 8 = %v, want THROW-EVENT invocation", asFCLTU.Operation)
	}
	if asRAF.Operation != sle.OpTransferBuffer {
		t.Errorf("RAF tag 8 = %v, want TRANSFER-BUFFER", asRAF.Operation)
	}
}
