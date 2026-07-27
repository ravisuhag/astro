package sle

import "errors"

// Sentinel errors returned by the SLE codecs and the association machine.
var (
	// ErrDataTooShort indicates the input ended before a field it must contain.
	ErrDataTooShort = errors.New("data too short for the SLE field being read")

	// ErrInvalidTag indicates a BER tag the decoder did not expect here.
	ErrInvalidTag = errors.New("unexpected BER tag")

	// ErrInvalidLength indicates a BER length that is malformed or beyond the
	// bytes available.
	ErrInvalidLength = errors.New("invalid BER length")

	// ErrIndefiniteLength indicates the indefinite-length form where this
	// decoder requires a definite one.
	ErrIndefiniteLength = errors.New("indefinite BER length is not supported here")

	// ErrLengthTooLarge indicates a BER length beyond the configured maximum.
	// A length field can name far more than any real PDU contains, so a cap
	// is what stops one hostile message exhausting memory.
	ErrLengthTooLarge = errors.New("BER length exceeds the maximum this decoder accepts")

	// ErrIntegerOverflow indicates a BER INTEGER too large for the Go type
	// receiving it.
	ErrIntegerOverflow = errors.New("BER integer does not fit")

	// ErrInvalidMessageType indicates a TML message type outside the three of
	// CCSDS 913.1-B-2 table 3-1.
	ErrInvalidMessageType = errors.New("invalid TML message type")

	// ErrInvalidProtocolID indicates a TML context message whose protocol
	// identification is not 'ISP1'.
	ErrInvalidProtocolID = errors.New("invalid TML protocol identification: expected ISP1")

	// ErrInvalidProtocolVersion indicates a TML context message with a
	// protocol version other than 1.
	ErrInvalidProtocolVersion = errors.New("invalid TML protocol version")

	// ErrInvalidContextLength indicates a TML context message body that is not
	// the 12 octets §3.3.2.2.4 requires.
	ErrInvalidContextLength = errors.New("TML context message body must be 12 octets")

	// ErrNonEmptyHeartbeat indicates a heartbeat message carrying a body,
	// contrary to §3.3.2.2.5.
	ErrNonEmptyHeartbeat = errors.New("TML heartbeat message must have an empty body")

	// ErrMessageTooLarge indicates a TML message body beyond the configured
	// maximum.
	ErrMessageTooLarge = errors.New("TML message body exceeds the maximum this reader accepts")

	// ErrInvalidCredentials indicates credentials that cannot be decoded.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrAuthenticationFailed indicates a credential digest that did not match.
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrCredentialsExpired indicates credentials whose time is further from
	// now than the acceptable delay, per §3.1.2.2.1.
	ErrCredentialsExpired = errors.New("credentials are outside the acceptable time window")

	// ErrInvalidIdentifier indicates an authority identifier outside the
	// 3-to-16 character range, or one containing a space.
	ErrInvalidIdentifier = errors.New("invalid authority identifier")

	// ErrInvalidVersionNumber indicates a version number outside 1 to 65535.
	ErrInvalidVersionNumber = errors.New("invalid version number: must be 1 to 65535")

	// ErrWrongState indicates an operation the association state does not allow.
	ErrWrongState = errors.New("operation not allowed in the current association state")

	// ErrNotBound indicates an operation requiring an established association.
	ErrNotBound = errors.New("association is not bound")

	// ErrAlreadyBound indicates a BIND on an association that already has one.
	ErrAlreadyBound = errors.New("association is already bound")

	// ErrBindRejected indicates the peer refused the BIND.
	ErrBindRejected = errors.New("peer rejected the bind")

	// ErrInvalidReportingCycle indicates a reporting cycle outside the 2-to-600
	// second range of ReportingCycle.
	ErrInvalidReportingCycle = errors.New("invalid reporting cycle: must be 2 to 600 seconds")

	// ErrNotStarted indicates a data-transfer operation attempted before START.
	ErrNotStarted = errors.New("service instance is not started")

	// ErrAlreadyStarted indicates a START on an already-active service instance.
	ErrAlreadyStarted = errors.New("service instance is already started")

	// ErrUnexpectedPDU indicates a PDU the service state does not allow, which
	// the state tables answer with a PEER-ABORT for protocol error.
	ErrUnexpectedPDU = errors.New("PDU not allowed in the current service state")

	// ErrUnknownInvokeId indicates a return whose invoke identifier matches no
	// outstanding invocation.
	ErrUnknownInvokeId = errors.New("return does not match any outstanding invocation")

	// ErrCltuOutOfSequence indicates a CLTU identification that is not the one
	// the provider expects next, per CCSDS 912.1-B-5 §3.6.2.5.
	ErrCltuOutOfSequence = errors.New("CLTU identification is out of sequence")
)
