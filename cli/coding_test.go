package cli

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

// --- COP-1 CLCW ---

// The CLCW is the part of COP-1 that goes on the wire, so a round trip
// through it is the thing worth pinning.
func TestCLCWRoundTrip(t *testing.T) {
	encoded, err := runCLI(t, nil, "cop", "clcw-encode",
		"--vcid", "3", "--report-value", "7", "--wait")
	if err != nil {
		t.Fatalf("clcw-encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "cop", "clcw-decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("clcw-decode failed: %v", err)
	}

	var clcw struct {
		VirtualChannelID uint8 `json:"virtual_channel_id"`
		ReportValue      uint8 `json:"report_value"`
		WaitFlag         bool  `json:"wait"`
		RetransmitFlag   bool  `json:"retransmit"`
		COPInEffect      uint8 `json:"cop_in_effect"`
	}
	if err := json.Unmarshal([]byte(out), &clcw); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}

	if clcw.VirtualChannelID != 3 {
		t.Errorf("vcid = %d, want 3", clcw.VirtualChannelID)
	}
	if clcw.ReportValue != 7 {
		t.Errorf("report value = %d, want 7", clcw.ReportValue)
	}
	if !clcw.WaitFlag {
		t.Error("the wait flag did not survive the round trip")
	}
	if clcw.RetransmitFlag {
		t.Error("the retransmit flag was set when it was not asked for")
	}
	// COP In Effect is 01 for COP-1 and the encoder sets it, because that is
	// the only procedure this package implements.
	if clcw.COPInEffect != 1 {
		t.Errorf("cop_in_effect = %d, want 1 for COP-1", clcw.COPInEffect)
	}
}

// A CLCW is exactly four octets, so it fits a TM frame's Operational Control
// Field. That is the whole reason the command exists.
func TestCLCWIsFourOctets(t *testing.T) {
	encoded, err := runCLI(t, nil, "cop", "clcw-encode", "--vcid", "0", "--report-value", "1")
	if err != nil {
		t.Fatalf("clcw-encode failed: %v", err)
	}

	octets, err := hex.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		t.Fatalf("decoding %q: %v", encoded, err)
	}
	if len(octets) != 4 {
		t.Errorf("a CLCW came out %d octets, want 4", len(octets))
	}
}

// And it goes straight into a frame's OCF, which is the pipeline the help
// promises.
func TestCLCWFitsATMFrameOCF(t *testing.T) {
	clcw, err := runCLI(t, nil, "cop", "clcw-encode", "--vcid", "0", "--report-value", "5")
	if err != nil {
		t.Fatalf("clcw-encode failed: %v", err)
	}

	frame, err := runCLI(t, nil, "tm", "encode",
		"--scid", "42", "--vcid", "0", "--data", "0102", "--ocf", strings.TrimSpace(clcw))
	if err != nil {
		t.Fatalf("tm encode with the CLCW as OCF failed: %v", err)
	}
	if strings.TrimSpace(frame) == "" {
		t.Error("no frame came back")
	}
}

func TestCLCWDecodeRejectsWrongLength(t *testing.T) {
	if _, err := runCLI(t, []byte("0102"), "cop", "clcw-decode", "--input", "hex"); err == nil {
		t.Error("clcw-decode accepted two octets")
	}
}

// --- Proximity-1 data link ---

