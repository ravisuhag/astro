package sle_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/sle"
)

// Hand-encoded wire vectors for the encodings the audit found broken: the
// service instance identifier's OBJECT IDENTIFIERs, the primitive
// PEER-ABORT, the lossFrameSync notification, and the CltuLastProcessed
// CHOICE. Each expected byte string is written out from the ASN.1 by hand,
// so a regression cannot hide behind a symmetric round trip.

func TestBindInvocationSIIWireVector(t *testing.T) {
	invocation := &sle.BindInvocation{
		InitiatorIdentifier:     "CTRL",
		ResponderPortIdentifier: "PORT",
		ServiceType:             sle.AppReturnAllFrames,
		VersionNumber:           2,
		ServiceInstanceIdentifier: sle.ServiceInstanceIdentifier{
			{Identifier: "sagr", Value: "3"},
			{Identifier: "raf", Value: "onlc1"},
		},
	}
	got, err := invocation.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}

	want := []byte{
		0x80, 0x00, // credentials: unused [0] NULL
		0x1A, 0x04, 'C', 'T', 'R', 'L', // initiator VisibleString
		0x1A, 0x04, 'P', 'O', 'R', 'T', // responder port VisibleString
		0x02, 0x01, 0x00, // serviceType: rtnAllFrames (0)
		0x02, 0x01, 0x02, // versionNumber: 2
		0x30, 0x24, // serviceInstanceIdentifier SEQUENCE, 36 octets
		// SET { SEQUENCE { sagr OID 1.3.112.4.3.1.2.52, "3" } }
		0x31, 0x0E, 0x30, 0x0C,
		0x06, 0x07, 0x2B, 0x70, 0x04, 0x03, 0x01, 0x02, 0x34,
		0x1A, 0x01, '3',
		// SET { SEQUENCE { raf OID 1.3.112.4.3.1.2.22, "onlc1" } }
		0x31, 0x12, 0x30, 0x10,
		0x06, 0x07, 0x2B, 0x70, 0x04, 0x03, 0x01, 0x02, 0x16,
		0x1A, 0x05, 'o', 'n', 'l', 'c', '1',
	}
	if !bytes.Equal(got, want) {
		t.Errorf("BIND invocation:\n got  % X\n want % X", got, want)
	}

	// And back: the OIDs resolve to their operator names.
	decoded, err := sle.DecodeBindInvocation(got)
	if err != nil {
		t.Fatalf("DecodeBindInvocation() = %v", err)
	}
	sii := decoded.ServiceInstanceIdentifier
	if len(sii) != 2 || sii[0].Identifier != "sagr" || sii[1].Identifier != "raf" {
		t.Errorf("decoded SII = %v", sii)
	}
	if sii[0].Legacy || sii[1].Legacy {
		t.Error("OID-form attributes flagged as legacy")
	}
}

func TestBindInvocationAcceptsLegacyStringSII(t *testing.T) {
	// The pre-OID form this package once emitted: identifiers as
	// VisibleStrings. Still decodable, and flagged.
	legacy := []byte{
		0x80, 0x00,
		0x1A, 0x04, 'C', 'T', 'R', 'L',
		0x1A, 0x04, 'P', 'O', 'R', 'T',
		0x02, 0x01, 0x00,
		0x02, 0x01, 0x02,
		0x30, 0x0D,
		0x31, 0x0B, 0x30, 0x09,
		0x1A, 0x03, 'r', 'a', 'f',
		0x1A, 0x02, 'o', '1',
	}
	decoded, err := sle.DecodeBindInvocation(legacy)
	if err != nil {
		t.Fatalf("DecodeBindInvocation() = %v", err)
	}
	sii := decoded.ServiceInstanceIdentifier
	if len(sii) != 1 || sii[0].Identifier != "raf" || sii[0].Value != "o1" {
		t.Fatalf("decoded SII = %v", sii)
	}
	if !sii[0].Legacy {
		t.Error("a VisibleString identifier was not flagged as legacy")
	}
}

