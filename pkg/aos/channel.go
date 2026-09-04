package aos

import (
	"errors"

	"github.com/ravisuhag/astro/pkg/sdl"
)

// ChannelConfig defines the fixed parameters of an AOS physical channel
// per CCSDS 732.0-B-4. All frames on a physical channel share the same
// fixed length and optional field configuration.
type ChannelConfig struct {
	FrameLength   int    // Total frame length in octets (fixed per physical channel)
	InsertZoneLen int    // Insert zone length in bytes (0 if none)
	HasOCF        bool   // Whether Operational Control Field (4 bytes) is present
	HasFHEC       bool   // Whether Frame Header Error Control (RS(10,6), 2 bytes) is present
	HasFECF       bool   // Whether Frame Error Control Field (CRC-16) is present
	IdlePattern   []byte // Idle fill pattern (repeating); empty means DefaultIdleFill
}

// DataFieldCapacity returns the maximum Transfer Frame Data Field size,
// including any M_PDU or B_PDU header carried at the start of the data
// field. The caller is responsible for accounting for those 2 bytes when
// sizing payloads for M_PDU/B_PDU services.
func (c ChannelConfig) DataFieldCapacity() int {
	capacity := c.FrameLength - PrimaryHeaderSize - c.InsertZoneLen
	if c.HasFHEC {
		capacity -= FHECSize
	}
	if c.HasOCF {
		capacity -= OCFSize
	}
	if c.HasFECF {
		capacity -= FECFSize
	}
	return capacity
}

// VirtualChannel is a frame buffer for a single AOS virtual channel.
type VirtualChannel = sdl.Channel[*TransferFrame]

// NewVirtualChannel creates a new AOS Virtual Channel.
func NewVirtualChannel(vcid uint8, bufferSize int) *VirtualChannel {
	return sdl.NewChannel[*TransferFrame](vcid, bufferSize)
}

// VirtualChannelMultiplexer is a weighted round-robin frame scheduler
// for AOS Virtual Channels.
type VirtualChannelMultiplexer = sdl.Multiplexer[*TransferFrame]

// NewMultiplexer creates a new AOS Virtual Channel multiplexer.
func NewMultiplexer() *VirtualChannelMultiplexer {
	return sdl.NewMultiplexer[*TransferFrame]()
}

// AOSServiceManager manages multiple AOS services and Master Channels.
type AOSServiceManager = sdl.ServiceManager[ServiceType, *TransferFrame]

// NewAOSServiceManager creates a new AOS Service Manager.
func NewAOSServiceManager() *AOSServiceManager {
	return sdl.NewServiceManager[ServiceType, *TransferFrame]()
}

// scidKey converts an 8-bit AOS SCID to the uint16 used as the map key
// by the underlying sdl multiplexer.
func scidKey(scid uint8) uint16 { return uint16(scid) }

// FrameGapDetector tracks per-VC 24-bit frame counts to detect gaps
// caused by lost frames during transmission.
type FrameGapDetector struct {
	vc *sdl.GapCounter[uint32]
}

// NewFrameGapDetector creates a new detector.
func NewFrameGapDetector() *FrameGapDetector {
	// The AOS VC Frame Count is twenty-four bits.
	return &FrameGapDetector{vc: sdl.NewGapCounter[uint32](MaxVCFrameCount)}
}

// Track examines the frame's VC frame count and records any gap.
// Returns the VC gap (0 means no gap or first frame for that VCID).
func (d *FrameGapDetector) Track(frame *TransferFrame) int {
	return d.vc.Track(frame.Header.VCID, frame.Header.VCFrameCount)
}

// VCFrameGap returns the VC gap detected by the last Track call.
func (d *FrameGapDetector) VCFrameGap() int {
	return d.vc.LastGap()
}

// MasterChannel manages AOS Transfer Frames for a Master Channel
// identified by SCID.
type MasterChannel struct {
	scid        uint8
	config      ChannelConfig
	mux         *VirtualChannelMultiplexer
	channels    map[uint8]*VirtualChannel
	detector    *FrameGapDetector
	idleCounter *FrameCounter
}

// NewMasterChannel creates a new Master Channel for the given spacecraft ID.
func NewMasterChannel(scid uint8, config ChannelConfig) *MasterChannel {
	return &MasterChannel{
		scid:        scid,
		config:      config,
		mux:         NewMultiplexer(),
		channels:    make(map[uint8]*VirtualChannel),
		detector:    NewFrameGapDetector(),
		idleCounter: NewFrameCounter(),
	}
}

