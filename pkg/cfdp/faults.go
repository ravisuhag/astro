package cfdp

// FaultHandler is a disposition for a fault condition, per CCSDS 727.0-B-5
// Clause 4.8. Table 4-1 assigns every fault condition a default handler; a fault
// handler override TLV (clause 5.4.4) or per-transaction configuration replaces it.
//
// The numeric values are the handler codes of the fault handler override TLV,
// so a FaultHandler travels on the wire unchanged.
type FaultHandler uint8

const (
	// FaultHandlerCancel issues a Notice of Cancellation ('0001'): the
	// transaction closes out with the fault's condition code.
	FaultHandlerCancel FaultHandler = 0x1
	// FaultHandlerSuspend issues a Notice of Suspension ('0010'): the
	// transaction goes quiet until the caller resumes it.
	FaultHandlerSuspend FaultHandler = 0x2
	// FaultHandlerIgnore ignores the fault ('0011') and lets the transaction
	// carry on.
	FaultHandlerIgnore FaultHandler = 0x3
	// FaultHandlerAbandon abandons the transaction ('0100') with no further
	// protocol activity, not even a Finished PDU.
	FaultHandlerAbandon FaultHandler = 0x4
)

// String names the handler.
func (h FaultHandler) String() string {
	switch h {
	case FaultHandlerCancel:
		return "issue notice of cancellation"
	case FaultHandlerSuspend:
		return "issue notice of suspension"
	case FaultHandlerIgnore:
		return "ignore"
	case FaultHandlerAbandon:
		return "abandon transaction"
	default:
		return "unknown"
	}
}

// Valid reports whether the handler is one of the four defined codes.
func (h FaultHandler) Valid() bool {
	return h >= FaultHandlerCancel && h <= FaultHandlerAbandon
}

// DefaultFaultHandler returns table 4-1's default disposition for a fault
// condition: every condition defaults to a Notice of Cancellation.
func DefaultFaultHandler(ConditionCode) FaultHandler {
	return FaultHandlerCancel
}

// FaultHandlerOverrideTLV builds the fault handler override TLV of clause 5.4.4:
// one octet holding the condition code (4 bits) and the handler code (4 bits).
func FaultHandlerOverrideTLV(cond ConditionCode, handler FaultHandler) (TLV, error) {
	if !handler.Valid() {
		return TLV{}, ErrInvalidFaultHandler
	}
	return TLV{
		Type:  TLVFaultHandlerOverride,
		Value: []byte{byte(cond&0x0F)<<4 | byte(handler&0x0F)},
	}, nil
}

// DecodeFaultHandlerOverride reads a fault handler override TLV (clause 5.4.4).
func DecodeFaultHandlerOverride(t TLV) (ConditionCode, FaultHandler, error) {
	if t.Type != TLVFaultHandlerOverride || len(t.Value) < 1 {
		return 0, 0, ErrInvalidFaultHandler
	}
	cond := ConditionCode(t.Value[0] >> 4)
	handler := FaultHandler(t.Value[0] & 0x0F)
	if !handler.Valid() {
		return 0, 0, ErrInvalidFaultHandler
	}
	return cond, handler, nil
}

// faultError maps a fault condition to the sentinel error surfaced when the
// fault interrupts the handling of a PDU. Conditions raised by the caller's
// own timers map to nil: the caller declared them, so they are not errors to
// report back.
func faultError(cond ConditionCode) error {
	switch cond {
	case CondFileSizeError:
		return ErrFileSizeError
	case CondFileChecksumFailure:
		return ErrChecksumFailure
	case CondFilestoreRejection:
		return ErrFilestoreRejection
	case CondUnsupportedChecksumType:
		return ErrUnsupportedChecksumType
	default:
		return nil
	}
}
