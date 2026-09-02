package sdls_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/sdls"
)

// TestProcessSecurityForChannelBinding checks the clause 4.2.4.3 SA verification:
// an SA bound to a channel set accepts frames from those channels only.
func TestProcessSecurityForChannelBinding(t *testing.T) {
	bound := sdls.ChannelID{TFVN: 0, SCID: 0x2A, VCID: 3, MAPID: sdls.NoMAP}
	other := sdls.ChannelID{TFVN: 0, SCID: 0x2A, VCID: 5, MAPID: sdls.NoMAP}

	tx := newTestSA(t, sdls.AuthenticatedEncryption)
	frameHeader := []byte{0x02, 0x3E}
	protected, err := tx.ApplySecurity(frameHeader, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}

	newRX := func() *sdls.SecurityAssociation {
		rx := newTestSA(t, sdls.AuthenticatedEncryption)
		rx.Channels = []sdls.ChannelID{bound}
		return rx
	}

	// The agreed channel passes.
	if _, _, err := sdls.ProcessSecurityForChannel(protected, frameHeader, bound, sdls.StaticLookup(newRX())); err != nil {
		t.Errorf("bound channel rejected: %v", err)
	}

	// Any other channel is refused before cryptographic work, with no data.
	_, data, err := sdls.ProcessSecurityForChannel(protected, frameHeader, other, sdls.StaticLookup(newRX()))
	if !errors.Is(err, sdls.ErrSAChannelMismatch) {
		t.Errorf("error = %v, want ErrSAChannelMismatch", err)
	}
	if data != nil {
		t.Error("returned data for a channel the SA is not bound to")
	}

	// A MAP-level binding distinguishes MAPs on the same virtual channel.
	mapBound := bound
	mapBound.MAPID = 1
	rx := newTestSA(t, sdls.AuthenticatedEncryption)
	rx.Channels = []sdls.ChannelID{mapBound}
	if _, _, err := sdls.ProcessSecurityForChannel(protected, frameHeader, bound, sdls.StaticLookup(rx)); !errors.Is(err, sdls.ErrSAChannelMismatch) {
		t.Errorf("GVCID matched a GMAP_ID binding: error = %v, want ErrSAChannelMismatch", err)
	}
}

// TestProcessSecurityForChannelUnbound checks that an SA with no declared
// channel set behaves like plain ProcessSecurity: any channel is accepted.
func TestProcessSecurityForChannelUnbound(t *testing.T) {
	tx := newTestSA(t, sdls.AuthenticatedEncryption)
	rx := newTestSA(t, sdls.AuthenticatedEncryption)
	frameHeader := []byte{0x02, 0x3E}

	protected, err := tx.ApplySecurity(frameHeader, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}

	ch := sdls.ChannelID{TFVN: 12, SCID: 0xFFFF, VCID: 63, MAPID: 15}
	if _, _, err := sdls.ProcessSecurityForChannel(protected, frameHeader, ch, sdls.StaticLookup(rx)); err != nil {
		t.Errorf("an unbound SA rejected a channel: %v", err)
	}
}
