package tmdl

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