func TestPXDLRoundTrip(t *testing.T) {
	encoded, err := runCLI(t, nil, "pxdl", "encode",
		"--scid", "42", "--port", "1", "--data", "0102030405")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "pxdl", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var frame struct {
		SpacecraftID uint16 `json:"spacecraft_id"`
		PortID       uint8  `json:"port_id"`
		Data         string `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &frame); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}

	if frame.SpacecraftID != 42 {
		t.Errorf("scid = %d, want 42", frame.SpacecraftID)
	}
	if frame.PortID != 1 {
		t.Errorf("port = %d, want 1", frame.PortID)
	}
	if frame.Data != "0102030405" {
		t.Errorf("data = %q, want 0102030405", frame.Data)
	}
}

func TestPXDLDecodeRejectsRubbish(t *testing.T) {
	if _, err := runCLI(t, []byte("00"), "pxdl", "decode", "--input", "hex"); err == nil {
		t.Error("decode accepted one octet as a frame")
	}
}

// --- Proximity-1 coding ---

func TestPXSCPLTURoundTrip(t *testing.T) {
	frame, err := runCLI(t, nil, "pxdl", "encode",
		"--scid", "42", "--port", "1", "--data", "0102030405")
	if err != nil {
		t.Fatalf("pxdl encode failed: %v", err)
	}
	original := strings.TrimSpace(frame)

	pltu, err := runCLI(t, []byte(original), "pxsc", "wrap", "--input", "hex")
	if err != nil {
		t.Fatalf("wrap failed: %v", err)
	}

	recovered, err := runCLI(t, []byte(strings.TrimSpace(pltu)), "pxsc", "unwrap", "--input", "hex")
	if err != nil {
		t.Fatalf("unwrap failed: %v", err)
	}
	if got := strings.TrimSpace(recovered); got != original {
		t.Errorf("PLTU round trip changed the frame:\n got %s\nwant %s", got, original)
	}
}

// A PLTU whose CRC does not match is corrupt, and passing the frame on would
// put bad data into the layer above.
func TestPXSCUnwrapRejectsBadCRC(t *testing.T) {
	pltu, err := runCLI(t, []byte("802a1809000102030405"), "pxsc", "wrap", "--input", "hex")
	if err != nil {
		t.Fatalf("wrap failed: %v", err)
	}

	corrupted := []byte(strings.TrimSpace(pltu))
	if corrupted[len(corrupted)-1] == '0' {
		corrupted[len(corrupted)-1] = '1'
	} else {
		corrupted[len(corrupted)-1] = '0'
	}

	if _, err := runCLI(t, corrupted, "pxsc", "unwrap", "--input", "hex"); err == nil {
		t.Error("unwrap accepted a PLTU whose CRC does not match")
	}
}

// The convolutional round trip has to come out exact. A Viterbi decoder holds
// back the last 35 bits of a stream, so encode appends a tail; without it the
// round trip would silently lose its last few octets.
func TestPXSCConvolutionalRoundTripIsExact(t *testing.T) {
	original := "faf320802a180900010203040566d00a35"

	symbols, err := runCLI(t, []byte(original), "pxsc", "encode", "--input", "hex")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := runCLI(t, []byte(strings.TrimSpace(symbols)), "pxsc", "decode", "--input", "hex")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got := strings.TrimSpace(decoded); got != original {
		t.Errorf("the coded round trip changed the data:\n got %s\nwant %s", got, original)
	}
}

// Without the tail the tail is lost, which is what --flush=false documents
// and what the default exists to avoid.
func TestPXSCWithoutFlushLosesTheTail(t *testing.T) {
	original := "faf320802a180900010203040566d00a35"

	symbols, err := runCLI(t, []byte(original), "pxsc", "encode",
		"--input", "hex", "--flush=false")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := runCLI(t, []byte(strings.TrimSpace(symbols)), "pxsc", "decode", "--input", "hex")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got := strings.TrimSpace(decoded); got == original {
		t.Error("decoding an unflushed stream returned everything, so the tail is not being held back")
	}
}

// The code corrects errors, which is the point of having it.
func TestPXSCViterbiCorrectsAFlippedBit(t *testing.T) {
	original := "802a1809000102030405"

	symbols, err := runCLI(t, []byte(original), "pxsc", "encode", "--input", "hex")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	octets, err := hex.DecodeString(strings.TrimSpace(symbols))
	if err != nil {
		t.Fatalf("decoding the symbols: %v", err)
	}
	// Flip one bit in the middle of the symbol stream.
	octets[len(octets)/2] ^= 0x01

	decoded, err := runCLI(t, []byte(hex.EncodeToString(octets)), "pxsc", "decode", "--input", "hex")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got := strings.TrimSpace(decoded); got != original {
		t.Errorf("the decoder did not correct a single flipped bit:\n got %s\nwant %s", got, original)
	}
}

func TestPXSCSyncFindsWrappedFrames(t *testing.T) {
	pltu, err := runCLI(t, []byte("802a1809000102030405"), "pxsc", "wrap", "--input", "hex")
	if err != nil {
		t.Fatalf("wrap failed: %v", err)
	}
	stream := "dead" + strings.TrimSpace(pltu) + strings.TrimSpace(pltu)

	out, err := runCLI(t, []byte(stream), "pxsc", "sync", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	var frames []string
	if err := json.Unmarshal([]byte(out), &frames); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(frames) != 2 {
		t.Fatalf("found %d frames, want 2 after the leading junk", len(frames))
	}
	for i, frame := range frames {
		if frame != "802a1809000102030405" {
			t.Errorf("frame %d = %s, want the original", i, frame)
		}
	}
}

// --- Optical coding ---

func TestOCSCCondition(t *testing.T) {
	// Four 256-octet frames.
	var frames []byte
	for i := 0; i < 1024; i++ {
		frames = append(frames, byte((i*5)%256))
	}

	out, err := runCLI(t, []byte(hex.EncodeToString(frames)), "ocsc", "condition",
		"--input", "hex", "--frame-len", "256", "--rate", "1/2", "--format", "json")
	if err != nil {
		t.Fatalf("condition failed: %v", err)
	}

	var blocks []struct {
		Bits   int    `json:"bits"`
		Octets int    `json:"octets"`
		Data   string `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &blocks); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(blocks) == 0 {
		t.Fatal("no codeblocks came back")
	}
	// A codeblock is a bit string whose length is not a whole number of
	// octets, which is why the bit count is reported separately.
	for i, block := range blocks {
		if block.Bits == 0 {
			t.Errorf("codeblock %d reports zero bits", i)
		}
		if want := (block.Bits + 7) / 8; block.Octets != want {
			t.Errorf("codeblock %d is %d bits but %d octets, want %d",
				i, block.Bits, block.Octets, want)
		}
	}
}

