package tmdl

import (
	"bytes"
	"errors"
	"slices"
)

// DefaultASM returns the standard CCSDS Attached Sync Marker (0x1ACFFC1D)
// used to identify the start of each Transfer Frame in the bitstream.
// A fresh copy is returned each call to prevent accidental mutation.
func DefaultASM() []byte {
	return []byte{0x1A, 0xCF, 0xFC, 0x1D}
}

// ChannelConfig defines the fixed parameters of a physical channel
// per CCSDS 132.0-B-3. All frames on a physical channel share the
// same fixed length and optional field configuration.
type ChannelConfig struct {
	FrameLength int  // Total frame length in octets (fixed per physical channel)
	HasOCF      bool // Whether Operational Control Field (4 bytes) is present
	HasFEC      bool // Whether Frame Error Control (2-byte CRC) is present
}

// DataFieldCapacity returns the maximum data field size available
// in frames on this physical channel. secondaryHeaderLen is the
// length of the secondary header data field (0 if not present);
// when present, the encoded secondary header adds 1 prefix byte
// plus secondaryHeaderLen data bytes.
func (c ChannelConfig) DataFieldCapacity(secondaryHeaderLen int) int {
	capacity := c.FrameLength - 6 // primary header is always 6 bytes
	if secondaryHeaderLen > 0 {
		capacity -= 1 + secondaryHeaderLen // 1 prefix byte + data
	}
	if c.HasOCF {
		capacity -= 4
	}
	if c.HasFEC {
		capacity -= 2
	}
	return capacity
}

// PhysicalChannel represents a single physical communication link
// that carries one or more Master Channels. It handles MC-level
// multiplexing (send path), demultiplexing (receive path), and
// physical-layer framing (ASM, randomization) per CCSDS 132.0-B-3.
type PhysicalChannel struct {
	Name            string        // Channel identifier (TM-68)
	ASM             []byte        // Attached Sync Marker; nil uses DefaultASM
	Randomize       bool          // Whether to apply CCSDS pseudo-randomization
	config          ChannelConfig
	masterChannels  map[uint16]*MasterChannel
	priority        map[uint16]int
	sortedSCIDs     []uint16
	currentIndex    int
	remainingWeight int
}

// NewPhysicalChannel creates a physical channel with the given configuration.
func NewPhysicalChannel(name string, config ChannelConfig) *PhysicalChannel {
	return &PhysicalChannel{
		Name:           name,
		config:         config,
		masterChannels: make(map[uint16]*MasterChannel),
		priority:       make(map[uint16]int),
	}
}

// AddMasterChannel registers a Master Channel with a priority weight
// for the MC multiplexing scheme. Priority must be at least 1.
func (pc *PhysicalChannel) AddMasterChannel(mc *MasterChannel, priority int) {
	if priority < 1 {
		priority = 1
	}
	scid := mc.SCID()
	pc.masterChannels[scid] = mc
	pc.priority[scid] = priority

	pc.sortedSCIDs = make([]uint16, 0, len(pc.masterChannels))
	for s := range pc.masterChannels {
		pc.sortedSCIDs = append(pc.sortedSCIDs, s)
	}
	slices.Sort(pc.sortedSCIDs)

	pc.currentIndex = 0
	if len(pc.sortedSCIDs) > 0 {
		pc.remainingWeight = pc.priority[pc.sortedSCIDs[0]]
	}
}

// GetNextFrame selects the next frame for transmission using weighted
// round-robin MC multiplexing across registered Master Channels.
func (pc *PhysicalChannel) GetNextFrame() (*TMTransferFrame, error) {
	if len(pc.sortedSCIDs) == 0 {
		return nil, ErrNoMasterChannels
	}

	for range len(pc.sortedSCIDs) {
		scid := pc.sortedSCIDs[pc.currentIndex]
		mc := pc.masterChannels[scid]

		if mc.HasPendingFrames() {
			frame, err := mc.GetNextFrame()
			if err != nil {
				return nil, err
			}
			pc.remainingWeight--
			if pc.remainingWeight <= 0 {
				pc.advanceToNext()
			}
			return frame, nil
		}

		pc.advanceToNext()
	}

	return nil, ErrNoFramesAvailable
}

