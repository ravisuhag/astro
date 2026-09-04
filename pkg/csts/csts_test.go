package csts_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/internal/ber"
	"github.com/ravisuhag/astro/pkg/csts"
)

// CCSDS 921.1-B-2 prints no worked example and no octets. It is an abstract
// specification with an ASN.1 annex, so there is nothing to transcribe the way
// annex G of the navigation standards lets us transcribe a file.
//
// The wire forms below are therefore derived from annex F rather than
// published, and each one is spelled out octet by octet so the derivation can
// be checked against the module. A round trip would only prove the encoder and
// the decoder agree with each other, and a misread of an ASN.1 module is
// exactly the kind of mistake they would agree on.
//
// The tagging is the part worth checking hardest. Every module in annex F uses
// IMPLICIT TAGS, so a CHOICE alternative's context tag *replaces* the tag of
// the type underneath it. Where an alternative is a SEQUENCE, the [n] is
// constructed and its content is the fields directly — not a SEQUENCE nested
// inside. Getting that wrong produces a PDU one level too deep, which this
// package's own reader would accept and no other implementation would.

// An UNBIND invocation, from annex F3.5:
//
//	UnbindInvocation ::= SEQUENCE
//	{ standardInvocationHeader  StandardInvocationHeader
//	, unbindInvocationExtension Extended }
//
//	a2 17                             [2] constructed, 23 octets
//	   30 13                          standardInvocationHeader, SEQUENCE
//	      80 00                       invokerCredentials, 'unused' [0]
//	      02 01 07                    invokeId, INTEGER 7
//	      30 0c                       procedureName, SEQUENCE
//	         06 08 2b7004040101 0301  procedureType, OID 1.3.112.4.4.1.1.3.1
//	         82 00                    procedureRole, 'associationControl' [2]
//	   81 00                          unbindInvocationExtension, 'notUsed' [1]
//
// The OID's first content octet is 0x2b, which is 43: X.690 clause 8.19.4
// packs the first two arcs as 40*1 + 3.
const unbindInvocationHex = "a21730138000020107300c06082b7004040101030182008100"

// A PEER-ABORT invocation, from annex F3.5:
//
//	PeerAbortInvocation ::= SEQUENCE { diagnostic PeerAbortDiagnostic }
//	PeerAbortDiagnostic ::= OCTET STRING (SIZE(1))
//
//	a4 03            [4] constructed, 3 octets
//	   04 01 28      diagnostic, OCTET STRING, 0x28 = 40 = accessDenied
//
// It is the one operation with no standard header, which clause 3.3.1.1 states
// outright: ISP1 carries an abort as a single octet, so there is nowhere to
// put one.
const peerAbortHex = "a403040128"

// An UNBIND return, from annex F3.5 and F3.3:
//
//	UnbindReturn ::= StandardReturnHeader
//
//	a3 09                [3] constructed, 9 octets
//	   80 00             performerCredentials, 'unused' [0]
//	   02 01 07          invokeId, INTEGER 7, copied from the invocation
//	   a0 02             result, 'positive' [0]
//	      81 00          Extended, 'notUsed' [1]
//
// This is the alternative that shows the tagging rule. UnbindReturn is another
// name for StandardReturnHeader, which is a SEQUENCE, so the [3] replaces that
// SEQUENCE's tag. There is no 30 in this encoding.
const unbindReturnHex = "a3098000020107a0028100"

func TestEncodeUnbindInvocation(t *testing.T) {
	pdu := &csts.PDU{
		Type: csts.OpUnbindInvocation,
		Unbind: &csts.UnbindInvocation{
			Header: csts.InvocationHeader{
				InvokeID: 7,
				Procedure: csts.ProcedureName{
					Type: csts.OIDAssociationControl,
					Role: csts.RoleAssociationControl,
				},
			},
		},
	}
	assertEncodes(t, pdu, unbindInvocationHex)

	back := mustDecode(t, unbindInvocationHex)
	if back.Type != csts.OpUnbindInvocation || back.Unbind == nil {
		t.Fatalf("decoded as %s", back.Type)
	}
	header, ok := back.Header()
	if !ok {
		t.Fatal("an UNBIND invocation should carry a standard invocation header")
	}
	if header.InvokeID != 7 {
		t.Errorf("invokeId = %d", header.InvokeID)
	}
	if header.InvokerCredentials.Used {
		t.Error("credentials should be 'unused'")
	}
	if !header.Procedure.Type.Equal(csts.OIDAssociationControl) {
		t.Errorf("procedure type = %s", header.Procedure.Type)
	}
	if header.Procedure.Role != csts.RoleAssociationControl {
		t.Errorf("procedure role = %s", header.Procedure.Role)
	}
}

