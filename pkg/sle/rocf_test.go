package sle_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/pkg/sle"
)

func TestControlWordTypeRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		filter sle.ControlWordType
	}{
		{"all control words", sle.ControlWordType{Kind: sle.ControlWordAll}},
		{"CLCW from any TC VC", sle.ControlWordType{Kind: sle.ControlWordCLCW}},
		{"CLCW from TC VC 3", sle.ControlWordType{
			Kind:                sle.ControlWordCLCW,
			TCVirtualChannel:    3,
			HasTCVirtualChannel: true,
		}},
		{"not CLCW", sle.ControlWordType{Kind: sle.ControlWordNotCLCW}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded := sle.AppendControlWordType(nil, test.filter)
			element, err := sle.NewDecoder(encoded).Next()
			if err != nil {
				t.Fatalf("Next() = %v", err)
			}
			got, err := sle.DecodeControlWordType(element)
			if err != nil {
				t.Fatalf("DecodeControlWordType() = %v", err)
			}
			if got != test.filter {
				t.Errorf("filter = %+v, want %+v", got, test.filter)
			}
		})
	}
}

func TestROCFStartInvocationRoundTrip(t *testing.T) {
	want := &sle.ROCFStartInvocation{
		InvokeId:       11,
		StartTime:      sle.ConditionalTime{Known: true, Time: mustTime(t, testTime)},
		RequestedGVCID: sle.GVCID{SpacecraftID: 200, VersionNumber: sle.FrameVersionTM, VirtualChannelID: 2},
		ControlWordType: sle.ControlWordType{
			Kind:                sle.ControlWordCLCW,
			TCVirtualChannel:    1,
			HasTCVirtualChannel: true,
		},
		UpdateMode: sle.UpdateChangeBased,
	}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeROCFStartInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeROCFStartInvocation() = %v", err)
	}
	if got.RequestedGVCID != want.RequestedGVCID {
		t.Errorf("channel = %+v, want %+v", got.RequestedGVCID, want.RequestedGVCID)
	}
	if got.ControlWordType != want.ControlWordType {
		t.Errorf("control word = %+v, want %+v", got.ControlWordType, want.ControlWordType)
	}
	if got.UpdateMode != want.UpdateMode {
		t.Errorf("update mode = %v, want %v", got.UpdateMode, want.UpdateMode)
	}
}

// TestROCFTransferDataCarriesFourOctets pins the payload size. An operational
// control field is the four octets a TM or AOS frame ends with, and a CLCW is
// what usually sits in them — pkg/cop decodes those.
func TestROCFTransferDataCarriesFourOctets(t *testing.T) {
	ocf := []byte{0x00, 0x01, 0x02, 0x03}

	want := &sle.ROCFTransferDataInvocation{
		EarthReceiveTime: mustTime(t, testTime),
		AntennaId:        sle.AntennaId{Local: []byte("DSS-43")},
		Data:             ocf,
	}
	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeROCFTransferDataInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeROCFTransferDataInvocation() = %v", err)
	}
	if !bytes.Equal(got.Data, ocf) {
		t.Errorf("OCF = % X, want % X", got.Data, ocf)
	}
}

func TestROCFStatusReportCountsFramesAndOCFsSeparately(t *testing.T) {
	// The two counters differ on purpose: not every frame carries a control
	// field the filter wants.
	want := &sle.ROCFStatusReportInvocation{
		ProcessedFrameNumber: 10000,
		DeliveredOCFsNumber:  120,
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
	got, err := sle.DecodeROCFStatusReportInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeROCFStatusReportInvocation() = %v", err)
	}
	if *got != *want {
		t.Errorf("report = %+v, want %+v", *got, *want)
	}
}

func TestROCFTransferBufferRoundTrip(t *testing.T) {
	buffer := sle.ROCFTransferBuffer{
		{OCF: &sle.ROCFTransferDataInvocation{
			EarthReceiveTime: mustTime(t, testTime),
			AntennaId:        sle.AntennaId{Local: []byte("A")},
			Data:             []byte{1, 2, 3, 4},
		}},
		{Notification: &sle.SyncNotifyInvocation{Kind: sle.NotifyExcessiveDataBacklog}},
		{OCF: &sle.ROCFTransferDataInvocation{
			EarthReceiveTime: mustTime(t, testTime),
			AntennaId:        sle.AntennaId{Local: []byte("A")},
			Data:             []byte{5, 6, 7, 8},
		}},
	}

	encoded, err := buffer.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeROCFTransferBuffer(encoded)
	if err != nil {
		t.Fatalf("DecodeROCFTransferBuffer() = %v", err)
	}
	ocfs := got.OCFs()
	if len(ocfs) != 2 {
		t.Fatalf("OCFs() returned %d, want 2", len(ocfs))
	}
	if !bytes.Equal(ocfs[1].Data, []byte{5, 6, 7, 8}) {
		t.Errorf("second OCF = % X, want 05 06 07 08", ocfs[1].Data)
	}
}

func TestROCFStartReturnRoundTrip(t *testing.T) {
	want := &sle.ROCFStartReturn{
		InvokeId:           6,
		SpecificDiagnostic: sle.ROCFStartInvalidUpdateMode,
	}
	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeROCFStartReturn(encoded)
	if err != nil {
		t.Fatalf("DecodeROCFStartReturn() = %v", err)
	}
	if got.Positive {
		t.Fatal("decoded as positive")
	}
	if got.SpecificDiagnostic != sle.ROCFStartInvalidUpdateMode {
		t.Errorf("diagnostic = %v, want invalid update mode", got.SpecificDiagnostic)
	}
}

func TestROCFDecodersRejectTruncation(t *testing.T) {
	start := &sle.ROCFStartInvocation{
		InvokeId:        1,
		RequestedGVCID:  sle.GVCID{SpacecraftID: 5, VersionNumber: sle.FrameVersionTM, VirtualChannelID: 1},
		ControlWordType: sle.ControlWordType{Kind: sle.ControlWordAll},
	}
	encoded, err := start.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	for n := range len(encoded) {
		if _, err := sle.DecodeROCFStartInvocation(encoded[:n]); err == nil {
			t.Errorf("DecodeROCFStartInvocation accepted %d of %d octets", n, len(encoded))
		}
	}
}
