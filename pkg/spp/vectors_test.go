package spp_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	spp "github.com/ravisuhag/astro/pkg/spp"
)

// The wire vectors for this package live in vectors/spp/. They pin the
// clause 4.1.3 bit layout one field at a time, so a packing error names
// the field instead of hiding inside a whole packet.

// headerFrom builds a PrimaryHeader from a vector's fields. Absent fields
// take the value the standard's common case uses, which each vector's note
// states where it matters.
func headerFrom(f vectors.Fields) (spp.PrimaryHeader, error) {
	var h spp.PrimaryHeader

	version, err := f.UintOr("version", 0)
	if err != nil {
		return h, err
	}
	ptype, err := f.UintOr("packet_type", 0)
	if err != nil {
		return h, err
	}
	shf, err := f.UintOr("secondary_header_flag", 0)
	if err != nil {
		return h, err
	}
	apid, err := f.Uint("apid")
	if err != nil {
		return h, err
	}
	flags, err := f.UintOr("sequence_flags", uint64(spp.SeqFlagUnsegmented))
	if err != nil {
		return h, err
	}
	count, err := f.UintOr("sequence_count", 0)
	if err != nil {
		return h, err
	}
	length, err := f.UintOr("packet_length", 0)
	if err != nil {
		return h, err
	}

	h.Version = uint8(version)
	h.Type = uint8(ptype)
	h.SecondaryHeaderFlag = uint8(shf)
	h.APID = uint16(apid)
	h.SequenceFlags = uint8(flags)
	h.SequenceCount = uint16(count)
	h.PacketLength = uint16(length)
	return h, nil
}

func TestHeaderVectors(t *testing.T) {
	vectors.RunFile(t, "spp/header.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			h, err := headerFrom(f)
			if err != nil {
				return nil, err
			}
			return h.Encode()
		},

		DecodeFn: func(input []byte, _ vectors.Fields) (vectors.Fields, error) {
			var h spp.PrimaryHeader
			if err := h.Decode(input); err != nil {
				return nil, err
			}
			return vectors.Fields{
				"version":               h.Version,
				"packet_type":           h.Type,
				"secondary_header_flag": h.SecondaryHeaderFlag,
				"apid":                  h.APID,
				"sequence_flags":        h.SequenceFlags,
				"sequence_count":        h.SequenceCount,
				"packet_length":         h.PacketLength,
			}, nil
		},
	})
}

// buildPacket constructs a SpacePacket from a vector's fields and config.
func buildPacket(f, config vectors.Fields) (*spp.SpacePacket, error) {
	apid, err := f.Uint("apid")
	if err != nil {
		return nil, err
	}
	ptype, err := f.UintOr("packet_type", uint64(spp.PacketTypeTM))
	if err != nil {
		return nil, err
	}
	data, err := f.HexOr("data", nil)
	if err != nil {
		return nil, err
	}

	var opts []spp.PacketOption
	if f.Has("sequence_count") {
		n, err := f.Uint("sequence_count")
		if err != nil {
			return nil, err
		}
		opts = append(opts, spp.WithSequenceCount(uint16(n)))
	}
	if f.Has("sequence_flags") {
		flags, err := f.Uint("sequence_flags")
		if err != nil {
			return nil, err
		}
		opts = append(opts, spp.WithSequenceFlags(uint8(flags)))
	}
	// The error control field is a mission extension carried inside the
	// data field, so its presence is channel agreement, not wire content.
	if on, err := config.BoolOr("error_control", false); err != nil {
		return nil, err
	} else if on {
		opts = append(opts, spp.WithErrorControl())
	}

	return spp.NewSpacePacket(uint16(apid), uint8(ptype), data, opts...)
}

func TestPacketVectors(t *testing.T) {
	vectors.RunFile(t, "spp/packet.json", vectors.Impl{
		EncodeFn: func(f, config vectors.Fields) ([]byte, error) {
			pkt, err := buildPacket(f, config)
			if err != nil {
				return nil, err
			}
			return pkt.Encode()
		},

		ConstructFn: func(f, config vectors.Fields) error {
			pkt, err := buildPacket(f, config)
			if err != nil {
				return err
			}
			// A range rule the constructor accepts must still be caught
			// before the octets go out, so the reject covers both.
			_, err = pkt.Encode()
			return err
		},

		DecodeFn: func(input []byte, config vectors.Fields) (vectors.Fields, error) {
			var opts []spp.DecodeOption
			if on, err := config.BoolOr("error_control", false); err != nil {
				return nil, err
			} else if on {
				opts = append(opts, spp.WithDecodeErrorControl())
			}
			pkt, err := spp.Decode(input, opts...)
			if err != nil {
				return nil, err
			}
			h := pkt.PrimaryHeader
			return vectors.Fields{
				"version":               h.Version,
				"packet_type":           h.Type,
				"secondary_header_flag": h.SecondaryHeaderFlag,
				"apid":                  h.APID,
				"sequence_flags":        h.SequenceFlags,
				"sequence_count":        h.SequenceCount,
				"packet_length":         h.PacketLength,
				"data":                  pkt.UserData,
			}, nil
		},
	})
}
