package sdl_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/sdl"
)

func TestChannel_AddNext(t *testing.T) {
	ch := sdl.NewChannel[string](1, 10)
	if err := ch.Add("frame-1"); err != nil {
		t.Fatal(err)
	}
	if ch.Len() != 1 {
		t.Errorf("Len = %d, want 1", ch.Len())
	}
	got, err := ch.Next()
	if err != nil {
		t.Fatal(err)
	}
	if got != "frame-1" {
		t.Errorf("got %q, want 'frame-1'", got)
	}
	if ch.Len() != 0 {
		t.Errorf("Len after Next = %d, want 0", ch.Len())
	}
}

func TestChannel_FIFO(t *testing.T) {
	ch := sdl.NewChannel[int](1, 10)
	_ = ch.Add(1)
	_ = ch.Add(2)
	_ = ch.Add(3)

	for _, want := range []int{1, 2, 3} {
		got, _ := ch.Next()
		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	}
}

func TestChannel_BufferFull(t *testing.T) {
	ch := sdl.NewChannel[string](1, 1)
	_ = ch.Add("a")
	err := ch.Add("b")
	if !errors.Is(err, sdl.ErrBufferFull) {
		t.Errorf("expected ErrBufferFull, got %v", err)
	}
}

func TestChannel_Empty(t *testing.T) {
	ch := sdl.NewChannel[string](1, 10)
	_, err := ch.Next()
	if !errors.Is(err, sdl.ErrNoFramesAvailable) {
		t.Errorf("expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestChannel_HasFrames(t *testing.T) {
	ch := sdl.NewChannel[string](1, 10)
	if ch.HasFrames() {
		t.Error("expected false")
	}
	_ = ch.Add("a")
	if !ch.HasFrames() {
		t.Error("expected true")
	}
}
