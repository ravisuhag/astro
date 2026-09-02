package sle_test

import (
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/sle"
)

func FuzzBERDecoder(f *testing.F) {
	f.Add(sle.AppendInteger(nil, 42))
	f.Add(sle.AppendOctetString(nil, []byte("content")))
	f.Add(sle.AppendSequence(nil, sle.AppendInteger(nil, 1)))
	f.Add(sle.AppendElement(nil, sle.ClassContext, true, 100, []byte{1, 2}))
	f.Add([]byte{})
	f.Add([]byte{0x30, 0x80})
	f.Add([]byte{0x02, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic, and every element reported must lie inside
		// the buffer it came from.
		d := sle.NewDecoderWithLimit(data, 1<<16)
		for i := 0; i < 64; i++ {
			e, err := d.Next()
			if err != nil {
				return
			}
			if len(e.Bytes) > len(data) {
				t.Fatalf("element content is %d octets from a %d-octet buffer",
					len(e.Bytes), len(data))
			}
			// The value readers must not panic on any content.
			_, _ = e.Int64()
			_, _ = e.Uint64()
			_, _ = e.Bool()
			_ = e.String()

			if e.Constructed {
				nested := d.Nested(e)
				for j := 0; j < 16; j++ {
					if _, err := nested.Next(); err != nil {
						break
					}
				}
			}
		}
	})
}

func FuzzDecodeTMLMessage(f *testing.F) {
	if encoded, err := (&sle.Message{Type: sle.MessageSLEPDU, Body: []byte("pdu")}).Encode(); err == nil {
		f.Add(encoded)
	}
	if encoded, err := (&sle.ContextMessage{HeartbeatInterval: 30, DeadFactor: 3}).Message().Encode(); err == nil {
		f.Add(encoded)
	}
	if encoded, err := sle.HeartbeatMessage().Encode(); err == nil {
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add(make([]byte, 8))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic, and a message that decodes must re-encode to
		// exactly the octets it consumed.
		m, consumed, err := sle.DecodeMessageWithLimit(data, 1<<16)
		if err != nil {
			return
		}
		reEncoded, err := m.Encode()
		if err != nil {
			t.Fatalf("a decoded message failed to re-encode: %v", err)
		}
		if len(reEncoded) != consumed {
			t.Fatalf("re-encoded %d octets, consumed %d", len(reEncoded), consumed)
		}

		// A context message body that fails to parse is a legitimate error,
		// not a bug: TML framing validates the message shape, and ISP1
		// semantics (the protocol ID and version) are a layer above it.
		// What matters here is that parsing never panics.
		if m.Type == sle.MessageContext {
			_, _ = sle.DecodeContextMessage(m.Body)
		}
	})
}

func FuzzDecodeSLEPDUs(f *testing.F) {
	b := &sle.BindInvocation{
		InitiatorIdentifier:     "CTRL-CENTRE",
		ResponderPortIdentifier: "PORT",
		ServiceType:             sle.AppReturnAllFrames,
		VersionNumber:           5,
	}
	if encoded, err := b.Encode(); err == nil {
		f.Add(encoded)
	}
	r := &sle.BindReturn{ResponderIdentifier: "GROUND-STN", Positive: true, VersionNumber: 5}
	if encoded, err := r.Encode(); err == nil {
		f.Add(encoded)
	}
	if encoded, err := (&sle.UnbindInvocation{Reason: sle.UnbindEnd}).Encode(); err == nil {
		f.Add(encoded)
	}
	f.Add((&sle.PeerAbort{Diagnostic: sle.AbortProtocolError}).Encode())
	f.Add([]byte{})
	f.Add(make([]byte, 16))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: arbitrary bytes through every PDU decoder never panic.
		_, _ = sle.DecodeBindInvocation(data)
		_, _ = sle.DecodeBindReturn(data)
		_, _ = sle.DecodeUnbindInvocation(data)
		_, _ = sle.DecodeUnbindReturn(data)
		_, _ = sle.DecodePeerAbort(data)
		_, _ = sle.DecodeCredentials(data)
	})
}