func TestPeerAbortWireVector(t *testing.T) {
	// Annex A2.2: peerAbortInvocation [104] IMPLICIT SlePeerAbort, and the
	// module is IMPLICIT TAGS, a primitive [104] holding the bare
	// diagnostic octet. 104 needs the high-tag form: 9F 68.
	assoc, err := sle.NewAssociation(sle.AssociationConfig{
		Role:            sle.RoleUser,
		LocalIdentifier: "CTRL-CENTRE",
	})
	if err != nil {
		t.Fatal(err)
	}
	user, err := sle.NewRAFUser(sle.ServiceConfig{
		Association:   assoc,
		Version:       5,
		ResponderPort: "PORT",
	})
	if err != nil {
		t.Fatal(err)
	}

	user.PeerAbort(sle.AbortProtocolError, time.Unix(0, 0))
	pdu, ok := user.NextPDU()
	if !ok {
		t.Fatal("no PEER-ABORT was queued")
	}
	want := []byte{0x9F, 0x68, 0x01, 0x03}
	if !bytes.Equal(pdu, want) {
		t.Errorf("PEER-ABORT = % X, want % X", pdu, want)
	}

	// And back through the PDU decoder.
	decoded, err := sle.DecodePDU(want, sle.ServiceRAF)
	if err != nil {
		t.Fatalf("DecodePDU() = %v", err)
	}
	if decoded.Operation != sle.OpPeerAbort {
		t.Fatalf("operation = %v, want PEER-ABORT", decoded.Operation)
	}
	abort, err := sle.DecodePeerAbort(decoded.Content)
	if err != nil {
		t.Fatalf("DecodePeerAbort() = %v", err)
	}
	if abort.Diagnostic != sle.AbortProtocolError {
		t.Errorf("diagnostic = %v, want protocol error", abort.Diagnostic)
	}
	if abort.UrgentData() != 0x03 {
		t.Errorf("urgent data octet = %#02x, want 0x03", abort.UrgentData())
	}
}

func TestLossFrameSyncWireVector(t *testing.T) {
	notification := &sle.SyncNotifyInvocation{
		Kind: sle.NotifyLossFrameSync,
		LockStatus: &sle.LockStatusReport{
			Time:                 sle.Time{Days: 1, Milliseconds: 2, Microseconds: 3},
			CarrierLockStatus:    sle.LockInLock,
			SubcarrierLockStatus: sle.LockOutOfLock,
			SymbolSyncLockStatus: sle.LockNotInUse,
		},
	}
	got, err := notification.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}

	// lossFrameSync [0] IMPLICIT SEQUENCE: the tag replaces the SEQUENCE's,
	// so the four fields sit directly under [0].
	want := []byte{
		0x80, 0x00, // credentials: unused [0] NULL
		0xA0, 0x13, // lossFrameSync [0], constructed, 19 octets
		0x80, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x03, // time [0] 8 octets
		0x02, 0x01, 0x00, // carrier: in lock
		0x02, 0x01, 0x01, // subcarrier: out of lock
		0x02, 0x01, 0x02, // symbol sync: not in use
	}
	if !bytes.Equal(got, want) {
		t.Errorf("SYNC-NOTIFY:\n got  % X\n want % X", got, want)
	}

	decoded, err := sle.DecodeSyncNotifyInvocation(want)
	if err != nil {
		t.Fatalf("DecodeSyncNotifyInvocation() = %v", err)
	}
	if decoded.Kind != sle.NotifyLossFrameSync || decoded.LockStatus == nil {
		t.Fatal("the lossFrameSync alternative did not decode")
	}
	if decoded.LockStatus.SubcarrierLockStatus != sle.LockOutOfLock {
		t.Errorf("subcarrier = %v, want out of lock", decoded.LockStatus.SubcarrierLockStatus)
	}
}

func TestCltuLastProcessedWireVector(t *testing.T) {
	processed := sle.CltuLastProcessed{
		Processed:          true,
		CltuIdentification: 42,
		// RadiationStartTime undefined: [0] NULL.
		Status: sle.CltuRadiated,
	}
	got := sle.AppendCltuLastProcessed(nil, processed)

	// cltuProcessed [1] IMPLICIT SEQUENCE: fields directly under [1].
	want := []byte{
		0xA1, 0x08, // cltuProcessed [1], constructed, 8 octets
		0x02, 0x01, 0x2A, // cltuIdentification: 42
		0x80, 0x00, // radiationStartTime: undefined [0] NULL
		0x02, 0x01, 0x00, // cltuStatus: radiated
	}
	if !bytes.Equal(got, want) {
		t.Errorf("CltuLastProcessed:\n got  % X\n want % X", got, want)
	}

	e, err := sle.NewDecoder(want).Next()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := sle.DecodeCltuLastProcessed(e)
	if err != nil {
		t.Fatalf("DecodeCltuLastProcessed() = %v", err)
	}
	if !decoded.Processed || decoded.CltuIdentification != 42 || decoded.Status != sle.CltuRadiated {
		t.Errorf("decoded = %+v", decoded)
	}

	// CltuLastOk takes the same implicit-tag shape.
	ok := sle.CltuLastOk{
		Ok:                 true,
		CltuIdentification: 7,
		RadiationStopTime:  sle.Time{Days: 1, Milliseconds: 2, Microseconds: 3},
	}
	gotOk := sle.AppendCltuLastOk(nil, ok)
	wantOk := []byte{
		0xA1, 0x0D,
		0x02, 0x01, 0x07,
		0x80, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x03,
	}
	if !bytes.Equal(gotOk, wantOk) {
		t.Errorf("CltuLastOk:\n got  % X\n want % X", gotOk, wantOk)
	}
}

