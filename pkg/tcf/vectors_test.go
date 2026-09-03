package tcf_test

import (
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/tcf"
)

// The wire vectors for this package live in vectors/tcf/. They pin the
// P-field and T-field packing from explicit field values.
//
// They deliberately stop short of the time.Time conversion path: Level 1
// CUC adds the TAI-UTC offset in effect at the given instant, so a vector
// derived that way would pin astro's leap-second table rather than the
// standard's layout. The conversion stays covered by the Go tests in this
// package until the table itself gets the scrutiny it deserves.

func TestTimeCodeVectors(t *testing.T) {
	vectors.RunFile(t, "tcf/timecode.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			// A P-field vector carries no code selector.
			if !f.Has("code") {
				ext, err := f.Bool("extension")
				if err != nil {
					return nil, err
				}
				id, err := f.Uint("time_code_id")
				if err != nil {
					return nil, err
				}
				detail, err := f.Uint("detail")
				if err != nil {
					return nil, err
				}
				extDetail, err := f.UintOr("ext_detail", 0)
				if err != nil {
					return nil, err
				}
				p := tcf.PField{
					Extension:  ext,
					TimeCodeID: uint8(id),
					Detail:     uint8(detail),
					ExtDetail:  uint8(extDetail),
				}
				return p.Encode()
			}

			code, err := f.Str("code")
			if err != nil {
				return nil, err
			}
			switch code {
			case "cuc":
				coarse, err := f.Uint("coarse_time")
				if err != nil {
					return nil, err
				}
				cb, err := f.Uint("coarse_bytes")
				if err != nil {
					return nil, err
				}
				fine, err := f.UintOr("fine_time", 0)
				if err != nil {
					return nil, err
				}
				fb, err := f.UintOr("fine_bytes", 0)
				if err != nil {
					return nil, err
				}
				c := tcf.CUC{
					CoarseTime:  coarse,
					FineTime:    fine,
					CoarseBytes: uint8(cb),
					FineBytes:   uint8(fb),
					Epoch:       tcf.CCSDSEpoch,
				}
				return c.EncodeTField()

			case "cds":
				day, err := f.Uint("day")
				if err != nil {
					return nil, err
				}
				db, err := f.Uint("day_bytes")
				if err != nil {
					return nil, err
				}
				ms, err := f.Uint("milliseconds")
				if err != nil {
					return nil, err
				}
				subms, err := f.UintOr("submilliseconds", 0)
				if err != nil {
					return nil, err
				}
				sb, err := f.UintOr("subms_bytes", 0)
				if err != nil {
					return nil, err
				}
				c := tcf.CDS{
					Day:             uint32(day),
					Milliseconds:    uint32(ms),
					Submilliseconds: uint32(subms),
					DayBytes:        uint8(db),
					SubmsBytes:      uint8(sb),
					Epoch:           tcf.CCSDSEpoch,
				}
				return c.EncodeTField()

			default:
				return nil, fmt.Errorf("unknown time code %q", code)
			}
		},

		DecodeFn: func(input []byte, _ vectors.Fields) (vectors.Fields, error) {
			// A two-octet input with the extension flag set is a P-field on
			// its own; anything longer is a full CUC.
			if len(input) == 2 && input[0]&0x80 != 0 {
				var p tcf.PField
				if err := p.Decode(input); err != nil {
					return nil, err
				}
				return vectors.Fields{
					"extension":    p.Extension,
					"time_code_id": p.TimeCodeID,
					"detail":       p.Detail,
					"ext_detail":   p.ExtDetail,
				}, nil
			}
			c, err := tcf.DecodeCUC(input, tcf.CCSDSEpoch)
			if err != nil {
				return nil, err
			}
			return vectors.Fields{
				"coarse_time":  c.CoarseTime,
				"coarse_bytes": c.CoarseBytes,
				"fine_time":    c.FineTime,
				"fine_bytes":   c.FineBytes,
			}, nil
		},
	})
}
