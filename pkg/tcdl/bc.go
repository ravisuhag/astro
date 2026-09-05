package tcdl

// Control command (Type-BC frame) contents per CCSDS 232.0-B-4 4.1.3.3.
//
// A Type-BC frame (Bypass=1, Control Command=1) carries exactly one of two
// COP-1 control commands in its data field:
//
//   - Unlock: a single octet 0x00
//   - Set V(R): three octets 0x82 0x00 <V(R)>
//
// BC frames never carry a segment header and their Frame Sequence Number
// is all zeros.

// ControlCommandType identifies a decoded COP-1 control command.
type ControlCommandType int

const (
	// ControlUnlock is the Unlock control command (data field 0x00).
	ControlUnlock ControlCommandType = iota

	// ControlSetVR is the Set V(R) control command
	// (data field 0x82 0x00 <V(R)>).
	ControlSetVR
)

// BuildUnlockCommand returns the data field of an Unlock control command.
func BuildUnlockCommand() []byte {
	return []byte{0x00}
}

// BuildSetVRCommand returns the data field of a Set V(R) control command
// for the given V(R) value.
func BuildSetVRCommand(vr uint8) []byte {
	return []byte{0x82, 0x00, vr}
}

// ParseControlCommand decodes the data field of a Type-BC frame.
// For Unlock it returns (ControlUnlock, 0, nil); for Set V(R) it returns
// (ControlSetVR, vr, nil). Any other content returns
// ErrInvalidControlCommand.
func ParseControlCommand(data []byte) (ControlCommandType, uint8, error) {
	switch {
	case len(data) == 1 && data[0] == 0x00:
		return ControlUnlock, 0, nil
	case len(data) == 3 && data[0] == 0x82 && data[1] == 0x00:
		return ControlSetVR, data[2], nil
	default:
		return 0, 0, ErrInvalidControlCommand
	}
}

// NewUnlockFrame builds a Type-BC frame carrying the Unlock control
// command (Bypass=1, Control Command=1, N(S)=0, no segment header).
func NewUnlockFrame(scid uint16, vcid uint8) (*TCTransferFrame, error) {
	return NewTransferFrame(scid, vcid, BuildUnlockCommand(), WithControlCommand())
}

// NewSetVRFrame builds a Type-BC frame carrying the Set V(R) control
// command for the given V(R) value (Bypass=1, Control Command=1, N(S)=0,
// no segment header).
func NewSetVRFrame(scid uint16, vcid uint8, vr uint8) (*TCTransferFrame, error) {
	return NewTransferFrame(scid, vcid, BuildSetVRCommand(vr), WithControlCommand())
}
