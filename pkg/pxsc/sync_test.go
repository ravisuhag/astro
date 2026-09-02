package pxsc_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/pkg/pxdl"
	"github.com/ravisuhag/astro/pkg/pxsc"
)

// buildFrame returns one encoded Version-3 Transfer Frame.
func buildFrame(t *testing.T, scid uint16, payload string) []byte {
	t.Helper()
	f, err := pxdl.NewTransferFrame(scid, 0, []byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestSynchronizerFindsPLTUsInAStream(t *testing.T) {
	// A realistic stream: idle, PLTU, idle, PLTU, idle.
	first := buildFrame(t, 10, "first frame")
	second := buildFrame(t, 20, "second frame payload")

	firstPLTU, err := pxsc.WrapPLTU(first)
	if err != nil {
		t.Fatal(err)
	}
	secondPLTU, err := pxsc.WrapPLTU(second)
	if err != nil {
		t.Fatal(err)
	}

	var stream []byte
	stream = append(stream, pxsc.IdleData(20)...)
	stream = append(stream, firstPLTU...)
	stream = append(stream, pxsc.IdleData(13)...)
	stream = append(stream, secondPLTU...)
	stream = append(stream, pxsc.IdleData(7)...)

	s := pxsc.NewSynchronizer()
	units := s.Scan(stream)

	if len(units) != 2 {
		t.Fatalf("found %d PLTUs, want 2", len(units))
	}
	if !bytes.Equal(units[0].Frame, first) {
		t.Error("the first frame does not match")
	}
	if !bytes.Equal(units[1].Frame, second) {
		t.Error("the second frame does not match")
	}
	if units[0].Offset != 20 {
		t.Errorf("the first PLTU is at offset %d, want 20", units[0].Offset)
	}
}

func TestSynchronizerSkipsCorruptPLTU(t *testing.T) {
	// A PLTU whose CRC fails must not be delivered (clause 3.6), but a good one
	// after it must still be found.
	good := buildFrame(t, 30, "intact frame")
	bad := buildFrame(t, 40, "corrupt frame")

	badPLTU, err := pxsc.WrapPLTU(bad)
	if err != nil {
		t.Fatal(err)
	}
	badPLTU[pxsc.ASMSize+3] ^= 0xFF // break the frame, leave the marker

	goodPLTU, err := pxsc.WrapPLTU(good)
	if err != nil {
		t.Fatal(err)
	}

	stream := append(append([]byte{}, badPLTU...), goodPLTU...)

	s := pxsc.NewSynchronizer()
	units := s.Scan(stream)

	for _, u := range units {
		if bytes.Equal(u.Frame, bad) {
			t.Error("a corrupt PLTU was delivered")
		}
	}
	found := false
	for _, u := range units {
		if bytes.Equal(u.Frame, good) {
			found = true
		}
	}
	if !found {
		t.Error("the intact PLTU after a corrupt one was not found")
	}
}

func TestSynchronizerHandlesMarkerInsideFrameData(t *testing.T) {
	// The sync marker is only 24 bits, so it can appear inside frame data by
	// chance. The CRC is what tells a real PLTU from a coincidence.
	payload := append([]byte("before"), pxsc.DefaultASM()...)
	payload = append(payload, []byte("after")...)

	f, err := pxdl.NewTransferFrame(50, 0, payload)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	pltu, err := pxsc.WrapPLTU(frame)
	if err != nil {
		t.Fatal(err)
	}

	s := pxsc.NewSynchronizer()
	units := s.Scan(pltu)

	if len(units) == 0 {
		t.Fatal("no PLTU found")
	}
	if !bytes.Equal(units[0].Frame, frame) {
		t.Error("the frame containing a marker pattern was not recovered intact")
	}
}

func TestSynchronizerEmptyAndJunkInput(t *testing.T) {
	s := pxsc.NewSynchronizer()

	if units := s.Scan(nil); len(units) != 0 {
		t.Errorf("found %d PLTUs in nothing", len(units))
	}
	if units := s.Scan(pxsc.IdleData(100)); len(units) != 0 {
		t.Errorf("found %d PLTUs in pure idle data", len(units))
	}
	if units := s.Scan(bytes.Repeat([]byte{0xFF}, 100)); len(units) != 0 {
		t.Errorf("found %d PLTUs in junk", len(units))
	}
}

func TestSynchronizerTruncatedTrailingPLTU(t *testing.T) {
	frame := buildFrame(t, 60, "a frame that gets cut off")
	pltu, err := pxsc.WrapPLTU(frame)
	if err != nil {
		t.Fatal(err)
	}

	s := pxsc.NewSynchronizer()
	units := s.Scan(pltu[:len(pltu)-3])

	for _, u := range units {
		if bytes.Equal(u.Frame, frame) {
			t.Error("a truncated PLTU was delivered as complete")
		}
	}
}

func TestScanFrames(t *testing.T) {
	first := buildFrame(t, 1, "one")
	second := buildFrame(t, 2, "two")

	firstPLTU, _ := pxsc.WrapPLTU(first)
	secondPLTU, _ := pxsc.WrapPLTU(second)
	stream := append(append([]byte{}, firstPLTU...), secondPLTU...)

	frames := pxsc.NewSynchronizer().ScanFrames(stream)
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(frames))
	}
}

