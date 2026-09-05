package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

// fixedSizer treats every unit as n octets.
func fixedSizer(n int) UnitSizer {
	return func([]byte) int { return n }
}

func TestStreamUnitsSplitsFixedLengthUnits(t *testing.T) {
	t.Parallel()
	input := bytes.Repeat([]byte{0xAA, 0xBB, 0xCC, 0xDD}, 5)

	var got [][]byte
	err := streamUnits(bytes.NewReader(input), fixedSizer(4), 4,
		func(unit []byte) error {
			// The slice aliases the reader's buffer, so copy before keeping.
			got = append(got, append([]byte(nil), unit...))
			return nil
		}, nil)
	if err != nil {
		t.Fatalf("streamUnits() = %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d units, want 5", len(got))
	}
	for i, unit := range got {
		if !bytes.Equal(unit, []byte{0xAA, 0xBB, 0xCC, 0xDD}) {
			t.Errorf("unit %d = % X", i, unit)
		}
	}
}

func TestStreamUnitsReportsTrailingOctets(t *testing.T) {
	t.Parallel()
	// Two whole units and three octets left over.
	input := append(bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 2), 0xFF, 0xFE, 0xFD)

	units := 0
	trailing := -1
	err := streamUnits(bytes.NewReader(input), fixedSizer(4), 4,
		func([]byte) error { units++; return nil },
		func(n int) { trailing = n })
	if err != nil {
		t.Fatalf("streamUnits() = %v", err)
	}
	if units != 2 {
		t.Errorf("got %d units, want 2", units)
	}
	if trailing != 3 {
		t.Errorf("trailing = %d, want 3", trailing)
	}
}

func TestStreamUnitsEmptyInput(t *testing.T) {
	t.Parallel()
	called := false
	trailingCalled := false
	err := streamUnits(bytes.NewReader(nil), fixedSizer(4), 4,
		func([]byte) error { called = true; return nil },
		func(int) { trailingCalled = true })
	if err != nil {
		t.Fatalf("streamUnits() = %v", err)
	}
	if called {
		t.Error("the handler ran on empty input")
	}
	if trailingCalled {
		t.Error("trailing was reported for empty input")
	}
}

// TestStreamUnitsGrowsToFindLength covers a variable-length header: the sizer
// cannot answer from one octet and needs a second.
func TestStreamUnitsGrowsToFindLength(t *testing.T) {
	t.Parallel()
	// The second octet carries the length.
	sizer := func(data []byte) int {
		if len(data) < 2 {
			return -1
		}
		return int(data[1])
	}

	input := []byte{0x00, 0x04, 0xAA, 0xBB, 0x00, 0x03, 0xCC}

	var sizes []int
	err := streamUnits(bytes.NewReader(input), sizer, 1,
		func(unit []byte) error { sizes = append(sizes, len(unit)); return nil }, nil)
	if err != nil {
		t.Fatalf("streamUnits() = %v", err)
	}
	if len(sizes) != 2 || sizes[0] != 4 || sizes[1] != 3 {
		t.Errorf("unit sizes = %v, want [4 3]", sizes)
	}
}

func TestStreamUnitsRejectsOversizedUnit(t *testing.T) {
	t.Parallel()
	sizer := func([]byte) int { return maxStreamUnit + 1 }

	err := streamUnits(bytes.NewReader([]byte{1, 2, 3, 4}), sizer, 1,
		func([]byte) error { return nil }, nil)
	if err == nil {
		t.Error("streamUnits accepted a unit past the maximum")
	}
}

func TestStreamUnitsPropagatesHandlerError(t *testing.T) {
	t.Parallel()
	input := bytes.Repeat([]byte{0x01, 0x02}, 4)

	err := streamUnits(bytes.NewReader(input), fixedSizer(2), 2,
		func([]byte) error { return io.ErrUnexpectedEOF }, nil)
	if err == nil {
		t.Error("a handler error was swallowed")
	}
}

