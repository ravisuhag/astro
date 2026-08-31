package tcsc

// Physical Layer Operations Procedures (PLOP) per CCSDS 231.0-B-4
// section 7: PLOP-1 in §7.4 and PLOP-2 in §7.5, with the acquisition
// sequence of §7.2.2 and the idle sequence of §7.2.4. Section 8 of that
// book is Managed Parameters, not PLOP.
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
// bit pattern '01010101...' (0x55 octets). CCSDS 231.0-B-4 §7.2.2
// recommends at least 16 octets (128 bits) of acquisition sequence.

// PLOP identifies a Physical Layer Operations Procedure.
type PLOP int

const (
	// PLOP1 is PLOP-1 (§7.4): the session ends after each CLTU, so each
	// CLTU is preceded by its own acquisition sequence.
	PLOP1 PLOP = 1

	// PLOP2 is PLOP-2 (§7.5): one session carries many CLTUs, separated by
	// idle sequence. This is the CCSDS-recommended procedure.
	PLOP2 PLOP = 2
)

const (
	// DefaultAcquisitionOctets is the recommended minimum acquisition
	// sequence length (128 bits) per CCSDS 231.0-B-4 §7.2.2.
	DefaultAcquisitionOctets = 16

	// DefaultIdleOctets is a practical default idle sequence length
	// between consecutive CLTUs under PLOP-2, chosen by this library and
	// not by the standard.
	//
	// §7.2.4 constrains nothing here: the idle sequence is "an
	// unconstrained number of bits", and the PLOP-2 figure shows it as
	// optional, so zero octets is conformant too. The length a mission
	// actually uses is a managed parameter (section 8).
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
