package tmdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

func TestChannelConfig_DataFieldCapacity(t *testing.T) {
	tests := []struct {
		name               string
		config             tmdl.ChannelConfig
		secondaryHeaderLen int
		want               int
	}{
		{
			name:               "minimal frame (header only)",
			config:             tmdl.ChannelConfig{FrameLength: 100},
			secondaryHeaderLen: 0,
			want:               94, // 100 - 6
		},
		{
			name:               "with OCF",
			config:             tmdl.ChannelConfig{FrameLength: 100, HasOCF: true},
			secondaryHeaderLen: 0,
			want:               90, // 100 - 6 - 4
		},
		{
			name:               "with FEC",
			config:             tmdl.ChannelConfig{FrameLength: 100, HasFEC: true},
			secondaryHeaderLen: 0,
			want:               92, // 100 - 6 - 2
		},
		{
			name:               "with OCF and FEC",
			config:             tmdl.ChannelConfig{FrameLength: 100, HasOCF: true, HasFEC: true},
			secondaryHeaderLen: 0,
			want:               88, // 100 - 6 - 4 - 2
		},
		{
			name:               "with secondary header",
			config:             tmdl.ChannelConfig{FrameLength: 100, HasOCF: true, HasFEC: true},
			secondaryHeaderLen: 3,
			want:               84, // 100 - 6 - (1+3) - 4 - 2
		},
		{
			name:               "CCSDS typical 1115-byte frame",
			config:             tmdl.ChannelConfig{FrameLength: 1115, HasOCF: true, HasFEC: true},
			secondaryHeaderLen: 0,
			want:               1103, // 1115 - 6 - 4 - 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.DataFieldCapacity(tt.secondaryHeaderLen)
			if got != tt.want {
				t.Errorf("DataFieldCapacity(%d) = %d, want %d", tt.secondaryHeaderLen, got, tt.want)
			}
		})
	}
}