func TestEncodePeerAbort(t *testing.T) {
	pdu := &csts.PDU{
		Type:      csts.OpPeerAbortInvocation,
		PeerAbort: &csts.PeerAbortInvocation{Diagnostic: csts.AbortAccessDenied},
	}
	assertEncodes(t, pdu, peerAbortHex)

	back := mustDecode(t, peerAbortHex)
	if back.PeerAbort == nil || back.PeerAbort.Diagnostic != csts.AbortAccessDenied {
		t.Fatalf("decoded peer abort = %+v", back.PeerAbort)
	}
	// Clause 3.3.1.1: the PEER-ABORT is the one invocation with no header.
	if _, ok := back.Header(); ok {
		t.Error("a PEER-ABORT should carry no standard invocation header")
	}
}

func TestEncodeUnbindReturn(t *testing.T) {
	pdu := &csts.PDU{
		Type:         csts.OpUnbindReturn,
		UnbindReturn: &csts.UnbindReturn{Header: csts.ReturnHeader{InvokeID: 7, Positive: true}},
	}
	assertEncodes(t, pdu, unbindReturnHex)

	back := mustDecode(t, unbindReturnHex)
	header, ok := back.ReturnHeader()
	if !ok {
		t.Fatal("an UNBIND return should carry a standard return header")
	}
	if !header.Positive || header.InvokeID != 7 {
		t.Errorf("return header = %+v", header)
	}
}

// The alternatives defined as 'X ::= StandardReturnHeader' must not carry a
// SEQUENCE inside their context tag. This checks the octets directly, because
// an extra level of nesting round-trips perfectly within one implementation.
func TestReturnHeaderIsNotDoubleWrapped(t *testing.T) {
	for _, operation := range []csts.OperationType{
		csts.OpUnbindReturn, csts.OpStartReturn, csts.OpStopReturn,
		csts.OpGetReturn, csts.OpProcessDataReturn,
		csts.OpExecuteDirectiveReturn, csts.OpExecuteDirectiveAcknowledge,
	} {
		pdu := &csts.PDU{Type: operation}
		if operation == csts.OpUnbindReturn {
			pdu.UnbindReturn = &csts.UnbindReturn{Header: csts.ReturnHeader{Positive: true}}
		} else {
			pdu.Return = &csts.StandardReturn{Header: csts.ReturnHeader{Positive: true}}
		}

		encoded, err := pdu.Encode()
		if err != nil {
			t.Fatalf("%s: Encode: %v", operation, err)
		}
		// Past the context tag and its length, the first octet is the
		// credentials CHOICE — 0x80 — and not a SEQUENCE tag of 0x30.
		if len(encoded) < 3 {
			t.Fatalf("%s: encoded to %d octets", operation, len(encoded))
		}
		if encoded[2] == 0x30 {
			t.Errorf("%s: a SEQUENCE was written inside the context tag: %x", operation, encoded)
		}
	}
}

