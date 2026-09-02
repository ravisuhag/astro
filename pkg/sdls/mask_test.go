package sdls_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/pkg/sdls"
)

// TestBaselineAuthMaskLayouts pins the exact bytes of each per-frame-type
// constructor: ones everywhere except the clause 4.2.2.6.2 mandatory exclusions.
func TestBaselineAuthMaskLayouts(t *testing.T) {
	gcmFL := sdls.FieldLengths{IV: sdls.GCMIVSize, MAC: 16} // 14-octet header
	tcFL := sdls.FieldLengths{SeqNum: 4, MAC: 16}           // 6-octet header

	ones := func(n int) []byte { return bytes.Repeat([]byte{0xFF}, n) }
	zeros := func(n int) []byte { return make([]byte, n) }
	cat := func(parts ...[]byte) []byte {
		var out []byte
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	// The security header portion under the GCM layout: SPI covered, IV not.
	gcmSecHeader := cat(ones(sdls.SPISize), zeros(sdls.GCMIVSize))

	tests := []struct {
		name string
		got  []byte
		want []byte
	}{
		{
			// TM: 6-octet primary header, MCFC at octet 2 excluded.
			name: "TM without secondary header",
			got:  sdls.BaselineAuthMaskTM(0, gcmFL),
			want: cat(ones(2), zeros(1), ones(3), gcmSecHeader),
		},
		{
			// TM secondary header octets stay covered.
			name: "TM with 4-octet secondary header",
			got:  sdls.BaselineAuthMaskTM(4, gcmFL),
			want: cat(ones(2), zeros(1), ones(3), ones(4), gcmSecHeader),
		},
		{
			// TC: nothing mandatorily excluded; clause E2 layout has no IV.
			name: "TC without segment header",
			got:  sdls.BaselineAuthMaskTC(false, tcFL),
			want: cat(ones(5), ones(tcFL.HeaderSize())),
		},
		{
			// TC segment header is covered per clause 4.2.2.6.2.
			name: "TC with segment header",
			got:  sdls.BaselineAuthMaskTC(true, tcFL),
			want: cat(ones(6), ones(tcFL.HeaderSize())),
		},
		{
			// AOS: FHEC (2 octets after the 6-octet header) and insert zone excluded.
			name: "AOS with FHEC and 4-octet insert zone",
			got:  sdls.BaselineAuthMaskAOS(true, 4, gcmFL),
			want: cat(ones(6), zeros(2), zeros(4), gcmSecHeader),
		},
		{
			name: "AOS bare",
			got:  sdls.BaselineAuthMaskAOS(false, 0, gcmFL),
			want: cat(ones(6), gcmSecHeader),
		},
		{
			// USLP: insert zone excluded, variable primary header covered.
			name: "USLP with 7-octet header and 2-octet insert zone",
			got:  sdls.BaselineAuthMaskUSLP(7, 2, gcmFL),
			want: cat(ones(7), zeros(2), gcmSecHeader),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !bytes.Equal(tt.got, tt.want) {
				t.Errorf("mask\n got  %x\n want %x", tt.got, tt.want)
			}
		})
	}
}

// TestBaselineMaskTMExcludesMCFC proves the exclusion works end to end: a TM
// Master Channel Frame Count rewritten between sender and receiver must not
// break the MAC, while a change to any covered octet must.
func TestBaselineMaskTMExcludesMCFC(t *testing.T) {
	fl := sdls.FieldLengths{IV: sdls.GCMIVSize, MAC: 16}
	mask := sdls.BaselineAuthMaskTM(0, fl)

	tx := newTestSA(t, sdls.AuthenticatedEncryption)
	tx.AuthMask = mask
	rx := newTestSA(t, sdls.AuthenticatedEncryption)
	rx.AuthMask = mask

	// A six-octet TM primary header.
	frameHeader := []byte{0x02, 0x3E, 0x00, 0x07, 0x18, 0x00}
	protected, err := tx.ApplySecurity(frameHeader, []byte("telemetry"))
	if err != nil {
		t.Fatal(err)
	}

	// The multiplexer rewrites the MCFC (octet 2) downstream of security.
	remuxed := append([]byte(nil), frameHeader...)
	remuxed[2] = 0xA7
	if _, _, err := sdls.ProcessSecurity(protected, remuxed, sdls.StaticLookup(rx)); err != nil {
		t.Errorf("a rewritten MCFC broke verification: %v", err)
	}

	// Every other header octet stays covered.
	for i := range frameHeader {
		if i == 2 {
			continue
		}
		bad := append([]byte(nil), frameHeader...)
		bad[i] ^= 0x01
		rx2 := newTestSA(t, sdls.AuthenticatedEncryption)
		rx2.AuthMask = mask
		if _, _, err := sdls.ProcessSecurity(protected, bad, sdls.StaticLookup(rx2)); err == nil {
			t.Errorf("a change to covered header octet %d went undetected", i)
		}
	}
}

