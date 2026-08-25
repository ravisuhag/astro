package tmsc_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmsc"
)

// --- RS(255,223) Tests ---

func TestRS255_223_EncodeLength(t *testing.T) {
	rs := tmsc.NewRS255_223()
	data := make([]byte, 223)
	for i := range data {
		data[i] = byte(i)
	}
	cw, err := rs.Encode(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(cw) != 255 {
		t.Errorf("codeword length = %d, want 255", len(cw))
	}
	// Data portion should be preserved
	if !bytes.Equal(cw[:223], data) {
		t.Error("codeword data portion differs from input")
	}
}

func TestRS255_223_RoundTrip_NoErrors(t *testing.T) {
	rs := tmsc.NewRS255_223()
	data := make([]byte, 223)
	for i := range data {
		data[i] = byte(i * 3)
	}
	cw, _ := rs.Encode(data)
	decoded, corr, err := rs.Decode(cw)
	if err != nil {
		t.Fatal(err)
	}
	if corr != 0 {
		t.Errorf("corrections = %d, want 0", corr)
	}
	if !bytes.Equal(decoded, data) {
		t.Error("decoded data differs from original")
	}
}

func TestRS255_223_CorrectErrors(t *testing.T) {
	rs := tmsc.NewRS255_223()
	rng := rand.New(rand.NewPCG(42, 0))

	tests := []struct {
		name  string
		nerrs int
	}{
		{"1 error", 1},
		{"8 errors", 8},
		{"16 errors (max)", 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, 223)
			for i := range data {
				data[i] = byte(rng.IntN(256))
			}
			cw, _ := rs.Encode(data)

			// Inject errors at random positions
			positions := rng.Perm(255)[:tt.nerrs]
			for _, pos := range positions {
				cw[pos] ^= byte(rng.IntN(255) + 1)
			}

			decoded, corr, err := rs.Decode(cw)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if corr != tt.nerrs {
				t.Errorf("corrections = %d, want %d", corr, tt.nerrs)
			}
			if !bytes.Equal(decoded, data) {
				t.Error("decoded data differs from original")
			}
		})
	}
}

func TestRS255_223_Uncorrectable(t *testing.T) {
	rs := tmsc.NewRS255_223()
	rng := rand.New(rand.NewPCG(99, 0))

	data := make([]byte, 223)
	for i := range data {
		data[i] = byte(rng.IntN(256))
	}
	cw, _ := rs.Encode(data)

	// Inject 17 errors (exceeds correction capability of 16)
	positions := rng.Perm(255)[:17]
	for _, pos := range positions {
		cw[pos] ^= byte(rng.IntN(255) + 1)
	}

	_, _, err := rs.Decode(cw)
	if !errors.Is(err, tmsc.ErrUncorrectable) {
		t.Errorf("expected ErrUncorrectable, got %v", err)
	}
}

// --- RS(255,239) Tests ---

func TestRS255_239_RoundTrip_NoErrors(t *testing.T) {
	rs := tmsc.NewRS255_239()
	data := make([]byte, 239)
	for i := range data {
		data[i] = byte(i)
	}
	cw, _ := rs.Encode(data)
	if len(cw) != 255 {
		t.Fatalf("codeword length = %d, want 255", len(cw))
	}

	decoded, corr, err := rs.Decode(cw)
	if err != nil {
		t.Fatal(err)
	}
	if corr != 0 {
		t.Errorf("corrections = %d, want 0", corr)
	}
	if !bytes.Equal(decoded, data) {
		t.Error("decoded data differs from original")
	}
}

func TestRS255_239_CorrectErrors(t *testing.T) {
	rs := tmsc.NewRS255_239()
	rng := rand.New(rand.NewPCG(77, 0))

	tests := []struct {
		name  string
		nerrs int
	}{
		{"1 error", 1},
		{"4 errors", 4},
		{"8 errors (max)", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, 239)
			for i := range data {
				data[i] = byte(rng.IntN(256))
			}
			cw, _ := rs.Encode(data)

			positions := rng.Perm(255)[:tt.nerrs]
			for _, pos := range positions {
				cw[pos] ^= byte(rng.IntN(255) + 1)
			}

			decoded, corr, err := rs.Decode(cw)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if corr != tt.nerrs {
				t.Errorf("corrections = %d, want %d", corr, tt.nerrs)
			}
			if !bytes.Equal(decoded, data) {
				t.Error("decoded data differs from original")
			}
		})
	}
}