// The framework PDU tag is the whole of what says which operation a message
// is, and annex F3.15 numbers the alternatives in tens rather than
// consecutively — [10] and [11] for the START pair, [20] and [21] for STOP —
// which leaves room between them for a future issue to add messages.
//
// A wrong tag here names a different operation rather than failing, so these
// are pinned against the module.
func TestOperationTags(t *testing.T) {
	tests := []struct {
		operation csts.OperationType
		tag       uint32
		name      string
	}{
		{csts.OpBindInvocation, 0, "BIND invocation"},
		{csts.OpBindReturn, 1, "BIND return"},
		{csts.OpUnbindInvocation, 2, "UNBIND invocation"},
		{csts.OpUnbindReturn, 3, "UNBIND return"},
		{csts.OpPeerAbortInvocation, 4, "PEER-ABORT invocation"},
		{csts.OpStartInvocation, 10, "START invocation"},
		{csts.OpStartReturn, 11, "START return"},
		{csts.OpStopInvocation, 20, "STOP invocation"},
		{csts.OpStopReturn, 21, "STOP return"},
		{csts.OpExecuteDirectiveInvocation, 30, "EXECUTE-DIRECTIVE invocation"},
		{csts.OpExecuteDirectiveAcknowledge, 31, "EXECUTE-DIRECTIVE acknowledgement"},
		{csts.OpExecuteDirectiveReturn, 32, "EXECUTE-DIRECTIVE return"},
		{csts.OpGetInvocation, 40, "GET invocation"},
		{csts.OpGetReturn, 41, "GET return"},
		{csts.OpNotifyInvocation, 50, "NOTIFY invocation"},
		{csts.OpProcessDataInvocation, 60, "PROCESS-DATA invocation"},
		{csts.OpProcessDataReturn, 61, "PROCESS-DATA return"},
		{csts.OpForwardBuffer, 62, "forward buffer"},
		{csts.OpTransferDataInvocation, 70, "TRANSFER-DATA invocation"},
		{csts.OpReturnBuffer, 71, "return buffer"},
	}

	if len(tests) != 20 {
		t.Fatalf("annex F3.15 has 20 alternatives, this table has %d", len(tests))
	}
	for _, tt := range tests {
		if uint32(tt.operation) != tt.tag {
			t.Errorf("%s = tag %d, want %d", tt.name, tt.operation, tt.tag)
		}
		if got := tt.operation.String(); got != tt.name {
			t.Errorf("tag %d names %q, want %q", tt.tag, got, tt.name)
		}
		if !tt.operation.Known() {
			t.Errorf("tag %d is not recognised", tt.tag)
		}
	}

	// A tag between the defined ones is not an operation. The gaps are room
	// for a future issue, not values to guess at.
	for _, tag := range []csts.OperationType{5, 12, 22, 42, 72, 200} {
		if tag.Known() {
			t.Errorf("tag %d was recognised as an operation", tag)
		}
	}
}

// The object identifier tree of annex F3.1. An OID is how CSTS identifies a
// procedure, and a wrong arc names a different one rather than failing.
func TestObjectIdentifiers(t *testing.T) {
	tests := []struct {
		oid  csts.OID
		want string
		name string
	}{
		{csts.OIDCSS, "1.3.112.4.4", "css"},
		{csts.OIDCSTS, "1.3.112.4.4.1", "csts"},
		{csts.OIDFramework, "1.3.112.4.4.1.1", "framework"},
		{csts.OIDServices, "1.3.112.4.4.1.2", "services"},
		{csts.OIDOperations, "1.3.112.4.4.1.1.2", "operations"},
		{csts.OIDProcedures, "1.3.112.4.4.1.1.3", "procedures"},
		{csts.OIDBindInvocation, "1.3.112.4.4.1.1.2.1", "bindInvocation"},
		{csts.OIDProcessDataReturn, "1.3.112.4.4.1.1.2.18", "processDataReturn"},
		{csts.OIDAssociationControl, "1.3.112.4.4.1.1.3.1", "associationControl"},
		{csts.OIDThrowEvent, "1.3.112.4.4.1.1.3.7", "throwEvent"},
		// A derived procedure sits under the procedure it derives from, not
		// beside it: Cyclic Report is {unbufferedDataDelivery 1 1}.
		{csts.OIDCyclicReport, "1.3.112.4.4.1.1.3.2.1.1", "cyclicReport"},
		{csts.OIDBufferedDataProcessing, "1.3.112.4.4.1.1.3.4.1.1", "bufferedDataProcessing"},
		{csts.OIDSequenceControlledDataProcessing, "1.3.112.4.4.1.1.3.4.1.2", "sequenceControlledDataProcessing"},
	}

	for _, tt := range tests {
		if got := tt.oid.String(); got != tt.want {
			t.Errorf("%s = %s, want %s", tt.name, got, tt.want)
		}
	}
}

func TestProcedureTypeNames(t *testing.T) {
	tests := []struct {
		oid  csts.OID
		want string
	}{
		{csts.OIDAssociationControl, "Association Control"},
		{csts.OIDBufferedDataDelivery, "Buffered Data Delivery"},
		{csts.OIDCyclicReport, "Cyclic Report"},
		{csts.OIDSequenceControlledDataProcessing, "Sequence-Controlled Data Processing"},
	}
	for _, tt := range tests {
		if got := csts.ProcedureTypeName(tt.oid); got != tt.want {
			t.Errorf("ProcedureTypeName(%s) = %q, want %q", tt.oid, got, tt.want)
		}
	}

	// A service specification may define its own procedure types, and this
	// package has no registry to look one up in. An empty answer is honest;
	// a guess would not be.
	if got := csts.ProcedureTypeName(csts.OID{1, 3, 112, 4, 4, 1, 2, 99}); got != "" {
		t.Errorf("an unregistered procedure type was named %q", got)
	}
}

