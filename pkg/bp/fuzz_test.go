package bp_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
)

func FuzzDecodeBundle(f *testing.F) {
	primary := &bp.PrimaryBlock{
		Destination:       bp.IPNEndpoint(2, 1),
		Source:            bp.IPNEndpoint(1, 1),
		ReportTo:          bp.IPNEndpoint(1, 0),
		Custodian:         bp.NullEndpoint,
		CreationTimestamp: bp.CreationTimestamp{Time: 1, SequenceNumber: 1},
		Lifetime:          3600,
	}

	// A plain bundle.
	if b, err := bp.NewBundle(primary, []byte("payload")); err == nil {
		if encoded, err := b.Encode(); err == nil {
			f.Add(encoded)
		}
	}

	// One with an ECOS block.
	if b, err := bp.NewBundle(primary, []byte("payload"),
		bp.WithECOS(bp.ECOS{Flags: bp.ECOSCritical, Ordinal: 9})); err == nil {
		if encoded, err := b.Encode(); err == nil {
			f.Add(encoded)
		}
	}

	// A fragment.
	fragPrimary := *primary
	fragPrimary.Flags |= bp.FlagFragment
	fragPrimary.FragmentOffset = 100
	fragPrimary.TotalADULength = 1000
	if b, err := bp.NewBundle(&fragPrimary, []byte("piece")); err == nil {
		if encoded, err := b.Encode(); err == nil {
			f.Add(encoded)
		}
	}

	// An administrative record.
	adminPrimary := *primary
	adminPrimary.Flags |= bp.FlagAdminRecord
	report := &bp.StatusReport{
		Flags:             bp.StatusDelivered,
		DeliveryTime:      bp.DTNTime{Seconds: 1},
		CreationTimestamp: bp.CreationTimestamp{Time: 1, SequenceNumber: 1},
		SourceEndpoint:    bp.IPNEndpoint(1, 1),
	}
	if payload, err := bp.NewStatusReportRecord(report).Encode(); err == nil {
		if b, err := bp.NewBundle(&adminPrimary, payload); err == nil {
			if encoded, err := b.Encode(); err == nil {
				f.Add(encoded)
			}
		}
	}

	f.Add([]byte{})
	f.Add([]byte{6})
	f.Add(make([]byte, 32))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic, and a bundle that decodes must re-encode.
		b, err := bp.DecodeBundleWithOptions(data, bp.DecodeOptions{
			MaxBlockLength: 1 << 16,
			MaxBlocks:      16,
		})
		if err != nil {
			return
		}
		if _, err := b.Encode(); err != nil {
			t.Fatalf("a decoded bundle failed to re-encode: %v", err)
		}

		// The parsed views must not panic either.
		_, _ = b.Payload()
		_, _ = b.ECOS()
		if b.Primary.IsAdminRecord() {
			_, _ = b.AdminRecord()
		}
	})
}

func FuzzDecodeAdminRecord(f *testing.F) {
	report := &bp.StatusReport{
		Flags:             bp.StatusReceived | bp.StatusDelivered,
		ReceiptTime:       bp.DTNTime{Seconds: 1, Nanoseconds: 2},
		DeliveryTime:      bp.DTNTime{Seconds: 3},
		CreationTimestamp: bp.CreationTimestamp{Time: 1, SequenceNumber: 1},
		SourceEndpoint:    bp.IPNEndpoint(1, 1),
	}
	if encoded, err := bp.NewStatusReportRecord(report).Encode(); err == nil {
		f.Add(encoded)
	}

	signal := &bp.CustodySignal{
		Succeeded:         true,
		SignalTime:        bp.DTNTime{Seconds: 1},
		CreationTimestamp: bp.CreationTimestamp{Time: 1, SequenceNumber: 1},
		SourceEndpoint:    bp.IPNEndpoint(1, 1),
	}
	if encoded, err := bp.NewCustodySignalRecord(signal).Encode(); err == nil {
		f.Add(encoded)
	}

	f.Add([]byte{})
	f.Add([]byte{0x10})
	f.Add([]byte{0x21, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic.
		record, err := bp.DecodeAdminRecord(data)
		if err != nil {
			return
		}
		if _, err := record.Encode(); err != nil {
			return
		}
	})
}

func FuzzReassemble(f *testing.F) {
	f.Add(uint64(1000), 100, 5)
	f.Add(uint64(0), 1, 1)
	f.Add(uint64(37), 8, 3)

	f.Fuzz(func(t *testing.T, size uint64, maxPayload, keep int) {
		// Property: fragmentation and reassembly never panic, and whenever
		// reassembly succeeds it reproduces the original payload exactly.
		if size > 1<<16 || maxPayload <= 0 || maxPayload > 1<<16 {
			return
		}

		payload := make([]byte, size)
		for i := range payload {
			payload[i] = byte(i)
		}

		primary := &bp.PrimaryBlock{
			Destination:       bp.IPNEndpoint(2, 1),
			Source:            bp.IPNEndpoint(1, 1),
			ReportTo:          bp.NullEndpoint,
			Custodian:         bp.NullEndpoint,
			CreationTimestamp: bp.CreationTimestamp{Time: 1, SequenceNumber: 1},
		}
		b, err := bp.NewBundle(primary, payload)
		if err != nil {
			return
		}

		fragments, err := b.Fragment(maxPayload)
		if err != nil {
			return
		}
		if keep < 0 || keep > len(fragments) {
			keep = len(fragments)
		}

		rebuilt, err := bp.Reassemble(fragments[:keep])
		if err != nil {
			return
		}
		got, err := rebuilt.Payload()
		if err != nil {
			t.Fatalf("a reassembled bundle has no payload: %v", err)
		}
		if len(got) != len(payload) {
			t.Fatalf("reassembled %d octets, want %d", len(got), len(payload))
		}
		for i := range got {
			if got[i] != payload[i] {
				t.Fatalf("octet %d differs after reassembly", i)
			}
		}
	})
}
