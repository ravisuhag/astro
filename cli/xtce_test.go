package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A small database with a base container and two derived ones, so match has
// something to choose between.
const testXTCE = `<?xml version="1.0" encoding="UTF-8"?>
<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Sat">
  <TelemetryMetaData>
    <ParameterTypeSet>
      <IntegerParameterType name="U8" signed="false">
        <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
      </IntegerParameterType>
      <EnumeratedParameterType name="Mode">
        <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
        <EnumerationList>
          <Enumeration value="0" label="SAFE"/>
          <Enumeration value="1" label="NOMINAL"/>
        </EnumerationList>
      </EnumeratedParameterType>
      <FloatParameterType name="Volts" sizeInBits="64">
        <IntegerDataEncoding sizeInBits="8" encoding="unsigned">
          <DefaultCalibrator>
            <PolynomialCalibrator>
              <Term coefficient="0.1" exponent="1"/>
            </PolynomialCalibrator>
          </DefaultCalibrator>
        </IntegerDataEncoding>
      </FloatParameterType>
    </ParameterTypeSet>
    <ParameterSet>
      <Parameter name="Type" parameterTypeRef="U8"/>
      <Parameter name="Mode" parameterTypeRef="Mode"/>
      <Parameter name="Battery" parameterTypeRef="Volts"/>
    </ParameterSet>
    <ContainerSet>
      <SequenceContainer name="Packet" abstract="true">
        <EntryList><ParameterRefEntry parameterRef="Type"/></EntryList>
      </SequenceContainer>
      <SequenceContainer name="Housekeeping">
        <EntryList>
          <ParameterRefEntry parameterRef="Mode"/>
          <ParameterRefEntry parameterRef="Battery"/>
        </EntryList>
        <BaseContainer containerRef="Packet">
          <RestrictionCriteria>
            <Comparison parameterRef="Type" value="1"/>
          </RestrictionCriteria>
        </BaseContainer>
      </SequenceContainer>
      <SequenceContainer name="Science">
        <EntryList><ParameterRefEntry parameterRef="Battery"/></EntryList>
        <BaseContainer containerRef="Packet">
          <RestrictionCriteria>
            <Comparison parameterRef="Type" value="2"/>
          </RestrictionCriteria>
        </BaseContainer>
      </SequenceContainer>
    </ContainerSet>
  </TelemetryMetaData>
</SpaceSystem>`

// writeXTCE puts a database in a temporary file and returns its path.
func writeXTCE(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "mission.xml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestXTCEValidate(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	out, err := runCLI(t, nil, "xtce", "validate", path)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if !strings.Contains(out, "is valid") {
		t.Errorf("want a valid verdict, got:\n%s", out)
	}
	if !strings.Contains(out, "3 parameter(s)") {
		t.Errorf("want the parameter count, got:\n%s", out)
	}
	if !strings.Contains(out, "3 container(s)") {
		t.Errorf("want the container count, got:\n%s", out)
	}
}

// A database that breaks a semantic rule the schema cannot express must fail
// rather than load quietly.
func TestXTCEValidateRejectsBadDatabase(t *testing.T) {
	t.Parallel()
	broken := strings.Replace(testXTCE,
		`<Parameter name="Type" parameterTypeRef="U8"/>`,
		`<Parameter name="Type" parameterTypeRef="NoSuchType"/>`, 1)
	path := writeXTCE(t, broken)

	if _, err := runCLI(t, nil, "xtce", "validate", path); err == nil {
		t.Error("validate accepted a database with an unresolved type reference")
	}
}

func TestXTCEValidateMissingFile(t *testing.T) {
	t.Parallel()
	if _, err := runCLI(t, nil, "xtce", "validate", "no-such-file.xml"); err == nil {
		t.Error("validate accepted a missing file")
	}
}

