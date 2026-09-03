package vectors

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
      "note": "byte 0 = 01<<6 | 0xAB>>2 = 0x6a; byte 1 = (0xAB&0x3)<<6 | 42 = 0xea.",
      "fields": { "scid": 171, "vcid": 42, "data": "deadbeef" },
      "want": "6aea00010243deadbeef9e2c"
    }
  ],
  "decode": [
    {
      "name": "frame-with-fecf-inverse",
      "clause": "4.1",
      "note": "the inverse of the encode vector above.",
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
			old:     `"note": "byte 0 = 01<<6 | 0xAB>>2 = 0x6a; byte 1 = (0xAB&0x3)<<6 | 42 = 0xea.",`,
			new:     ``,
			wantMsg: "note is required",
		},
		{
			name:    "note too short to be a derivation",
			old:     `"note": "byte 0 = 01<<6 | 0xAB>>2 = 0x6a; byte 1 = (0xAB&0x3)<<6 | 42 = 0xea."`,
			new:     `"note": "todo."`,
			wantMsg: "note is required",
		},
		{
			name:    "note stops mid-sentence",
			old:     `| 42 = 0xea."`,
			new:     `| 42 = 0xea, and then"`,
			wantMsg: "stops mid-sentence",
		},
		{
			name:    "clause carries a document name",
			old:     `"clause": "4.1",` + "\n" + `      "note": "byte 0 = 01<<6`,
			new:     `"clause": "CCSDS 732.0-B-4 4.1",` + "\n" + `      "note": "byte 0 = 01<<6`,
			wantMsg: "put the document in",
		},
		{
			name:    "at_octet past the end of the input",
			old:     `"input": "6aea", "error": "truncated" }`,
			new:     `"input": "6aea", "at_octet": 9, "error": "truncated" }`,
			wantMsg: "at_octet is 9",
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

// TestSharedConstantsAgree enforces the invariants CONTRACT.md lists
// under "Constants shared across files". Each value is written out in
// more than one package so every file stands alone; an edit that fixes
// one and forgets the other is the failure this catches.
func TestSharedConstantsAgree(t *testing.T) {
	files, err := LoadAll()
	if err != nil {
		t.Fatalf("a committed vector file does not load: %v", err)
	}
	want := map[string]string{} // "pkg/vector" -> want octets
	for _, f := range files {
		for _, v := range f.Encode {
			want[f.Package+"/"+v.Name] = v.Want
		}
	}
	get := func(key string) string {
		v, ok := want[key]
		if !ok {
			t.Fatalf("CONTRACT.md names %q as a shared constant, but no such encode vector exists", key)
		}
		return v
	}

	// The optical sync marker is the TM one, adopted unchanged.
	if a, b := get("tmsc/attached-sync-marker"), get("ocsc/attached-sync-marker"); a != b {
		t.Errorf("attached sync marker has drifted: tmsc has %s, ocsc has %s", a, b)
	}

	// The USLP OID fill is the same generator as the OID randomizer, so the
	// shorter vector must be a prefix of the longer one.
	short, long := get("pn/oid-sequence-first-ten-octets"), get("usdl/oid-pn-fill-sequence-twenty-octets")
	if !strings.HasPrefix(long, short) {
		t.Errorf("OID sequence has drifted: usdl has %s, which does not open with pn's %s", long, short)
	}
}

// TestCoverageTableMatchesTheCorpus keeps COVERAGE.md honest. Its table
// was hand-maintained and drifted eleven vectors behind the files across
// four packages, which is invisible to a reader and misleads anyone
// sizing up the corpus before trusting it.
func TestCoverageTableMatchesTheCorpus(t *testing.T) {
	files, err := LoadAll()
	if err != nil {
		t.Fatalf("a committed vector file does not load: %v", err)
	}
	actual := map[string]int{}
	total := 0
	for _, f := range files {
		n := len(f.Encode) + len(f.Decode) + len(f.Reject) + len(f.Sequence)
		actual[f.Package] += n
		total += n
	}

	raw, err := os.ReadFile(filepath.Join(Root(), "COVERAGE.md"))
	if err != nil {
		t.Fatalf("COVERAGE.md: %v", err)
	}

	// | `spp` | CCSDS 133.0-B-2 | 28 | — |
	row := regexp.MustCompile("(?m)^\\| `([a-z0-9]+)` \\| [^|]*\\| *(\\d+|—) *\\|")
	claimed := map[string]int{}
	for _, m := range row.FindAllStringSubmatch(string(raw), -1) {
		if m[2] == "—" {
			claimed[m[1]] = 0
			continue
		}
		n, _ := strconv.Atoi(m[2])
		claimed[m[1]] = n
	}

	for pkg, want := range actual {
		got, listed := claimed[pkg]
		if !listed {
			t.Errorf("COVERAGE.md has no row for %q, which holds %d vectors", pkg, want)
			continue
		}
		if got != want {
			t.Errorf("COVERAGE.md says %q has %d vectors, the files hold %d", pkg, got, want)
		}
	}
	for pkg := range claimed {
		if _, ok := actual[pkg]; !ok {
			t.Errorf("COVERAGE.md lists %q, which has no vector file", pkg)
		}
	}

	sum := regexp.MustCompile(`\| \*\*Total\*\* \| \| \*\*(\d+)\*\* \|`).FindSubmatch(raw)
	if sum == nil {
		t.Fatal("COVERAGE.md has no total row")
	}
	if n, _ := strconv.Atoi(string(sum[1])); n != total {
		t.Errorf("COVERAGE.md totals %d vectors, the files hold %d", n, total)
	}
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
	    "note": "the widest value an SDNV can carry, ten octets.",
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
