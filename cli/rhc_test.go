package cli

import (
	"encoding/hex"
	"strings"
	"testing"
)

// housekeeping is what POCKET+ is for: a stream of equal-length vectors
// where almost nothing changes from one to the next.
func housekeeping(cycles int) []byte {
	base := []byte{0xA5, 0x00, 0xFF, 0x10, 0x00, 0x33, 0x00, 0x01}

	var out []byte
	for i := 0; i < cycles; i++ {
		vector := append([]byte{}, base...)
		if i%7 == 0 {
			vector[3] ^= 0x08
		}
		out = append(out, vector...)
	}
	return out
}

// The property that matters: compress and decompress here are inverses.
func TestRHCRoundTripIsExact(t *testing.T) {
	original := housekeeping(40)

	listing, err := runCLI(t, []byte(hex.EncodeToString(original)), "rhc", "compress",
		"--input", "hex", "--vector-bits", "64")
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	recovered, err := runCLI(t, []byte(listing), "rhc", "decompress",
		"--vector-bits", "64", "--format", "hex")
	if err != nil {
		t.Fatalf("decompress failed: %v", err)
	}

	var got []byte
	for _, line := range nonEmptyLines(recovered) {
		vector, err := hex.DecodeString(strings.TrimSpace(line))
		if err != nil {
			t.Fatalf("decoding a recovered vector: %v", err)
		}
		got = append(got, vector...)
	}

	if string(got) != string(original) {
		t.Errorf("round trip changed the data: %d octets in, %d out", len(original), len(got))
	}
}

// Housekeeping that barely changes should compress hard, or the coder is not
// doing what it exists to do.
func TestRHCCompressesStaticHousekeeping(t *testing.T) {
	listing, err := runCLI(t, []byte(hex.EncodeToString(housekeeping(40))), "rhc", "compress",
		"--input", "hex", "--vector-bits", "64")
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	lines := nonEmptyLines(listing)
	if len(lines) != 40 {
		t.Fatalf("got %d cycles, want 40", len(lines))
	}

	// After the first cycle carries the whole vector, the rest should be far
	// smaller than the 64 bits they stand for.
	total := 0
	for _, line := range lines {
		bitLen, _, err := parseListingLine(strings.TrimSpace(line))
		if err != nil {
			t.Fatalf("parsing %q: %v", line, err)
		}
		total += bitLen
	}
	if total >= 40*64 {
		t.Errorf("output is %d bits, no smaller than the %d it started at", total, 40*64)
	}
}

// The vector length has to match on both sides. It is not in the listing,
// because the standard defines no container that would carry it.
func TestRHCVectorBitsMustMatch(t *testing.T) {
	listing, err := runCLI(t, []byte(hex.EncodeToString(housekeeping(8))), "rhc", "compress",
		"--input", "hex", "--vector-bits", "64")
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	out, err := runCLI(t, []byte(listing), "rhc", "decompress",
		"--vector-bits", "32", "--format", "hex")
	if err != nil {
		// Failing is the right outcome.
		return
	}
	// If it did not fail, it must at least not have produced the original.
	if len(nonEmptyLines(out)) == 8 {
		t.Error("decompressing at the wrong vector length produced a plausible result")
	}
}

// Input that is not a whole number of vectors means the flag and the data
// disagree about the vector length.
func TestRHCRejectsPartialVector(t *testing.T) {
	if _, err := runCLI(t, []byte("a5000000"), "rhc", "compress",
		"--input", "hex", "--vector-bits", "64"); err == nil {
		t.Error("compress accepted four octets as 64-bit vectors")
	}
}

func TestRHCRejectsBadFlags(t *testing.T) {
	for name, args := range map[string][]string{
		"no vector bits":      {"rhc", "compress", "--input", "hex", "--vector-bits", "0"},
		"robustness too high": {"rhc", "compress", "--input", "hex", "--vector-bits", "64", "--robustness", "9"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := runCLI(t, []byte(hex.EncodeToString(housekeeping(2))), args...); err == nil {
				t.Errorf("%s was accepted", name)
			}
		})
	}
}

func TestRHCEmptyInput(t *testing.T) {
	if _, err := runCLI(t, []byte(""), "rhc", "compress",
		"--input", "hex", "--vector-bits", "64"); err == nil {
		t.Error("compress accepted empty input")
	}
}

// A malformed listing is reported rather than fed to the decompressor.
func TestRHCDecompressRejectsMalformedListing(t *testing.T) {
	for name, listing := range map[string]string{
		"no bit length":           "a5b6c7\n",
		"bit length not a number": "many a5b6c7\n",
		"not hex":                 "16 zzzz\n",
		"too few octets":          "64 a5\n",
		"empty":                   "\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := runCLI(t, []byte(listing), "rhc", "decompress",
				"--vector-bits", "64", "--format", "hex"); err == nil {
				t.Errorf("decompress accepted a listing with %s", name)
			}
		})
	}
}

// parseListingLine is the listing parser, so its contract is worth pinning
// directly as well as through the commands.
func TestParseListingLine(t *testing.T) {
	bitLen, octets, err := parseListingLine("18 f14280")
	if err != nil {
		t.Fatalf("parseListingLine: %v", err)
	}
	if bitLen != 18 {
		t.Errorf("bitLen = %d, want 18", bitLen)
	}
	if hex.EncodeToString(octets) != "f14280" {
		t.Errorf("octets = %x, want f14280", octets)
	}

	// A line claiming more bits than it carries octets for is a broken line,
	// not something to read past the end of.
	if _, _, err := parseListingLine("64 a5"); err == nil {
		t.Error("a line claiming 64 bits in one octet was accepted")
	}
	if _, _, err := parseListingLine("-1 a5"); err == nil {
		t.Error("a negative bit length was accepted")
	}
}