// TestBaselineMaskAOSExcludesFHECAndInsertZone proves the AOS exclusions end
// to end: the FHEC and the insert zone may change in flight, the rest of the
// header may not.
func TestBaselineMaskAOSExcludesFHECAndInsertZone(t *testing.T) {
	const insertZone = 4
	fl := sdls.FieldLengths{IV: sdls.GCMIVSize, MAC: 16}
	mask := sdls.BaselineAuthMaskAOS(true, insertZone, fl)

	tx := newTestSA(t, sdls.AuthenticatedEncryption)
	tx.AuthMask = mask
	rx := newTestSA(t, sdls.AuthenticatedEncryption)
	rx.AuthMask = mask

	// 6-octet primary header, 2-octet FHEC, 4-octet insert zone.
	frameHeader := []byte{0x40, 0x2A, 0x07, 0x00, 0x00, 0x00, 0xBE, 0xEF, 0x11, 0x22, 0x33, 0x44}
	protected, err := tx.ApplySecurity(frameHeader, []byte("aos data"))
	if err != nil {
		t.Fatal(err)
	}

	for i := range frameHeader {
		changed := append([]byte(nil), frameHeader...)
		changed[i] ^= 0xFF

		rx2 := newTestSA(t, sdls.AuthenticatedEncryption)
		rx2.AuthMask = mask
		_, _, err := sdls.ProcessSecurity(protected, changed, sdls.StaticLookup(rx2))

		excluded := i >= 6 // FHEC and insert zone octets
		if excluded && err != nil {
			t.Errorf("octet %d is excluded but its change broke verification: %v", i, err)
		}
		if !excluded && err == nil {
			t.Errorf("a change to covered header octet %d went undetected", i)
		}
	}
}

// TestBaselineMaskUSLPExcludesInsertZone proves the USLP insert zone
// exclusion end to end.
func TestBaselineMaskUSLPExcludesInsertZone(t *testing.T) {
	const headerLen, insertZone = 7, 3
	fl := sdls.FieldLengths{IV: sdls.GCMIVSize, MAC: 16}
	mask := sdls.BaselineAuthMaskUSLP(headerLen, insertZone, fl)

	tx := newTestSA(t, sdls.AuthenticatedEncryption)
	tx.AuthMask = mask
	rx := newTestSA(t, sdls.AuthenticatedEncryption)
	rx.AuthMask = mask

	frameHeader := []byte{0xC0, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0xAA, 0xBB, 0xCC}
	protected, err := tx.ApplySecurity(frameHeader, []byte("uslp data"))
	if err != nil {
		t.Fatal(err)
	}

	// Insert zone octets may change freely.
	changed := append([]byte(nil), frameHeader...)
	changed[headerLen] ^= 0xFF
	changed[headerLen+2] ^= 0xFF
	if _, _, err := sdls.ProcessSecurity(protected, changed, sdls.StaticLookup(rx)); err != nil {
		t.Errorf("an insert zone change broke verification: %v", err)
	}

	// Primary header octets may not.
	bad := append([]byte(nil), frameHeader...)
	bad[3] ^= 0x01
	rx2 := newTestSA(t, sdls.AuthenticatedEncryption)
	rx2.AuthMask = mask
	if _, _, err := sdls.ProcessSecurity(protected, bad, sdls.StaticLookup(rx2)); err == nil {
		t.Error("a change to a covered USLP header octet went undetected")
	}
}

// TestBaselineMaskTCCoversWholeHeader proves the TC mask covers the primary
// and segment headers: any header change must break the MAC.
func TestBaselineMaskTCCoversWholeHeader(t *testing.T) {
	fl := sdls.FieldLengths{SeqNum: 4, MAC: 16}
	mask := sdls.BaselineAuthMaskTC(true, fl)

	tx := newCMACSA(t)
	tx.AuthMask = mask

	frameHeader := []byte{0x20, 0x00, 0x00, 0x0A, 0x00, 0xC0} // 5 + segment header
	protected, err := tx.ApplySecurity(frameHeader, []byte("telecommand"))
	if err != nil {
		t.Fatal(err)
	}

	for i := range frameHeader {
		bad := append([]byte(nil), frameHeader...)
		bad[i] ^= 0x01
		rx := newCMACSA(t)
		rx.AuthMask = mask
		if _, _, err := sdls.ProcessSecurity(protected, bad, sdls.StaticLookup(rx)); err == nil {
			t.Errorf("a change to TC header octet %d went undetected", i)
		}
	}
}
