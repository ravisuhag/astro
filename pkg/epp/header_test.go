package epp_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/epp"
)

func TestHeaderSizeFromLoL(t *testing.T) {
	tests := []struct {
		name string
		lol  uint8
		want int
	}{
		{"LoL 00 -> 1 octet", epp.LoLNone, 1},
		{"LoL 01 -> 2 octets", epp.LoL1Octet, 2},
		{"LoL 10 -> 4 octets", epp.LoL2Octet, 4},
		{"LoL 11 -> 8 octets", epp.LoL4Octet, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := epp.Header{PVN: epp.PVN, LengthOfLength: tt.lol}
			if got := h.Size(); got != tt.want {
				t.Errorf("Size() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHeaderDecodeInvalidPVN(t *testing.T) {
	// The pre-rewrite (wrong) layout put PVN in the top nibble, emitting
	// first bytes 0x70-0x7F. Those must now be rejected: bits 0-2 are '011'.
	for _, b := range []byte{0x00, 0x70, 0x74, 0x7F} {
		var h epp.Header
		if err := h.Decode([]byte{b, 0x00}); !errors.Is(err, epp.ErrInvalidPVN) {
			t.Errorf("Decode(0x%02X) = %v, want ErrInvalidPVN", b, err)
		}
	}
}

func TestHeaderDecodeNonIdleOneOctet(t *testing.T) {
	// PVN='111', PID='001' (LTP), LoL='00', forbidden by 4.1.2.4.4.
	var h epp.Header
	if err := h.Decode([]byte{0xE4}); !errors.Is(err, epp.ErrNonIdleOneOctetHeader) {
		t.Errorf("Decode(0xE4) = %v, want ErrNonIdleOneOctetHeader", err)
	}
}

func TestHeaderDecodeTooShort(t *testing.T) {
	var h epp.Header
	if err := h.Decode(nil); !errors.Is(err, epp.ErrDataTooShort) {
		t.Errorf("Expected ErrDataTooShort, got %v", err)
	}
	if err := h.Decode([]byte{}); !errors.Is(err, epp.ErrDataTooShort) {
		t.Errorf("Expected ErrDataTooShort, got %v", err)
	}
}

func TestHeaderDecodeTooShortForLoL(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"2-octet header, 1 byte", []byte{0xE9}},
		{"4-octet header, 2 bytes", []byte{0xEA, 0x00}},
		{"8-octet header, 4 bytes", []byte{0xEB, 0x00, 0x00, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h epp.Header
			if err := h.Decode(tt.data); !errors.Is(err, epp.ErrDataTooShort) {
				t.Errorf("Decode = %v, want ErrDataTooShort", err)
			}
		})
	}
}

func TestHeaderValidateExtensionRules(t *testing.T) {
	// PID '110' needs a 4- or 8-octet header to carry the extension field.
	h := epp.Header{PVN: epp.PVN, ProtocolID: epp.ProtocolIDExtended, LengthOfLength: epp.LoL1Octet, PacketLength: 3}
	if err := h.Validate(); !errors.Is(err, epp.ErrExtendedNeedsLongHeader) {
		t.Errorf("Validate = %v, want ErrExtendedNeedsLongHeader", err)
	}

	// A non-'110' PID must keep the extension field at zero (4.1.2.6.3).
	h = epp.Header{
		PVN: epp.PVN, ProtocolID: epp.ProtocolIDIPE, LengthOfLength: epp.LoL2Octet,
		ExtendedProtocolID: 5, PacketLength: 6,
	}
	if err := h.Validate(); !errors.Is(err, epp.ErrExtensionMustBeZero) {
		t.Errorf("Validate = %v, want ErrExtensionMustBeZero", err)
	}
}

func TestHeaderValidateFieldRanges(t *testing.T) {
	h := epp.Header{PVN: epp.PVN, ProtocolID: epp.ProtocolIDIPE, LengthOfLength: epp.LoL2Octet, UserDefined: 16, PacketLength: 6}
	if err := h.Validate(); !errors.Is(err, epp.ErrInvalidUserDefined) {
		t.Errorf("Validate = %v, want ErrInvalidUserDefined", err)
	}

	h = epp.Header{PVN: epp.PVN, ProtocolID: epp.ProtocolIDExtended, LengthOfLength: epp.LoL2Octet, ExtendedProtocolID: 16, PacketLength: 6}
	if err := h.Validate(); !errors.Is(err, epp.ErrInvalidExtendedProtocolID) {
		t.Errorf("Validate = %v, want ErrInvalidExtendedProtocolID", err)
	}

	h = epp.Header{PVN: epp.PVN, ProtocolID: epp.ProtocolIDIPE, LengthOfLength: 4, PacketLength: 6}
	if err := h.Validate(); !errors.Is(err, epp.ErrInvalidLengthOfLength) {
		t.Errorf("Validate = %v, want ErrInvalidLengthOfLength", err)
	}

	// CCSDS Defined Field only exists in the 8-octet header.
	h = epp.Header{PVN: epp.PVN, ProtocolID: epp.ProtocolIDIPE, LengthOfLength: epp.LoL2Octet, CCSDSDefined: 1, PacketLength: 6}
	if err := h.Validate(); !errors.Is(err, epp.ErrFieldNeedsLongerHeader) {
		t.Errorf("Validate = %v, want ErrFieldNeedsLongerHeader", err)
	}
}

func TestHeaderValidateInvalidPVN(t *testing.T) {
	h := epp.Header{PVN: 0, ProtocolID: 0}
	if err := h.Validate(); !errors.Is(err, epp.ErrInvalidPVN) {
		t.Errorf("Expected ErrInvalidPVN, got %v", err)
	}
}

func TestHeaderSize(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int
	}{
		{"idle", []byte{0xE0}, 1},
		{"2-octet", []byte{0xE9}, 2},
		{"4-octet", []byte{0xEA}, 4},
		{"8-octet", []byte{0xFB}, 8},
		{"wrong PVN", []byte{0x70}, -1},
		{"empty", []byte{}, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := epp.HeaderSize(tt.data); got != tt.want {
				t.Errorf("HeaderSize() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHeaderHumanize(t *testing.T) {
	h := epp.Header{
		PVN:            epp.PVN,
		ProtocolID:     epp.ProtocolIDIPE,
		LengthOfLength: epp.LoL1Octet,
		PacketLength:   100,
	}
	s := h.Humanize()
	if s == "" {
		t.Error("Humanize returned empty string")
	}
}

// TestHeaderProtocolIDNames pins the name shown for every Protocol ID to the
// SANA "Protocol Identifier for Encapsulation Packet Protocol" registry, which
// 4.1.2.3.3 makes normative. The registry has eight records; only '101' has no
// entry, so only '101' is reported as reserved.
func TestHeaderProtocolIDNames(t *testing.T) {
	tests := []struct {
		pid  uint8
		want string
	}{
		{epp.ProtocolIDIdle, "Idle"},
		{epp.ProtocolIDLTP, "LTP"},
		{epp.ProtocolIDIPE, "Internet Protocol Extension"},
		{epp.ProtocolIDCFDP, "CFDP"},
		{epp.ProtocolIDBP, "Bundle Protocol"},
		{5, "Reserved"},
		{epp.ProtocolIDExtended, "Protocol ID Extension"},
		{epp.ProtocolIDMission, "Mission-Specific"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			h := epp.Header{
				PVN:            epp.PVN,
				ProtocolID:     tt.pid,
				LengthOfLength: epp.LoL1Octet,
				PacketLength:   3,
			}
			want := "Protocol ID: " + strconv.Itoa(int(tt.pid)) + " (" + tt.want + ")"
			if got := h.Humanize(); !strings.Contains(got, want) {
				t.Errorf("Humanize() for Protocol ID %d = %q, want it to contain %q", tt.pid, got, want)
			}
		})
	}
}
