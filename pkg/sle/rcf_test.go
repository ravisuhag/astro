package sle_test

import (
	"bytes"
	"github.com/ravisuhag/astro/pkg/sle"
	"testing"
)

func TestGVCIDRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		channel sle.GVCID
	}{
		{"TM virtual channel", sle.GVCID{SpacecraftID: 1023, VersionNumber: sle.FrameVersionTM, VirtualChannelID: 7}},
		{"TM master channel", sle.GVCID{SpacecraftID: 42, VersionNumber: sle.FrameVersionTM, MasterChannel: true}},
		{"AOS virtual channel", sle.GVCID{SpacecraftID: 255, VersionNumber: sle.FrameVersionAOS, VirtualChannelID: 63}},
		{"USLP virtual channel", sle.GVCID{SpacecraftID: 65535, VersionNumber: sle.FrameVersionUSLP, VirtualChannelID: 63}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := sle.AppendGVCID(nil, test.channel)
			if err != nil {
				t.Fatalf("AppendGVCID() = %v", err)
			}
			element, err := sle.NewDecoder(encoded).Next()
			if err != nil {
				t.Fatalf("Next() = %v", err)
			}
			got, err := sle.DecodeGVCID(element)
			if err != nil {
				t.Fatalf("DecodeGVCID() = %v", err)
			}
			if got != test.channel {
				t.Errorf("GVCID = %+v, want %+v", got, test.channel)
			}
		})
	}
}

// TestGVCIDVersionNumbersAreTheWireValues pins the numbers to the GvcId
// SEQUENCE of CCSDS 911.2-B-4 annex A. USLP is the trap: it is called
// "version 4" but travels as the four-bit Transfer Frame Version Number
// '1100', which is 12.
func TestGVCIDVersionNumbersAreTheWireValues(t *testing.T) {
	if sle.FrameVersionTM != 0 {
		t.Errorf("TM version = %d, want 0", sle.FrameVersionTM)
	}
	if sle.FrameVersionAOS != 1 {
		t.Errorf("AOS version = %d, want 1", sle.FrameVersionAOS)
	}
	if sle.FrameVersionUSLP != 12 {
		t.Errorf("USLP version = %d, want 12", sle.FrameVersionUSLP)
	}
}

func TestGVCIDRejectsOutOfRangeSpacecraftIDs(t *testing.T) {
	tests := []struct {
		name    string
		channel sle.GVCID
	}{
		{"TM past 10 bits", sle.GVCID{SpacecraftID: 1024, VersionNumber: sle.FrameVersionTM}},
		{"AOS past 8 bits", sle.GVCID{SpacecraftID: 256, VersionNumber: sle.FrameVersionAOS}},
		{"unknown version", sle.GVCID{SpacecraftID: 1, VersionNumber: 5}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.channel.Validate(); err == nil {
				t.Error("Validate() accepted an out-of-range channel")
			}
			if _, err := sle.AppendGVCID(nil, test.channel); err == nil {
				t.Error("AppendGVCID() accepted an out-of-range channel")
			}
		})
	}
}

func TestRCFStartInvocationRoundTrip(t *testing.T) {
	want := &sle.RCFStartInvocation{
		InvokeId:  9,
		StartTime: sle.ConditionalTime{Known: true, Time: mustTime(t, testTime)},
		RequestedGVCID: sle.GVCID{
			SpacecraftID:     100,
			VersionNumber:    sle.FrameVersionAOS,
			VirtualChannelID: 3,
		},
	}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeRCFStartInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeRCFStartInvocation() = %v", err)
	}
	if got.RequestedGVCID != want.RequestedGVCID {
		t.Errorf("channel = %+v, want %+v", got.RequestedGVCID, want.RequestedGVCID)
	}
	if got.InvokeId != want.InvokeId {
		t.Errorf("InvokeId = %d, want %d", got.InvokeId, want.InvokeId)
	}
}

