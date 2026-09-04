package usdl

import (
	"errors"
	"sync"

	"github.com/ravisuhag/astro/pkg/sdl"
)

// ChannelConfig defines the managed parameters of a USLP channel.
//
// IdlePattern is the project-specified idle pattern that fills the unused
// tail of fixed-length TFDZs behind the Last Valid Octet Pointer and the
// body of Encapsulation Idle Packets (clause 4.1.4.3 note 1). It does not fill
// OID frames: their TFDZ carries the mandatory PN sequence (clause 4.1.4.1.10).
type ChannelConfig struct {
	FrameLength   int    // Total frame length in octets (fixed per physical channel; 0 = variable)
	HasOCF        bool   // Whether the Operational Control Field (4 bytes) is carried
	HasFECF       bool   // Whether the 16-bit Frame Error Control Field is present
	InsertZoneLen int    // Insert zone length in bytes (0 if none)
	VCFCountLen   uint8  // VCF Count field length in octets (0-7; 0 = no count)
	IdlePattern   []byte // Idle fill pattern (repeating); empty means DefaultIdleFill
}

// DataFieldCapacity returns the maximum Transfer Frame Data Zone size for
// fixed-length frames, given the size of the TFDF header in use (1 octet,
// or 3 when the construction rule carries a pointer). Fixed-length frames
// use the full (non-truncated) primary header.
func (c ChannelConfig) DataFieldCapacity(dfhSize int) int {
	capacity := c.FrameLength - PrimaryHeaderBaseSize - int(c.VCFCountLen)
	capacity -= c.InsertZoneLen
	capacity -= dfhSize
	if c.HasOCF {
		capacity -= OCFSize
	}
	if c.HasFECF {
		capacity -= FECSize16
	}
	return capacity
}

// VirtualChannel is a frame buffer for a single USLP virtual channel. It
// owns the MAP demultiplexer for the up-to-16 MAP channels it carries
// (clause 4.3): services pull their own MAP's frames via NextForMAP, and frames
// for other MAP channels are held for their services rather than lost.
type VirtualChannel struct {
	*sdl.Channel[*TransferFrame]

	mu        sync.Mutex
	mapQueues map[uint8][]*TransferFrame
}

// NewVirtualChannel creates a new USLP Virtual Channel with the given VCID and buffer capacity.
func NewVirtualChannel(vcid uint8, bufferSize int) *VirtualChannel {
	return &VirtualChannel{Channel: sdl.NewChannel[*TransferFrame](vcid, bufferSize)}
}

// NextForMAP returns the next frame for the given MAP channel. Frames of
// other MAP IDs pulled from the shared VC buffer are queued for their own
// services instead of being discarded (clause 4.3 MAP demultiplexing); OID
// frames carry no service data and are dropped.
func (vc *VirtualChannel) NextForMAP(mapid uint8) (*TransferFrame, error) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	if q := vc.mapQueues[mapid]; len(q) > 0 {
		frame := q[0]
		vc.mapQueues[mapid] = q[1:]
		return frame, nil
	}
	for {
		frame, err := vc.Next()
		if err != nil {
			return nil, err
		}
		if IsIdleFrame(frame) {
			continue
		}
		if frame.Header.MAPID == mapid {
			return frame, nil
		}
		if vc.mapQueues == nil {
			vc.mapQueues = make(map[uint8][]*TransferFrame)
		}
		vc.mapQueues[frame.Header.MAPID] = append(vc.mapQueues[frame.Header.MAPID], frame)
	}
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

// FrameGapDetector tracks per-VC Virtual Channel Frame Counts to detect
// gaps caused by lost frames (CCSDS 732.1-B-3 clause 4.1.2.12). The count width
// is the managed VCF Count Length of the channel (clause 4.1.2.11). Sequence-
// controlled and expedited frames keep separate counts per VC
// (clause 4.1.2.12.4-12.5), so tracking is keyed by both the VCID and the
// Bypass/Sequence Control Flag.
type FrameGapDetector struct {
	countLen uint8
	vc       *sdl.GapCounter[uint64]
}

// NewFrameGapDetector creates a detector for the given VCF Count field
// length in octets (0-7). With a length of zero, no count is carried and
// Track always reports no gap.
func NewFrameGapDetector(countLen uint8) *FrameGapDetector {
	if countLen > MaxVCFCountLen {
		countLen = MaxVCFCountLen
	}
	return &FrameGapDetector{
		countLen: countLen,
		vc:       sdl.NewGapCounter[uint64](maxVCFCount(countLen)),
	}
}