func TestRS255_239_Uncorrectable(t *testing.T) {
	rs := tmsc.NewRS255_239()
	rng := rand.New(rand.NewPCG(55, 0))

	data := make([]byte, 239)
	for i := range data {
		data[i] = byte(rng.IntN(256))
	}
	cw, _ := rs.Encode(data)

	// Inject 9 errors (exceeds capability of 8)
	positions := rng.Perm(255)[:9]
	for _, pos := range positions {
		cw[pos] ^= byte(rng.IntN(255) + 1)
	}

	_, _, err := rs.Decode(cw)
	if !errors.Is(err, tmsc.ErrUncorrectable) {
		t.Errorf("expected ErrUncorrectable, got %v", err)
	}
}

// --- Input Validation Tests ---

func TestRS_Encode_WrongLength(t *testing.T) {
	rs := tmsc.NewRS255_223()
	_, err := rs.Encode([]byte{0x01, 0x02})
	if !errors.Is(err, tmsc.ErrInvalidDataLength) {
		t.Errorf("expected ErrInvalidDataLength, got %v", err)
	}
}

func TestRS_Decode_WrongLength(t *testing.T) {
	rs := tmsc.NewRS255_223()
	_, _, err := rs.Decode([]byte{0x01, 0x02})
	if !errors.Is(err, tmsc.ErrInvalidDataLength) {
		t.Errorf("expected ErrInvalidDataLength, got %v", err)
	}
}

func TestRS_Encode_DoesNotMutateInput(t *testing.T) {
	rs := tmsc.NewRS255_223()
	data := make([]byte, 223)
	for i := range data {
		data[i] = byte(i)
	}
	saved := make([]byte, len(data))
	copy(saved, data)

	_, _ = rs.Encode(data)
	if !bytes.Equal(data, saved) {
		t.Error("Encode must not modify the input slice")
	}
}

func TestRS_Encode_Deterministic(t *testing.T) {
	rs := tmsc.NewRS255_223()
	data := make([]byte, 223)
	for i := range data {
		data[i] = byte(i)
	}
	cw1, _ := rs.Encode(data)
	cw2, _ := rs.Encode(data)
	if !bytes.Equal(cw1, cw2) {
		t.Error("same input should produce same codeword")
	}
}

func TestRS_NRoots_DataLen(t *testing.T) {
	rs223 := tmsc.NewRS255_223()
	if rs223.NRoots() != 32 {
		t.Errorf("NRoots = %d, want 32", rs223.NRoots())
	}
	if rs223.DataLen() != 223 {
		t.Errorf("DataLen = %d, want 223", rs223.DataLen())
	}

	rs239 := tmsc.NewRS255_239()
	if rs239.NRoots() != 16 {
		t.Errorf("NRoots = %d, want 16", rs239.NRoots())
	}
	if rs239.DataLen() != 239 {
		t.Errorf("DataLen = %d, want 239", rs239.DataLen())
	}
}

// --- Interleaving Tests ---

func TestRS_Interleave_RoundTrip(t *testing.T) {
	rs := tmsc.NewRS255_223()
	rng := rand.New(rand.NewPCG(123, 0))

	depths := []int{1, 2, 3, 4, 5, 8}
	for _, depth := range depths {
		t.Run("depth="+string(rune('0'+depth)), func(t *testing.T) {
			data := make([]byte, depth*223)
			for i := range data {
				data[i] = byte(rng.IntN(256))
			}

			encoded, err := rs.EncodeInterleaved(data, depth)
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) != depth*255 {
				t.Fatalf("encoded length = %d, want %d", len(encoded), depth*255)
			}

			decoded, corr, err := rs.DecodeInterleaved(encoded, depth)
			if err != nil {
				t.Fatal(err)
			}
			if corr != 0 {
				t.Errorf("corrections = %d, want 0", corr)
			}
			if !bytes.Equal(decoded, data) {
				t.Error("interleaved round-trip failed")
			}
		})
	}
}