// The names list prints must be the qualified ones the other subcommands
// take, or the output cannot be used as input.
func TestXTCEListPrintsQualifiedNames(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	out, err := runCLI(t, nil, "xtce", "list", path, "--kind", "containers")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	for _, want := range []string{"/Sat/Packet", "/Sat/Housekeeping", "/Sat/Science"} {
		if !strings.Contains(out, want) {
			t.Errorf("list did not name %s:\n%s", want, out)
		}
	}
	// The abstract container is marked, because it cannot be decoded against.
	if !strings.Contains(out, "(abstract)") {
		t.Errorf("list did not mark the abstract container:\n%s", out)
	}
}

func TestXTCEListJSON(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	out, err := runCLI(t, nil, "xtce", "list", path, "--format", "json")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var listing map[string][]string
	if err := json.Unmarshal([]byte(out), &listing); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(listing["parameters"]) != 3 {
		t.Errorf("got %d parameters, want 3", len(listing["parameters"]))
	}
	if len(listing["systems"]) != 1 {
		t.Errorf("got %d systems, want 1", len(listing["systems"]))
	}
}

func TestXTCEListKindFilter(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	out, err := runCLI(t, nil, "xtce", "list", path, "--kind", "parameters")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if strings.Contains(out, "Containers") {
		t.Errorf("--kind parameters listed containers too:\n%s", out)
	}
}

func TestXTCEListRejectsUnknownKind(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	if _, err := runCLI(t, nil, "xtce", "list", path, "--kind", "widgets"); err == nil {
		t.Error("list accepted an unknown --kind")
	}
}

// The layout is the field map a decode reads against, so the offsets and
// widths have to be right.
func TestXTCELayout(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	out, err := runCLI(t, nil, "xtce", "layout", path, "/Sat/Housekeeping", "--format", "json")
	if err != nil {
		t.Fatalf("layout failed: %v", err)
	}

	var layout struct {
		Container string `json:"container"`
		BitSize   uint   `json:"bit_size"`
		MinOctets int    `json:"min_octets"`
		Fields    []struct {
			Name      string `json:"name"`
			Type      string `json:"type"`
			BitOffset uint   `json:"bit_offset"`
			BitSize   uint   `json:"bit_size"`
		} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(out), &layout); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}

	// The inherited field comes first, then the container's own two.
	if len(layout.Fields) != 3 {
		t.Fatalf("got %d fields, want 3 (one inherited, two own)", len(layout.Fields))
	}
	if layout.Fields[0].Name != "/Sat/Type" {
		t.Errorf("first field = %s, want the inherited /Sat/Type", layout.Fields[0].Name)
	}
	if layout.Fields[0].BitOffset != 0 || layout.Fields[0].BitSize != 8 {
		t.Errorf("inherited field at bit %d width %d, want 0 and 8",
			layout.Fields[0].BitOffset, layout.Fields[0].BitSize)
	}
	if layout.Fields[1].BitOffset != 8 {
		t.Errorf("second field at bit %d, want 8", layout.Fields[1].BitOffset)
	}
	if layout.BitSize != 24 || layout.MinOctets != 3 {
		t.Errorf("size = %d bits / %d octets, want 24 and 3", layout.BitSize, layout.MinOctets)
	}
}

func TestXTCELayoutUnknownContainer(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	if _, err := runCLI(t, nil, "xtce", "layout", path, "/Sat/Nope"); err == nil {
		t.Error("layout accepted an unknown container")
	}
}