func TestObjectIdentifierRoundTrip(t *testing.T) {
	oid := []uint32{1, 3, 112, 4, 3, 1, 2, 52}
	encoded, err := sle.AppendObjectIdentifier(nil, oid)
	if err != nil {
		t.Fatalf("AppendObjectIdentifier() = %v", err)
	}
	want := []byte{0x06, 0x07, 0x2B, 0x70, 0x04, 0x03, 0x01, 0x02, 0x34}
	if !bytes.Equal(encoded, want) {
		t.Errorf("OID = % X, want % X", encoded, want)
	}

	e, err := sle.NewDecoder(encoded).Next()
	if err != nil {
		t.Fatal(err)
	}
	arcs, err := e.ObjectIdentifier()
	if err != nil {
		t.Fatalf("ObjectIdentifier() = %v", err)
	}
	if len(arcs) != len(oid) {
		t.Fatalf("arcs = %v, want %v", arcs, oid)
	}
	for i := range oid {
		if arcs[i] != oid[i] {
			t.Fatalf("arcs = %v, want %v", arcs, oid)
		}
	}
}

func TestGetParameterRoundTrip(t *testing.T) {
	invocation := &sle.GetParameterInvocation{InvokeId: 9, Parameter: 6}
	encoded, err := invocation.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	gotInvocation, err := sle.DecodeGetParameterInvocation(encoded)
	if err != nil {
		t.Fatalf("DecodeGetParameterInvocation() = %v", err)
	}
	if gotInvocation.InvokeId != 9 || gotInvocation.Parameter != 6 {
		t.Errorf("decoded invocation = %+v", gotInvocation)
	}

	// Negative return: unknown parameter, the clean answer for a parameter
	// this package does not model.
	negative := &sle.GetParameterReturn{
		InvokeId:           9,
		SpecificDiagnostic: sle.GetParameterUnknown,
	}
	encoded, err = negative.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	gotReturn, err := sle.DecodeGetParameterReturn(encoded)
	if err != nil {
		t.Fatalf("DecodeGetParameterReturn() = %v", err)
	}
	if gotReturn.Positive || gotReturn.UsedCommon || gotReturn.SpecificDiagnostic != sle.GetParameterUnknown {
		t.Errorf("decoded return = %+v", gotReturn)
	}

	// Positive return: the parameter CHOICE travels as raw BER.
	parameter := sle.AppendTaggedInteger(nil, 6, 1) // e.g. deliveryMode
	positive := &sle.GetParameterReturn{InvokeId: 10, Positive: true, Parameter: parameter}
	encoded, err = positive.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	gotReturn, err = sle.DecodeGetParameterReturn(encoded)
	if err != nil {
		t.Fatalf("DecodeGetParameterReturn() = %v", err)
	}
	if !gotReturn.Positive || !bytes.Equal(gotReturn.Parameter, parameter) {
		t.Errorf("decoded return = %+v", gotReturn)
	}
}

