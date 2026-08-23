package tcdl

import "github.com/ravisuhag/astro/pkg/sdl"

// VirtualChannel is a frame buffer for a single TC virtual channel.
type VirtualChannel = sdl.Channel[*TCTransferFrame]

// NewVirtualChannel creates a new TC Virtual Channel with the given VCID and buffer capacity.
func NewVirtualChannel(vcid uint8, bufferSize int) *VirtualChannel {
	return sdl.NewChannel[*TCTransferFrame](vcid, bufferSize)
}

// VirtualChannelMultiplexer is a weighted round-robin frame scheduler
// for TC Virtual Channels.
type VirtualChannelMultiplexer = sdl.Multiplexer[*TCTransferFrame]

// NewMultiplexer creates a new TC Virtual Channel multiplexer.
func NewMultiplexer() *VirtualChannelMultiplexer {
	return sdl.NewMultiplexer[*TCTransferFrame]()
}

// TCServiceManager manages multiple TC services and Master Channels.
type TCServiceManager = sdl.ServiceManager[ServiceType, *TCTransferFrame]

// NewTCServiceManager creates a new TC Service Manager.
func NewTCServiceManager() *TCServiceManager {
	return sdl.NewServiceManager[ServiceType, *TCTransferFrame]()
}

// FrameGapDetector tracks per-VC frame sequence numbers to detect gaps
// caused by lost frames. TC has only per-VC sequence numbers (no MC counter).
type FrameGapDetector struct {
	vc *sdl.GapCounter[uint8]
}

// NewFrameGapDetector creates a new detector.
func NewFrameGapDetector() *FrameGapDetector {
	// The Frame Sequence Number is eight bits.
	return &FrameGapDetector{vc: sdl.NewGapCounter[uint8](0xFF)}
}

// Track examines the frame's VC sequence number and records any gap.
// Returns the VC gap (0 means no gap or first frame).
func (d *FrameGapDetector) Track(frame *TCTransferFrame) int {
	return d.vc.Track(frame.Header.VirtualChannelID, frame.Header.FrameSequenceNum)
}

// VCFrameGap returns the VC gap detected by the last Track call.
func (d *FrameGapDetector) VCFrameGap() int {
	return d.vc.LastGap()
}

// MasterChannel manages TC Transfer Frames for a Master Channel identified by SCID.
type MasterChannel struct {
	scid     uint16
	mux      *VirtualChannelMultiplexer
	channels map[uint8]*VirtualChannel
	detector *FrameGapDetector
}

// NewMasterChannel creates a new Master Channel for the given spacecraft ID.
func NewMasterChannel(scid uint16) *MasterChannel {
	return &MasterChannel{
		scid:     scid,
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
func (mc *MasterChannel) AddFrame(frame *TCTransferFrame) error {
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

// GetNextFrame retrieves the next frame from the multiplexer.
func (mc *MasterChannel) GetNextFrame() (*TCTransferFrame, error) {
	return mc.mux.Next()
}

// HasPendingFrames checks if any Virtual Channel has pending frames.
func (mc *MasterChannel) HasPendingFrames() bool {
	return mc.mux.HasPending()
}

// VCFrameGap returns the VC gap from the last AddFrame call.
func (mc *MasterChannel) VCFrameGap() int {
	return mc.detector.VCFrameGap()
}
