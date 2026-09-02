package cli

import (
	"fmt"
	"io"

	"github.com/ravisuhag/astro/pkg/sdl"
)

// The receive loop.
//
// Gap detection and virtual channel demultiplexing are the same two jobs for
// every frame protocol here: read frames in order, pull the channel
// identifiers and frame counts out of each header, then either report the
// counts that went missing or pass on the frames belonging to one channel.
// Only the header layout differs between protocols.
//
// So it is written once. A protocol joins by naming its counter widths and
// supplying a function that reads one frame's identifiers. The counting
// itself comes from sdl.GapCounter, which already carries the four field
// widths and the AOS cycle fold, and is tested there.

// frameIdent is everything the loop needs from one frame, whatever protocol
// carried it.
type frameIdent struct {
	// scid and vcid name the channel the frame arrived on.
	scid uint16
	vcid uint8

	// vcCount is the virtual channel frame count, in the width the protocol's
	// vcMask states.
	vcCount uint64

	// mcCount is the master channel frame count. Only TM has one, so hasMC is
	// false for the others and no master channel gaps are reported for them.
	mcCount uint64
	hasMC   bool

	// cycle is the count cycle some protocols pair with the VC count. It is
	// meaningful only when hasCycle is set, which follows the frame's own
	// usage flag rather than the protocol.
	cycle    uint8
	hasCycle bool
}

// frameProtocol describes one protocol to the receive loop.
type frameProtocol struct {
	// vcMask and mcMask are counter field widths in the all-ones form
	// sdl.NewGapCounter wants: 0xFF for eight bits, 0xFFFFFF for
	// twenty-four. mcMask is zero where there is no master channel count.
	vcMask uint64
	mcMask uint64

	// cycleMask is the width of the cycle field paired with the VC count, or
	// zero for a protocol that has none.
	cycleMask uint8

	// ident reads one frame's identifiers. An error is reported and that
	// frame skipped; the loop carries on with the next, because one corrupt
	// frame in a capture should not end the scan.
	ident func(raw []byte) (frameIdent, error)
}

// channelCounters holds the counters for one spacecraft.
//
// They are per spacecraft because counts from different spacecraft are
// unrelated. A ground station recording a shared downlink, or a capture
// spliced from two passes, holds more than one SCID, and comparing a count
// from one against a count from another invents gaps that did not happen.
type channelCounters struct {
	vc *sdl.GapCounter[uint64]
	mc *sdl.GapCounter[uint64]

	// cycleMode records whether the first frame seen on a channel used the
	// cycle field, and cycleSeen whether there has been one at all.
	//
	// sdl.GapCounter documents that Track and TrackWithCycle must not be
	// mixed on one channel: they disagree about where the counter wraps, so a
	// gap measured across the switch would be measured against the wrong
	// modulus. A frame whose usage flag disagrees with its channel's is
	// reported and left uncounted rather than folded in anyway.
	cycleMode map[uint8]bool
	cycleSeen map[uint8]bool
}

// gapScanner reports counter discontinuities across a stream of frames.
type gapScanner struct {
	proto  frameProtocol
	out    io.Writer
	errOut io.Writer

	spacecraft map[uint16]*channelCounters

	frames  int
	skipped int
	gaps    int
	missing int
}

func newGapScanner(proto frameProtocol, out, errOut io.Writer) *gapScanner {
	return &gapScanner{
		proto:      proto,
		out:        out,
		errOut:     errOut,
		spacecraft: make(map[uint16]*channelCounters),
	}
}

// countersFor returns the counters for one spacecraft, creating them on first
// sight.
func (g *gapScanner) countersFor(scid uint16) *channelCounters {
	if c, ok := g.spacecraft[scid]; ok {
		return c
	}

	c := &channelCounters{
		vc:        sdl.NewGapCounter[uint64](g.proto.vcMask),
		cycleMode: make(map[uint8]bool),
		cycleSeen: make(map[uint8]bool),
	}
	if g.proto.mcMask != 0 {
		c.mc = sdl.NewGapCounter[uint64](g.proto.mcMask)
	}
	g.spacecraft[scid] = c

	return c
}

