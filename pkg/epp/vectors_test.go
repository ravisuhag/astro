package epp_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/epp"
)

// The wire vectors for this package live in vectors/epp/. All four header
// sizes are pinned, because the size is a pure function of the 2-bit
// Length of Length field and getting that mapping wrong silently shifts
// every field after it.

func headerFrom(f vectors.Fields) (epp.Header, error) {
	var h epp.Header

	pvn, err := f.UintOr("pvn", uint64(epp.PVN))
	if err != nil {
		return h, err
	}
	pid, err := f.Uint("protocol_id")
	if err != nil {
		return h, err
	}
	lol, err := f.Uint("length_of_length")
	if err != nil {
		return h, err
	}
	udf, err := f.UintOr("user_defined", 0)
	if err != nil {
		return h, err
	}
	pie, err := f.UintOr("extended_protocol_id", 0)
	if err != nil {
		return h, err
	}
	ccsds, err := f.UintOr("ccsds_defined", 0)
	if err != nil {
		return h, err
	}
	length, err := f.UintOr("packet_length", 0)
	if err != nil {
		return h, err
	}

	h.PVN = uint8(pvn)
	h.ProtocolID = uint8(pid)
	h.LengthOfLength = uint8(lol)
	h.UserDefined = uint8(udf)
	h.ExtendedProtocolID = uint8(pie)
	h.CCSDSDefined = uint16(ccsds)
	h.PacketLength = uint32(length)
	return h, nil
}

func TestHeaderVectors(t *testing.T) {
	vectors.RunFile(t, "epp/header.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			h, err := headerFrom(f)
			if err != nil {
				return nil, err
			}
			return h.Encode()
		},

		DecodeFn: func(input []byte, _ vectors.Fields) (vectors.Fields, error) {
			var h epp.Header
			if err := h.Decode(input); err != nil {
				return nil, err
			}
			return vectors.Fields{
				"pvn":                  h.PVN,
				"protocol_id":          h.ProtocolID,
				"length_of_length":     h.LengthOfLength,
				"user_defined":         h.UserDefined,
				"extended_protocol_id": h.ExtendedProtocolID,
				"ccsds_defined":        h.CCSDSDefined,
				"packet_length":        h.PacketLength,
			}, nil
		},
	})
}
