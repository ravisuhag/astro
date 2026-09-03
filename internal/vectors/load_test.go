package vectors

import (
	"strings"
	"testing"
)

// good is a complete, valid vector file that the loader must accept.
// Every known-bad case below is this document with one thing changed, so a
// failure isolates the rule under test.
const good = `{
  "schema_version": 1,
  "standard": "CCSDS 732.0-B-4",
  "package": "aos",
  "source": "hand-computed from the field layouts",
  "encode": [
    {
      "name": "frame-with-fecf",
      "clause": "4.1",
      "note": "byte 0 = 01<<6 | 0xAB>>2 = 0x6a; byte 1 = (0xAB&0x3)<<6 | 42 = 0xea",
      "fields": { "scid": 171, "vcid": 42, "data": "deadbeef" },
      "want": "6aea00010243deadbeef9e2c"
    }
  ],
  "decode": [
    {
      "name": "frame-with-fecf-inverse",
      "clause": "4.1",
      "note": "the inverse of the encode vector above",
      "input": "6aea00010243deadbeef9e2c",
      "fields": { "scid": 171, "vcid": 42, "data": "deadbeef" }
    }
  ],
  "reject": [
    { "name": "header-truncated", "clause": "4.1", "input": "6aea", "error": "truncated" },
    { "name": "vcid-above-3-bit-max", "clause": "4.1.2.3",
      "fields": { "scid": 171, "vcid": 8 }, "error": "field_out_of_range" }
  ]
}`

func TestLoaderAcceptsThePlansExample(t *testing.T) {
	f, err := Parse([]byte(good), "example.json")
	if err != nil {
		t.Fatalf("the plan's own example must load: %v", err)
	}
	if f.Package != "aos" || len(f.Encode) != 1 || len(f.Decode) != 1 || len(f.Reject) != 2 {
		t.Fatalf("parsed shape is wrong: %+v", f)
	}
	if got := f.Encode[0].Fields; !got.Has("scid") {
		t.Error("encode fields did not survive the parse")
	}
}

// TestLoaderRejects is the agreement check between this loader and
// vectors/schema.json. Every case here is a rule the schema states; if the
// two ever drift, one of them fails this table.
func TestLoaderRejects(t *testing.T) {
	tests := []struct {
		name string
		// old is replaced by new in the good document above.
		old, new string
		wantMsg  string
	}{
		{
			name:    "note stripped",
			old:     `"note": "byte 0 = 01<<6 | 0xAB>>2 = 0x6a; byte 1 = (0xAB&0x3)<<6 | 42 = 0xea",`,
			new:     ``,
			wantMsg: "note is required",
		},
		{
			name:    "note too short to be a derivation",
			old:     `"note": "byte 0 = 01<<6 | 0xAB>>2 = 0x6a; byte 1 = (0xAB&0x3)<<6 | 42 = 0xea"`,
			new:     `"note": "todo"`,
			wantMsg: "note is required",
		},
		{
			name:    "unknown top-level key",
			old:     `"standard": "CCSDS 732.0-B-4",`,
			new:     `"standard": "CCSDS 732.0-B-4", "flavour": "vanilla",`,
			wantMsg: "unknown field",
		},
		{
			name:    "want is not hex",
			old:     `"want": "6aea00010243deadbeef9e2c"`,
			new:     `"want": "6AEA00010243DEADBEEF9E2C"`,
			wantMsg: "not lowercase hex",
		},
		{
			name:    "want has an odd number of hex digits",
			old:     `"want": "6aea00010243deadbeef9e2c"`,
			new:     `"want": "6ae"`,
			wantMsg: "not lowercase hex",
		},
		{
			name:    "error name outside the vocabulary",
			old:     `"error": "truncated"`,
			new:     `"error": "malformed"`,
			wantMsg: "not in the vocabulary",
		},
		{
			name:    "reject sets both input and fields",
			old:     `{ "name": "header-truncated", "clause": "4.1", "input": "6aea", "error": "truncated" }`,
			new:     `{ "name": "header-truncated", "clause": "4.1", "input": "6aea", "fields": {"scid": 1}, "error": "truncated" }`,
			wantMsg: "not both",
		},
		{
			name:    "reject sets neither input nor fields",
			old:     `{ "name": "header-truncated", "clause": "4.1", "input": "6aea", "error": "truncated" }`,
			new:     `{ "name": "header-truncated", "clause": "4.1", "error": "truncated" }`,
			wantMsg: "exactly one of input or fields",
		},
		{
			name:    "duplicate vector name",
			old:     `"name": "frame-with-fecf-inverse"`,
			new:     `"name": "frame-with-fecf"`,
			wantMsg: "name already used",
		},
		{
			name:    "name is not kebab-case",
			old:     `"name": "frame-with-fecf",`,
			new:     `"name": "FrameWithFECF",`,
			wantMsg: "lowercase words joined by hyphens",
		},
		{
			name:    "field name is not snake_case",
			old:     `"fields": { "scid": 171, "vcid": 42, "data": "deadbeef" },`,
			new:     `"fields": { "spacecraftID": 171 },`,
			wantMsg: "must be snake_case",
		},
		{
			name:    "wrong schema version",
			old:     `"schema_version": 1,`,
			new:     `"schema_version": 2,`,
			wantMsg: "schema_version is 2",
		},
		{
			name:    "source missing",
			old:     `"source": "hand-computed from the field layouts",`,
			new:     ``,
			wantMsg: "source is missing",
		},
		{
			name:    "unknown requires capability",
			old:     `"clause": "4.1",` + "\n" + `      "note": "byte 0 = 01<<6`,
			new:     `"clause": "4.1", "requires": ["telepathy"],` + "\n" + `      "note": "byte 0 = 01<<6`,
			wantMsg: "unknown capability",
		},
		{
			name:    "buffer_too_small without the encode_into capability",
			old:     `"error": "truncated" }`,
			new:     `"error": "buffer_too_small" }`,
			wantMsg: "only reachable with",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := strings.Replace(good, tt.old, tt.new, 1)
			if doc == good {
				t.Fatalf("test setup did not change the document; old string not found")
			}
			_, err := Parse([]byte(doc), "bad.json")
			if err == nil {
				t.Fatalf("loader accepted a document the schema forbids")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("error did not explain the problem\n  got  %v\n  want it to mention %q", err, tt.wantMsg)
			}
		})
	}
}

