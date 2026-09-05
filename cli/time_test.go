package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

const testTimestamp = "2026-07-12T10:30:00Z"

func TestTimeRoundTripCodecs(t *testing.T) {
	t.Parallel()
	// Each binary codec encodes to hex and decodes back to the same instant.
	for _, codec := range []string{"cuc", "cds", "ccs"} {
		t.Run(codec, func(t *testing.T) {
			encoded, err := runCLI(t, nil, "time", "encode",
				"--codec", codec, "--time", testTimestamp, "--format", "hex")
			if err != nil {
				t.Fatalf("encode failed: %v", err)
			}
			encoded = strings.TrimSpace(encoded)
			if encoded == "" {
				t.Fatal("encode produced no output")
			}

			out, err := runCLI(t, []byte(encoded), "time", "decode",
				"--codec", codec, "--input", "hex", "--format", "json")
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			var decoded struct {
				Format string `json:"format"`
				Time   string `json:"time"`
				Hex    string `json:"hex"`
			}
			if err := json.Unmarshal([]byte(out), &decoded); err != nil {
				t.Fatalf("unmarshal %q: %v", out, err)
			}
			if decoded.Time != testTimestamp {
				t.Errorf("time = %q, want %q", decoded.Time, testTimestamp)
			}
			if !strings.EqualFold(decoded.Format, codec) {
				t.Errorf("format = %q, want %q", decoded.Format, codec)
			}
			if decoded.Hex != encoded {
				t.Errorf("hex = %q, want %q", decoded.Hex, encoded)
			}
		})
	}
}

func TestTimeCUCFineBytes(t *testing.T) {
	t.Parallel()
	// Adding fine-time octets must lengthen the code.
	coarse, err := runCLI(t, nil, "time", "encode",
		"--codec", "cuc", "--time", testTimestamp, "--fine-bytes", "0", "--format", "hex")
	if err != nil {
		t.Fatalf("encode without fine bytes failed: %v", err)
	}
	fine, err := runCLI(t, nil, "time", "encode",
		"--codec", "cuc", "--time", testTimestamp, "--fine-bytes", "2", "--format", "hex")
	if err != nil {
		t.Fatalf("encode with fine bytes failed: %v", err)
	}

	// 2 extra octets is 4 extra hex characters.
	if len(strings.TrimSpace(fine)) != len(strings.TrimSpace(coarse))+4 {
		t.Errorf("fine-bytes=2 gave %q, want 4 more hex characters than %q",
			strings.TrimSpace(fine), strings.TrimSpace(coarse))
	}
}

func TestTimeCCSMonthDayVariant(t *testing.T) {
	t.Parallel()
	out, err := runCLI(t, nil, "time", "encode",
		"--codec", "ccs", "--time", testTimestamp, "--month-day", "--format", "hex")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := runCLI(t, []byte(strings.TrimSpace(out)), "time", "decode",
		"--codec", "ccs", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var frame struct {
		MonthDay bool   `json:"month_day"`
		Time     string `json:"time"`
	}
	if err := json.Unmarshal([]byte(decoded), &frame); err != nil {
		t.Fatalf("unmarshal %q: %v", decoded, err)
	}
	if !frame.MonthDay {
		t.Error("month_day = false, want true")
	}
	if frame.Time != testTimestamp {
		t.Errorf("time = %q, want %q", frame.Time, testTimestamp)
	}
}

func TestTimeNow(t *testing.T) {
	t.Parallel()
	out, err := runCLI(t, nil, "time", "now")
	if err != nil {
		t.Fatalf("time now failed: %v", err)
	}
	for _, want := range []string{"CUC", "CDS", "CCS"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not mention %s:\n%s", want, out)
		}
	}
}

func TestTimeInspect(t *testing.T) {
	t.Parallel()
	encoded, err := runCLI(t, nil, "time", "encode",
		"--codec", "cuc", "--time", testTimestamp, "--format", "hex")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "time", "inspect", "--input", "hex")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if !strings.Contains(strings.ToUpper(out), "CUC") {
		t.Errorf("inspect output does not identify the codec:\n%s", out)
	}
}

func TestTimeDecodeUnknownCodec(t *testing.T) {
	t.Parallel()
	_, err := runCLI(t, []byte("1c80e5cb28"), "time", "decode",
		"--codec", "sundial", "--input", "hex")
	if err == nil {
		t.Fatal("expected an error for an unknown codec, got nil")
	}
}