func TestRS_Interleave_WithErrors(t *testing.T) {
	rs := tmsc.NewRS255_223()
	rng := rand.New(rand.NewPCG(456, 0))
	depth := 5

	data := make([]byte, depth*223)
	for i := range data {
		data[i] = byte(rng.IntN(256))
	}

	encoded, _ := rs.EncodeInterleaved(data, depth)

	// Inject 8 errors per sub-codeword (well within correction limit of 16)
	for d := 0; d < depth; d++ {
		positions := rng.Perm(255)[:8]
		for _, pos := range positions {
			encoded[pos*depth+d] ^= byte(rng.IntN(255) + 1)
		}
	}

	decoded, corr, err := rs.DecodeInterleaved(encoded, depth)
	if err != nil {
		t.Fatalf("DecodeInterleaved failed: %v", err)
	}
	if corr != depth*8 {
		t.Errorf("total corrections = %d, want %d", corr, depth*8)
	}
	if !bytes.Equal(decoded, data) {
		t.Error("interleaved decode with errors failed")
	}
}

func TestRS_Interleave_InvalidDepth(t *testing.T) {
	rs := tmsc.NewRS255_223()
	data := make([]byte, 7*223)
	_, err := rs.EncodeInterleaved(data, 7)
	if !errors.Is(err, tmsc.ErrInvalidInterleaveDepth) {
		t.Errorf("expected ErrInvalidInterleaveDepth, got %v", err)
	}
}

func TestRS_Interleave_WrongDataLength(t *testing.T) {
	rs := tmsc.NewRS255_223()
	_, err := rs.EncodeInterleaved([]byte{0x01}, 2)
	if !errors.Is(err, tmsc.ErrInvalidDataLength) {
		t.Errorf("expected ErrInvalidDataLength, got %v", err)
	}
}

// --- Shortened Codeblock (Virtual Fill) Tests ---

func TestRS_Shortened_RoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(2024, 0))

	tests := []struct {
		name  string
		rs    *tmsc.RSCodec
		depth int
		fill  int
	}{
		{"(255,223) depth=1 fill=100", tmsc.NewRS255_223(), 1, 100},
		{"(255,223) depth=5 fill=115", tmsc.NewRS255_223(), 5, 115},
		{"(255,239) depth=1 fill=39", tmsc.NewRS255_239(), 1, 39},
		{"(255,239) depth=4 fill=156", tmsc.NewRS255_239(), 4, 156},
		{"(255,223) depth=2 fill=0 (no shortening)", tmsc.NewRS255_223(), 2, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := tt.rs.DataLen()
			data := make([]byte, tt.depth*k-tt.fill)
			for i := range data {
				data[i] = byte(rng.IntN(256))
			}

			encoded, err := tt.rs.EncodeShortened(data, tt.depth, tt.fill)
			if err != nil {
				t.Fatal(err)
			}
			if want := tt.depth*255 - tt.fill; len(encoded) != want {
				t.Fatalf("encoded length = %d, want %d", len(encoded), want)
			}

			decoded, corr, err := tt.rs.DecodeShortened(encoded, tt.depth, tt.fill)
			if err != nil {
				t.Fatal(err)
			}
			if corr != 0 {
				t.Errorf("corrections = %d, want 0", corr)
			}
			if !bytes.Equal(decoded, data) {
				t.Error("shortened round-trip failed")
			}
		})
	}
}