// TestStreamUnitsIsIncremental is the property the whole file exists for: a
// unit is handed over as soon as it is complete, not when the input ends.
//
// The reader below never reaches EOF until the test allows it, so a
// read-everything implementation would block here forever.
func TestStreamUnitsIsIncremental(t *testing.T) {
	t.Parallel()
	pipeReader, pipeWriter := io.Pipe()
	delivered := make(chan int, 4)

	go func() {
		_ = streamUnits(pipeReader, fixedSizer(2), 2,
			func(unit []byte) error {
				delivered <- int(unit[0])
				return nil
			}, nil)
	}()

	if _, err := pipeWriter.Write([]byte{0x01, 0x02}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-delivered:
		if got != 1 {
			t.Errorf("first unit = %d, want 1", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the first unit was not delivered while the input was still open")
	}

	if _, err := pipeWriter.Write([]byte{0x03, 0x04}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-delivered:
		if got != 3 {
			t.Errorf("second unit = %d, want 3", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the second unit was not delivered")
	}

	_ = pipeWriter.Close()
}

func TestStreamFixedSplitsOnLength(t *testing.T) {
	t.Parallel()
	input := []byte{1, 1, 2, 2, 3, 3}
	var got [][]byte

	err := streamFixed(bytes.NewReader(input), 2,
		func(unit []byte) error {
			got = append(got, append([]byte{}, unit...))
			return nil
		}, nil)
	if err != nil {
		t.Fatalf("streamFixed() = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d units, want 3", len(got))
	}
	if got[2][0] != 3 {
		t.Errorf("third unit = %v, want it to start with 3", got[2])
	}
}

func TestStreamFixedReportsTrailingOctets(t *testing.T) {
	t.Parallel()
	input := []byte{1, 1, 2}
	trailing := 0

	err := streamFixed(bytes.NewReader(input), 2,
		func([]byte) error { return nil },
		func(n int) { trailing = n })
	if err != nil {
		t.Fatalf("streamFixed() = %v", err)
	}
	if trailing != 1 {
		t.Errorf("trailing = %d, want 1", trailing)
	}
}

// A frame longer than maxUnitHeader must still read. The sizer path probes at
// most maxUnitHeader octets for a length, which a fixed frame length routinely
// exceeds; streamFixed is not allowed to inherit that ceiling.
func TestStreamFixedAcceptsFramesLongerThanAHeader(t *testing.T) {
	t.Parallel()
	const size = maxUnitHeader * 4
	input := bytes.Repeat([]byte{0xA5}, size*2)
	units := 0

	err := streamFixed(bytes.NewReader(input), size,
		func(unit []byte) error {
			if len(unit) != size {
				t.Errorf("unit length = %d, want %d", len(unit), size)
			}
			units++
			return nil
		}, nil)
	if err != nil {
		t.Fatalf("streamFixed() = %v", err)
	}
	if units != 2 {
		t.Errorf("got %d units, want 2", units)
	}
}

func TestStreamFixedEmptyInput(t *testing.T) {
	t.Parallel()
	err := streamFixed(bytes.NewReader(nil), 4,
		func([]byte) error {
			t.Error("a handler ran on empty input")
			return nil
		}, nil)
	if err != nil {
		t.Fatalf("streamFixed() = %v", err)
	}
}

// The live-pipe property for fixed-length frames: a frame is handed over the
// moment its last octet arrives, not when the input ends. A
// read-everything implementation blocks here forever.
func TestStreamFixedIsIncremental(t *testing.T) {
	t.Parallel()
	pipeReader, pipeWriter := io.Pipe()
	delivered := make(chan int, 2)

	go func() {
		_ = streamFixed(pipeReader, 2,
			func(unit []byte) error {
				delivered <- int(unit[0])
				return nil
			}, nil)
	}()

	if _, err := pipeWriter.Write([]byte{0x07, 0x08}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-delivered:
		if got != 7 {
			t.Errorf("first frame = %d, want 7", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the first frame was not delivered while the input was still open")
	}

	_ = pipeWriter.Close()
}

var testMarker = []byte{0x1A, 0xCF, 0xFC, 0x1D}

// markedStream builds a stream of size-octet units, each opening with the
// marker, with the given noise spliced in front.
func markedStream(noise []byte, size, count int) []byte {
	stream := append([]byte{}, noise...)
	for i := 0; i < count; i++ {
		unit := make([]byte, size)
		copy(unit, testMarker)
		unit[len(testMarker)] = byte(i)
		stream = append(stream, unit...)
	}
	return stream
}

func TestStreamMarkedFindsEveryUnit(t *testing.T) {
	t.Parallel()
	const size = 8
	var offsets []int64

	err := streamMarked(bytes.NewReader(markedStream(nil, size, 3)), testMarker, size,
		func(unit []byte, offset int64) error {
			offsets = append(offsets, offset)
			return nil
		}, nil, nil)
	if err != nil {
		t.Fatalf("streamMarked() = %v", err)
	}
	if len(offsets) != 3 {
		t.Fatalf("got %d units, want 3", len(offsets))
	}
	for i, offset := range offsets {
		if want := int64(i * size); offset != want {
			t.Errorf("unit %d at offset %d, want %d", i, offset, want)
		}
	}
}

// A capture normally starts part way through a frame, so leading octets that
// are not a unit are skipped rather than failed, and the offsets stay
// absolute.
func TestStreamMarkedSkipsLeadingNoise(t *testing.T) {
	t.Parallel()
	const size = 8
	noise := []byte{0xDE, 0xAD, 0xBE}
	var offsets []int64
	skippedTotal := 0

	err := streamMarked(bytes.NewReader(markedStream(noise, size, 2)), testMarker, size,
		func(unit []byte, offset int64) error {
			offsets = append(offsets, offset)
			return nil
		},
		func(offset int64, n int) { skippedTotal += n },
		nil)
	if err != nil {
		t.Fatalf("streamMarked() = %v", err)
	}
	if len(offsets) != 2 {
		t.Fatalf("got %d units, want 2", len(offsets))
	}
	if offsets[0] != int64(len(noise)) {
		t.Errorf("first unit at offset %d, want %d", offsets[0], len(noise))
	}
	if skippedTotal != len(noise) {
		t.Errorf("skipped %d octets, want %d", skippedTotal, len(noise))
	}
}

// Alignment is reacquired at the next marker: junk in the middle of a stream
// costs the units it overwrote and nothing more.
func TestStreamMarkedResyncsAfterJunk(t *testing.T) {
	t.Parallel()
	const size = 8
	stream := markedStream(nil, size, 1)
	stream = append(stream, bytes.Repeat([]byte{0x00}, 5)...)
	stream = append(stream, markedStream(nil, size, 1)...)

	units := 0
	err := streamMarked(bytes.NewReader(stream), testMarker, size,
		func([]byte, int64) error {
			units++
			return nil
		}, nil, nil)
	if err != nil {
		t.Fatalf("streamMarked() = %v", err)
	}
	if units != 2 {
		t.Errorf("got %d units, want 2 either side of the junk", units)
	}
}

// A marker split across two reads must still be found. The reader keeps the
// last few octets of each window for exactly this.
func TestStreamMarkedFindsAMarkerAcrossAReadBoundary(t *testing.T) {
	t.Parallel()
	const size = 6
	// Noise whose length leaves the marker straddling a window edge.
	stream := markedStream(bytes.Repeat([]byte{0x00}, size-2), size, 1)

	units := 0
	err := streamMarked(bytes.NewReader(stream), testMarker, size,
		func([]byte, int64) error {
			units++
			return nil
		}, nil, nil)
	if err != nil {
		t.Fatalf("streamMarked() = %v", err)
	}
	if units != 1 {
		t.Errorf("got %d units, want 1", units)
	}
}

func TestStreamMarkedReportsATruncatedUnit(t *testing.T) {
	t.Parallel()
	const size = 8
	stream := append([]byte{}, testMarker...)
	stream = append(stream, 0x01)

	truncated := -1
	err := streamMarked(bytes.NewReader(stream), testMarker, size,
		func([]byte, int64) error {
			t.Error("an incomplete unit was handed over")
			return nil
		}, nil,
		func(offset int64, n int) { truncated = n })
	if err != nil {
		t.Fatalf("streamMarked() = %v", err)
	}
	if truncated != len(stream) {
		t.Errorf("truncated = %d, want %d", truncated, len(stream))
	}
}

func TestStreamMarkedNoMarkerAtAll(t *testing.T) {
	t.Parallel()
	err := streamMarked(bytes.NewReader(bytes.Repeat([]byte{0x00}, 32)), testMarker, 8,
		func([]byte, int64) error {
			t.Error("a unit was found in a stream with no marker")
			return nil
		}, nil, nil)
	if err != nil {
		t.Fatalf("streamMarked() = %v", err)
	}
}

func TestStreamMarkedRejectsBadArguments(t *testing.T) {
	t.Parallel()
	nop := func([]byte, int64) error { return nil }

	if err := streamMarked(bytes.NewReader(nil), nil, 8, nop, nil, nil); err == nil {
		t.Error("an empty marker was accepted")
	}
	if err := streamMarked(bytes.NewReader(nil), testMarker, 2, nop, nil, nil); err == nil {
		t.Error("a unit shorter than the marker was accepted")
	}
	if err := streamMarked(bytes.NewReader(nil), testMarker, maxStreamUnit+1, nop, nil, nil); err == nil {
		t.Error("a unit past the maximum was accepted")
	}
}

// The live-pipe property for marker framing.
func TestStreamMarkedIsIncremental(t *testing.T) {
	t.Parallel()
	const size = 8
	pipeReader, pipeWriter := io.Pipe()
	delivered := make(chan int64, 2)

	go func() {
		_ = streamMarked(pipeReader, testMarker, size,
			func(unit []byte, offset int64) error {
				delivered <- offset
				return nil
			}, nil, nil)
	}()

	if _, err := pipeWriter.Write(markedStream(nil, size, 1)); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-delivered:
		if got != 0 {
			t.Errorf("first unit at offset %d, want 0", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the first unit was not delivered while the input was still open")
	}

	_ = pipeWriter.Close()
}

func TestHexFilterStripsLayout(t *testing.T) {
	t.Parallel()
	source := strings.NewReader("de ad\nbe\tef\r\n")

	got, err := io.ReadAll(newHexFilter(source))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if string(got) != "deadbeef" {
		t.Errorf("filtered to %q, want %q", got, "deadbeef")
	}
}

// TestHexFilterHandlesAWholeWhitespaceRead guards the loop that would
// otherwise return (0, nil), which io.ReadAll treats as no progress.
func TestHexFilterHandlesAWholeWhitespaceRead(t *testing.T) {
	t.Parallel()
	source := strings.NewReader(strings.Repeat(" ", 100) + "abcd")

	got, err := io.ReadAll(newHexFilter(source))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if string(got) != "abcd" {
		t.Errorf("filtered to %q, want %q", got, "abcd")
	}
}

// TestOpenInputRejectsUnknownFormat and TestOpenInputMissingFile call
// openInput directly rather than through runCLI, but openInput reads the
// package-global os.Stdin unconditionally on entry (even when it goes on to
// use a named file instead), so they still take stdinMu to stay race-free
// against any other test's os.Stdin swap.
func TestOpenInputRejectsUnknownFormat(t *testing.T) {
	t.Parallel()
	stdinMu.Lock()
	defer stdinMu.Unlock()
	if _, _, err := openInput(nil, "base64"); err == nil {
		t.Error("openInput accepted an unknown format")
	}
}

func TestOpenInputMissingFile(t *testing.T) {
	t.Parallel()
	stdinMu.Lock()
	defer stdinMu.Unlock()
	if _, _, err := openInput([]string{"no-such-file.bin"}, "bin"); err == nil {
		t.Error("openInput accepted a missing file")
	}
}
