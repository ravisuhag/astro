package cli

import (
	"strings"
	"testing"
)

func TestGenNegativeSize(t *testing.T) {
	// Used to panic with a raw Go stack trace from make([]byte, -1).
	_, err := runCLI(t, nil, "spp", "gen", "--size=-1")
	if err == nil {
		t.Fatal("expected an error for negative --size, got nil")
	}
	if !strings.Contains(err.Error(), "size must be >= 0") {
		t.Errorf("error = %q, want it to mention size must be >= 0", err)
	}
}

func TestGenOversize(t *testing.T) {
	_, err := runCLI(t, nil, "spp", "gen", "--size", "20000000")
	if err == nil {
		t.Fatal("expected an error for oversized --size, got nil")
	}
	if !strings.Contains(err.Error(), "size must be <=") {
		t.Errorf("error = %q, want it to mention the size ceiling", err)
	}
}

func TestGenNegativeCount(t *testing.T) {
	_, err := runCLI(t, nil, "spp", "gen", "--count=-1")
	if err == nil {
		t.Fatal("expected an error for negative --count, got nil")
	}
	if !strings.Contains(err.Error(), "count must be >= 0") {
		t.Errorf("error = %q, want it to mention count must be >= 0", err)
	}
}

func TestGenTypeCaseInsensitive(t *testing.T) {
	// spp encode accepted --type TM while spp gen rejected it.
	out, err := runCLI(t, nil, "spp", "gen", "--type", "TM", "--count", "1", "--format", "hex")
	if err != nil {
		t.Fatalf("gen --type TM failed: %v", err)
	}
	if len(strings.TrimSpace(out)) == 0 {
		t.Fatal("expected hex output")
	}
}

func TestGenOutputIsDecodable(t *testing.T) {
	// Each gen command emits one hex unit per line; feed every line back
	// through that protocol's own decoder.
	tests := []struct {
		name     string
		genArgs  []string
		decode   []string
		sizeFlag string
	}{
		{name: "spp", genArgs: []string{"spp", "gen"}, decode: []string{"spp", "decode"}, sizeFlag: "--size"},
		{name: "epp", genArgs: []string{"epp", "gen"}, decode: []string{"epp", "decode"}, sizeFlag: "--size"},
		{name: "tm", genArgs: []string{"tm", "gen"}, decode: []string{"tm", "decode"}, sizeFlag: "--data-size"},
		{name: "tc", genArgs: []string{"tc", "gen"}, decode: []string{"tc", "decode"}, sizeFlag: "--data-size"},
		{name: "aos", genArgs: []string{"aos", "gen"}, decode: []string{"aos", "decode"}, sizeFlag: "--data-size"},
		{name: "usdl", genArgs: []string{"usdl", "gen"}, decode: []string{"usdl", "decode"}, sizeFlag: "--data-size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append(append([]string{}, tt.genArgs...),
				"--count", "3", "--format", "hex", tt.sizeFlag, "16")
			out, err := runCLI(t, nil, args...)
			if err != nil {
				t.Fatalf("gen failed: %v", err)
			}

			lines := nonEmptyLines(out)
			if len(lines) != 3 {
				t.Fatalf("got %d units, want 3:\n%s", len(lines), out)
			}

			for i, line := range lines {
				decodeArgs := append(append([]string{}, tt.decode...),
					"--input", "hex", "--format", "json")
				if _, err := runCLI(t, []byte(line), decodeArgs...); err != nil {
					t.Errorf("unit %d does not decode: %v", i, err)
				}
			}
		})
	}
}

func TestGenCADUAndCLTUUnwrap(t *testing.T) {
	tests := []struct {
		name   string
		gen    []string
		unwrap []string
	}{
		{name: "cadu", gen: []string{"cadu", "gen"}, unwrap: []string{"cadu", "unwrap"}},
		{name: "cltu", gen: []string{"cltu", "gen"}, unwrap: []string{"cltu", "unwrap"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append(append([]string{}, tt.gen...),
				"--count", "2", "--format", "hex", "--data-size", "16")
			out, err := runCLI(t, nil, args...)
			if err != nil {
				t.Fatalf("gen failed: %v", err)
			}

			lines := nonEmptyLines(out)
			if len(lines) != 2 {
				t.Fatalf("got %d units, want 2:\n%s", len(lines), out)
			}

			for i, line := range lines {
				unwrapArgs := append(append([]string{}, tt.unwrap...), "--input", "hex")
				if _, err := runCLI(t, []byte(line), unwrapArgs...); err != nil {
					t.Errorf("unit %d does not unwrap: %v", i, err)
				}
			}
		})
	}
}

func TestGenUnknownOutputFormat(t *testing.T) {
	// writeGenOutput rejects anything but bin and hex.
	_, err := runCLI(t, nil, "spp", "gen", "--count", "1", "--format", "yaml")
	if err == nil {
		t.Fatal("expected an error for an unknown --format, got nil")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("error = %q, want it to mention the unknown format", err)
	}
}

func TestGenZeroCount(t *testing.T) {
	out, err := runCLI(t, nil, "spp", "gen", "--count", "0", "--format", "hex")
	if err != nil {
		t.Fatalf("gen --count 0 failed: %v", err)
	}
	// The "Generated N packet(s)" summary goes to stderr, so stdout is empty.
	if got := len(nonEmptyLines(out)); got != 0 {
		t.Errorf("got %d packets on stdout, want 0:\n%s", got, out)
	}
}