func FuzzCredentialsVerify(f *testing.F) {
	f.Add([]byte("user"), []byte("password"), int32(1))
	f.Add([]byte(""), []byte(""), int32(0))

	f.Fuzz(func(t *testing.T, user, password []byte, random int32) {
		// Property: credentials verify against the inputs that made them, and
		// nothing panics on odd user names or passwords.
		if random < 0 {
			return
		}
		now := time.Unix(1700000000, 0).UTC()

		creds, err := sle.GenerateCredentials(now, random, string(user), password)
		if err != nil {
			return
		}
		if err := creds.Verify(now, time.Minute, string(user), password); err != nil {
			t.Fatalf("credentials failed to verify against their own inputs: %v", err)
		}

		encoded, err := creds.Encode()
		if err != nil {
			t.Fatalf("encoding failed: %v", err)
		}
		got, err := sle.DecodeCredentials(encoded)
		if err != nil {
			t.Fatalf("decoding our own credentials failed: %v", err)
		}
		if got.RandomNumber != random {
			t.Fatalf("random number = %d, want %d", got.RandomNumber, random)
		}
	})
}

// seedTime is a fixed CCSDS time for the service fuzz seeds.
func seedTime() sle.Time {
	t, err := sle.NewTime(time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC))
	if err != nil {
		return sle.Time{}
	}
	return t
}

// addEncoded seeds a corpus with an encodable PDU, skipping it if the encoder
// refuses.
func addEncoded(f *testing.F, encode func() ([]byte, error)) {
	if encoded, err := encode(); err == nil {
		f.Add(encoded)
	}
}

func FuzzDecodeRAFPDU(f *testing.F) {
	addEncoded(f, (&sle.RAFStartInvocation{
		InvokeId:              1,
		StartTime:             sle.ConditionalTime{Known: true, Time: seedTime()},
		RequestedFrameQuality: sle.FrameQualityAll,
	}).Encode)
	addEncoded(f, (&sle.RAFStartReturn{InvokeId: 1, Positive: true}).Encode)
	addEncoded(f, (&sle.RAFTransferDataInvocation{
		EarthReceiveTime: seedTime(),
		AntennaId:        sle.AntennaId{Local: []byte("DSS-25")},
		Data:             []byte{0x1A, 0xCF, 0xFC, 0x1D},
	}).Encode)
	addEncoded(f, (&sle.SyncNotifyInvocation{Kind: sle.NotifyEndOfData}).Encode)
	addEncoded(f, (&sle.RAFStatusReportInvocation{DeliveredFrameNumber: 3}).Encode)
	addEncoded(f, sle.RAFTransferBuffer{
		{Notification: &sle.SyncNotifyInvocation{Kind: sle.NotifyEndOfData}},
	}.Encode)
	f.Add([]byte{})
	f.Add([]byte{0x30, 0x80})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: arbitrary bytes through every RAF decoder never panic and
		// never allocate from an attacker-chosen length.
		_, _ = sle.DecodeRAFStartInvocation(data)
		_, _ = sle.DecodeRAFStartReturn(data)
		_, _ = sle.DecodeRAFTransferDataInvocation(data)
		_, _ = sle.DecodeSyncNotifyInvocation(data)
		_, _ = sle.DecodeRAFStatusReportInvocation(data)
		_, _ = sle.DecodeRAFTransferBuffer(data)
		_, _ = sle.DecodeStopInvocation(data)
		_, _ = sle.DecodeAcknowledgement(data)
		_, _ = sle.DecodeScheduleStatusReportInvocation(data)
		_, _ = sle.DecodeScheduleStatusReportReturn(data)
		_, _ = sle.DecodePDU(data, sle.ServiceRAF)
	})
}

func FuzzDecodeRCFPDU(f *testing.F) {
	channel := sle.GVCID{SpacecraftID: 42, VersionNumber: sle.FrameVersionTM, VirtualChannelID: 1}

	addEncoded(f, (&sle.RCFStartInvocation{InvokeId: 1, RequestedGVCID: channel}).Encode)
	addEncoded(f, (&sle.RCFStartReturn{InvokeId: 1, Positive: true}).Encode)
	addEncoded(f, (&sle.RCFTransferDataInvocation{
		EarthReceiveTime: seedTime(),
		AntennaId:        sle.AntennaId{Local: []byte("A")},
		Data:             []byte{1, 2, 3, 4},
	}).Encode)
	addEncoded(f, (&sle.RCFStatusReportInvocation{DeliveredFrameNumber: 7}).Encode)
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: no RCF decoder panics, including on a GVCID whose version
		// number names no frame format.
		_, _ = sle.DecodeRCFStartInvocation(data)
		_, _ = sle.DecodeRCFStartReturn(data)
		_, _ = sle.DecodeRCFTransferDataInvocation(data)
		_, _ = sle.DecodeRCFStatusReportInvocation(data)
		_, _ = sle.DecodeRCFTransferBuffer(data)
		_, _ = sle.DecodePDU(data, sle.ServiceRCF)
	})
}