// TestEveryCommittedVectorLoads is the gate `make vectors` runs. It walks
// vectors/ so a fixture cannot be committed that the loader rejects.
func TestEveryCommittedVectorLoads(t *testing.T) {
	files, err := LoadAll()
	if err != nil {
		t.Fatalf("a committed vector file does not load: %v", err)
	}
	var vectors, unverified int
	for _, f := range files {
		vectors += len(f.Encode) + len(f.Decode) + len(f.Reject) + len(f.Sequence)
		for _, v := range f.Encode {
			if v.Source == "unverified" {
				unverified++
			}
		}
		for _, v := range f.Decode {
			if v.Source == "unverified" {
				unverified++
			}
		}
	}
	t.Logf("%d vector files, %d vectors, %d marked unverified",
		len(files), vectors, unverified)
}

func TestFieldsAccessors(t *testing.T) {
	f, err := Parse([]byte(good), "example.json")
	if err != nil {
		t.Fatal(err)
	}
	fields := f.Encode[0].Fields

	if got, err := fields.Uint("scid"); err != nil || got != 171 {
		t.Errorf("Uint(scid) = %d, %v; want 171, nil", got, err)
	}
	if got, err := fields.Hex("data"); err != nil || string(got) != "\xde\xad\xbe\xef" {
		t.Errorf("Hex(data) = %x, %v", got, err)
	}
	if _, err := fields.Uint("nope"); err == nil {
		t.Error("Uint of a missing field must report it")
	}
	if _, err := fields.Hex("scid"); err == nil {
		t.Error("Hex of a number must report the type mismatch")
	}
}

// TestWideIntegersSurviveTheRoundTrip guards the rule that anything past
// 2^53 is written as a decimal string. pkg/sdnv tests MaxUint64, and a
// bare JSON number would quietly lose it.
func TestWideIntegersSurviveTheRoundTrip(t *testing.T) {
	doc := `{
	  "schema_version": 1, "standard": "RFC 5050", "package": "sdnv",
	  "source": "RFC 5050 clause 4.1",
	  "encode": [{
	    "name": "max-uint64", "clause": "4.1",
	    "note": "the widest value an SDNV can carry, ten octets",
	    "fields": { "value": "18446744073709551615" },
	    "want": "81ffffffffffffffff7f"
	  }]
	}`
	f, err := Parse([]byte(doc), "wide.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.Encode[0].Fields.Uint("value")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1<<64-1 {
		t.Errorf("value = %d, want %d", got, uint64(1<<64-1))
	}
}
