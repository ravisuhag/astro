package tcdl

// FrameGapDetector tracks per-VC frame sequence numbers to detect gaps
// caused by lost frames. TC has only per-VC sequence numbers (no MC counter).
type FrameGapDetector struct {
	expectedVC map[uint8]uint8
	vcInit     map[uint8]bool
	lastVCGap  int
}

// NewFrameGapDetector creates a new detector.
func NewFrameGapDetector() *FrameGapDetector {
	return &FrameGapDetector{
		expectedVC: make(map[uint8]uint8),
		vcInit:     make(map[uint8]bool),
	}
}

// Track examines the frame's VC sequence number and records any gap.
// Returns the VC gap (0 means no gap or first frame).
func (d *FrameGapDetector) Track(frame *TCTransferFrame) int {
	vcid := frame.Header.VirtualChannelID
	if d.vcInit[vcid] {
		d.lastVCGap = int((frame.Header.FrameSequenceNum - d.expectedVC[vcid]) & 0xFF)
	} else {
		d.vcInit[vcid] = true
		d.lastVCGap = 0
	}
	d.expectedVC[vcid] = frame.Header.FrameSequenceNum + 1
	return d.lastVCGap
}

// VCFrameGap returns the VC gap detected by the last Track call.
func (d *FrameGapDetector) VCFrameGap() int {
	return d.lastVCGap
}