// GetNextFrameOrIdle returns the next frame from MC multiplexing,
// or an idle frame if no Master Channel has pending data.
func (pc *PhysicalChannel) GetNextFrameOrIdle() (*TMTransferFrame, error) {
	frame, err := pc.GetNextFrame()
	if err == nil {
		return frame, nil
	}
	if !errors.Is(err, ErrNoFramesAvailable) && !errors.Is(err, ErrNoMasterChannels) {
		return nil, err
	}
	if pc.config.FrameLength == 0 {
		return nil, ErrNoFramesAvailable
	}
	var scid uint16
	if len(pc.sortedSCIDs) > 0 {
		scid = pc.sortedSCIDs[0]
	}
	return NewIdleFrame(scid, 7, pc.config)
}

// Wrap produces a Channel Access Data Unit (CADU) from a TM Transfer Frame.
// It encodes the frame, optionally applies CCSDS pseudo-randomization,
// and prepends the Attached Sync Marker per CCSDS 132.0-B-3 §4.2.7.
func (pc *PhysicalChannel) Wrap(frame *TMTransferFrame) ([]byte, error) {
	encoded, err := frame.Encode()
	if err != nil {
		return nil, err
	}
	if pc.Randomize {
		pn := generatePNSequence(len(encoded))
		for i := range encoded {
			encoded[i] ^= pn[i]
		}
	}
	asm := pc.asm()
	cadu := make([]byte, len(asm)+len(encoded))
	copy(cadu, asm)
	copy(cadu[len(asm):], encoded)
	return cadu, nil
}

// Unwrap extracts a TM Transfer Frame from a Channel Access Data Unit.
// It validates and strips the ASM, optionally de-randomizes, and decodes
// the frame per CCSDS 132.0-B-3 §4.3.7.
func (pc *PhysicalChannel) Unwrap(cadu []byte) (*TMTransferFrame, error) {
	asm := pc.asm()
	if len(cadu) < len(asm) {
		return nil, ErrDataTooShort
	}
	if !bytes.Equal(cadu[:len(asm)], asm) {
		return nil, ErrSyncMarkerMismatch
	}
	data := make([]byte, len(cadu)-len(asm))
	copy(data, cadu[len(asm):])
	if pc.Randomize {
		pn := generatePNSequence(len(data))
		for i := range data {
			data[i] ^= pn[i]
		}
	}
	return DecodeTMTransferFrame(data)
}

// AddFrame demultiplexes an inbound frame to the appropriate Master Channel
// based on the Spacecraft ID in the frame header.
func (pc *PhysicalChannel) AddFrame(frame *TMTransferFrame) error {
	mc, ok := pc.masterChannels[frame.Header.SpacecraftID]
	if !ok {
		return ErrMasterChannelNotFound
	}
	return mc.AddFrame(frame)
}

// HasPendingFrames checks if any Master Channel has pending frames.
func (pc *PhysicalChannel) HasPendingFrames() bool {
	for _, mc := range pc.masterChannels {
		if mc.HasPendingFrames() {
			return true
		}
	}
	return false
}

// Len returns the number of registered Master Channels.
func (pc *PhysicalChannel) Len() int {
	return len(pc.masterChannels)
}

func (pc *PhysicalChannel) asm() []byte {
	if pc.ASM != nil {
		return pc.ASM
	}
	return DefaultASM()
}

func (pc *PhysicalChannel) advanceToNext() {
	pc.currentIndex = (pc.currentIndex + 1) % len(pc.sortedSCIDs)
	scid := pc.sortedSCIDs[pc.currentIndex]
	pc.remainingWeight = pc.priority[scid]
}

// generatePNSequence produces the CCSDS pseudo-random sequence using an
// 8-bit LFSR with polynomial h(x) = x^8 + x^7 + x^5 + x^3 + 1,
// initialized to all 1s per CCSDS 131.0-B.
func generatePNSequence(length int) []byte {
	seq := make([]byte, length)
	reg := uint8(0xFF)
	for i := range length {
		var b uint8
		for bit := 7; bit >= 0; bit-- {
			output := (reg >> 7) & 1
			b |= output << uint(bit)
			// Taps: x^8(bit7), x^7(bit6), x^5(bit4), x^3(bit2)
			feedback := ((reg >> 7) ^ (reg >> 6) ^ (reg >> 4) ^ (reg >> 2)) & 1
			reg = ((reg << 1) | feedback) & 0xFF
		}
		seq[i] = b
	}
	return seq
}
