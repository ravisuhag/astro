package sle

import (
	"errors"

	"github.com/ravisuhag/astro/internal/ber"
)

// Sentinel errors returned by the SLE codecs and the association machine.
var (
	// The BER codec moved to internal/ber so that pkg/csts could share it:
	// the CSTS specification framework is a different set of ASN.1 modules
	// carried by the same encoding. These names are aliases of the ones
	// there, so a caller comparing against sle.ErrInvalidTag is comparing
	// against the same value the codec returns.
	ErrDataTooShort            = ber.ErrDataTooShort
	ErrInvalidTag              = ber.ErrInvalidTag
	ErrInvalidLength           = ber.ErrInvalidLength
	ErrIndefiniteLength        = ber.ErrIndefiniteLength
	ErrInvalidObjectIdentifier = ber.ErrInvalidObjectIdentifier
	ErrLengthTooLarge          = ber.ErrLengthTooLarge
	ErrIntegerOverflow         = ber.ErrIntegerOverflow

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
	// the 12 octets clause 3.3.2.2.4 requires.
	ErrInvalidContextLength = errors.New("TML context message body must be 12 octets")

	// ErrInvalidContextParameters indicates a TML context message whose
	// parameters are out of range: a nonzero heartbeat interval needs a
	// nonzero dead factor.
	ErrInvalidContextParameters = errors.New("invalid TML context parameters")

	// ErrNonEmptyHeartbeat indicates a heartbeat message carrying a body,
	// contrary to clause 3.3.2.2.5.
	ErrNonEmptyHeartbeat = errors.New("TML heartbeat message must have an empty body")

	// ErrMessageTooLarge indicates a TML message body beyond the configured
	// maximum.
	ErrMessageTooLarge = errors.New("TML message body exceeds the maximum this reader accepts")

	// ErrInvalidCredentials indicates credentials that cannot be decoded.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrAuthenticationFailed indicates a credential digest that did not match.
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrCredentialsExpired indicates credentials whose time is further from
	// now than the acceptable delay, per clause 3.1.2.2.1.
	ErrCredentialsExpired = errors.New("credentials are outside the acceptable time window")

	// ErrInvalidIdentifier indicates an authority identifier outside the
	// 3-to-16 character range, or one containing a space.
	ErrInvalidIdentifier = errors.New("invalid authority identifier")

	// ErrInvalidVersionNumber indicates a version number outside 1 to 65535.
	ErrInvalidVersionNumber = errors.New("invalid version number: must be 1 to 65535")

	// ErrWrongState indicates an operation the association state does not allow.
	ErrWrongState = errors.New("operation not allowed in the current association state")

	// ErrInvalidProductionConfig indicates a transfer buffer size below one
	// or a negative latency limit.
	ErrInvalidProductionConfig = errors.New("invalid production configuration")

	// ErrProductionNotRunning indicates a frame offered while production is
	// halted or interrupted. A provider in either state has no data to
	// deliver, and buffering one anyway would deliver it late and out of
	// sequence when production resumed.
	ErrProductionNotRunning = errors.New("production is not running")

	// ErrDuplicateInstance indicates two service instances configured with
	// one identifier. A BIND naming it would be ambiguous, and the standard
	// gives no way to disambiguate.
	ErrDuplicateInstance = errors.New("service instance is already configured")

	// ErrUnknownInstance indicates a service instance identifier the complex
	// does not serve.
	ErrUnknownInstance = errors.New("no such service instance")

	// ErrInstanceInUse indicates a BIND to an instance that is already bound.
	ErrInstanceInUse = errors.New("service instance is already in use")

	// ErrVersionNotSupported indicates a BIND asking for a version the
	// instance was not configured for.
	ErrVersionNotSupported = errors.New("service version is not supported by this instance")

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

	// ErrDuplicateInvokeId indicates an invocation reusing an invoke
	// identifier already seen on this association. The provider machines
	// answer it with the 'duplicate invoke ID' diagnostic.
	ErrDuplicateInvokeId = errors.New("invoke identifier already used on this association")

	// ErrInvokeIdExhausted indicates that the next invoke identifier a user
	// would assign is still awaiting its return. InvokeId is 16 bits, so
	// this means more than 65536 confirmed operations (for example, FCLTU
	// TRANSFER-DATA CLTUs) are outstanding at once: the identifier space has
	// wrapped onto one still in flight. Refusing locally turns what would
	// otherwise surface as a remote 'duplicate invoke ID' into a
	// diagnosable local error.
	ErrInvokeIdExhausted = errors.New("no invoke identifier available: too many confirmed operations are outstanding")

	// ErrCltuOutOfSequence indicates a CLTU identification that is not the one
	// the provider expects next, per CCSDS 912.1-B-5 clause 3.6.2.5.
	ErrCltuOutOfSequence = errors.New("CLTU identification is out of sequence")
)
