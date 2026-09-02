package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// pusMessage is the JSON shape decode emits.
type pusMessage struct {
	Direction  string `json:"direction"`
	Service    uint8  `json:"service"`
	Subtype    uint8  `json:"subtype"`
	HeaderSize int    `json:"header_octets"`
	Body       string `json:"body"`
	BodyKnown  bool   `json:"body_decoded"`
	BodyDetail string `json:"body_detail"`
	BodyError  string `json:"body_error"`
}

func decodePUSJSON(t *testing.T, out string) pusMessage {
	t.Helper()

	var message pusMessage
	if err := json.Unmarshal([]byte(out), &message); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	return message
}

func TestPUSTCRoundTrip(t *testing.T) {
	encoded, err := runCLI(t, nil, "pus", "encode",
		"--direction", "tc", "--service", "3", "--subtype", "1", "--data", "0a0b0c")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "pus", "decode",
		"--direction", "tc", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	message := decodePUSJSON(t, out)
	if message.Service != 3 || message.Subtype != 1 {
		t.Errorf("got ST[%d,%d], want ST[3,1]", message.Service, message.Subtype)
	}
	if message.Body != "0a0b0c" {
		t.Errorf("body = %q, want 0a0b0c", message.Body)
	}
	if message.Direction != "tc" {
		t.Errorf("direction = %q, want tc", message.Direction)
	}
}

// A TM report is time tagged, so the tag has to survive the round trip.
func TestPUSTMRoundTripKeepsTheTimeTag(t *testing.T) {
	encoded, err := runCLI(t, nil, "pus", "encode",
		"--direction", "tm", "--service", "1", "--subtype", "1",
		"--time-tag", "2026-09-01T12:00:00Z", "--data", "0064c000")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "pus", "decode",
		"--direction", "tm", "--input", "hex")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !strings.Contains(out, "2026-09-01T12:00:00Z") {
		t.Errorf("the time tag did not survive the round trip:\n%s", out)
	}
}

// The point of going through the registry: a body whose service is
// implemented is decoded, not just shown as octets.
func TestPUSDecodesAKnownBody(t *testing.T) {
	encoded, err := runCLI(t, nil, "pus", "encode",
		"--direction", "tm", "--service", "1", "--subtype", "1",
		"--time-tag", "2026-09-01T12:00:00Z", "--data", "0064c000")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "pus", "decode",
		"--direction", "tm", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	message := decodePUSJSON(t, out)
	if !message.BodyKnown {
		t.Errorf("ST[1,1] body was not decoded: %s", message.BodyError)
	}
	if !strings.Contains(message.BodyDetail, "verification report") {
		t.Errorf("body detail = %q, want it to name the report", message.BodyDetail)
	}
	// The APID the verification report refers to came from the body octets.
	if !strings.Contains(message.BodyDetail, "100") {
		t.Errorf("body detail = %q, want the request APID 100", message.BodyDetail)
	}
}

// A body whose service is not implemented is reported as raw octets with the
// reason, not guessed at. Silently showing octets as if understood is the
// failure worth avoiding.
func TestPUSUnknownBodyIsReportedNotGuessed(t *testing.T) {
	encoded, err := runCLI(t, nil, "pus", "encode",
		"--direction", "tc", "--service", "200", "--subtype", "9", "--data", "deadbeef")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "pus", "decode",
		"--direction", "tc", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	message := decodePUSJSON(t, out)
	if message.BodyKnown {
		t.Error("an unimplemented service claimed its body was decoded")
	}
	if message.BodyError == "" {
		t.Error("no reason given for the undecoded body")
	}
	if message.Body != "deadbeef" {
		t.Errorf("body = %q, want the raw octets deadbeef", message.Body)
	}
}

// The text output says outright that a body was not decoded, so nobody reads
// the octets as an interpretation.
func TestPUSUnknownBodySaysSoInText(t *testing.T) {
	encoded, err := runCLI(t, nil, "pus", "encode",
		"--direction", "tc", "--service", "200", "--subtype", "9", "--data", "deadbeef")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "pus", "decode",
		"--direction", "tc", "--input", "hex")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !strings.Contains(out, "not decoded") {
		t.Errorf("text output does not flag the undecoded body:\n%s", out)
	}
}