func TestBindRoundTrip(t *testing.T) {
	original := &csts.PDU{
		Type: csts.OpBindInvocation,
		Bind: &csts.BindInvocation{
			Header: csts.InvocationHeader{
				InvokerCredentials: csts.Credentials{Used: true, Value: bytes.Repeat([]byte{0xAB}, 16)},
				InvokeID:           1,
				Procedure: csts.ProcedureName{
					Type: csts.OIDAssociationControl,
					Role: csts.RoleAssociationControl,
				},
			},
			InitiatorIdentifier:     "CNES",
			ResponderPortIdentifier: "PORT-A",
			ServiceType:             csts.OID{1, 3, 112, 4, 4, 1, 2, 1},
			VersionNumber:           2,
			ServiceInstance: csts.ServiceInstanceIdentifier{
				SpacecraftID:   csts.OID{1, 3, 112, 4, 3, 1},
				FacilityID:     csts.OID{1, 3, 112, 4, 3, 2},
				ServiceType:    csts.OID{1, 3, 112, 4, 4, 1, 2, 1},
				InstanceNumber: 3,
			},
		},
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := csts.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	bind := back.Bind
	if bind == nil {
		t.Fatalf("decoded as %s", back.Type)
	}
	if bind.InitiatorIdentifier != "CNES" || bind.ResponderPortIdentifier != "PORT-A" {
		t.Errorf("identifiers = %q / %q", bind.InitiatorIdentifier, bind.ResponderPortIdentifier)
	}
	if bind.VersionNumber != 2 {
		t.Errorf("version = %d", bind.VersionNumber)
	}
	if !bind.ServiceInstance.SpacecraftID.Equal(csts.OID{1, 3, 112, 4, 3, 1}) {
		t.Errorf("spacecraft ID = %s", bind.ServiceInstance.SpacecraftID)
	}
	if bind.ServiceInstance.InstanceNumber != 3 {
		t.Errorf("instance number = %d", bind.ServiceInstance.InstanceNumber)
	}
	if !bind.Header.InvokerCredentials.Used || len(bind.Header.InvokerCredentials.Value) != 16 {
		t.Errorf("credentials = %+v", bind.Header.InvokerCredentials)
	}

	again, err := back.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(again, encoded) {
		t.Errorf("the second encoding differs:\n%x\n%x", encoded, again)
	}
}

// A negative return carries a diagnostic, and the four defined alternatives
// each have their own shape. The extension alternative is carried as octets,
// because its syntax is named by the procedure rather than by this document.
func TestDiagnostics(t *testing.T) {
	tests := []struct {
		name       string
		diagnostic csts.Diagnostic
		want       string
	}{
		{
			name: "invalid parameter value",
			diagnostic: csts.Diagnostic{
				Kind:         csts.DiagnosticInvalidParameterValue,
				Text:         "out of range",
				Appellations: []string{"reportingCycle"},
			},
			want: "invalid parameter value: out of range (reportingCycle)",
		},
		{
			name: "conflicting values",
			diagnostic: csts.Diagnostic{
				Kind:         csts.DiagnosticConflictingValues,
				Text:         "cannot both be set",
				Appellations: []string{"startTime", "stopTime"},
			},
			want: "conflicting values: cannot both be set (startTime, stopTime)",
		},
		{
			name:       "other reason",
			diagnostic: csts.Diagnostic{Kind: csts.DiagnosticOtherReason, Text: "busy"},
			want:       "other reason: busy",
		},
		{
			name:       "unsupported option",
			diagnostic: csts.Diagnostic{Kind: csts.DiagnosticUnsupportedOption, Text: "picoseconds"},
			want:       "unsupported option: picoseconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdu := &csts.PDU{
				Type: csts.OpStartReturn,
				Return: &csts.StandardReturn{Header: csts.ReturnHeader{
					InvokeID:   5,
					Positive:   false,
					Diagnostic: tt.diagnostic,
				}},
			}
			encoded, err := pdu.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			back, err := csts.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			header, _ := back.ReturnHeader()
			if header.Positive {
				t.Fatal("a negative return decoded as positive")
			}
			if got := header.Diagnostic.String(); got != tt.want {
				t.Errorf("diagnostic = %q, want %q", got, tt.want)
			}
		})
	}
}

