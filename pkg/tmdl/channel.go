package tmdl

import (
	"errors"

	"github.com/ravisuhag/astro/pkg/sdl"
)

// VirtualChannel is a frame buffer for a single TM virtual channel.
type VirtualChannel = sdl.Channel[*TMTransferFrame]

// NewVirtualChannel creates a new TM Virtual Channel with the given VCID and buffer capacity.
func NewVirtualChannel(vcid uint8, bufferSize int) *VirtualChannel {
	return sdl.NewChannel[*TMTransferFrame](vcid, bufferSize)
}

// VirtualChannelMultiplexer is a weighted round-robin frame scheduler
// for TM Virtual Channels.
type VirtualChannelMultiplexer = sdl.Multiplexer[*TMTransferFrame]

// NewMultiplexer creates a new TM Virtual Channel multiplexer.
func NewMultiplexer() *VirtualChannelMultiplexer {
	return sdl.NewMultiplexer[*TMTransferFrame]()
}

// TMServiceManager manages multiple TM services and Master Channels,
// wiring the pipeline: Service → VirtualChannel → Mux → MasterChannel.
type TMServiceManager = sdl.ServiceManager[ServiceType, *TMTransferFrame]

// NewTMServiceManager creates a new TM Service Manager.
func NewTMServiceManager() *TMServiceManager {
	return sdl.NewServiceManager[ServiceType, *TMTransferFrame]()
}

// FrameGapDetector tracks Master Channel and Virtual Channel frame counts
// to detect gaps caused by lost frames during transmission.
// Per CCSDS 132.0-B-3, MCFrameCount and VCFrameCount are 8-bit counters
// that wrap from 255 to 0.
type FrameGapDetector struct {
	// TM is the only protocol here that counts frames per master channel as
	// well as per virtual channel, so it keeps two counters. The master
	// channel is tracked as a single pseudo-channel.
	mc *sdl.GapCounter[uint8]
	vc *sdl.GapCounter[uint8]

	lastMCGap int
	lastVCID  uint8
}

// NewFrameGapDetector creates a new detector. The first frame seen
// initializes the expected counts (no gap reported).
func NewFrameGapDetector() *FrameGapDetector {
	// Both TM frame counts are eight bits.
	return &FrameGapDetector{
		mc: sdl.NewGapCounter[uint8](0xFF),
		vc: sdl.NewGapCounter[uint8](0xFF),
	}
}

// Track examines the frame's MC and VC counts and records any gaps.
// A gap of N means N frames were lost between the previous frame and this one.
// Returns the MC gap and VC gap for convenience.
func (d *FrameGapDetector) Track(frame *TMTransferFrame) (mcGap, vcGap int) {
	vcid := frame.Header.VirtualChannelID

	d.lastMCGap = d.mc.Track(0, frame.Header.MCFrameCount)
	vcGap = d.vc.Track(vcid, frame.Header.VCFrameCount)
	d.lastVCID = vcid

	return d.lastMCGap, vcGap
}

// MCFrameGap returns the MC gap detected by the last Track call.
// 0 means no gap (or first frame).
func (d *FrameGapDetector) MCFrameGap() int {
	return d.lastMCGap
}

// VCFrameGap returns the VC gap detected by the last Track call.
// 0 means no gap (or first frame for that VCID).
func (d *FrameGapDetector) VCFrameGap() int {
	return d.vc.LastGap()
}

// MasterChannel manages TM Transfer Frames for a Master Channel identified by SCID.
type MasterChannel struct {
	scid     uint16
	config   ChannelConfig
	mux      *VirtualChannelMultiplexer
	channels map[uint8]*VirtualChannel
	detector *FrameGapDetector
	counter  *FrameCounter

	// oidFill is the master channel's persistent PN generator for OID frame
	// data fields. CCSDS 132.0-B-3 §4.1.4.6.2.1 forbids restarting the
	// sequence between frames, so it lives here rather than per frame.
	oidFill *OIDSequence

	// idleVCID, when set, is the VCID idle frames are emitted on. §4.1.4.6.3
	// requires an OID frame's VCID to be one used for transferring packets.
	idleVCID *uint8

	// fshSupplier and ocfSupplier are the MC_FSH and MC_OCF service users of
	// CCSDS 132.0-B-3 §3.8 and §3.9: their SDUs are placed into every frame
	// released through this Master Channel (§4.2.5), overwriting whatever the
	// Virtual Channel level left in the fields.
	fshSupplier func() []byte
	ocfSupplier func() []byte

	// lastFSH and lastOCF hold the SDUs decommutated from the most recently
	// received frame — the MC_FSH.indication and MC_OCF.indication of
	// §3.8.3.3 and §3.9.3.3, produced by the Master Channel Reception
	// Function of §4.3.5.
	lastFSH []byte
	lastOCF []byte
}

