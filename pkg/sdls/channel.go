package sdls

// NoMAP marks a ChannelID whose frame type has no MAP multiplexing, or a
// USLP/TC channel where the MAP ID is not part of the binding.
const NoMAP = -1

// ChannelID names one channel an SA can be bound to: a Global Virtual
// Channel Identifier (Transfer Frame Version Number, Spacecraft ID, Virtual
// Channel ID), optionally extended to a Global MAP ID with the MAP ID of a
// TC or USLP frame.
//
// §4.2.2.2 requires a Security Association to apply to an agreed set of
// GVCIDs or GMAP_IDs, and §4.2.4.3 requires the receiving end to verify that
// the SA named by the SPI is in fact the one for the channel the frame
// arrived on. List the agreed channels in SecurityAssociation.Channels and
// call ProcessSecurityForChannel to have that check enforced.
type ChannelID struct {
	// TFVN is the Transfer Frame Version Number: 0 for TM, 1 for TC and AOS
	// (each per its own numbering), 12 (0b1100) for USLP.
	TFVN uint8

	// SCID is the Spacecraft Identifier.
	SCID uint16

	// VCID is the Virtual Channel Identifier.
	VCID uint8

	// MAPID extends the GVCID to a GMAP_ID for TC and USLP. Set it to NoMAP
	// when the frames have no MAP, or when the binding stops at the virtual
	// channel. The zero value is MAP 0, a real MAP: fill this field
	// explicitly.
	MAPID int
}

// servesChannel reports whether ch is in the SA's agreed channel set. An
// empty set declares no binding, so every channel matches.
func (sa *SecurityAssociation) servesChannel(ch ChannelID) bool {
	if len(sa.Channels) == 0 {
		return true
	}
	for _, bound := range sa.Channels {
		if bound == ch {
			return true
		}
	}
	return false
}
