package tmdl

// ChannelConfig defines the fixed parameters of a physical channel
// per CCSDS 132.0-B-3. All frames on a physical channel share the
// same fixed length and optional field configuration.
type ChannelConfig struct {
	FrameLength int  // Total frame length in octets (fixed per physical channel)
	HasOCF      bool // Whether Operational Control Field (4 bytes) is present
	HasFEC      bool // Whether Frame Error Control (2-byte CRC) is present
}

// DataFieldCapacity returns the maximum data field size available
// in frames on this physical channel.
func (c ChannelConfig) DataFieldCapacity() int {
	capacity := c.FrameLength - 6 // primary header is always 6 bytes
	if c.HasOCF {
		capacity -= 4
	}
	if c.HasFEC {
		capacity -= 2
	}
	return capacity
}