// TestRS_Shortened_MatchesVirtualFillDefinition pins the semantics: a
// shortened codeblock is the full codeblock of the zero-prefixed data with
// the leading virtual fill stripped (CCSDS 131.0-B-5 4.3.7.3, 4.3.7.4).
func TestRS_Shortened_MatchesVirtualFillDefinition(t *testing.T) {
	rs := tmsc.NewRS255_223()
	const depth, fill = 3, 33

	data := make([]byte, depth*223-fill)
	for i := range data {
		data[i] = byte(i * 5)
	}

	short, err := rs.EncodeShortened(data, depth, fill)
	if err != nil {
		t.Fatal(err)
	}

	padded := make([]byte, depth*223)
	copy(padded[fill:], data)
	full, err := rs.EncodeInterleaved(padded, depth)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(short, full[fill:]) {
		t.Error("shortened codeblock differs from zero-prefixed codeblock minus fill")
	}
	for _, b := range full[:fill] {
		if b != 0 {
			t.Fatal("virtual fill positions must encode as zeros")
		}
	}
}

func TestRS_Shortened_CorrectErrors(t *testing.T) {
	rs := tmsc.NewRS255_223()
	rng := rand.New(rand.NewPCG(31337, 0))
	const depth, fill = 5, 115

	data := make([]byte, depth*223-fill)
	for i := range data {
		data[i] = byte(rng.IntN(256))
	}
	encoded, err := rs.EncodeShortened(data, depth, fill)
	if err != nil {
		t.Fatal(err)
	}

	// 16 errors per codeword is the maximum for (255,223). Transmitted
	// position p belongs to codeword (p+fill) mod depth, so spread the
	// errors evenly by stepping through whole interleaved columns.
	nerrs := 16 * depth
	for i := range nerrs {
		encoded[i*(len(encoded)/nerrs)] ^= 0xA5
	}

	decoded, corr, err := rs.DecodeShortened(encoded, depth, fill)
	if err != nil {
		t.Fatalf("DecodeShortened failed: %v", err)
	}
	if corr == 0 {
		t.Error("expected corrections to be reported")
	}
	if !bytes.Equal(decoded, data) {
		t.Error("shortened decode with errors failed")
	}
}

func TestRS_Shortened_Uncorrectable(t *testing.T) {
	rs := tmsc.NewRS255_239()
	rng := rand.New(rand.NewPCG(4242, 0))
	const depth, fill = 1, 100

	data := make([]byte, 239-fill)
	for i := range data {
		data[i] = byte(rng.IntN(256))
	}
	encoded, _ := rs.EncodeShortened(data, depth, fill)

	// 9 errors exceeds the capability of 8.
	positions := rng.Perm(len(encoded))[:9]
	for _, pos := range positions {
		encoded[pos] ^= byte(rng.IntN(255) + 1)
	}

	_, _, err := rs.DecodeShortened(encoded, depth, fill)
	if err == nil {
		t.Error("expected an error decoding past the correction capability")
	}
}

func TestRS_Shortened_InvalidVirtualFill(t *testing.T) {
	rs := tmsc.NewRS255_223()

	tests := []struct {
		name  string
		depth int
		fill  int
	}{
		{"negative", 1, -1},
		{"not a multiple of depth", 2, 3},
		{"consumes all data symbols", 1, 223},
		{"exceeds data symbols", 5, 5 * 223},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rs.EncodeShortened(nil, tt.depth, tt.fill)
			if !errors.Is(err, tmsc.ErrInvalidVirtualFill) {
				t.Errorf("Encode: expected ErrInvalidVirtualFill, got %v", err)
			}
			_, _, err = rs.DecodeShortened(nil, tt.depth, tt.fill)
			if !errors.Is(err, tmsc.ErrInvalidVirtualFill) {
				t.Errorf("Decode: expected ErrInvalidVirtualFill, got %v", err)
			}
		})
	}

	// Invalid depth wins over invalid fill, matching the interleaved API.
	if _, err := rs.EncodeShortened(nil, 7, 7); !errors.Is(err, tmsc.ErrInvalidInterleaveDepth) {
		t.Errorf("expected ErrInvalidInterleaveDepth, got %v", err)
	}
	// A fill that is valid but does not match the data length.
	if _, err := rs.EncodeShortened(make([]byte, 10), 1, 100); !errors.Is(err, tmsc.ErrInvalidDataLength) {
		t.Errorf("expected ErrInvalidDataLength, got %v", err)
	}
}

