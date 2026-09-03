package cop_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/cop"
)

// The wire vectors for this package live in vectors/cop/. They cover the
// CLCW only. FOP-1 and FARM-1 are state machines, and a single
// input/output pair cannot express a sequence.
//
// Every field narrower than the type that carries it has a reject vector.
// Encode packs by shifting, so an unchecked out-of-range value would be
// substituted rather than refused — a VCID of 64 would go out as 0, and a
// CLCW naming the wrong virtual channel is worse than no CLCW.

func clcwFrom(f vectors.Fields) (cop.CLCW, error) {
	var c cop.CLCW

	status, err := f.UintOr("status_field", 0)
	if err != nil {
		return c, err
	}
	inEffect, err := f.UintOr("cop_in_effect", 1)
	if err != nil {
		return c, err
	}
	vcid, err := f.Uint("virtual_channel_id")
	if err != nil {
		return c, err
	}
	norf, err := f.BoolOr("no_rf_available_flag", false)
	if err != nil {
		return c, err
	}
	nobl, err := f.BoolOr("no_bit_lock_flag", false)
	if err != nil {
		return c, err
	}
	lockout, err := f.BoolOr("lockout_flag", false)
	if err != nil {
		return c, err
	}
	wait, err := f.BoolOr("wait_flag", false)
	if err != nil {
		return c, err
	}
	retx, err := f.BoolOr("retransmit_flag", false)
	if err != nil {
		return c, err
	}
	farmb, err := f.UintOr("farm_b_counter", 0)
	if err != nil {
		return c, err
	}
	vr, err := f.UintOr("report_value", 0)
	if err != nil {
		return c, err
	}

	c.StatusField = uint8(status)
	c.COPInEffect = uint8(inEffect)
	c.VirtualChannelID = uint8(vcid)
	c.NoRFAvailableFlag = norf
	c.NoBitLockFlag = nobl
	c.LockoutFlag = lockout
	c.WaitFlag = wait
	c.RetransmitFlag = retx
	c.FARMBCounter = uint8(farmb)
	c.ReportValue = uint8(vr)
	return c, nil
}

func TestCLCWVectors(t *testing.T) {
	vectors.RunFile(t, "cop/clcw.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			c, err := clcwFrom(f)
			if err != nil {
				return nil, err
			}
			return c.Encode()
		},

		ConstructFn: func(f, _ vectors.Fields) error {
			c, err := clcwFrom(f)
			if err != nil {
				return err
			}
			_, err = c.Encode()
			return err
		},

		DecodeFn: func(input []byte, _ vectors.Fields) (vectors.Fields, error) {
			var c cop.CLCW
			if err := c.Decode(input); err != nil {
				return nil, err
			}
			return vectors.Fields{
				"control_word_type":    c.ControlWordType,
				"version":              c.Version,
				"status_field":         c.StatusField,
				"cop_in_effect":        c.COPInEffect,
				"virtual_channel_id":   c.VirtualChannelID,
				"no_rf_available_flag": c.NoRFAvailableFlag,
				"no_bit_lock_flag":     c.NoBitLockFlag,
				"lockout_flag":         c.LockoutFlag,
				"wait_flag":            c.WaitFlag,
				"retransmit_flag":      c.RetransmitFlag,
				"farm_b_counter":       c.FARMBCounter,
				"report_value":         c.ReportValue,
			}, nil
		},
	})
}