// TestRCFStartCarriesNoFrameQuality records the difference from RAF. RCF
// delivers only good frames, so its START has no quality field and its
// diagnostics gain 'invalid GVCID' instead.
func TestRCFStartCarriesNoFrameQuality(t *testing.T) {
	if sle.RCFStartInvalidGVCID != 5 {
		t.Errorf("invalid GVCID diagnostic = %d, want 5", sle.RCFStartInvalidGVCID)
	}

	rcf := &sle.RCFStartInvocation{InvokeId: 1, RequestedGVCID: sle.GVCID{VersionNumber: sle.FrameVersionTM}}
	raf := &sle.RAFStartInvocation{InvokeId: 1}

	rcfBytes, err := rcf.Encode()
	if err != nil {
		t.Fatalf("RCF Encode() = %v", err)
	}
	rafBytes, err := raf.Encode()
	if err != nil {
		t.Fatalf("RAF Encode() = %v", err)
	}
	if bytes.Equal(rcfBytes, rafBytes) {
		t.Error("RCF and RAF START encoded identically; one of the codecs is wrong")
	}
}

func TestRCFStartReturnRoundTrip(t *testing.T) {
	want := &sle.RCFStartReturn{InvokeId: 4, Positive: true}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeRCFStartReturn(encoded)
	if err != nil {
		t.Fatalf("DecodeRCFStartReturn() = %v", err)
	}
	if !got.Positive || got.InvokeId != 4 {
		t.Errorf("return = %+v, want positive with invoke id 4", *got)
	}
}

func TestRCFTransferDataRoundTrip(t *testing.T) {
	want := &sle.RCFTransferDataInvocation{
		EarthReceiveTime:   mustTime(t, testTime),
		AntennaId:          sle.AntennaId{Local: []byte("DSS-14")},
		DataLinkContinuity: 0,
		Data:               bytes.Repeat([]byte{0x5A}, 892),
	}

	encoded, err := want.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeRCFTransferDataInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeRCFTransferDataInvocation() = %v", err)
	}
	if !bytes.Equal(got.Data, want.Data) {
		t.Errorf("frame did not survive: %d octets back, want %d", len(got.Data), len(want.Data))
	}
}

func TestRCFStatusReportOmitsTheErrorFreeCount(t *testing.T) {
	// RAF's report has two counters, RCF's has one: RCF never delivers a bad
	// frame, so an error-free count would say nothing.
	rcf := &sle.RCFStatusReportInvocation{DeliveredFrameNumber: 500, ProductionStatus: sle.ProductionRunning}
	raf := &sle.RAFStatusReportInvocation{DeliveredFrameNumber: 500, ProductionStatus: sle.ProductionRunning}

	rcfBytes, err := rcf.Encode()
	if err != nil {
		t.Fatalf("RCF Encode() = %v", err)
	}
	rafBytes, err := raf.Encode()
	if err != nil {
		t.Fatalf("RAF Encode() = %v", err)
	}
	if len(rcfBytes) >= len(rafBytes) {
		t.Errorf("RCF report is %d octets and RAF's is %d; RCF should be shorter by one INTEGER",
			len(rcfBytes), len(rafBytes))
	}

	got, err := sle.DecodeRCFStatusReportInvocation(rcfBytes)
	if err != nil {
		t.Fatalf("DecodeRCFStatusReportInvocation() = %v", err)
	}
	if got.DeliveredFrameNumber != 500 {
		t.Errorf("DeliveredFrameNumber = %d, want 500", got.DeliveredFrameNumber)
	}
}

func TestRCFTransferBufferRoundTrip(t *testing.T) {
	buffer := sle.RCFTransferBuffer{
		{Frame: &sle.RCFTransferDataInvocation{
			EarthReceiveTime: mustTime(t, testTime),
			AntennaId:        sle.AntennaId{Local: []byte("A")},
			Data:             []byte{9, 9, 9, 9},
		}},
		{Notification: &sle.SyncNotifyInvocation{Kind: sle.NotifyEndOfData}},
	}

	encoded, err := buffer.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := sle.DecodeRCFTransferBuffer(encoded)
	if err != nil {
		t.Fatalf("DecodeRCFTransferBuffer() = %v", err)
	}
	if len(got.Frames()) != 1 {
		t.Fatalf("Frames() returned %d, want 1", len(got.Frames()))
	}
}

func TestRCFDecodersRejectTruncation(t *testing.T) {
	start := &sle.RCFStartInvocation{
		InvokeId:       2,
		RequestedGVCID: sle.GVCID{SpacecraftID: 5, VersionNumber: sle.FrameVersionTM, VirtualChannelID: 1},
	}
	encoded, err := start.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	for n := range len(encoded) {
		if _, err := sle.DecodeRCFStartInvocation(encoded[:n]); err == nil {
			t.Errorf("DecodeRCFStartInvocation accepted %d of %d octets", n, len(encoded))
		}
	}
}