// --- Decoder Self-Consistency ---

// TestRS_Decode_ResultIsAlwaysACodeword hammers the decoder with error
// patterns past its correction capability. Whenever it claims success, the
// data it returns must re-encode to a codeword within NRoots()/2 symbol
// changes of what was received: a decoder that skips inconsistent Forney
// positions or returns without recomputing syndromes fails this.
func TestRS_Decode_ResultIsAlwaysACodeword(t *testing.T) {
	rs := tmsc.NewRS255_239()
	rng := rand.New(rand.NewPCG(7, 7))

	succeeded := 0
	for range 300 {
		data := make([]byte, 239)
		for i := range data {
			data[i] = byte(rng.IntN(256))
		}
		cw, _ := rs.Encode(data)

		for _, pos := range rng.Perm(255)[:10+rng.IntN(30)] {
			cw[pos] ^= byte(rng.IntN(255) + 1)
		}

		decoded, corr, err := rs.Decode(cw)
		if err != nil {
			continue
		}
		succeeded++

		if corr > rs.NRoots()/2 {
			t.Fatalf("claimed %d corrections, beyond the capability of %d", corr, rs.NRoots()/2)
		}
		recoded, err := rs.Encode(decoded)
		if err != nil {
			t.Fatal(err)
		}
		dist := 0
		for i := range recoded {
			if recoded[i] != cw[i] {
				dist++
			}
		}
		if dist != corr {
			t.Fatalf("decoder changed %d symbols but reported %d: result is not a consistent codeword", dist, corr)
		}
	}
	t.Logf("%d of 300 overloaded patterns were (mis)decoded to a valid codeword", succeeded)
}

// --- Interoperability Golden Vectors ---
//
// Expected parity bytes generated with libfec's CCSDS dual-basis codecs
// (encode_rs_ccsds for (255,223); init_rs_char(8, 0x187, 112, 11, 16, 0)
// plus the dual-basis transform for (255,239)), the same code used by
// gr-satellites and other CCSDS ground software. Data pattern:
// data[i] = (i*7 + 1) mod 256.

func interopData(n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte((i*7 + 1) % 256)
	}
	return data
}

func hexBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRS255_223_LibfecGoldenParity(t *testing.T) {
	rs := tmsc.NewRS255_223()
	cw, err := rs.Encode(interopData(223))
	if err != nil {
		t.Fatal(err)
	}
	want := hexBytes(t, "02dd3a85516a45e4791b58d7fe96efe03f8c48800992818037f327634c7340ac")
	if !bytes.Equal(cw[223:], want) {
		t.Errorf("parity mismatch with libfec dual-basis CCSDS codec\ngot  %x\nwant %x", cw[223:], want)
	}
}

func TestRS255_239_LibfecGoldenParity(t *testing.T) {
	rs := tmsc.NewRS255_239()
	cw, err := rs.Encode(interopData(239))
	if err != nil {
		t.Fatal(err)
	}
	want := hexBytes(t, "6a408a5f6b203d9f62a27db41be89735")
	if !bytes.Equal(cw[239:], want) {
		t.Errorf("parity mismatch with libfec dual-basis CCSDS codec\ngot  %x\nwant %x", cw[239:], want)
	}
}

func TestRS_AllZeroData_ZeroParity(t *testing.T) {
	// The all-zero codeword is a codeword of every linear code, and the
	// dual-basis transform maps zero to zero.
	for _, rs := range []*tmsc.RSCodec{tmsc.NewRS255_223(), tmsc.NewRS255_239()} {
		cw, err := rs.Encode(make([]byte, rs.DataLen()))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(cw, make([]byte, 255)) {
			t.Errorf("nroots=%d: all-zero data must give all-zero codeword", rs.NRoots())
		}
	}
}
