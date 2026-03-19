package tcdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
)

func TestVirtualChannel_AddGetFrame(t *testing.T) {
	vc := tcdl.NewVirtualChannel(5, 10)
	frame, _ := tcdl.NewTCTransferFrame(42, 5, []byte("data"))
	if err := vc.Add(frame); err != nil {
		t.Fatal(err)
	}
	if vc.Len() != 1 {
		t.Errorf("Len = %d, want 1", vc.Len())
	}
	got, err := vc.Next()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.DataField, []byte("data")) {
		t.Error("got different frame")
	}
}

func TestVirtualChannel_BufferFull(t *testing.T) {
	vc := tcdl.NewVirtualChannel(1, 1)
	frame, _ := tcdl.NewTCTransferFrame(42, 1, []byte("a"))
	_ = vc.Add(frame)
	err := vc.Add(frame)
	if !errors.Is(err, tcdl.ErrBufferFull) {
		t.Errorf("expected ErrBufferFull, got %v", err)
	}
}
