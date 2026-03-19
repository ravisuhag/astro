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
	expectedMC uint8
	mcInit     bool

	expectedVC map[uint8]uint8
	vcInit     map[uint8]bool

	lastMCGap int
	lastVCGap int
	lastVCID  uint8
}

// NewFrameGapDetector creates a new detector. The first frame seen
// initializes the expected counts (no gap reported).
func NewFrameGapDetector() *FrameGapDetector {
	return &FrameGapDetector{
		expectedVC: make(map[uint8]uint8),
		vcInit:     make(map[uint8]bool),
	}
}

// Track examines the frame's MC and VC counts and records any gaps.
// A gap of N means N frames were lost between the previous frame and this one.
// Returns the MC gap and VC gap for convenience.
func (d *FrameGapDetector) Track(frame *TMTransferFrame) (mcGap, vcGap int) {
	vcid := frame.Header.VirtualChannelID

	// MC gap detection
	if d.mcInit {
		d.lastMCGap = int((frame.Header.MCFrameCount - d.expectedMC) & 0xFF)
	} else {
		d.mcInit = true
		d.lastMCGap = 0
	}
	d.expectedMC = frame.Header.MCFrameCount + 1

	// VC gap detection
	if d.vcInit[vcid] {
		d.lastVCGap = int((frame.Header.VCFrameCount - d.expectedVC[vcid]) & 0xFF)
	} else {
		d.vcInit[vcid] = true
		d.lastVCGap = 0
	}
	d.expectedVC[vcid] = frame.Header.VCFrameCount + 1
	d.lastVCID = vcid

	return d.lastMCGap, d.lastVCGap
}

// MCFrameGap returns the MC gap detected by the last Track call.
// 0 means no gap (or first frame).
func (d *FrameGapDetector) MCFrameGap() int {
	return d.lastMCGap
}

// VCFrameGap returns the VC gap detected by the last Track call.
// 0 means no gap (or first frame for that VCID).
func (d *FrameGapDetector) VCFrameGap() int {
	return d.lastVCGap
}

// MasterChannel manages TM Transfer Frames for a Master Channel identified by SCID.
type MasterChannel struct {
	scid     uint16
	config   ChannelConfig
	mux      *VirtualChannelMultiplexer
	channels map[uint8]*VirtualChannel
	detector *FrameGapDetector
}

// NewMasterChannel creates a new Master Channel for the given spacecraft ID.
func NewMasterChannel(scid uint16, config ChannelConfig) *MasterChannel {
	return &MasterChannel{
		scid:     scid,
		config:   config,
		mux:      NewMultiplexer(),
		channels: make(map[uint8]*VirtualChannel),
		detector: NewFrameGapDetector(),
	}
}

// SCID returns the Spacecraft Identifier for this Master Channel.
func (mc *MasterChannel) SCID() uint16 { return mc.scid }

// AddVirtualChannel registers a Virtual Channel with this Master Channel.
func (mc *MasterChannel) AddVirtualChannel(vc *VirtualChannel, priority int) {
	mc.channels[vc.ID] = vc
	mc.mux.AddChannel(vc, priority)
}

// AddFrame routes an inbound frame to the appropriate Virtual Channel.
func (mc *MasterChannel) AddFrame(frame *TMTransferFrame) error {
	if frame.Header.SpacecraftID != mc.scid {
		return ErrSCIDMismatch
	}
	mc.detector.Track(frame)
	vc, ok := mc.channels[frame.Header.VirtualChannelID]
	if !ok {
		return ErrVirtualChannelNotFound
	}
	return vc.Add(frame)
}

// MCFrameGap returns the MC gap from the last AddFrame call.
func (mc *MasterChannel) MCFrameGap() int { return mc.detector.MCFrameGap() }

// VCFrameGap returns the VC gap from the last AddFrame call.
func (mc *MasterChannel) VCFrameGap() int { return mc.detector.VCFrameGap() }

// GetNextFrame retrieves the next frame from the multiplexer.
func (mc *MasterChannel) GetNextFrame() (*TMTransferFrame, error) {
	return mc.mux.Next()
}

// GetNextFrameOrIdle returns the next frame or an idle frame if none available.
func (mc *MasterChannel) GetNextFrameOrIdle() (*TMTransferFrame, error) {
	frame, err := mc.mux.Next()
	if err == nil {
		return frame, nil
	}
	if !errors.Is(err, sdl.ErrNoFramesAvailable) {
		return nil, err
	}
	if mc.config.FrameLength == 0 {
		return nil, sdl.ErrNoFramesAvailable
	}
	return NewIdleFrame(mc.scid, 7, mc.config)
}

// HasPendingFrames checks if any Virtual Channel has pending frames.
func (mc *MasterChannel) HasPendingFrames() bool {
	return mc.mux.HasPending()
}
