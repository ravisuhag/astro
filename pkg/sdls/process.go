package sdls

import "encoding/binary"

// SALookup returns the Security Association registered for an SPI. It should
// return ErrUnknownSPI when the index is not configured.
type SALookup func(spi uint16) (*SecurityAssociation, error)

// StaticLookup builds an SALookup over a fixed set of Security Associations,
// keyed by their SPI. It is the common case: SAs preloaded before a mission
// starts (§2.3.1.5).
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

// ProcessSecurity reverses ApplySecurity, per CCSDS 355.0-B-2 §4.2.4.
//
// dataField is the carrier frame's data field: Security Header, then the
// protected data, then the Security Trailer. frameHeader is the same header
// prefix that the sender authenticated. lookup resolves the SPI to its SA.
//
// It returns the decoded Security Header and the recovered Transfer Frame Data
// Field. On any verification failure it returns a nil data field: no partial
// plaintext ever escapes, per §4.2.4.2.3.
//
// Tag comparison is left to crypto/cipher's GCM Open, which is constant time.
// This package never compares MACs itself.
func ProcessSecurity(dataField, frameHeader []byte, lookup SALookup) (*SecurityHeader, []byte, error) {
	if lookup == nil {
		return nil, nil, ErrUnknownSPI
	}
	// The SPI is always the leading two octets, whatever the SA says (§4.1.1.2.1).
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

	gcm, err := sa.newGCM()
	if err != nil {
		return nil, nil, err
	}
	if len(header.IV) != gcm.NonceSize() {
		return nil, nil, ErrInvalidFieldLengths
	}

	var plaintext []byte
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

	// Anti-replay runs only now, after the MAC has verified, so a forged frame
	// cannot advance the receiver's window (§4.2.4.4).
	counter := header.SeqNum
	if sa.usesIVAsSequence() {
		counter = header.IV
	}
	if err := sa.checkAndAdvanceSequence(counter); err != nil {
		return nil, nil, err
	}

	// §4.2.3.3 b) records fill bytes in the Pad Length field; strip them.
	if pad := header.PadCount(); pad > 0 {
		if pad > len(plaintext) {
			return nil, nil, ErrInvalidPadLength
		}
		plaintext = plaintext[:len(plaintext)-pad]
	}

	return header, plaintext, nil
}