func TestProviderRefusesDuplicateInvokeId(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	assoc, err := sle.NewAssociation(sle.AssociationConfig{
		Role:            sle.RoleProvider,
		LocalIdentifier: "GROUND-STN",
	})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := sle.NewRAFProvider(sle.ServiceConfig{
		Association:   assoc,
		Version:       5,
		ResponderPort: "PORT",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Bind, so the instance is in state 2 and START is decodable there.
	bind := &sle.BindInvocation{
		InitiatorIdentifier:       "CTRL-CENTRE",
		ResponderPortIdentifier:   "PORT",
		ServiceType:               sle.AppReturnAllFrames,
		VersionNumber:             5,
		ServiceInstanceIdentifier: sle.ServiceInstanceIdentifier{{Identifier: "raf", Value: "onlc1"}},
	}
	bindContent, err := bind.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.HandlePDU(sle.AppendPDU(nil, 100, bindContent), now); err != nil {
		t.Fatalf("HandlePDU(BIND) = %v", err)
	}
	for {
		if _, ok := provider.NextPDU(); !ok {
			break
		}
	}

	start := &sle.RAFStartInvocation{InvokeId: 4, RequestedFrameQuality: sle.FrameQualityAll}
	content, err := start.Encode()
	if err != nil {
		t.Fatal(err)
	}
	pdu := sle.AppendPDU(nil, 0, content)

	if _, err := provider.HandlePDU(pdu, now); err != nil {
		t.Fatalf("first START = %v", err)
	}

	// The same invoke identifier again: refused with the diagnostic, and a
	// negative return queued.
	if _, err := provider.HandlePDU(pdu, now); !errors.Is(err, sle.ErrDuplicateInvokeId) {
		t.Fatalf("second START = %v, want ErrDuplicateInvokeId", err)
	}
	answer, ok := provider.NextPDU()
	if !ok {
		t.Fatal("no negative return was queued")
	}
	decoded, err := sle.DecodePDU(answer, sle.ServiceRAF)
	if err != nil || decoded.Operation != sle.OpStartReturn {
		t.Fatalf("queued answer = %v, %v", decoded, err)
	}
	startReturn, err := sle.DecodeRAFStartReturn(decoded.Content)
	if err != nil {
		t.Fatal(err)
	}
	if startReturn.Positive || !startReturn.UsedCommon ||
		startReturn.CommonDiagnostic != sle.DiagDuplicateInvokeId {
		t.Errorf("negative return = %+v", startReturn)
	}
}

func TestAuthLevelAllChecksServicePDUs(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	assoc, err := sle.NewAssociation(sle.AssociationConfig{
		Role:            sle.RoleProvider,
		LocalIdentifier: "GROUND-STN",
		PeerIdentifier:  "CTRL-CENTRE",
		UserName:        "GROUND-STN",
		Password:        []byte("provider-secret"),
		PeerPassword:    []byte("user-secret"),
		AuthLevel:       sle.AuthLevelAll,
	})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := sle.NewRAFProvider(sle.ServiceConfig{
		Association:   assoc,
		Version:       5,
		ResponderPort: "PORT",
	})
	if err != nil {
		t.Fatal(err)
	}

	creds, err := sle.GenerateCredentials(now, 7, "CTRL-CENTRE", []byte("user-secret"))
	if err != nil {
		t.Fatal(err)
	}
	bind := &sle.BindInvocation{
		Credentials:               creds,
		InitiatorIdentifier:       "CTRL-CENTRE",
		ResponderPortIdentifier:   "PORT",
		ServiceType:               sle.AppReturnAllFrames,
		VersionNumber:             5,
		ServiceInstanceIdentifier: sle.ServiceInstanceIdentifier{{Identifier: "raf", Value: "onlc1"}},
	}
	bindContent, err := bind.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.HandlePDU(sle.AppendPDU(nil, 100, bindContent), now); err != nil {
		t.Fatalf("HandlePDU(BIND) = %v", err)
	}

	// A START with the unused-credentials alternative must be refused at
	// authentication level 'all'.
	bare, err := (&sle.RAFStartInvocation{InvokeId: 1, RequestedFrameQuality: sle.FrameQualityAll}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.HandlePDU(sle.AppendPDU(nil, 0, bare), now); !errors.Is(err, sle.ErrInvalidCredentials) {
		t.Fatalf("unauthenticated START = %v, want ErrInvalidCredentials", err)
	}

	// The same START with good credentials passes.
	startCreds, err := sle.GenerateCredentials(now, 8, "CTRL-CENTRE", []byte("user-secret"))
	if err != nil {
		t.Fatal(err)
	}
	signed, err := (&sle.RAFStartInvocation{
		Credentials:           startCreds,
		InvokeId:              1,
		RequestedFrameQuality: sle.FrameQualityAll,
	}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.HandlePDU(sle.AppendPDU(nil, 0, signed), now); err != nil {
		t.Fatalf("authenticated START = %v", err)
	}
}
