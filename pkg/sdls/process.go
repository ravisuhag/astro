package sdls

import (
	"crypto/subtle"
	"encoding/binary"
)

// SALookup returns the Security Association registered for an SPI. It should
// return ErrUnknownSPI when the index is not configured.
type SALookup func(spi uint16) (*SecurityAssociation, error)

// StaticLookup builds an SALookup over a fixed set of Security Associations,
// keyed by their SPI. It is the common case: SAs preloaded before a mission
// starts (clause 2.3.1.5).
func StaticLookup(sas ...*SecurityAssociation) SALookup {
	table := make(map[uint16]*SecurityAssociation, len(sas))
	for _, sa := range sas {
		if sa != nil {
			table[sa.SPI] = sa
		}
	}
	return func(spi uint16) (*SecurityAssociation, error) {
		sa, ok := table[spi]
		if !ok {
			return nil, ErrUnknownSPI
		}
		return sa, nil
	}
}

// ProcessSecurity reverses ApplySecurity, per CCSDS 355.0-B-2 clause 4.2.4.
//
// dataField is the carrier frame's data field: Security Header, then the
// protected data, then the Security Trailer. frameHeader is the same header
// prefix that the sender authenticated. lookup resolves the SPI to its SA.
//
// It returns the decoded Security Header and the recovered Transfer Frame Data
// Field. On any verification failure it returns a nil data field: no partial
// plaintext ever escapes, per clause 4.2.4.2.3.
//
// GCM tag comparison is left to crypto/cipher's Open, which is constant time;
// the CMAC path compares in constant time with crypto/subtle.
//
// ProcessSecurity verifies the SPI only. It has no way to know which channel
// the frame arrived on, so the clause 4.2.4.3 check that the SA is the one agreed
// for that channel is left to the caller. Use ProcessSecurityForChannel to
// have this package enforce it.
func ProcessSecurity(dataField, frameHeader []byte, lookup SALookup) (*SecurityHeader, []byte, error) {
	return processSecurity(dataField, frameHeader, nil, lookup)
}

// ProcessSecurityForChannel is ProcessSecurity with the receiving channel in
// hand: ch identifies the Global Virtual Channel (or Global MAP) the frame
// arrived on. When the SA that the SPI names declares a channel binding in
// its Channels list, the frame is rejected with ErrSAChannelMismatch unless
// ch is in that list (the SA verification of clause 4.2.4.3) before any
// cryptographic work. An SA with an empty Channels list accepts any channel.
func ProcessSecurityForChannel(dataField, frameHeader []byte, ch ChannelID, lookup SALookup) (*SecurityHeader, []byte, error) {
	return processSecurity(dataField, frameHeader, &ch, lookup)
}

// processSecurity is the shared body of the two entry points; ch is nil when
// the caller supplied no channel context.
func processSecurity(dataField, frameHeader []byte, ch *ChannelID, lookup SALookup) (*SecurityHeader, []byte, error) {
	if lookup == nil {
		return nil, nil, ErrUnknownSPI
	}
	// The SPI is always the leading two octets, whatever the SA says (clause 4.1.1.2.1).
	if len(dataField) < SPISize {
		return nil, nil, ErrDataTooShort
	}

	spi := binary.BigEndian.Uint16(dataField[:SPISize])
	sa, err := lookup(spi)
	if err != nil {
		return nil, nil, err
	}
	if sa == nil {
		return nil, nil, ErrUnknownSPI
	}
	if err := sa.Validate(); err != nil {
		return nil, nil, err
	}
	// Clause 4.2.4.3: the SA must be the one agreed for the channel the frame
	// arrived on. Checked before any cryptographic work, like the SPI.
	if ch != nil && !sa.servesChannel(*ch) {
		return nil, nil, ErrSAChannelMismatch
	}
	if sa.Mode == Encryption {
		return nil, nil, ErrUnsupportedMode
	}

	header, consumed, err := DecodeSecurityHeader(dataField, sa.FieldLengths)
	if err != nil {
		return nil, nil, err
	}

	macLen := sa.FieldLengths.MAC
	if len(dataField) < consumed+macLen {
		return nil, nil, ErrDataTooShort
	}

	headerBytes := dataField[:consumed]
	body := dataField[consumed : len(dataField)-macLen]
	mac := dataField[len(dataField)-macLen:]

	prefix, err := sa.buildAuthPayloadPrefix(frameHeader, headerBytes)
	if err != nil {
		return nil, nil, err
	}

	var plaintext []byte

	// CMAC is not an AEAD: verify the tag directly and take the body as it
	// stands, since clause E2 authenticates without encrypting.
	if sa.usesCMAC() {
		expected, err := sa.cmacTag(prefix, body)
		if err != nil {
			return nil, nil, err
		}
		// Constant time: a variable-time compare here would leak how many
		// leading octets of a forged tag were correct.
		if subtle.ConstantTimeCompare(expected, mac) != 1 {
			return nil, nil, ErrAuthenticationFailed
		}
		plaintext = copySlice(body)

		return sa.finishProcessing(header, plaintext)
	}

	gcm, err := sa.newGCM()
	if err != nil {
		return nil, nil, err
	}
	if len(header.IV) != gcm.NonceSize() {
		return nil, nil, ErrInvalidFieldLengths
	}

	switch sa.Mode {
	case AuthenticatedEncryption:
		sealed := make([]byte, 0, len(body)+len(mac))
		sealed = append(sealed, body...)
		sealed = append(sealed, mac...)

		plaintext, err = gcm.Open(nil, header.IV, sealed, prefix)
		if err != nil {
			return nil, nil, ErrAuthenticationFailed
		}

	case Authentication:
		aad := make([]byte, 0, len(prefix)+len(body))
		aad = append(aad, prefix...)
		aad = append(aad, body...)

		if _, err := gcm.Open(nil, header.IV, mac, aad); err != nil {
			return nil, nil, ErrAuthenticationFailed
		}
		plaintext = copySlice(body)

	default:
		return nil, nil, ErrUnsupportedMode
	}

	return sa.finishProcessing(header, plaintext)
}

// finishProcessing runs the steps that follow a verified MAC, whichever
// algorithm produced it: anti-replay, then padding removal.
func (sa *SecurityAssociation) finishProcessing(header *SecurityHeader, plaintext []byte) (*SecurityHeader, []byte, error) {
	// Anti-replay runs only now, after the MAC has verified, so a forged frame
	// cannot advance the receiver's window (clause 4.2.4.4).
	counter := header.SeqNum
	if sa.usesIVAsSequence() {
		counter = header.IV
	}
	if err := sa.checkAndAdvanceSequence(counter); err != nil {
		return nil, nil, err
	}

	// Clause 4.2.3.3 b) records fill bytes in the Pad Length field; strip them.
	if pad := header.PadCount(); pad > 0 {
		if pad > len(plaintext) {
			return nil, nil, ErrInvalidPadLength
		}
		plaintext = plaintext[:len(plaintext)-pad]
	}

	return header, plaintext, nil
}