// SCID returns the 8-bit Spacecraft Identifier for this Master Channel.
func (mc *MasterChannel) SCID() uint16 { return scidKey(mc.scid) }

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

// VCFrameGap returns the VC gap from the last AddFrame call.
func (mc *MasterChannel) VCFrameGap() int {
	return mc.detector.VCFrameGap()
}

// GetNextFrame retrieves the next frame from the multiplexer.
func (mc *MasterChannel) GetNextFrame() (*TransferFrame, error) {
	return mc.mux.Next()
}

// GetNextFrameOrIdle returns the next frame or an OID idle frame if
// no Virtual Channel has pending data.
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
	idle, err := NewIdleFrame(mc.scid, mc.config)
	if err != nil {
		return nil, err
	}
	// OID frames run their own VC frame count on VC 63 (CCSDS 732.0-B-4
	// Clause 4.1.2.5.2: the count is maintained per Virtual Channel).
	if err := stampFrame(idle, mc.idleCounter, OIDVCID); err != nil {
		return nil, err
	}
	return idle, nil
}

// scidByte returns the 8-bit AOS Spacecraft Identifier.
func (mc *MasterChannel) scidByte() uint8 { return mc.scid }

// HasPendingFrames checks if any Virtual Channel has pending frames.
func (mc *MasterChannel) HasPendingFrames() bool {
	return mc.mux.HasPending()
}

// PhysicalChannel represents a single AOS physical communication link
// that carries one or more Master Channels.
type PhysicalChannel struct {
	Name        string
	config      ChannelConfig
	channels    *sdl.MasterChannelSet[*TransferFrame, *MasterChannel]
	idleCounter *FrameCounter
}

// NewPhysicalChannel creates a physical channel with the given configuration.
func NewPhysicalChannel(name string, config ChannelConfig) *PhysicalChannel {
	return &PhysicalChannel{
		Name:        name,
		config:      config,
		channels:    sdl.NewMasterChannelSet[*TransferFrame, *MasterChannel](),
		idleCounter: NewFrameCounter(),
	}
}

// AddMasterChannel registers a Master Channel with a priority weight.
func (pc *PhysicalChannel) AddMasterChannel(mc *MasterChannel, priority int) {
	pc.channels.Add(mc, priority)
}

// GetNextFrame selects the next frame for transmission.
func (pc *PhysicalChannel) GetNextFrame() (*TransferFrame, error) {
	return pc.channels.Next()
}

// GetNextFrameOrIdle returns the next frame from MC multiplexing,
// or an idle frame if no Master Channel has pending data.
func (pc *PhysicalChannel) GetNextFrameOrIdle() (*TransferFrame, error) {
	frame, err := pc.GetNextFrame()
	if err == nil {
		return frame, nil
	}
	if !errors.Is(err, sdl.ErrNoFramesAvailable) && !errors.Is(err, sdl.ErrNoMasterChannels) {
		return nil, err
	}
	if pc.config.FrameLength == 0 {
		return nil, sdl.ErrNoFramesAvailable
	}
	// The lowest registered SCID, so the fill is the same on every run.
	// Ranging the registry would take whichever channel Go's map iteration
	// yielded, and a physical channel carrying two spacecraft would stamp its
	// idle frames inconsistently. pkg/tmdl already chose the lowest; this now
	// comes from the same place, so the two cannot drift apart again.
	var scid uint8
	if mc, ok := pc.channels.Lowest(); ok {
		scid = mc.scidByte()
	}
	idle, err := NewIdleFrame(scid, pc.config)
	if err != nil {
		return nil, err
	}
	if err := stampFrame(idle, pc.idleCounter, OIDVCID); err != nil {
		return nil, err
	}
	return idle, nil
}

// AddFrame demultiplexes an inbound frame to the appropriate Master Channel.
func (pc *PhysicalChannel) AddFrame(frame *TransferFrame) error {
	return pc.channels.Route(scidKey(frame.Header.SCID), frame)
}

// HasPendingFrames checks if any Master Channel has pending frames.
func (pc *PhysicalChannel) HasPendingFrames() bool {
	return pc.channels.HasPending()
}

// Len returns the number of registered Master Channels.
func (pc *PhysicalChannel) Len() int {
	return pc.channels.Len()
}
