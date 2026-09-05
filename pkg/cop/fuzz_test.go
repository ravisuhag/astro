package cop_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/cop"
)

// FuzzCLCWDecode fuzzes the wire decoder for the four-octet Communications
// Link Control Word (CCSDS 232.0-B-4 clause 4.2). It is the operational
// control field's contents, so every octet reaches this decoder straight off
// the downlink.
func FuzzCLCWDecode(f *testing.F) {
	// Seed with a real, valid CLCW.
	seed := &cop.CLCW{COPInEffect: 1, VirtualChannelID: 5, ReportValue: 42}
	if encoded, err := seed.Encode(); err == nil {
		f.Add(encoded)
	}
	f.Add([]byte{0x01, 0x00, 0x00, 0x00})
	f.Add([]byte{})
	f.Add(make([]byte, 3))
	f.Add(make([]byte, 4))
	f.Add(make([]byte, 5))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. Errors are fine; a short buffer or a
		// reserved-bit violation is a malformed CLCW, not a crash.
		var c cop.CLCW
		_ = c.Decode(data)
	})
}

// FuzzFARMProcessFrame fuzzes FARM-1's frame acceptance path (CCSDS
// 232.1-B-2 clause 6.3): the bypass and control-command flags and the frame
// sequence number arrive in the TC transfer frame's primary header, and
// dataField is the whole frame data field an attacker's transmitter chose,
// so all four reach this call before FARM-1 has decided the frame is even
// well-formed.
func FuzzFARMProcessFrame(f *testing.F) {
	f.Add(uint8(0), uint8(0), uint8(0), []byte{})
	f.Add(uint8(0), uint8(0), uint8(1), []byte{0x01, 0x02, 0x03, 0x04})
	f.Add(uint8(1), uint8(0), uint8(0), []byte{})
	f.Add(uint8(0), uint8(1), uint8(0), []byte{0x00})
	f.Add(uint8(0), uint8(1), uint8(0), []byte{0x00, 0x00, 0x00})
	f.Add(uint8(0), uint8(1), uint8(0), []byte{0x82, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, bypassFlag, controlCommandFlag, frameSeqNum uint8, dataField []byte) {
		// Property: never panic, regardless of the FARM-1 state a sequence of
		// arbitrary frames leaves the machine in. A fresh machine per call
		// keeps one panicking input from being masked by state a previous
		// input left behind.
		farm := cop.NewFARM(0, 8)
		_, _ = farm.ProcessFrame(bypassFlag, controlCommandFlag, frameSeqNum, dataField)
		// Also drive a few more frames through the same machine, since a
		// FARM-1 defect is as likely to be in a state transition as in a
		// single frame's decode.
		for i := 0; i < 4; i++ {
			_, _ = farm.ProcessFrame(bypassFlag, controlCommandFlag, frameSeqNum+uint8(i), dataField)
		}
	})
}
