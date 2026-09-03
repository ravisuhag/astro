package tmdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/tmdl"
)

// The wire vectors for this package live in vectors/tmdl/.
//
// Each vector isolates one primary header field, so a packing error names
// the field rather than hiding in a whole frame. A length field written
// one octet short and read back the same way passes every round trip while
// a conforming receiver misparses every frame; pinned octets catch that,
// round trips never will.

func headerFrom(f vectors.Fields) (tmdl.PrimaryHeader, error) {
	var h tmdl.PrimaryHeader

	version, err := f.UintOr("version_number", 0)
	if err != nil {
		return h, err
	}
	scid, err := f.Uint("spacecraft_id")
	if err != nil {
		return h, err
	}
	vcid, err := f.Uint("virtual_channel_id")
	if err != nil {
		return h, err
	}
	ocf, err := f.BoolOr("ocf_flag", false)
	if err != nil {
		return h, err
	}
	mcfc, err := f.UintOr("mc_frame_count", 0)
	if err != nil {
		return h, err
	}
	vcfc, err := f.UintOr("vc_frame_count", 0)
	if err != nil {
		return h, err
	}
	fsh, err := f.BoolOr("fsh_flag", false)
	if err != nil {
		return h, err
	}
	sync, err := f.BoolOr("sync_flag", false)
	if err != nil {
		return h, err
	}
	order, err := f.BoolOr("packet_order_flag", false)
	if err != nil {
		return h, err
	}
	seglen, err := f.UintOr("segment_length_id", 3)
	if err != nil {
		return h, err
	}
	fhp, err := f.UintOr("first_header_pointer", 0)
	if err != nil {
		return h, err
	}

	h.VersionNumber = uint8(version)
	h.SpacecraftID = uint16(scid)
	h.VirtualChannelID = uint8(vcid)
	h.OCFFlag = ocf
	h.MCFrameCount = uint8(mcfc)
	h.VCFrameCount = uint8(vcfc)
	h.FSHFlag = fsh
	h.SyncFlag = sync
	h.PacketOrderFlag = order
	h.SegmentLengthID = uint8(seglen)
	h.FirstHeaderPtr = uint16(fhp)
	return h, nil
}

func TestPrimaryHeaderVectors(t *testing.T) {
	vectors.RunFile(t, "tmdl/header.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			h, err := headerFrom(f)
			if err != nil {
				return nil, err
			}
			return h.Encode()
		},

		ConstructFn: func(f, _ vectors.Fields) error {
			h, err := headerFrom(f)
			if err != nil {
				return err
			}
			// Range rules are enforced on the way out, so the reject has
			// to reach Encode rather than stopping at the struct.
			if err := h.Validate(); err != nil {
				return err
			}
			_, err = h.Encode()
			return err
		},

		DecodeFn: func(input []byte, _ vectors.Fields) (vectors.Fields, error) {
			var h tmdl.PrimaryHeader
			if err := h.Decode(input); err != nil {
				return nil, err
			}
			return vectors.Fields{
				"version_number":       h.VersionNumber,
				"spacecraft_id":        h.SpacecraftID,
				"virtual_channel_id":   h.VirtualChannelID,
				"ocf_flag":             h.OCFFlag,
				"mc_frame_count":       h.MCFrameCount,
				"vc_frame_count":       h.VCFrameCount,
				"fsh_flag":             h.FSHFlag,
				"sync_flag":            h.SyncFlag,
				"packet_order_flag":    h.PacketOrderFlag,
				"segment_length_id":    h.SegmentLengthID,
				"first_header_pointer": h.FirstHeaderPtr,
			}, nil
		},
	})
}
