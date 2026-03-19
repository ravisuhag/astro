package sdl_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/sdl"
)

func TestMultiplexer_RoundRobin(t *testing.T) {
	mux := sdl.NewMultiplexer[string]()
	ch1 := sdl.NewChannel[string](1, 10)
	ch2 := sdl.NewChannel[string](2, 10)
	mux.AddChannel(ch1, 1)
	mux.AddChannel(ch2, 1)

	ch1.Add("a")
	ch2.Add("b")

	got1, _ := mux.Next()
	got2, _ := mux.Next()
	if got1 != "a" {
		t.Errorf("first = %q, want 'a'", got1)
	}
	if got2 != "b" {
		t.Errorf("second = %q, want 'b'", got2)
	}
}

func TestMultiplexer_WeightedPriority(t *testing.T) {
	mux := sdl.NewMultiplexer[string]()
	ch1 := sdl.NewChannel[string](1, 10)
	ch2 := sdl.NewChannel[string](2, 10)
	mux.AddChannel(ch1, 2)
	mux.AddChannel(ch2, 1)

	for range 3 {
		ch1.Add("hi")
		ch2.Add("lo")
	}

	// Priority 2:1 → ch1, ch1, ch2, ch1, ch2, ...
	expected := []string{"hi", "hi", "lo", "hi", "lo"}
	for i, want := range expected {
		got, _ := mux.Next()
		if got != want {
			t.Errorf("frame %d = %q, want %q", i, got, want)
		}
	}
}

func TestMultiplexer_NoChannels(t *testing.T) {
	mux := sdl.NewMultiplexer[string]()
	_, err := mux.Next()
	if !errors.Is(err, sdl.ErrNoChannels) {
		t.Errorf("expected ErrNoChannels, got %v", err)
	}
}

func TestMultiplexer_AllEmpty(t *testing.T) {
	mux := sdl.NewMultiplexer[string]()
	ch := sdl.NewChannel[string](1, 10)
	mux.AddChannel(ch, 1)
	_, err := mux.Next()
	if !errors.Is(err, sdl.ErrNoFramesAvailable) {
		t.Errorf("expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestMultiplexer_HasPending(t *testing.T) {
	mux := sdl.NewMultiplexer[string]()
	ch := sdl.NewChannel[string](1, 10)
	mux.AddChannel(ch, 1)

	if mux.HasPending() {
		t.Error("expected false")
	}
	ch.Add("a")
	if !mux.HasPending() {
		t.Error("expected true")
	}
}

func TestMultiplexer_Len(t *testing.T) {
	mux := sdl.NewMultiplexer[string]()
	if mux.Len() != 0 {
		t.Errorf("Len = %d, want 0", mux.Len())
	}
	mux.AddChannel(sdl.NewChannel[string](1, 10), 1)
	mux.AddChannel(sdl.NewChannel[string](2, 10), 1)
	if mux.Len() != 2 {
		t.Errorf("Len = %d, want 2", mux.Len())
	}
}