// track takes one frame and reports any gap before it.
//
// It is the handler the stream readers drive, so its signature is theirs: the
// slice aliases the read buffer and nothing here keeps it.
func (g *gapScanner) track(raw []byte) error {
	ident, err := g.proto.ident(raw)
	if err != nil {
		g.skipped++
		_, _ = fmt.Fprintf(g.errOut, "Warning: frame #%d decode error: %v, skipping\n", g.frames+g.skipped, err)
		return nil
	}
	g.frames++

	counters := g.countersFor(ident.scid)
	index := g.frames + g.skipped

	// Master channel first, so the two lines for one frame read in field
	// order.
	if ident.hasMC && counters.mc != nil {
		if gap := counters.mc.Track(0, ident.mcCount); gap > 0 {
			g.gaps++
			g.missing += gap
			_, _ = fmt.Fprintf(g.out, "MC gap: frame #%d, %d frame(s) missing before MC=%d (SCID=%d)\n",
				index, gap, ident.mcCount, ident.scid)
		}
	}

	// A channel has to stay in one cycle mode for its counts to be
	// comparable.
	useCycle := ident.hasCycle && g.proto.cycleMask != 0
	if counters.cycleSeen[ident.vcid] && counters.cycleMode[ident.vcid] != useCycle {
		_, _ = fmt.Fprintf(g.errOut,
			"Warning: frame #%d on VCID=%d changed frame count cycle usage; gap not measured across the change\n",
			index, ident.vcid)
		counters.cycleMode[ident.vcid] = useCycle
		counters.vc.Reset()
		return nil
	}
	counters.cycleSeen[ident.vcid] = true
	counters.cycleMode[ident.vcid] = useCycle

	var gap int
	if useCycle {
		gap = counters.vc.TrackWithCycle(ident.vcid, ident.vcCount, ident.cycle, g.proto.cycleMask)
	} else {
		gap = counters.vc.Track(ident.vcid, ident.vcCount)
	}
	if gap > 0 {
		g.gaps++
		g.missing += gap
		_, _ = fmt.Fprintf(g.out, "VC gap: frame #%d, %d frame(s) missing before VC=%d (SCID=%d VCID=%d)\n",
			index, gap, ident.vcCount, ident.scid, ident.vcid)
	}

	return nil
}

// summary prints the closing count.
func (g *gapScanner) summary() {
	_, _ = fmt.Fprintf(g.out, "\nScanned %d frame(s), found %d gap(s)", g.frames, g.gaps)
	if g.missing > 0 {
		_, _ = fmt.Fprintf(g.out, ", %d frame(s) missing", g.missing)
	}
	_, _ = fmt.Fprintln(g.out, ".")

	if g.skipped > 0 {
		_, _ = fmt.Fprintf(g.out, "%d frame(s) could not be decoded and were skipped.\n", g.skipped)
	}
	if len(g.spacecraft) > 1 {
		_, _ = fmt.Fprintf(g.out, "%d spacecraft seen; counts were compared within each, not across them.\n",
			len(g.spacecraft))
	}
}

// demuxer passes on the frames belonging to one virtual channel.
type demuxer struct {
	proto  frameProtocol
	vcid   uint8
	out    io.Writer
	errOut io.Writer

	// emit writes one matching frame in whatever format the command was
	// asked for. The raw slice aliases the read buffer, so an emitter that
	// keeps it must copy.
	emit func(raw []byte, index int, ident frameIdent) error

	frames  int
	skipped int
	matched int
}

// filter takes one frame and emits it if it is on the wanted channel.
func (d *demuxer) filter(raw []byte) error {
	ident, err := d.proto.ident(raw)
	if err != nil {
		d.skipped++
		_, _ = fmt.Fprintf(d.errOut, "Warning: frame #%d decode error: %v, skipping\n", d.frames+d.skipped, err)
		return nil
	}
	d.frames++

	if ident.vcid != d.vcid {
		return nil
	}
	d.matched++

	return d.emit(raw, d.frames+d.skipped, ident)
}

// summary prints the closing count, for the formats where a trailing line is
// not mistaken for data.
func (d *demuxer) summary() {
	_, _ = fmt.Fprintf(d.out, "\nMatched %d of %d frame(s) on VCID=%d.\n", d.matched, d.frames, d.vcid)
	if d.skipped > 0 {
		_, _ = fmt.Fprintf(d.out, "%d frame(s) could not be decoded and were skipped.\n", d.skipped)
	}
}

// trailingWarning is the reporter the stream readers take for octets left
// over at the end.
//
// A capture cut mid-frame is normal, so this is a warning and the command
// still succeeds.
func trailingWarning(errOut io.Writer, frameLen int) func(n int) {
	return func(n int) {
		_, _ = fmt.Fprintf(errOut, "Warning: %d trailing octet(s) do not form a whole %d-octet frame, ignored\n",
			n, frameLen)
	}
}