func FuzzDecodeROCFPDU(f *testing.F) {
	channel := sle.GVCID{SpacecraftID: 42, VersionNumber: sle.FrameVersionAOS, VirtualChannelID: 1}

	addEncoded(f, (&sle.ROCFStartInvocation{
		InvokeId:        1,
		RequestedGVCID:  channel,
		ControlWordType: sle.ControlWordType{Kind: sle.ControlWordCLCW, TCVirtualChannel: 2, HasTCVirtualChannel: true},
		UpdateMode:      sle.UpdateChangeBased,
	}).Encode)
	addEncoded(f, (&sle.ROCFStartReturn{InvokeId: 1, Positive: true}).Encode)
	addEncoded(f, (&sle.ROCFTransferDataInvocation{
		EarthReceiveTime: seedTime(),
		AntennaId:        sle.AntennaId{Local: []byte("A")},
		Data:             []byte{0, 1, 2, 3},
	}).Encode)
	addEncoded(f, (&sle.ROCFStatusReportInvocation{DeliveredOCFsNumber: 9}).Encode)
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: no ROCF decoder panics, including on the nested
		// ControlWordType CHOICE.
		_, _ = sle.DecodeROCFStartInvocation(data)
		_, _ = sle.DecodeROCFStartReturn(data)
		_, _ = sle.DecodeROCFTransferDataInvocation(data)
		_, _ = sle.DecodeROCFStatusReportInvocation(data)
		_, _ = sle.DecodeROCFTransferBuffer(data)
		_, _ = sle.DecodePDU(data, sle.ServiceROCF)
	})
}

func FuzzDecodeFCLTUPDU(f *testing.F) {
	addEncoded(f, (&sle.FCLTUStartInvocation{InvokeId: 1, FirstCltuIdentification: 500}).Encode)
	addEncoded(f, (&sle.FCLTUStartReturn{InvokeId: 1, Positive: true, StartRadiationTime: seedTime()}).Encode)
	addEncoded(f, (&sle.FCLTUTransferDataInvocation{
		InvokeId:           1,
		CltuIdentification: 500,
		Data:               []byte{0xEB, 0x90, 0x00, 0x01},
	}).Encode)
	addEncoded(f, (&sle.FCLTUTransferDataReturn{InvokeId: 1, CltuIdentification: 501, Positive: true}).Encode)
	addEncoded(f, (&sle.FCLTUThrowEventInvocation{
		InvokeId:        1,
		EventIdentifier: 3,
		EventQualifier:  []byte("go"),
	}).Encode)
	addEncoded(f, (&sle.FCLTUThrowEventReturn{InvokeId: 1, Positive: true}).Encode)
	addEncoded(f, (&sle.FCLTUAsyncNotifyInvocation{Kind: sle.NotifyBufferEmpty}).Encode)
	addEncoded(f, (&sle.FCLTUStatusReportInvocation{NumberOfCltusRadiated: 4}).Encode)
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: no FCLTU decoder panics, including on the CltuLastProcessed
		// and CltuLastOk CHOICEs nested inside the notifications.
		_, _ = sle.DecodeFCLTUStartInvocation(data)
		_, _ = sle.DecodeFCLTUStartReturn(data)
		_, _ = sle.DecodeFCLTUTransferDataInvocation(data)
		_, _ = sle.DecodeFCLTUTransferDataReturn(data)
		_, _ = sle.DecodeFCLTUThrowEventInvocation(data)
		_, _ = sle.DecodeFCLTUThrowEventReturn(data)
		_, _ = sle.DecodeFCLTUAsyncNotifyInvocation(data)
		_, _ = sle.DecodeFCLTUStatusReportInvocation(data)
		_, _ = sle.DecodePDU(data, sle.ServiceFCLTU)
	})
}
