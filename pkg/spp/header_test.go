package spp_test

import (
	"github.com/ravisuhag/astro/pkg/spp"
	"testing"
)

func TestPrimaryHeaderEncodeDecode(t *testing.T) {
	original := &spp.PrimaryHeader{
		Version:             0,
		Type:                1,
		SecondaryHeaderFlag: 0,
		APID:                100,
		SequenceFlags:       3,
		SequenceCount:       16383,
		PacketLength:        1023,
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Failed to encode primary header: %v", err)
	}

	decoded := &spp.PrimaryHeader{}
	err = decoded.Decode(encoded)
	if err != nil {
		t.Fatalf("Failed to decode primary header: %v", err)
	}

	if *original != *decoded {
		t.Errorf("Decoded primary header does not match original. Got %+v, want %+v", decoded, original)
	}
}

func TestPrimaryHeaderVersion(t *testing.T) {
	tests := []struct {
		version uint8
		isValid bool
	}{
		{0, true},
		{1, false},
		{7, false},
	}

	for _, test := range tests {
		ph := &spp.PrimaryHeader{
			Version:             test.version,
			Type:                1,
			SecondaryHeaderFlag: 0,
			APID:                100,
			SequenceFlags:       3,
			SequenceCount:       16383,
			PacketLength:        1023,
		}

		_, err := ph.Encode()
		if test.isValid && err != nil {
			t.Errorf("Expected valid version %d, but got error: %v", test.version, err)
		} else if !test.isValid && err == nil {
			t.Errorf("Expected error for invalid version %d, but got none", test.version)
		}
	}
}

func TestPrimaryHeaderSecondaryHeaderFlag(t *testing.T) {
	tests := []struct {
		flag    uint8
		isValid bool
	}{
		{0, true},
		{1, true},
		{2, false},
		{255, false},
	}

	for _, test := range tests {
		ph := &spp.PrimaryHeader{
			Version:             0,
			Type:                1,
			SecondaryHeaderFlag: test.flag,
			APID:                100,
			SequenceFlags:       3,
			SequenceCount:       16383,
			PacketLength:        1023,
		}

		_, err := ph.Encode()
		if test.isValid && err != nil {
			t.Errorf("Expected valid flag %d, but got error: %v", test.flag, err)
		} else if !test.isValid && err == nil {
			t.Errorf("Expected error for invalid flag %d, but got none", test.flag)
		}
	}
}

func TestPrimaryHeaderType(t *testing.T) {
	tests := []struct {
		pType   uint8
		isValid bool
	}{
		{0, true},
		{1, true},
		{2, false},
	}

	for _, test := range tests {
		ph := &spp.PrimaryHeader{
			Version:             0,
			Type:                test.pType,
			SecondaryHeaderFlag: 0,
			APID:                100,
			SequenceFlags:       3,
			SequenceCount:       16383,
			PacketLength:        1023,
		}

		_, err := ph.Encode()
		if test.isValid && err != nil {
			t.Errorf("Expected valid type %d, but got error: %v", test.pType, err)
		} else if !test.isValid && err == nil {
			t.Errorf("Expected error for invalid type %d, but got none", test.pType)
		}
	}
}

func TestPrimaryHeaderAPID(t *testing.T) {
	tests := []struct {
		apid    uint16
		isValid bool
	}{
		{0, true},
		{2047, true},
		{2048, false},
		{65535, false},
	}

	for _, test := range tests {
		ph := &spp.PrimaryHeader{
			Version:             0,
			Type:                1,
			SecondaryHeaderFlag: 0,
			APID:                test.apid,
			SequenceFlags:       3,
			SequenceCount:       16383,
			PacketLength:        1023,
		}

		_, err := ph.Encode()
		if test.isValid && err != nil {
			t.Errorf("Expected valid APID %d, but got error: %v", test.apid, err)
		} else if !test.isValid && err == nil {
			t.Errorf("Expected error for invalid APID %d, but got none", test.apid)
		}
	}
}

func TestPrimaryHeaderSequenceFlags(t *testing.T) {
	tests := []struct {
		flags   uint8
		isValid bool
	}{
		{0, true},
		{1, true},
		{2, true},
		{3, true},
		{4, false},
	}

	for _, test := range tests {
		ph := &spp.PrimaryHeader{
			Version:             0,
			Type:                1,
			SecondaryHeaderFlag: 0,
			APID:                100,
			SequenceFlags:       test.flags,
			SequenceCount:       16383,
			PacketLength:        1023,
		}

		_, err := ph.Encode()
		if test.isValid && err != nil {
			t.Errorf("Expected valid sequence flags %d, but got error: %v", test.flags, err)
		} else if !test.isValid && err == nil {
			t.Errorf("Expected error for invalid sequence flags %d, but got none", test.flags)
		}
	}
}

func TestPrimaryHeaderSequenceCount(t *testing.T) {
	tests := []struct {
		sequenceCount uint16
		isValid       bool
	}{
		{0, true},
		{16383, true},
		{16384, false},
		{65535, false},
	}

	for _, test := range tests {
		ph := &spp.PrimaryHeader{
			Version:             0,
			Type:                1,
			SecondaryHeaderFlag: 0,
			APID:                100,
			SequenceFlags:       3,
			SequenceCount:       test.sequenceCount,
			PacketLength:        1023,
		}

		_, err := ph.Encode()
		if test.isValid && err != nil {
			t.Errorf("Expected valid SequenceCount %d, but got error: %v", test.sequenceCount, err)
		} else if !test.isValid && err == nil {
			t.Errorf("Expected error for invalid SequenceCount %d, but got none", test.sequenceCount)
		}
	}
}

func TestPrimaryHeaderDecodeDataTooShort(t *testing.T) {
	ph := &spp.PrimaryHeader{}
	err := ph.Decode([]byte{0x00, 0x01})
	if err == nil {
		t.Error("Expected error for short data, but got none")
	}
}