func TestOCSCConditionRejectsPartialFrame(t *testing.T) {
	if _, err := runCLI(t, []byte("0102030405"), "ocsc", "condition",
		"--input", "hex", "--frame-len", "256", "--rate", "1/2"); err == nil {
		t.Error("condition accepted five octets as 256-octet frames")
	}
}

func TestOCSCConditionRejectsUnknownRate(t *testing.T) {
	if _, err := runCLI(t, []byte("01"), "ocsc", "condition",
		"--input", "hex", "--frame-len", "1", "--rate", "7/8"); err == nil {
		t.Error("condition accepted a rate the standard does not define")
	}
}

// The randomiser is its own inverse, so applying it twice returns the input.
func TestOCSCRandomizeIsItsOwnInverse(t *testing.T) {
	original := "0123456789abcdef"

	once, err := runCLI(t, []byte(original), "ocsc", "randomize",
		"--input", "hex", "--bits", "64")
	if err != nil {
		t.Fatalf("randomize failed: %v", err)
	}
	if strings.TrimSpace(once) == original {
		t.Error("randomize returned its input unchanged")
	}

	twice, err := runCLI(t, []byte(strings.TrimSpace(once)), "ocsc", "randomize",
		"--input", "hex", "--bits", "64")
	if err != nil {
		t.Fatalf("randomize failed: %v", err)
	}
	if got := strings.TrimSpace(twice); got != original {
		t.Errorf("randomising twice gave %s, want the original %s", got, original)
	}
}

// A bit length longer than the input has octets for is a mismatch worth
// reporting rather than reading past the end of the buffer.
func TestOCSCRandomizeRejectsTooFewOctets(t *testing.T) {
	if _, err := runCLI(t, []byte("0102"), "ocsc", "randomize",
		"--input", "hex", "--bits", "64"); err == nil {
		t.Error("randomize accepted --bits 64 with two octets")
	}
}

