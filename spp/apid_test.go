package spp_test

import (
	"github.com/ravisuhag/astro/spp"
	"testing"
)

func TestNewAPIDManager(t *testing.T) {
	manager := spp.NewAPIDManager()
	if manager == nil {
		t.Fatal("Expected non-nil APIDManager instance")
	}
}

func TestReserveAPID(t *testing.T) {
	manager := spp.NewAPIDManager()

	err := manager.ReserveAPID(100)
	if err != nil {
		t.Fatalf("Failed to reserve APID: %v", err)
	}

	if !manager.IsAPIDReserved(100) {
		t.Errorf("Expected APID 100 to be reserved")
	}

	err = manager.ReserveAPID(100)
	if err == nil {
		t.Errorf("Expected error when reserving already reserved APID")
	}

	err = manager.ReserveAPID(2048)
	if err == nil {
		t.Errorf("Expected error when reserving invalid APID")
	}
}

func TestReleaseAPID(t *testing.T) {
	manager := spp.NewAPIDManager()

	err := manager.ReserveAPID(100)
	if err != nil {
		t.Fatalf("Failed to reserve APID: %v", err)
	}

	manager.ReleaseAPID(100)

	if manager.IsAPIDReserved(100) {
		t.Errorf("Expected APID 100 to be released")
	}
}

func TestIsAPIDReserved(t *testing.T) {
	manager := spp.NewAPIDManager()

	if manager.IsAPIDReserved(100) {
		t.Errorf("Expected APID 100 to be not reserved initially")
	}

	err := manager.ReserveAPID(100)
	if err != nil {
		t.Fatalf("Failed to reserve APID: %v", err)
	}

	if !manager.IsAPIDReserved(100) {
		t.Errorf("Expected APID 100 to be reserved")
	}
}

func TestDescribeAPID(t *testing.T) {
	tests := []struct {
		apid     uint16
		expected string
	}{
		{0, "Idle Packet"},
		{1, "Telemetry Packet"},
		{2, "Command Packet"},
		{999, "Unknown or Custom Packet"},
	}

	for _, test := range tests {
		description := spp.DescribeAPID(test.apid)
		if description != test.expected {
			t.Errorf("Expected description %q, got %q", test.expected, description)
		}
	}
}
