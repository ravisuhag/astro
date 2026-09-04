package csts

// PeerAbortDiagnostic is the single octet a PEER-ABORT carries
// (clause 3.6.2.2, annex F3.5).
//
// The value space is partitioned across the whole cross support family, and
// annex F3.5 sets out the partition. It matters because the same octet arrives
// from three different layers and means different things in each:
//
//	0–39     SLE (0–8 and 127 are used by all SLE services)
//	40–69    the CSTS Association Control procedure, below
//	70–125   reserved for a procedure to abort the association
//	126      otherReason
//	128–199  ISP1, the underlying protocol
//	200–250  the application, chosen per service type
//
// A value in the application range means nothing without knowing the service
// type, which is why this package names the framework's own values and reports
// the rest as a number.
type PeerAbortDiagnostic uint8

// The values the Association Control procedure defines (annex F3.5).
const (
	// AbortAccessDenied is refusal by the responder.
	AbortAccessDenied PeerAbortDiagnostic = 40
	// AbortUnexpectedResponderID is a BIND answered by an authority the
	// initiator did not expect.
	AbortUnexpectedResponderID PeerAbortDiagnostic = 41
	// AbortOperationalRequirement is an operational reason outside the
	// protocol.
	AbortOperationalRequirement PeerAbortDiagnostic = 42
	// AbortProtocolError is a violation of the protocol itself.
	AbortProtocolError PeerAbortDiagnostic = 43
	// AbortCommunicationsFailure is the transport failing underneath.
	AbortCommunicationsFailure PeerAbortDiagnostic = 44
	// AbortEncodingError is a PDU that could not be decoded.
	AbortEncodingError PeerAbortDiagnostic = 45
	// AbortResponseTimeout is a confirmed operation that went unanswered.
	AbortResponseTimeout PeerAbortDiagnostic = 46
	// AbortEndOfServiceProvisionPeriod is the scheduled period ending.
	AbortEndOfServiceProvisionPeriod PeerAbortDiagnostic = 47
	// AbortUnsolicitedInvokeID is a response whose invoke-id matches no
	// outstanding invocation.
	AbortUnsolicitedInvokeID PeerAbortDiagnostic = 48
	// AbortDuplicateInvokeID is an invoke-id already in use.
	AbortDuplicateInvokeID PeerAbortDiagnostic = 49
	// AbortInvalidProcedureName is a procedure-name naming no procedure of
	// this service instance.
	AbortInvalidProcedureName PeerAbortDiagnostic = 50
	// AbortUnrecognizedType is a PDU type the peer does not know.
	AbortUnrecognizedType PeerAbortDiagnostic = 51

	// AbortForwardBufferTooLarge is the one procedure-specific value the
	// framework defines, from the Buffered Data Processing procedure. Annex
	// F3.5 reserves 71 to 125 for procedures a future issue may add.
	AbortForwardBufferTooLarge PeerAbortDiagnostic = 70

	// AbortOtherReason is the catch-all.
	AbortOtherReason PeerAbortDiagnostic = 126
)

// abortNames covers the values the framework itself defines.
var abortNames = map[PeerAbortDiagnostic]string{
	AbortAccessDenied:                "access denied",
	AbortUnexpectedResponderID:       "unexpected responder id",
	AbortOperationalRequirement:      "operational requirement",
	AbortProtocolError:               "protocol error",
	AbortCommunicationsFailure:       "communications failure",
	AbortEncodingError:               "encoding error",
	AbortResponseTimeout:             "response timeout",
	AbortEndOfServiceProvisionPeriod: "end of service provision period",
	AbortUnsolicitedInvokeID:         "unsolicited invoke id",
	AbortDuplicateInvokeID:           "duplicate invoke id",
	AbortInvalidProcedureName:        "invalid procedure name",
	AbortUnrecognizedType:            "unrecognized type",
	AbortForwardBufferTooLarge:       "forward buffer too large",
	AbortOtherReason:                 "other reason",
}

// Origin says which layer of the cross support family allocated a diagnostic
// value, per the partition annex F3.5 sets out.
type Origin int

const (
	// OriginSLE is 0 to 39, allocated by the SLE transfer services.
	OriginSLE Origin = iota
	// OriginAssociationControl is 40 to 69, this framework's own.
	OriginAssociationControl
	// OriginProcedure is 70 to 125, reserved for a procedure to abort the
	// association with a reason of its own.
	OriginProcedure
	// OriginOtherReason is 126.
	OriginOtherReason
	// OriginISP is 128 to 199, allocated by the underlying protocol.
	OriginISP
	// OriginApplication is 200 to 250, chosen per service type — so the same
	// value means different things in different services.
	OriginApplication
	// OriginUnallocated is any value the partition does not cover: 127, and
	// 251 to 255.
	OriginUnallocated
)

func (o Origin) String() string {
	switch o {
	case OriginSLE:
		return "SLE"
	case OriginAssociationControl:
		return "CSTS Association Control"
	case OriginProcedure:
		return "CSTS procedure"
	case OriginOtherReason:
		return "other reason"
	case OriginISP:
		return "ISP1"
	case OriginApplication:
		return "application"
	}
	return "unallocated"
}

// Origin returns which layer allocated this value.
func (d PeerAbortDiagnostic) Origin() Origin {
	switch {
	case d <= 39:
		return OriginSLE
	case d >= 40 && d <= 69:
		return OriginAssociationControl
	case d >= 70 && d <= 125:
		return OriginProcedure
	case d == 126:
		return OriginOtherReason
	case d >= 128 && d <= 199:
		return OriginISP
	case d >= 200 && d <= 250:
		return OriginApplication
	}
	// 127 is used by all SLE services per annex F3.5's own note, but the
	// partition it prints does not give it a range, so it is reported as
	// unallocated rather than guessed at.
	return OriginUnallocated
}

// String names the diagnostic where the framework defines it, and otherwise
// says which layer the value came from.
//
// A value outside the framework's own range is deliberately not named. An
// application value means whatever its service type says, and this package
// has no service type to ask.
func (d PeerAbortDiagnostic) String() string {
	if name, ok := abortNames[d]; ok {
		return name
	}
	return d.Origin().String() + " diagnostic " + itoa(uint64(d))
}

// itoa formats a small non-negative integer without pulling in strconv.
func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
