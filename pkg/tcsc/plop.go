package tcsc

// Physical Layer Operations Procedures (PLOP) per CCSDS 231.0-B-4
// section 8, with the acquisition and idle sequences of section 7.
//
// A PLOP governs how CLTUs are placed onto the physical channel:
//
//   - PLOP-1 ends the communications session after each CLTU: every CLTU
//     is preceded by a fresh acquisition sequence.
//   - PLOP-2 keeps the session up across CLTUs: one acquisition sequence
//     starts the session, and an idle sequence keeps the channel modulated
//     between CLTUs. PLOP-2 is the CCSDS-recommended procedure.
//
// Both the acquisition sequence and the idle sequence are the alternating
// bit pattern '01010101...' (0x55 octets). CCSDS 231.0-B-4 recommends at
// least 16 octets (128 bits) of acquisition sequence.

// PLOP identifies a Physical Layer Operations Procedure.
type PLOP int

const (
	// PLOP1 is PLOP-1: the session ends after each CLTU, so each CLTU is
	// preceded by its own acquisition sequence.
	PLOP1 PLOP = 1

	// PLOP2 is PLOP-2: one session carries many CLTUs, separated by idle
	// sequence. This is the CCSDS-recommended procedure.
	PLOP2 PLOP = 2
)

const (
	// DefaultAcquisitionOctets is the recommended minimum acquisition
	// sequence length (128 bits) per CCSDS 231.0-B-4.
	DefaultAcquisitionOctets = 16

	// DefaultIdleOctets is a practical default idle sequence length
	// between consecutive CLTUs under PLOP-2. The standard requires at
	// least one octet of idle sequence; the exact amount is a managed
	// parameter.
	DefaultIdleOctets = 8

	// fillOctet is the alternating '01' bit pattern used by both the
	// acquisition and idle sequences.
	fillOctet = 0x55
)

// AcquisitionSequence returns an acquisition sequence of the given length
// in octets (alternating bit pattern 0x55). If octets is not positive,
// DefaultAcquisitionOctets is used.
func AcquisitionSequence(octets int) []byte {
	if octets <= 0 {
		octets = DefaultAcquisitionOctets
	}
	return fillSequence(octets)
}

// IdleSequence returns an idle sequence of the given length in octets
// (alternating bit pattern 0x55). If octets is not positive,
// DefaultIdleOctets is used.
func IdleSequence(octets int) []byte {
	if octets <= 0 {
		octets = DefaultIdleOctets
	}
	return fillSequence(octets)
}

func fillSequence(octets int) []byte {
	seq := make([]byte, octets)
	for i := range seq {
		seq[i] = fillOctet
	}
	return seq
}

// UplinkSequence assembles the symbol stream that a PLOP session places on
// the physical channel for the given CLTUs.
//
//   - PLOP-1: acquisition + CLTU, repeated for each CLTU (the carrier is
//     dropped between CLTUs, so each needs a new acquisition sequence).
//   - PLOP-2: acquisition + CLTU + idle + CLTU + ... (a single session;
//     idle sequence keeps the channel modulated between CLTUs).
//
// acqOctets and idleOctets select the sequence lengths; values that are
// not positive fall back to the defaults. Returns ErrEmptyData when no
// CLTUs are given.
func UplinkSequence(plop PLOP, cltus [][]byte, acqOctets, idleOctets int) ([]byte, error) {
	if len(cltus) == 0 {
		return nil, ErrEmptyData
	}
	if plop != PLOP1 && plop != PLOP2 {
		return nil, ErrInvalidPLOP
	}

	acq := AcquisitionSequence(acqOctets)
	idle := IdleSequence(idleOctets)

	var out []byte
	for i, cltu := range cltus {
		switch {
		case plop == PLOP1:
			// New session per CLTU: acquisition before every CLTU.
			out = append(out, acq...)
		case i == 0:
			// PLOP-2: one acquisition sequence starts the session.
			out = append(out, acq...)
		default:
			// PLOP-2: idle sequence between CLTUs.
			out = append(out, idle...)
		}
		out = append(out, cltu...)
	}
	return out, nil
}