func TestFindASM(t *testing.T) {
	stream := append(pxsc.IdleData(10), pxsc.DefaultASM()...)

	if got := pxsc.FindASM(stream, 0); got != 10 {
		t.Errorf("marker at %d, want 10", got)
	}
	if got := pxsc.FindASM(stream, 11); got != -1 {
		t.Errorf("found a marker at %d after the only one", got)
	}
	if got := pxsc.FindASM(nil, 0); got != -1 {
		t.Errorf("found a marker in nothing at %d", got)
	}
}

func TestPLTUThroughTheWholeStack(t *testing.T) {
	// A frame from pkg/pxdl, wrapped here, recovered, and decoded back.
	original, err := pxdl.NewTransferFrame(0x2AB, 5, []byte("end to end"),
		pxdl.WithQoS(pxdl.Expedited), pxdl.WithSequenceNumber(9))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := original.Encode()
	if err != nil {
		t.Fatal(err)
	}

	pltu, err := pxsc.WrapPLTU(encoded)
	if err != nil {
		t.Fatal(err)
	}

	recovered, err := pxsc.UnwrapPLTU(pltu)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := pxdl.DecodeTransferFrame(recovered)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.SCID != 0x2AB {
		t.Errorf("SCID = %#x, want 0x2AB", decoded.Header.SCID)
	}
	if decoded.Header.FrameSequenceNumber != 9 {
		t.Errorf("sequence = %d, want 9", decoded.Header.FrameSequenceNumber)
	}
	if string(decoded.DataField) != "end to end" {
		t.Errorf("payload = %q", decoded.DataField)
	}
}

func TestConvolutionalEncodeDoublesLength(t *testing.T) {
	// Rate 1/2: two output symbols per input bit.
	data := []byte("convolutional")
	encoded := pxsc.ConvolutionalEncode(data)

	if len(encoded) != len(data)*2 {
		t.Errorf("encoded %d octets from %d, want %d", len(encoded), len(data), len(data)*2)
	}
}

func TestConvolutionalKnownAnswers(t *testing.T) {
	// Independent vectors, computed with the libfec / gr-satellites
	// realization of the CCSDS 171/133 code: newest bit shifted into the
	// register LSB, taps 0x4F and 0x6D, G2 output inverted. They pin the code
	// to the convention every deployed CCSDS receiver uses.
	//
	// A reciprocal (mirror-image) encoder (the unreversed vectors on this
	// shift direction) decodes its own output but no one else's. It emits
	// 86B9 for the 0x80 input, which is how that bug is caught here.
	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		{"single one, LSB", []byte{0x01}, []byte{0x55, 0x56}},
		{"single one, MSB", []byte{0x80}, []byte{0xBA, 0x49}},
		{"two octets", []byte{0xA5, 0x3C}, []byte{0xB4, 0x80, 0xE3, 0xBC}},
		{"CCSDS", []byte("CCSDS"), []byte{0x6E, 0x9F, 0x23, 0x2F, 0x20, 0x93, 0x53, 0x19, 0xAA, 0x23}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := pxsc.ConvolutionalEncode(test.in)
			if !bytes.Equal(got, test.want) {
				t.Errorf("ConvolutionalEncode(%X) = %X, want %X", test.in, got, test.want)
			}
		})
	}
}

func TestConvolutionalEncoderIsDeterministic(t *testing.T) {
	data := []byte{0x00, 0xFF, 0xA5, 0x5A}

	first := pxsc.ConvolutionalEncode(data)
	second := pxsc.ConvolutionalEncode(data)

	if !bytes.Equal(first, second) {
		t.Error("two fresh encoders produced different output for the same input")
	}
}

func TestConvolutionalEncoderCarriesStateAcrossCalls(t *testing.T) {
	// Clause 3.4.3.2 encodes everything as one continuous stream, so the register
	// must not reset between calls.
	data := []byte{0xA5, 0x5A, 0x3C, 0xC3}

	whole := pxsc.ConvolutionalEncode(data)

	e := pxsc.NewConvolutionalEncoder()
	piecewise := append(e.Encode(data[:2]), e.Encode(data[2:])...)

	if !bytes.Equal(whole, piecewise) {
		t.Error("encoding in two calls differs from encoding in one; the register reset")
	}
}

func TestConvolutionalG2IsInverted(t *testing.T) {
	// Clause 3.4.3.1 note 1: the G2 output path is inverted. With a cleared
	// register and a zero input bit, G1 gives 0 and G2 gives 1.
	e := pxsc.NewConvolutionalEncoder()
	c1, c2 := e.EncodeBit(0)

	if c1 != 0 {
		t.Errorf("G1 output = %d, want 0 for a zero input from a clear register", c1)
	}
	if c2 != 1 {
		t.Errorf("G2 output = %d, want 1; the path should be inverted", c2)
	}
}

func TestConvolutionalReset(t *testing.T) {
	e := pxsc.NewConvolutionalEncoder()
	first := e.Encode([]byte{0xFF})

	e.Reset()
	second := e.Encode([]byte{0xFF})

	if !bytes.Equal(first, second) {
		t.Error("Reset did not restore the initial register state")
	}
}

func TestConvolutionalEmptyInput(t *testing.T) {
	if got := pxsc.ConvolutionalEncode(nil); got != nil {
		t.Errorf("encoding nothing gave %d octets, want nil", len(got))
	}
}