// The PEER-ABORT diagnostic is one octet whose value space is partitioned
// across the whole cross support family, and annex F3.5 sets out the
// partition. The same octet arrives from three different layers and means
// different things in each, so the origin is reported rather than a name being
// guessed at.
func TestPeerAbortPartition(t *testing.T) {
	tests := []struct {
		value  csts.PeerAbortDiagnostic
		origin csts.Origin
		name   string
	}{
		{0, csts.OriginSLE, "SLE diagnostic 0"},
		{39, csts.OriginSLE, "SLE diagnostic 39"},
		{40, csts.OriginAssociationControl, "access denied"},
		{41, csts.OriginAssociationControl, "unexpected responder id"},
		{51, csts.OriginAssociationControl, "unrecognized type"},
		{70, csts.OriginProcedure, "forward buffer too large"},
		{71, csts.OriginProcedure, "CSTS procedure diagnostic 71"},
		{126, csts.OriginOtherReason, "other reason"},
		{128, csts.OriginISP, "ISP1 diagnostic 128"},
		{200, csts.OriginApplication, "application diagnostic 200"},
		{250, csts.OriginApplication, "application diagnostic 250"},
		{255, csts.OriginUnallocated, "unallocated diagnostic 255"},
	}

	for _, tt := range tests {
		if got := tt.value.Origin(); got != tt.origin {
			t.Errorf("%d origin = %s, want %s", tt.value, got, tt.origin)
		}
		if got := tt.value.String(); got != tt.name {
			t.Errorf("%d = %q, want %q", tt.value, got, tt.name)
		}
	}

	// An application value means whatever its service type says, and this
	// package has no service type to ask — which is the same reason pkg/sle
	// refuses to decode a PDU without being told the service.
	if name := csts.PeerAbortDiagnostic(220).String(); strings.Contains(name, "denied") {
		t.Errorf("an application diagnostic was given a framework name: %q", name)
	}
}

func TestDecodeRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			// A tag in one of the gaps annex F3.15 leaves between operations.
			name:  "a CHOICE tag the framework does not define",
			input: "a5030401 28",
			want:  csts.ErrUnknownOperation,
		},
		{
			// A universal SEQUENCE where the CHOICE tag should be.
			name:  "not a context tag",
			input: "3003040128",
			want:  csts.ErrMalformedPDU,
		},
		{
			// Every alternative of annex F3.15 is a SEQUENCE, so the context
			// tag that replaces its universal tag is constructed. A primitive
			// one is a PDU no conforming peer produced. Found by FuzzDecode.
			name:  "a primitive alternative tag",
			input: "9e00",
			want:  csts.ErrMalformedPDU,
		},
		{
			name:  "octets after the PDU",
			input: peerAbortHex + "00",
			want:  csts.ErrTrailingContent,
		},
		{
			// Annex F3.5 fixes the peer abort diagnostic at one octet.
			name:  "a peer abort diagnostic of two octets",
			input: "a4040402 2828",
			want:  csts.ErrMalformedPDU,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := hex.DecodeString(strings.ReplaceAll(tt.input, " ", ""))
			if err != nil {
				t.Fatalf("bad test constant: %v", err)
			}
			if _, err := csts.Decode(data); !errors.Is(err, tt.want) {
				t.Errorf("Decode = %v, want %v", err, tt.want)
			}
		})
	}
}