// --- SDLS ---

func TestSDLSInspect(t *testing.T) {
	// SPI 1, a 12-octet IV, 8 octets of protected data, a 16-octet MAC.
	frame := "0001" + "aabbccddeeff001122334455" +
		"0011223344556677" + "00112233445566778899aabbccddeeff"

	out, err := runCLI(t, []byte(frame), "sdls", "inspect",
		"--input", "hex", "--iv", "12", "--mac", "16", "--format", "json")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	var header struct {
		SPI           uint16 `json:"spi"`
		IV            string `json:"iv"`
		HeaderOctets  int    `json:"header_octets"`
		PayloadOctets int    `json:"payload_octets"`
		MAC           string `json:"mac"`
	}
	if err := json.Unmarshal([]byte(out), &header); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}

	if header.SPI != 1 {
		t.Errorf("spi = %d, want 1", header.SPI)
	}
	if header.IV != "aabbccddeeff001122334455" {
		t.Errorf("iv = %q, want the 12 octets given", header.IV)
	}
	// Two octets of SPI plus twelve of IV.
	if header.HeaderOctets != 14 {
		t.Errorf("header = %d octets, want 14", header.HeaderOctets)
	}
	if header.PayloadOctets != 8 {
		t.Errorf("payload = %d octets, want 8", header.PayloadOctets)
	}
	if header.MAC != "00112233445566778899aabbccddeeff" {
		t.Errorf("mac = %q, want the 16 octets at the end", header.MAC)
	}
}

// The field widths are per Security Association and nothing in the header
// states them, so a wrong width shifts everything after the SPI. The SPI
// itself keeps its place, which is why it is reported separately.
func TestSDLSInspectWrongFieldWidths(t *testing.T) {
	frame := "0001" + "aabbccddeeff001122334455" + "0011223344556677"

	right, err := runCLI(t, []byte(frame), "sdls", "inspect",
		"--input", "hex", "--iv", "12", "--format", "json")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	wrong, err := runCLI(t, []byte(frame), "sdls", "inspect",
		"--input", "hex", "--iv", "8", "--format", "json")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	if right == wrong {
		t.Error("two different IV widths read the header identically")
	}

	// The SPI is at a fixed position, so it survives either way.
	for _, out := range []string{right, wrong} {
		var header struct {
			SPI uint16 `json:"spi"`
		}
		if err := json.Unmarshal([]byte(out), &header); err != nil {
			t.Fatalf("unmarshal %q: %v", out, err)
		}
		if header.SPI != 1 {
			t.Errorf("spi = %d, want 1 regardless of the field widths", header.SPI)
		}
	}
}

// A frame too short for the stated field widths is reported rather than read
// past the end of.
func TestSDLSInspectShortInput(t *testing.T) {
	if _, err := runCLI(t, []byte("0001aabb"), "sdls", "inspect",
		"--input", "hex", "--iv", "12"); err == nil {
		t.Error("inspect accepted a frame too short for a 12-octet IV")
	}
}

// Every command in this batch takes --format, so an unknown one has to fail
// the same way throughout.
func TestCodingCommandsRejectUnknownFormat(t *testing.T) {
	for name, args := range map[string][]string{
		"cop":  {"cop", "clcw-decode", "--input", "hex", "--format", "yaml"},
		"pxdl": {"pxdl", "decode", "--input", "hex", "--format", "yaml"},
		"pxsc": {"pxsc", "unwrap", "--input", "hex", "--format", "yaml"},
		"ocsc": {"ocsc", "randomize", "--input", "hex", "--bits", "8", "--format", "yaml"},
		"sdls": {"sdls", "inspect", "--input", "hex", "--format", "yaml"},
	} {
		t.Run(name, func(t *testing.T) {
			input := []byte("0001aabbccddeeff001122334455667788990011")
			if _, err := runCLI(t, input, args...); err == nil {
				t.Errorf("%s accepted an unknown format", name)
			}
		})
	}
}
