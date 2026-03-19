package tmsc_test

import (
	"bytes"
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
		name   string
		nerrs  int
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

	rs.Encode(data)
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