// The SIZE constraints of annex F3.3 and F3.5 are checked on the way out as
// well as on the way in, so this package cannot produce a PDU it would refuse.
func TestConstraints(t *testing.T) {
	bind := func(mutate func(*csts.BindInvocation)) *csts.PDU {
		b := &csts.BindInvocation{
			Header: csts.InvocationHeader{
				InvokeID:  1,
				Procedure: csts.ProcedureName{Type: csts.OIDAssociationControl, Role: csts.RoleAssociationControl},
			},
			InitiatorIdentifier:     "CNES",
			ResponderPortIdentifier: "PORT-A",
			ServiceType:             csts.OID{1, 3, 112, 4, 4, 1, 2, 1},
			VersionNumber:           1,
			ServiceInstance: csts.ServiceInstanceIdentifier{
				SpacecraftID: csts.OID{1, 3, 6}, FacilityID: csts.OID{1, 3, 6},
				ServiceType: csts.OID{1, 3, 6}, InstanceNumber: 1,
			},
		}
		mutate(b)
		return &csts.PDU{Type: csts.OpBindInvocation, Bind: b}
	}

	tests := []struct {
		name string
		pdu  *csts.PDU
		want error
	}{
		{
			// AuthorityIdentifier is SIZE (3..16).
			name: "an authority identifier of two characters",
			pdu:  bind(func(b *csts.BindInvocation) { b.InitiatorIdentifier = "CN" }),
			want: csts.ErrIdentifierLength,
		},
		{
			// IdentifierString is VisibleString (FROM (ALL EXCEPT " ")).
			name: "an authority identifier containing a blank",
			pdu:  bind(func(b *csts.BindInvocation) { b.InitiatorIdentifier = "C NES" }),
			want: csts.ErrIdentifierHasBlank,
		},
		{
			// VersionNumber is IntPos, which starts at 1.
			name: "a version number of zero",
			pdu:  bind(func(b *csts.BindInvocation) { b.VersionNumber = 0 }),
			want: csts.ErrInvalidVersion,
		},
		{
			// Credentials 'used' is SIZE (8..256).
			name: "credentials of four octets",
			pdu: bind(func(b *csts.BindInvocation) {
				b.Header.InvokerCredentials = csts.Credentials{Used: true, Value: []byte{1, 2, 3, 4}}
			}),
			want: csts.ErrCredentialsLength,
		},
		{
			// FunctionalResourceInstanceNumber and the secondary procedure
			// instance are IntPos, so zero is not a value.
			name: "a secondary procedure with instance number zero",
			pdu: bind(func(b *csts.BindInvocation) {
				b.Header.Procedure = csts.ProcedureName{
					Type: csts.OIDBufferedDataDelivery, Role: csts.RoleSecondary, Instance: 0,
				}
			}),
			want: csts.ErrInvalidProcedureName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.pdu.Encode(); !errors.Is(err, tt.want) {
				t.Errorf("Encode = %v, want %v", err, tt.want)
			}
		})
	}
}

// A secondary procedure instance survives the round trip, because it is what
// tells two instances of the same procedure type apart within one service.
func TestSecondaryProcedureInstance(t *testing.T) {
	pdu := &csts.PDU{
		Type: csts.OpStartInvocation,
		Start: &csts.StartInvocation{Header: csts.InvocationHeader{
			InvokeID: 9,
			Procedure: csts.ProcedureName{
				Type: csts.OIDCyclicReport, Role: csts.RoleSecondary, Instance: 4,
			},
		}},
	}

	encoded, err := pdu.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := csts.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	header, ok := back.Header()
	if !ok {
		t.Fatal("a START invocation should carry a header")
	}
	if header.Procedure.Role != csts.RoleSecondary || header.Procedure.Instance != 4 {
		t.Errorf("procedure = %+v", header.Procedure)
	}
	if !header.Procedure.Type.Equal(csts.OIDCyclicReport) {
		t.Errorf("procedure type = %s", header.Procedure.Type)
	}
}

// The three alternatives this package does not model keep their octets, so
// decoding one loses nothing.
func TestUnmodelledAlternativesKeepTheirOctets(t *testing.T) {
	content := ber.AppendOctetString(nil, []byte("buffer contents"))
	raw := ber.AppendElement(nil, ber.ClassContext, true, uint32(csts.OpReturnBuffer), content)

	back, err := csts.Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if back.Type != csts.OpReturnBuffer {
		t.Fatalf("decoded as %s", back.Type)
	}
	if !bytes.Equal(back.Content, content) {
		t.Errorf("content = %x, want %x", back.Content, content)
	}

	again, err := back.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(again, raw) {
		t.Errorf("re-encoding changed the octets:\n%x\n%x", raw, again)
	}
}

func TestHumanize(t *testing.T) {
	back := mustDecode(t, unbindInvocationHex)
	text := back.Humanize()
	for _, want := range []string{
		"UNBIND invocation",
		"Invoke ID ....... 7",
		"Association Control",
		"unused",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Humanize is missing %q:\n%s", want, text)
		}
	}

	abort := mustDecode(t, peerAbortHex)
	text = abort.Humanize()
	if !strings.Contains(text, "access denied") || !strings.Contains(text, "CSTS Association Control") {
		t.Errorf("Humanize did not report the abort:\n%s", text)
	}
}

func assertEncodes(t *testing.T, pdu *csts.PDU, wantHex string) {
	t.Helper()

	encoded, err := pdu.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		t.Fatalf("bad test constant: %v", err)
	}
	if !bytes.Equal(encoded, want) {
		t.Errorf("encoded\n  %x\nwant\n  %x", encoded, want)
	}
}

func mustDecode(t *testing.T, wantHex string) *csts.PDU {
	t.Helper()

	data, err := hex.DecodeString(wantHex)
	if err != nil {
		t.Fatalf("bad test constant: %v", err)
	}
	pdu, err := csts.Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return pdu
}
