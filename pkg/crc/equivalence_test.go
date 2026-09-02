package crc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/crc"
)

// The table-driven checksums are checked against the bit-by-bit definition.
//
// The functions below are the algorithms exactly as the standards describe
// them: shift the register one bit at a time and XOR in the generator
// polynomial whenever a one falls off the top. They are slow and obviously
// correct, which is what makes them a useful reference. Every published check
// value in crc_test.go pins them too, so a bug would have to be in both the
// reference and the table to survive.
//
// This is the only thing standing between a fast checksum and the failure
// CONTRIBUTING describes: an implementation that is self-consistent and
// wrong. A wrong table would round-trip perfectly and be rejected by every
// conforming receiver.

// bitwiseCRC16 is CCSDS 133.0-B-2 §4.1.4 written out: polynomial 0x1021,
// register preset to all ones, no reflection, no final XOR.
func bitwiseCRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// bitwiseCRC32 is CCSDS 211.2-B-3 annex C written out: polynomial
// 0x00A00805, register preset to zero, no reflection, no final XOR.
func bitwiseCRC32(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		crc ^= uint32(b) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ 0x00A00805
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// TestTableMatchesBitwise runs both checksums over every length from empty to
// a full frame and requires the table to agree with the bit loop on all of
// them.
//
// The data is a deterministic pattern rather than random, so a failure is
// reproducible without a seed. Two patterns are used: one that walks all 256
// byte values, and one of all-ones, which is what idle fill looks like and
// the case where a register preset to all ones behaves least like a preset of
// zero.
func TestTableMatchesBitwise(t *testing.T) {
	patterns := map[string]func(i int) byte{
		"counting": func(i int) byte { return byte(i) },
		"strided":  func(i int) byte { return byte(i * 31) },
		"idle":     func(int) byte { return 0xFF },
		"zeros":    func(int) byte { return 0x00 },
	}

	for name, fill := range patterns {
		t.Run(name, func(t *testing.T) {
			// 1115 is a frame; a few octets past it catches an off-by-one in
			// the loop bound.
			data := make([]byte, 1120)
			for i := range data {
				data[i] = fill(i)
			}

			for length := 0; length <= len(data); length++ {
				chunk := data[:length]

				if got, want := crc.ComputeCRC16(chunk), bitwiseCRC16(chunk); got != want {
					t.Fatalf("CRC16 over %d octets = 0x%04X, the bit loop gives 0x%04X", length, got, want)
				}
				if got, want := crc.ComputeCRC32(chunk), bitwiseCRC32(chunk); got != want {
					t.Fatalf("CRC32 over %d octets = 0x%08X, the bit loop gives 0x%08X", length, got, want)
				}
			}
		})
	}
}

// TestTableMatchesBitwiseForEverySingleByte checks all 256 single-byte
// messages one at a time, which is the case that exercises every table entry
// exactly once. A single mistyped entry would show up here and nowhere else
// if the longer messages happened not to reach it.
func TestTableMatchesBitwiseForEverySingleByte(t *testing.T) {
	for value := 0; value < 256; value++ {
		chunk := []byte{byte(value)}

		if got, want := crc.ComputeCRC16(chunk), bitwiseCRC16(chunk); got != want {
			t.Errorf("CRC16(0x%02X) = 0x%04X, the bit loop gives 0x%04X", value, got, want)
		}
		if got, want := crc.ComputeCRC32(chunk), bitwiseCRC32(chunk); got != want {
			t.Errorf("CRC32(0x%02X) = 0x%08X, the bit loop gives 0x%08X", value, got, want)
		}
	}
}

// TestTableMatchesBitwiseForEveryBytePair checks all 65536 two-byte messages,
// which is what exercises the table entry chosen from a register that already
// holds a value rather than from its preset.
func TestTableMatchesBitwiseForEveryBytePair(t *testing.T) {
	var chunk [2]byte

	for first := 0; first < 256; first++ {
		for second := 0; second < 256; second++ {
			chunk[0], chunk[1] = byte(first), byte(second)

			if got, want := crc.ComputeCRC16(chunk[:]), bitwiseCRC16(chunk[:]); got != want {
				t.Fatalf("CRC16(0x%02X 0x%02X) = 0x%04X, the bit loop gives 0x%04X",
					first, second, got, want)
			}
			if got, want := crc.ComputeCRC32(chunk[:]), bitwiseCRC32(chunk[:]); got != want {
				t.Fatalf("CRC32(0x%02X 0x%02X) = 0x%08X, the bit loop gives 0x%08X",
					first, second, got, want)
			}
		}
	}
}
