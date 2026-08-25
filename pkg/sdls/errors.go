package sdls

import "errors"

// Sentinel errors returned by the SDLS security protocol.
var (
	// ErrDataTooShort indicates the input is shorter than the fields it must contain.
	ErrDataTooShort = errors.New("data too short for the security header or trailer")

	// ErrInvalidSPI indicates a reserved Security Parameter Index value.
	// CCSDS 355.0-B-2 §4.1.1.2.3 reserves all-zeros (0) and all-ones (65535).
	ErrInvalidSPI = errors.New("invalid SPI: 0 and 65535 are reserved by CCSDS")

	// ErrInvalidKey indicates the SA key is not a valid AES-256 key.
	ErrInvalidKey = errors.New("invalid key: AES-256 requires exactly 32 bytes")

	// ErrInvalidMode indicates the SA service type is zero or unknown.
	ErrInvalidMode = errors.New("invalid mode: must be Authentication, Encryption, or AuthenticatedEncryption")

	// ErrInvalidFieldLengths indicates the SA's managed field widths are unusable.
	ErrInvalidFieldLengths = errors.New("invalid field lengths for the security header or trailer")

	// ErrHeaderTooLong indicates the security header exceeds the 64-octet cap
	// of CCSDS 355.0-B-2 §4.1.1.1.4.
	ErrHeaderTooLong = errors.New("security header exceeds the maximum of 64 octets")

	// ErrUnsupportedMode indicates a service type this package does not implement.
	ErrUnsupportedMode = errors.New("unsupported mode: encryption without authentication is not implemented")

	// ErrUnknownSPI indicates no Security Association is registered for the SPI.
	ErrUnknownSPI = errors.New("unknown SPI: no security association registered")

	// ErrAuthenticationFailed indicates the MAC did not verify. It is returned
	// for every verification failure so that callers cannot distinguish a bad
	// tag from a malformed frame.
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrReplayDetected indicates the anti-replay check rejected the frame,
	// per CCSDS 355.0-B-2 §2.3.2.3.
	ErrReplayDetected = errors.New("replay detected: sequence number rejected")

	// ErrIVExhausted indicates the initialization vector counter would wrap.
	// Reusing an IV with the same key destroys GCM's security guarantees, so
	// the SA refuses rather than wrapping.
	ErrIVExhausted = errors.New("initialization vector space exhausted for this key")

	// ErrSAChannelMismatch indicates the SA named by the SPI is not bound to
	// the channel the frame arrived on, per CCSDS 355.0-B-2 §4.2.4.3.
	ErrSAChannelMismatch = errors.New("security association is not bound to this channel")

	// ErrMaskTooShort indicates the authentication bit mask does not cover the
	// frame header and security header, per CCSDS 355.0-B-2 §4.2.2.6.2 a).
	ErrMaskTooShort = errors.New("authentication bit mask is shorter than the data it must cover")

	// ErrInvalidPadLength indicates the Pad Length field describes more fill
	// bytes than the recovered plaintext contains.
	ErrInvalidPadLength = errors.New("pad length exceeds the recovered data field")
)
