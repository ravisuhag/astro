package cop_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/cop"
	"github.com/ravisuhag/astro/pkg/tcdl"
)

// TestSetVR_RoundTrip_TCDLAndCOP drives a Set V(R) directive through the
// real frame codec: the ground FOP transmits a tcdl-built BC frame, the
// spacecraft decodes it and hands it to the FARM, and the returned CLCW
// completes the FOP initialisation.
func TestSetVR_RoundTrip_TCDLAndCOP(t *testing.T) {
	const scid, vcid, vr = 42, 1, 77

	// Ground side: build the BC Set V(R) frame and initiate AD with it.
	bcFrame, err := tcdl.NewSetVRFrame(scid, vcid, vr)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := bcFrame.Encode()
	if err != nil {
		t.Fatal(err)
	}

	fop := cop.NewFOP(scid, vcid, 10)
	if err := fop.InitiateADWithSetVR(vr, encoded); err != nil {
		t.Fatal(err)
	}

	// The FOP serves the BC frame for uplink.
	wire, _, ok := fop.GetNextFrame()
	if !ok {
		t.Fatal("BC frame not served")
	}

	// Spacecraft side: decode the frame and run it through the FARM.
	farm := cop.NewFARM(vcid, 10)
	decoded, err := tcdl.DecodeTransferFrame(wire)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.BypassFlag != 1 || decoded.Header.ControlCommandFlag != 1 {
		t.Fatalf("BC frame flags = (%d,%d), want (1,1)",
			decoded.Header.BypassFlag, decoded.Header.ControlCommandFlag)
	}
	accepted, err := farm.ProcessFrame(
		decoded.Header.BypassFlag,
		decoded.Header.ControlCommandFlag,
		decoded.Header.FrameSequenceNum,
		decoded.DataField,
	)
	if err != nil || !accepted {
		t.Fatalf("FARM rejected the Set V(R) frame: accepted=%v err=%v", accepted, err)
	}
	if farm.VR() != vr {
		t.Fatalf("FARM V(R) = %d, want %d", farm.VR(), vr)
	}

	// Return link: the CLCW confirms the directive and the FOP goes Active.
	if err := fop.ProcessCLCW(farm.GenerateCLCW()); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPActive {
		t.Fatalf("FOP state = %d, want FOPActive", fop.State())
	}
	if fop.VS() != vr {
		t.Fatalf("FOP V(S) = %d, want %d", fop.VS(), vr)
	}

	// And the very next AD frame flows in sequence end to end.
	dataFrame, err := tcdl.NewTransferFrame(scid, vcid, []byte("payload"),
		tcdl.WithSequenceNumber(vr))
	if err != nil {
		t.Fatal(err)
	}
	encodedData, err := dataFrame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := fop.TransmitFrame(encodedData); err != nil {
		t.Fatal(err)
	}
	wire, ns, ok := fop.GetNextFrame()
	if !ok || ns != vr {
		t.Fatalf("AD frame N(S) = %d (ok=%v), want %d", ns, ok, vr)
	}
	decoded, err = tcdl.DecodeTransferFrame(wire)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err = farm.ProcessFrame(0, 0, decoded.Header.FrameSequenceNum, decoded.DataField)
	if err != nil || !accepted {
		t.Fatalf("in-sequence AD frame rejected: accepted=%v err=%v", accepted, err)
	}
	if err := fop.ProcessCLCW(farm.GenerateCLCW()); err != nil {
		t.Fatal(err)
	}
	if fop.PendingCount() != 0 {
		t.Errorf("pending = %d, want 0 after acknowledgment", fop.PendingCount())
	}
}