// Track examines the frame's VCF Count and records any gap.
// Returns the VC gap (0 means no gap or first frame).
func (d *FrameGapDetector) Track(frame *TransferFrame) int {
	if d.countLen == 0 || frame.Header.VCFCountLen == 0 {
		return 0
	}
	// Clause 4.1.2.12.4-12.5: the sequence-controlled and expedited counts are
	// independent per VC. The VCID is 6 bits, so the bypass flag rides in
	// bit 6 of the tracking key.
	key := frame.Header.VCID
	if frame.Header.BypassSeqCtrl {
		key |= 0x40
	}
	return d.vc.Track(key, frame.Header.VCFCount)
}

// VCFrameGap returns the VC gap detected by the last Track call.
func (d *FrameGapDetector) VCFrameGap() int {
	return d.vc.LastGap()
}

// MasterChannel manages USLP Transfer Frames for a Master Channel identified by SCID.
type MasterChannel struct {
	scid        uint16
	config      ChannelConfig
	mux         *VirtualChannelMultiplexer
	channels    map[uint8]*VirtualChannel
	detector    *FrameGapDetector
	idleCounter *FrameCounter
	oidFill     *OIDSequence
	ocfSupplier func() []byte
}

// NewMasterChannel creates a new Master Channel for the given spacecraft ID.
func NewMasterChannel(scid uint16, config ChannelConfig) *MasterChannel {
	return &MasterChannel{
		scid:        scid,
		config:      config,
		mux:         NewMultiplexer(),
		channels:    make(map[uint8]*VirtualChannel),
		detector:    NewFrameGapDetector(config.VCFCountLen),
		idleCounter: NewFrameCounter(),
		oidFill:     NewOIDSequence(),
	}
}

// SCID returns the Spacecraft Identifier for this Master Channel.
func (mc *MasterChannel) SCID() uint16 { return mc.scid }

// SetOCFSupplier installs a callback that supplies the 4-octet Operational
// Control Field (typically a CLCW) for the OID frames GetNextFrameOrIdle
// generates on a channel configured with HasOCF. Without a supplier such a
// channel refuses to build idle frames (ErrNoOCFSupplier) rather than
// fabricating an all-zero Type-1 report.
func (mc *MasterChannel) SetOCFSupplier(supplier func() []byte) {
	mc.ocfSupplier = supplier
}

// AddVirtualChannel registers a Virtual Channel with this Master Channel.
func (mc *MasterChannel) AddVirtualChannel(vc *VirtualChannel, priority int) {
	mc.channels[vc.ID] = vc
	mc.mux.AddChannel(vc.Channel, priority)
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

// GetNextFrameOrIdle returns the next frame or an OID idle frame if none
// is available. OID frames exist only on fixed-length physical channels.
// Their TFDZ is drawn from the master channel's persistent PN sequence,
// which is never restarted across frames (clause 4.1.4.1.10).
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
	ocf, err := makeOCF(mc.config, mc.ocfSupplier)
	if err != nil {
		return nil, err
	}
	idle, err := NewIdleFrame(mc.scid, mc.config, mc.oidFill, ocf)
	if err != nil {
		return nil, err
	}
	if mc.config.VCFCountLen > 0 {
		if err := stampFrame(idle, mc.idleCounter, OIDVCID, mc.config.VCFCountLen); err != nil {
			return nil, err
		}
	}
	return idle, nil
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
	Name     string
	config   ChannelConfig
	channels *sdl.MasterChannelSet[*TransferFrame, *MasterChannel]
}

// NewPhysicalChannel creates a physical channel with the given configuration.
func NewPhysicalChannel(name string, config ChannelConfig) *PhysicalChannel {
	return &PhysicalChannel{
		Name:     name,
		config:   config,
		channels: sdl.NewMasterChannelSet[*TransferFrame, *MasterChannel](),
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

// AddFrame demultiplexes an inbound frame to the appropriate Master Channel.
func (pc *PhysicalChannel) AddFrame(frame *TransferFrame) error {
	return pc.channels.Route(frame.Header.SCID, frame)
}

// HasPendingFrames checks if any Master Channel has pending frames.
func (pc *PhysicalChannel) HasPendingFrames() bool {
	return pc.channels.HasPending()
}
