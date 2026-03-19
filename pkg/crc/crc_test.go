package crc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/crc"
)

func TestComputeCRC16(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want uint16
	}{
		{
			name: "standard ASCII 123456789",
			data: []byte("123456789"),
			want: 0x29B1,
		},
		{
			name: "empty input",
			data: []byte{},
			want: 0xFFFF,
		},
		{
			name: "single zero byte",
			data: []byte{0x00},
			want: 0xE1F0,
		},
		{
			name: "single 0xFF byte",
			data: []byte{0xFF},
			want: 0xFF00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := crc.ComputeCRC16(tt.data)
			if got != tt.want {
				t.Errorf("ComputeCRC16(%x) = 0x%04X, want 0x%04X", tt.data, got, tt.want)
			}
		})
	}
}
