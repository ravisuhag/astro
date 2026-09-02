package pxdl

import "time"

// Managed parameters, per CCSDS 211.0-B-6 clause 4 and annex C.
//
// Proximity-1 keeps its per-session configuration in a Management
// Information Base. Most of its entries drive the MAC sublayer and COP-P,
// which this package does not implement; the ones represented here are the
// parameters the frame layer itself consults, plus the timing parameters a
// caller running the link needs somewhere to keep.

// ManagedParameters holds the MIB entries relevant to this layer, named after
// the annex C parameters they represent.
//
// The zero value is not useful; start from DefaultManagedParameters.
type ManagedParameters struct {
	// LocalSpacecraftID is Local_Spacecraft_ID: the SCID of this node. Used
	// to validate a received frame whose Source-or-Destination Identifier
	// says the SCID names the destination (clause 3.2.2.9.3).
	LocalSpacecraftID uint16

	// RemoteSpacecraftID is Remote_Spacecraft_ID: the SCID of the node at
	// the far end of the link (clause 3.2.2.9.3).
	RemoteSpacecraftID uint16

	// SendMaximumFrameLength is Maximum_Frame_Length for the frames this
	// node transmits, in octets. Annex C keys the parameter to the frame
	// version in use; for Version-3 frames it can be at most 2048.
	SendMaximumFrameLength int

	// ReceiveMaximumFrameLength is Maximum_Frame_Length for the frames this
	// node accepts. The two directions of a Proximity-1 link often run at
	// very different data rates, so the negotiated maxima can differ too.
	ReceiveMaximumFrameLength int

	// MaximumPacketSize is Maximum_Packet_Size: the largest packet the
	// segmentation and reassembly process handles, in octets (clause 4.4.2.1).
	MaximumPacketSize int

	// SynchTimeout is Synch_Timeout: how long the receiving end waits
	// without frame synchronization before declaring the link lost.
	SynchTimeout time.Duration

	// PLCWRepeatInterval is PLCW_Repeat_Interval: how often COP-P repeats
	// the current PLCW when nothing has changed (annex C).
	PLCWRepeatInterval time.Duration
}

// DefaultManagedParameters returns parameters with the Version-3 frame bounds
// and the package's reassembly default. Spacecraft IDs and timing have no
// meaningful defaults; they come from the mission.
func DefaultManagedParameters() ManagedParameters {
	return ManagedParameters{
		SendMaximumFrameLength:    MaxFrameSize,
		ReceiveMaximumFrameLength: MaxFrameSize,
		MaximumPacketSize:         DefaultMaxPacketSize,
	}
}

// Validate checks the parameters against the Version-3 frame bounds.
func (m *ManagedParameters) Validate() error {
	if m.LocalSpacecraftID > 0x03FF || m.RemoteSpacecraftID > 0x03FF {
		return ErrInvalidSCID
	}
	if m.SendMaximumFrameLength < MinFrameSize || m.SendMaximumFrameLength > MaxFrameSize {
		return ErrInvalidFrameLength
	}
	if m.ReceiveMaximumFrameLength < MinFrameSize || m.ReceiveMaximumFrameLength > MaxFrameSize {
		return ErrInvalidFrameLength
	}
	return nil
}
