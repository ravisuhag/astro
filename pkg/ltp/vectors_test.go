package ltp_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/ltp"
)

// The wire vectors for this package live in vectors/ltp/. They cover the
// segment header only; the session and receiver machines need a sequence
// of calls, which no vector kind expresses.
//
// The header is variable length because the session ID is SDNV-encoded, so
// the extension-count octet is not at a fixed offset. The
// engine-id-crossing-the-sdnv-boundary vector is the one that catches a
// decoder assuming otherwise.

func TestSegmentHeaderVectors(t *testing.T) {
	vectors.RunFile(t, "ltp/header.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			stype, err := f.Uint("segment_type")
			if err != nil {
				return nil, err
			}
			engine, err := f.Uint("engine_id")
			if err != nil {
				return nil, err
			}
			session, err := f.Uint("session_number")
			if err != nil {
				return nil, err
			}
			h := ltp.Header{
				Type: ltp.SegmentType(stype),
				SessionID: ltp.SessionID{
					EngineID:      engine,
					SessionNumber: session,
				},
			}
			return h.Encode()
		},

		DecodeFn: func(input []byte, _ vectors.Fields) (vectors.Fields, error) {
			h, consumed, _, err := ltp.DecodeHeader(input)
			if err != nil {
				return nil, err
			}
			return vectors.Fields{
				"segment_type":   uint8(h.Type),
				"engine_id":      h.SessionID.EngineID,
				"session_number": h.SessionID.SessionNumber,
				"consumed":       consumed,
			}, nil
		},
	})
}