// NewMasterChannel creates a new Master Channel for the given spacecraft ID.
func NewMasterChannel(scid uint16, config ChannelConfig) *MasterChannel {
	return &MasterChannel{
		scid:     scid,
		config:   config,
		mux:      NewMultiplexer(),
		channels: make(map[uint8]*VirtualChannel),
		detector: NewFrameGapDetector(),
		oidFill:  NewOIDSequence(),
	}
}

// SCID returns the Spacecraft Identifier for this Master Channel.
func (mc *MasterChannel) SCID() uint16 { return mc.scid }

// SetFrameCounter installs the shared FrameCounter used to stamp the MC and
// VC frame counts of idle frames created by GetNextFrameOrIdle. Pass the same
// counter the channel's services use, so idle frames continue the master
// channel count per CCSDS 132.0-B-3 §4.1.2.5 instead of carrying zeros.
func (mc *MasterChannel) SetFrameCounter(counter *FrameCounter) {
	mc.counter = counter
}

// AddVirtualChannel registers a Virtual Channel with this Master Channel.
func (mc *MasterChannel) AddVirtualChannel(vc *VirtualChannel, priority int) {
	mc.channels[vc.ID] = vc
	mc.mux.AddChannel(vc, priority)
}

// SetIdleVCID pins the VCID that idle (OID) frames from GetNextFrameOrIdle
// are emitted on. CCSDS 132.0-B-3 §4.1.4.6.3 requires it to be one of the
// VCIDs used for transferring packets. Without an explicit choice, the lowest
// registered Virtual Channel's VCID is used.
func (mc *MasterChannel) SetIdleVCID(vcid uint8) {
	mc.idleVCID = &vcid
}

// SetFSHSupplier installs the MC_FSH service user (CCSDS 132.0-B-3 §3.8): a
// callback whose FSH_SDU is placed into the Transfer Frame Secondary Header
// of every frame released through this Master Channel, per the Master Channel
// Generation Function of §4.2.5.2. The SDU must be exactly the channel's
// ChannelConfig.FSHDataLength octets, and frames must carry a secondary
// header for it to fill — set FSHDataLength on the channel configuration.
func (mc *MasterChannel) SetFSHSupplier(supplier func() []byte) {
	mc.fshSupplier = supplier
}

// SetOCFSupplier installs the MC_OCF service user (CCSDS 132.0-B-3 §3.9): a
// callback whose 4-octet OCF_SDU is placed into the Operational Control Field
// of every frame released through this Master Channel, per §4.2.5.3. Frames
// must carry an OCF for it to fill — set HasOCF on the channel configuration.
func (mc *MasterChannel) SetOCFSupplier(supplier func() []byte) {
	mc.ocfSupplier = supplier
}

// applyMCFields places the MC_FSH and MC_OCF SDUs into a frame about to be
// released, per the Master Channel Generation Function (CCSDS 132.0-B-3
// §4.2.5). The frame keeps its fixed length: the fields were built into it at
// the Virtual Channel level and only their contents are overwritten here.
func (mc *MasterChannel) applyMCFields(frame *TMTransferFrame) error {
	changed := false
	if mc.fshSupplier != nil {
		if !frame.Header.FSHFlag {
			return ErrFSHNotPresent
		}
		sdu := mc.fshSupplier()
		if len(sdu) != len(frame.SecondaryHeader.DataField) {
			return ErrFSHSizeMismatch
		}
		copy(frame.SecondaryHeader.DataField, sdu)
		changed = true
	}
	if mc.ocfSupplier != nil {
		if !frame.Header.OCFFlag {
			return ErrOCFNotPresent
		}
		sdu := mc.ocfSupplier()
		if len(sdu) != 4 {
			return ErrInvalidOCFLength
		}
		copy(frame.OperationalControl, sdu)
		changed = true
	}
	if changed {
		return recomputeCRC(frame)
	}
	return nil
}

