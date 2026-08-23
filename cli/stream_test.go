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
	sizer := func([]byte) int { return maxStreamUnit + 1 }

	err := streamUnits(bytes.NewReader([]byte{1, 2, 3, 4}), sizer, 1,
		func([]byte) error { return nil }, nil)
	if err == nil {
		t.Error("streamUnits accepted a unit past the maximum")
	}
}

func TestStreamUnitsPropagatesHandlerError(t *testing.T) {
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

func TestHexFilterStripsLayout(t *testing.T) {
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
	source := strings.NewReader(strings.Repeat(" ", 100) + "abcd")

	got, err := io.ReadAll(newHexFilter(source))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if string(got) != "abcd" {
		t.Errorf("filtered to %q, want %q", got, "abcd")
	}
}

func TestOpenInputRejectsUnknownFormat(t *testing.T) {
	if _, _, err := openInput(nil, "base64"); err == nil {
		t.Error("openInput accepted an unknown format")
	}
}

func TestOpenInputMissingFile(t *testing.T) {
	if _, _, err := openInput([]string{"no-such-file.bin"}, "bin"); err == nil {
		t.Error("openInput accepted a missing file")
	}
}
