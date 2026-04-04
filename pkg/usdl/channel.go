package usdl

import (
	"errors"

	"github.com/ravisuhag/astro/pkg/sdl"
)

// ChannelConfig defines the fixed parameters of a USLP physical channel.
type ChannelConfig struct {
	FrameLength   int  // Total frame length in octets (fixed per physical channel; 0 = variable)
	HasOCF        bool // Whether Operational Control Field (4 bytes) is present
	HasFECF       bool // Whether Frame Error Control Field is present
	UseCRC32      bool // true=CRC-32 (4 bytes), false=CRC-16 (2 bytes)
	InsertZoneLen int  // Insert zone length in bytes (0 if none)
}

// DataFieldCapacity returns the maximum data field size available
// in frames on this physical channel. Only meaningful when FrameLength > 0
// (fixed-length mode). For fixed-length frames, EndOfFPH=1 so the primary
// header is 5 bytes (no Frame Length field). secondaryHeaderLen is reserved
// for future use (pass 0).
func (c ChannelConfig) DataFieldCapacity(secondaryHeaderLen int) int {
	// Fixed-length frames use the shorter 5-byte header (EndOfFPH=1)
	capacity := c.FrameLength - PrimaryHeaderFixedSize
	capacity -= c.InsertZoneLen
	capacity -= DataFieldHeaderSize
	if c.HasOCF {
		capacity -= 4
	}
	if c.HasFECF {
		if c.UseCRC32 {
			capacity -= FECSize32
		} else {
			capacity -= FECSize16
		}
	}
	return capacity
}

// VirtualChannel is a frame buffer for a single USLP virtual channel.
type VirtualChannel = sdl.Channel[*TransferFrame]

// NewVirtualChannel creates a new USLP Virtual Channel with the given VCID and buffer capacity.
func NewVirtualChannel(vcid uint8, bufferSize int) *VirtualChannel {
	return sdl.NewChannel[*TransferFrame](vcid, bufferSize)
}

// VirtualChannelMultiplexer is a weighted round-robin frame scheduler
// for USLP Virtual Channels.
type VirtualChannelMultiplexer = sdl.Multiplexer[*TransferFrame]

// NewMultiplexer creates a new USLP Virtual Channel multiplexer.
func NewMultiplexer() *VirtualChannelMultiplexer {
	return sdl.NewMultiplexer[*TransferFrame]()
}

// USDLServiceManager manages multiple USLP services and Master Channels.
type USDLServiceManager = sdl.ServiceManager[ServiceType, *TransferFrame]

// NewUSDLServiceManager creates a new USLP Service Manager.
func NewUSDLServiceManager() *USDLServiceManager {
	return sdl.NewServiceManager[ServiceType, *TransferFrame]()
}

// FrameGapDetector tracks per-VC frame sequence numbers to detect gaps
// caused by lost frames.
type FrameGapDetector struct {
	expectedVC map[uint8]uint16
	vcInit     map[uint8]bool
	lastVCGap  int
}

// NewFrameGapDetector creates a new detector.
func NewFrameGapDetector() *FrameGapDetector {
	return &FrameGapDetector{
		expectedVC: make(map[uint8]uint16),
		vcInit:     make(map[uint8]bool),
	}
}

// Track examines the frame's sequence number and records any gap.
// Returns the VC gap (0 means no gap or first frame).
func (d *FrameGapDetector) Track(frame *TransferFrame) int {
	vcid := frame.Header.VCID
	seq := frame.DataFieldHeader.SequenceNumber
	if d.vcInit[vcid] {
		d.lastVCGap = int((seq - d.expectedVC[vcid]) & 0xFFFF)
	} else {
		d.vcInit[vcid] = true
		d.lastVCGap = 0
	}
	d.expectedVC[vcid] = seq + 1
	return d.lastVCGap
}

// VCFrameGap returns the VC gap detected by the last Track call.
func (d *FrameGapDetector) VCFrameGap() int {
	return d.lastVCGap
}

// MasterChannel manages USLP Transfer Frames for a Master Channel identified by SCID.
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
func (mc *MasterChannel) AddFrame(frame *TransferFrame) error {
	if frame.Header.SCID != mc.scid {
		return ErrSCIDMismatch
	}
	mc.detector.Track(frame)
	vc, ok := mc.channels[frame.Header.VCID]
	if !ok {
		return ErrVirtualChannelNotFound
	}
	return vc.Add(frame)
}

// GetNextFrame retrieves the next frame from the multiplexer.
func (mc *MasterChannel) GetNextFrame() (*TransferFrame, error) {
	return mc.mux.Next()
}

// GetNextFrameOrIdle returns the next frame or an idle frame if none available.
func (mc *MasterChannel) GetNextFrameOrIdle() (*TransferFrame, error) {
	frame, err := mc.mux.Next()
	if err == nil {
		return frame, nil
	}
	if !errors.Is(err, sdl.ErrNoFramesAvailable) && !errors.Is(err, sdl.ErrNoChannels) {
		return nil, err
	}
	if mc.config.FrameLength == 0 {
		return nil, sdl.ErrNoFramesAvailable
	}
	return NewIdleFrame(mc.scid, 63, mc.config)
}

// HasPendingFrames checks if any Virtual Channel has pending frames.
func (mc *MasterChannel) HasPendingFrames() bool {
	return mc.mux.HasPending()
}

// VCFrameGap returns the VC gap from the last AddFrame call.
func (mc *MasterChannel) VCFrameGap() int {
	return mc.detector.VCFrameGap()
}

// PhysicalChannel represents a single USLP physical communication link.
type PhysicalChannel struct {
	Name           string
	config         ChannelConfig
	mux            *sdl.MCMultiplexer[*TransferFrame]
	masterChannels map[uint16]*MasterChannel
}

// NewPhysicalChannel creates a physical channel with the given configuration.
func NewPhysicalChannel(name string, config ChannelConfig) *PhysicalChannel {
	return &PhysicalChannel{
		Name:           name,
		config:         config,
		mux:            sdl.NewMCMultiplexer[*TransferFrame](),
		masterChannels: make(map[uint16]*MasterChannel),
	}
}

// AddMasterChannel registers a Master Channel with a priority weight.
func (pc *PhysicalChannel) AddMasterChannel(mc *MasterChannel, priority int) {
	pc.masterChannels[mc.SCID()] = mc
	pc.mux.Add(mc, priority)
}

// GetNextFrame selects the next frame for transmission.
func (pc *PhysicalChannel) GetNextFrame() (*TransferFrame, error) {
	return pc.mux.Next()
}

// AddFrame demultiplexes an inbound frame to the appropriate Master Channel.
func (pc *PhysicalChannel) AddFrame(frame *TransferFrame) error {
	mc, ok := pc.masterChannels[frame.Header.SCID]
	if !ok {
		return ErrMasterChannelNotFound
	}
	return mc.AddFrame(frame)
}

// HasPendingFrames checks if any Master Channel has pending frames.
func (pc *PhysicalChannel) HasPendingFrames() bool {
	return pc.mux.HasPending()
}