// The mission-tailorable widths change the header size, so a profile
// mismatch has to be expressible on the command line.
func TestPUSProfileChangesHeaderSize(t *testing.T) {
	plain, err := runCLI(t, nil, "pus", "encode",
		"--direction", "tc", "--service", "3", "--subtype", "1", "--data", "00")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	spared, err := runCLI(t, nil, "pus", "encode",
		"--direction", "tc", "--service", "3", "--subtype", "1", "--data", "00",
		"--tc-spare", "2")
	if err != nil {
		t.Fatalf("encode with spare failed: %v", err)
	}

	// Two octets of spare is four more hex characters.
	if len(strings.TrimSpace(spared)) != len(strings.TrimSpace(plain))+4 {
		t.Errorf("--tc-spare 2 did not add two octets: %q vs %q",
			strings.TrimSpace(spared), strings.TrimSpace(plain))
	}
}

// A TM header with no time field is shorter, and both ends have to agree.
func TestPUSTimeNone(t *testing.T) {
	encoded, err := runCLI(t, nil, "pus", "encode",
		"--direction", "tm", "--service", "1", "--subtype", "1",
		"--time", "none", "--data", "0064c000")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "pus", "decode",
		"--direction", "tm", "--time", "none", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	message := decodePUSJSON(t, out)
	if message.Body != "0064c000" {
		t.Errorf("body = %q, want 0064c000", message.Body)
	}
}

// Decoding with the wrong time format reads the body from the wrong offset,
// which is exactly why the flags exist.
func TestPUSTimeFormatMismatchChangesTheBody(t *testing.T) {
	encoded, err := runCLI(t, nil, "pus", "encode",
		"--direction", "tm", "--service", "1", "--subtype", "1",
		"--time", "none", "--data", "0064c000")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "pus", "decode",
		"--direction", "tm", "--input", "hex", "--format", "json")
	if err != nil {
		// Failing outright is a fine outcome too.
		return
	}

	message := decodePUSJSON(t, out)
	if message.Body == "0064c000" {
		t.Error("decoding with a time field the sender omitted still found the right body")
	}
}

func TestPUSServices(t *testing.T) {
	out, err := runCLI(t, nil, "pus", "services")
	if err != nil {
		t.Fatalf("services failed: %v", err)
	}
	// ST[1,1] is a report and ST[17,1] a request, so both lists are populated.
	if !strings.Contains(out, "ST[1,1]") {
		t.Errorf("services did not list ST[1,1]:\n%s", out)
	}
	if !strings.Contains(out, "ST[17,1]") {
		t.Errorf("services did not list ST[17,1]:\n%s", out)
	}
	if !strings.Contains(out, "Requests") || !strings.Contains(out, "Reports") {
		t.Errorf("services did not separate requests from reports:\n%s", out)
	}
}

func TestPUSServicesJSON(t *testing.T) {
	out, err := runCLI(t, nil, "pus", "services", "--format", "json")
	if err != nil {
		t.Fatalf("services failed: %v", err)
	}

	var listing map[string][]string
	if err := json.Unmarshal([]byte(out), &listing); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(listing["reports"]) == 0 {
		t.Error("no reports listed")
	}
	if len(listing["requests"]) == 0 {
		t.Error("no requests listed")
	}
}

func TestPUSRejectsBadFlags(t *testing.T) {
	for name, args := range map[string][]string{
		"unknown direction": {"pus", "decode", "--direction", "sideways", "--input", "hex"},
		"unknown time":      {"pus", "decode", "--time", "sundial", "--input", "hex"},
		"unknown format":    {"pus", "decode", "--format", "yaml", "--input", "hex"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := runCLI(t, []byte("2001010000"), args...); err == nil {
				t.Errorf("%s was accepted", name)
			}
		})
	}
}

// Too few octets for the header is reported rather than read past the end.
func TestPUSDecodeShortInput(t *testing.T) {
	if _, err := runCLI(t, []byte("20"), "pus", "decode",
		"--direction", "tc", "--input", "hex"); err == nil {
		t.Error("decode accepted an input too short for the header")
	}
}
