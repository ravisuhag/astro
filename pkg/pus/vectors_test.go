package pus_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/pus"
)

// The wire vectors for this package live in vectors/pus/. They cover the
// telecommand secondary header; the service state machines belong to plan
// 032.
//
// The PUS wire format is not self-describing: the spare width is a
// per-application-process declaration, so a decoder must already hold the
// mission profile. That is why the profile arrives in config rather than
// fields, and why tc-with-spare-inverse would leave an octet unconsumed if
// read under the wrong profile.

func profileFrom(config vectors.Fields) (pus.MissionProfile, error) {
	p := pus.DefaultProfile()
	spare, err := config.UintOr("tc_spare_bytes", 0)
	if err != nil {
		return p, err
	}
	p.TCSpareBytes = int(spare)
	return p, nil
}

func tcHeaderFrom(f vectors.Fields, config vectors.Fields) (*pus.TCHeader, error) {
	p, err := profileFrom(config)
	if err != nil {
		return nil, err
	}
	ack, err := f.Uint("ack_flags")
	if err != nil {
		return nil, err
	}
	service, err := f.Uint("service")
	if err != nil {
		return nil, err
	}
	subtype, err := f.Uint("subtype")
	if err != nil {
		return nil, err
	}
	source, err := f.Uint("source_id")
	if err != nil {
		return nil, err
	}
	return p.NewTCHeader(uint8(service), uint8(subtype), uint16(source),
		pus.AckFlags(ack)), nil
}

// TestTCHeaderInteropVectors runs headers captured from spacepackets, a Python
// implementation written from ECSS-E-ST-70-41C by other authors.
//
// This is the most valuable check in the package. The standard is behind
// registration, so every other pus vector rests on a reading of clauses this
// project cannot publish — and its ten citations are the only ones in the
// corpus never audited against the document, for the same reason. An
// independent implementation agreeing on the octets is the strongest
// corroboration available here.
func TestTCHeaderInteropVectors(t *testing.T) {
	vectors.RunFile(t, "pus/interop.json", vectors.Impl{
		EncodeFn: func(f, config vectors.Fields) ([]byte, error) {
			h, err := tcHeaderFrom(f, config)
			if err != nil {
				return nil, err
			}
			return h.Encode()
		},
	})
}

func TestTCHeaderVectors(t *testing.T) {
	vectors.RunFile(t, "pus/tc-header.json", vectors.Impl{
		EncodeFn: func(f, config vectors.Fields) ([]byte, error) {
			h, err := tcHeaderFrom(f, config)
			if err != nil {
				return nil, err
			}
			return h.Encode()
		},

		ConstructFn: func(f, config vectors.Fields) error {
			h, err := tcHeaderFrom(f, config)
			if err != nil {
				return err
			}
			_, err = h.Encode()
			return err
		},

		DecodeFn: func(input []byte, config vectors.Fields) (vectors.Fields, error) {
			p, err := profileFrom(config)
			if err != nil {
				return nil, err
			}
			h := &pus.TCHeader{Profile: p}
			if err := h.Decode(input); err != nil {
				return nil, err
			}
			return vectors.Fields{
				"ack_flags": uint8(h.AckFlags),
				"service":   h.Service,
				"subtype":   h.Subtype,
				"source_id": h.SourceID,
			}, nil
		},
	})
}
