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
		APID:                2047,
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
			Version:             1,
			Type:                1,
			SecondaryHeaderFlag: test.flag,
			APID:                2047,
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
			Version:             1,
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
			Version:             1,
			Type:                1,
			SecondaryHeaderFlag: 0,
			APID:                2047,
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

func TestSecondaryHeaderEncodeDecode(t *testing.T) {
	original := &spp.SecondaryHeader{
		Timestamp: 1234567890,
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Failed to encode secondary header: %v", err)
	}

	decoded := &spp.SecondaryHeader{}
	err = decoded.Decode(encoded)
	if err != nil {
		t.Fatalf("Failed to decode secondary header: %v", err)
	}

	if original.Timestamp != decoded.Timestamp {
		t.Errorf("Decoded secondary header does not match original. Got %+v, want %+v", decoded, original)
	}
}
