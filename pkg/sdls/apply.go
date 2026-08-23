package sdls

// buildAuthPayloadPrefix produces the masked authenticated data that precedes
// the frame data field, per CCSDS 355.0-B-2 §4.2.3.4 d).
//
// The Authentication Payload is the partial frame — primary header through the
// end of the data field — with the SA's authentication bit mask applied by a
// bitwise AND. For an AEAD algorithm §4.2.3.2.2.3 a) splits that payload in
// two: everything up to the data field becomes the associated data, and the
// data field itself becomes the plaintext. This returns the first part.
//
// The Initialization Vector is always zeroed here, whatever the mask says:
// §4.2.2.6.2 h) covers the Security Header "except for the mask bits
// corresponding to the Initialization Vector field".
func (sa *SecurityAssociation) buildAuthPayloadPrefix(frameHeader, securityHeader []byte) ([]byte, error) {
	prefix := make([]byte, 0, len(frameHeader)+len(securityHeader))
	prefix = append(prefix, frameHeader...)
	prefix = append(prefix, securityHeader...)

	if sa.AuthMask != nil {
		if len(sa.AuthMask) < len(prefix) {
			return nil, ErrMaskTooShort
		}
		for i := range prefix {
			prefix[i] &= sa.AuthMask[i]
		}
	}

	// Zero the IV wherever it sits inside the security header, after masking,
	// so no mask can accidentally pull it into the authenticated data.
	ivStart := len(frameHeader) + SPISize
	ivEnd := ivStart + sa.FieldLengths.IV
	if ivEnd <= len(prefix) {
		for i := ivStart; i < ivEnd; i++ {
			prefix[i] = 0
		}
	}

	return prefix, nil
}

// ApplySecurity protects one frame data field and returns the bytes to place
// in the carrier frame, per CCSDS 355.0-B-2 §4.2.3:
//
//	Security Header || data (ciphertext or plaintext) || Security Trailer
//
// frameHeader is the carrier frame's own bytes, from the first octet of the
// Transfer Frame Primary Header up to where the Security Header begins. SDLS
// authenticates those octets without encrypting them, subject to the SA's
// authentication bit mask. Pass nil to authenticate the security header and
// data alone.
//
// plaintext is the Transfer Frame Data Field to protect.
//
// The SA advances its IV counter on every successful call and refuses to reuse
// one, returning ErrIVExhausted when the counter space runs out.
//
// This package does not perform block padding: GCM is a stream mode and
// §E1.2 note 2 records that it needs none. The Pad Length field, if the SA
// declares one, is transmitted as zeros. ProcessSecurity still honors a
// non-zero Pad Length on receive.
func (sa *SecurityAssociation) ApplySecurity(frameHeader, plaintext []byte) ([]byte, error) {
	if err := sa.Validate(); err != nil {
		return nil, err
	}
	if sa.Mode == Encryption {
		// §2.3.3 permits it; this package deliberately does not.
		return nil, ErrUnsupportedMode
	}

	iv, err := sa.nextIV()
	if err != nil {
		return nil, err
	}
	seqNum, err := sa.nextSeqNum()
	if err != nil {
		return nil, err
	}

	header := &SecurityHeader{
		SPI:    sa.SPI,
		IV:     iv,
		SeqNum: seqNum,
	}
	if sa.FieldLengths.PadLen > 0 {
		header.PadLength = make([]byte, sa.FieldLengths.PadLen)
	}

	headerBytes, err := header.Encode()
	if err != nil {
		return nil, err
	}

	prefix, err := sa.buildAuthPayloadPrefix(frameHeader, headerBytes)
	if err != nil {
		return nil, err
	}

	var body, mac []byte

	// CMAC is not an AEAD and needs no nonce, so it does not go through the
	// GCM construction at all.
	if sa.usesCMAC() {
		mac, err = sa.cmacTag(prefix, plaintext)
		if err != nil {
			return nil, err
		}
		body = plaintext

		out := make([]byte, 0, len(headerBytes)+len(body)+len(mac))
		out = append(out, headerBytes...)
		out = append(out, body...)
		out = append(out, mac...)
		return out, nil
	}

	gcm, err := sa.newGCM()
	if err != nil {
		return nil, err
	}

	switch sa.Mode {
	case AuthenticatedEncryption:
		// §4.2.3.2.2.3: plaintext is the data field, associated data is the
		// masked prefix. Seal appends the tag, which becomes the trailer.
		sealed := gcm.Seal(nil, iv, plaintext, prefix)
		split := len(sealed) - sa.FieldLengths.MAC
		body = sealed[:split]
		mac = sealed[split:]

	case Authentication:
		// §4.2.3.2.2.2: the data field travels unencrypted and the MAC covers
		// the whole Authentication Payload. Sealing an empty plaintext with
		// prefix||plaintext as associated data is GMAC over exactly that.
		// The CMAC alternative of §E2 returned above.
		aad := make([]byte, 0, len(prefix)+len(plaintext))
		aad = append(aad, prefix...)
		aad = append(aad, plaintext...)

		mac = gcm.Seal(nil, iv, nil, aad)
		body = plaintext

	default:
		return nil, ErrUnsupportedMode
	}

	out := make([]byte, 0, len(headerBytes)+len(body)+len(mac))
	out = append(out, headerBytes...)
	out = append(out, body...)
	out = append(out, mac...)
	return out, nil
}

// cmacTag computes the AES-CMAC over the Authentication Payload, per §E2.
//
// The payload is the same one GMAC covers: the masked frame header and
// security header, then the data field. §E2c fixes the MAC at 128 bits, but
// the SA's declared width is honoured so a mission that truncates still
// interoperates with itself.
func (sa *SecurityAssociation) cmacTag(prefix, plaintext []byte) ([]byte, error) {
	mac, err := sa.newCMAC()
	if err != nil {
		return nil, err
	}

	payload := make([]byte, 0, len(prefix)+len(plaintext))
	payload = append(payload, prefix...)
	payload = append(payload, plaintext...)

	return mac.SumTruncated(payload, sa.FieldLengths.MAC)
}