// Decoding gives the engineering values by default: a calibrated number and
// an enumeration label rather than the counts on the wire.
func TestXTCEDecodeCalibrated(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	// Type 1, Mode 1 (NOMINAL), Battery 250 -> 25.0 volts.
	out, err := runCLI(t, []byte("0101fa"), "xtce", "decode", path,
		"--input", "hex", "--container", "/Sat/Housekeeping", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	values := decodeXTCEValues(t, out)
	if values["/Sat/Mode"] != "NOMINAL" {
		t.Errorf("Mode = %v, want the label NOMINAL", values["/Sat/Mode"])
	}
	if battery, ok := values["/Sat/Battery"].(float64); !ok || battery < 24.9 || battery > 25.1 {
		t.Errorf("Battery = %v, want about 25 volts", values["/Sat/Battery"])
	}
}

// With --raw the same packet gives the counts instead.
func TestXTCEDecodeRaw(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	out, err := runCLI(t, []byte("0101fa"), "xtce", "decode", path,
		"--input", "hex", "--container", "/Sat/Housekeeping", "--format", "json", "--raw")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	values := decodeXTCEValues(t, out)
	if battery, ok := values["/Sat/Battery"].(float64); !ok || battery != 250 {
		t.Errorf("raw Battery = %v, want the count 250", values["/Sat/Battery"])
	}
	if mode, ok := values["/Sat/Mode"].(float64); !ok || mode != 1 {
		t.Errorf("raw Mode = %v, want the count 1", values["/Sat/Mode"])
	}
}

func TestXTCEDecodeText(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	out, err := runCLI(t, []byte("0101fa"), "xtce", "decode", path,
		"--input", "hex", "--container", "/Sat/Housekeeping")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !strings.Contains(out, "NOMINAL") {
		t.Errorf("text output does not show the label:\n%s", out)
	}
	if !strings.Contains(out, "OFFSET") {
		t.Errorf("text output has no field table:\n%s", out)
	}
}

func TestXTCEDecodeRequiresContainer(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	if _, err := runCLI(t, []byte("0101fa"), "xtce", "decode", path, "--input", "hex"); err == nil {
		t.Error("decode ran without --container")
	}
}

// A packet too short for the container is reported rather than decoded into
// whatever happens to be there.
func TestXTCEDecodeShortPacket(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	if _, err := runCLI(t, []byte("01"), "xtce", "decode", path,
		"--input", "hex", "--container", "/Sat/Housekeeping"); err == nil {
		t.Error("decode accepted a packet too short for the container")
	}
}

// match is the ground station's job: work out what the packet is from its
// own contents.
func TestXTCEMatchPicksTheContainer(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	for _, tc := range []struct {
		packet string
		want   string
	}{
		{"0101fa", "Housekeeping"},
		{"02fa", "Science"},
	} {
		out, err := runCLI(t, []byte(tc.packet), "xtce", "match", path,
			"--input", "hex", "--root", "/Sat/Packet", "--format", "name")
		if err != nil {
			t.Fatalf("match %s failed: %v", tc.packet, err)
		}
		if got := strings.TrimSpace(out); got != tc.want {
			t.Errorf("packet %s matched %q, want %q", tc.packet, got, tc.want)
		}
	}
}

func TestXTCEMatchDecodes(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	out, err := runCLI(t, []byte("0101fa"), "xtce", "match", path,
		"--input", "hex", "--root", "/Sat/Packet")
	if err != nil {
		t.Fatalf("match failed: %v", err)
	}
	if !strings.Contains(out, "Matched container: Housekeeping") {
		t.Errorf("match did not name the container it chose:\n%s", out)
	}
	if !strings.Contains(out, "NOMINAL") {
		t.Errorf("match did not decode the packet:\n%s", out)
	}
}

// A packet that satisfies nothing is reported. It is a normal thing for a
// ground station to see, but the caller has to be told.
func TestXTCEMatchNoMatch(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	if _, err := runCLI(t, []byte("09ff"), "xtce", "match", path,
		"--input", "hex", "--root", "/Sat/Packet", "--format", "name"); err == nil {
		t.Error("match reported success for a packet no container claims")
	}
}

func TestXTCEMatchRequiresRoot(t *testing.T) {
	t.Parallel()
	path := writeXTCE(t, testXTCE)

	if _, err := runCLI(t, []byte("0101fa"), "xtce", "match", path, "--input", "hex"); err == nil {
		t.Error("match ran without --root")
	}
}

// decodeXTCEValues reads the JSON decode output into a name-to-value map.
func decodeXTCEValues(t *testing.T, out string) map[string]any {
	t.Helper()

	var rows []struct {
		Name  string `json:"name"`
		Value any    `json:"value"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}

	values := make(map[string]any, len(rows))
	for _, row := range rows {
		if row.Error != "" {
			t.Errorf("field %s failed to decode: %s", row.Name, row.Error)
			continue
		}
		values[row.Name] = row.Value
	}
	return values
}