// AddFrame routes an inbound frame to the appropriate Virtual Channel.
//
// Before routing, the Master Channel Reception Function of CCSDS 132.0-B-3
// §4.3.5 decommutates the frame: the secondary header and operational control
// field SDUs are recorded and readable from LastFSH and LastOCF, which is the
// delivery path of MC_FSH.indication and MC_OCF.indication.
func (mc *MasterChannel) AddFrame(frame *TMTransferFrame) error {
	if frame.Header.SpacecraftID != mc.scid {
		return ErrSCIDMismatch
	}
	mc.detector.Track(frame)
	if frame.Header.FSHFlag {
		mc.lastFSH = append([]byte(nil), frame.SecondaryHeader.DataField...)
	}
	if frame.Header.OCFFlag {
		mc.lastOCF = append([]byte(nil), frame.OperationalControl...)
	}
	vc, ok := mc.channels[frame.Header.VirtualChannelID]
	if !ok {
		return ErrVirtualChannelNotFound
	}
	return vc.Add(frame)
}

// LastFSH returns the FSH_SDU carried by the most recently received frame, or
// nil when none has carried one. It is the MC_FSH.indication of §3.8.3.3;
// pair it with MCFrameGap for the optional FSH_SDU Loss Flag.
func (mc *MasterChannel) LastFSH() []byte { return mc.lastFSH }

// LastOCF returns the OCF_SDU carried by the most recently received frame, or
// nil when none has carried one. It is the MC_OCF.indication of §3.9.3.3;
// pair it with MCFrameGap for the optional OCF_SDU Loss Flag.
func (mc *MasterChannel) LastOCF() []byte { return mc.lastOCF }

// MCFrameGap returns the MC gap from the last AddFrame call.
func (mc *MasterChannel) MCFrameGap() int { return mc.detector.MCFrameGap() }

// VCFrameGap returns the VC gap from the last AddFrame call.
func (mc *MasterChannel) VCFrameGap() int { return mc.detector.VCFrameGap() }

// GetNextFrame retrieves the next frame from the multiplexer, applying the
// Master Channel Generation Function of CCSDS 132.0-B-3 §4.2.5: the MC_FSH
// and MC_OCF SDUs, when suppliers are installed, are placed into the frame
// before it is released.
func (mc *MasterChannel) GetNextFrame() (*TMTransferFrame, error) {
	frame, err := mc.mux.Next()
	if err != nil {
		return nil, err
	}
	if err := mc.applyMCFields(frame); err != nil {
		return nil, err
	}
	return frame, nil
}

// idleFrameVCID picks the VCID for an OID frame. CCSDS 132.0-B-3 §4.1.4.6.3
// requires one of the VCIDs used for transferring packets: the pinned choice
// when SetIdleVCID was called, otherwise the lowest registered Virtual
// Channel, and only with no channel registered at all the fallback constant.
func (mc *MasterChannel) idleFrameVCID() uint8 {
	if mc.idleVCID != nil {
		return *mc.idleVCID
	}
	chosen, found := uint8(0), false
	for vcid := range mc.channels {
		if !found || vcid < chosen {
			chosen, found = vcid, true
		}
	}
	if found {
		return chosen
	}
	return IdleFrameVCID
}

// GetNextFrameOrIdle returns the next frame or an idle (OID) frame if none is
// available, which is the Virtual Channel Multiplexing Function's duty under
// CCSDS 132.0-B-3 §4.2.4.4: keep the transmitted stream continuous.
//
// Idle frames are stamped from the FrameCounter installed with
// SetFrameCounter, so their MC and VC frame counts continue the channel's
// sequence (§4.1.2.5); without a counter they carry zeros. Their data field
// is filled from the channel's persistent PN generator (§4.1.4.6.2) and their
// VCID is one that carries packets (§4.1.4.6.3). MC_FSH and MC_OCF SDUs are
// applied to them like any other released frame.
func (mc *MasterChannel) GetNextFrameOrIdle() (*TMTransferFrame, error) {
	frame, err := mc.GetNextFrame()
	if err == nil {
		return frame, nil
	}
	if !errors.Is(err, sdl.ErrNoFramesAvailable) {
		return nil, err
	}
	if mc.config.FrameLength == 0 {
		return nil, sdl.ErrNoFramesAvailable
	}
	idle, err := NewIdleFrameWithCounter(
		mc.scid, mc.idleFrameVCID(), mc.config, mc.counter, mc.oidFill)
	if err != nil {
		return nil, err
	}
	if err := mc.applyMCFields(idle); err != nil {
		return nil, err
	}
	return idle, nil
}

// HasPendingFrames checks if any Virtual Channel has pending frames.
func (mc *MasterChannel) HasPendingFrames() bool {
	return mc.mux.HasPending()
}
