// Package odm implements the CCSDS Orbit Data Messages, CCSDS 502.0-B-3, also
// published as ISO 26900.
//
// These are the plain-text files agencies and operators send each other to say
// where a spacecraft is. There are four of them, and which one to use depends
// on what you are saying:
//
//   - OPM, the Orbit Parameter Message: one state vector at one time, with
//     optional Keplerian elements, spacecraft parameters, a covariance matrix
//     and planned manoeuvres.
//   - OMM, the Orbit Mean-Elements Message: mean elements rather than a state
//     vector. This is what a two-line element set becomes when it is written
//     as a CCSDS message.
//   - OEM, the Orbit Ephemeris Message: a table of state vectors over a span,
//     with the interpolation the receiver should use between them.
//   - OCM, the Orbit Comprehensive Message: everything at once, including
//     physical properties and perturbation models.
//
// Each may be written in 'keyword = value' notation or in XML, and clause 1.1
// leaves the choice to the two parties exchanging the file.
//
// # What this package does
//
// It reads and writes the messages. It does no orbital mechanics: nothing here
// propagates a state vector, converts between reference frames, or turns mean
// elements into a position. Those are flight dynamics, they need models this
// package has no business choosing, and getting them wrong is a different kind
// of mistake from getting a file format wrong.
//
//	message, err := odm.DecodeOPM(data)
//	fmt.Println(message.Data.StateVector.X)
//
// # The values are not validated against reality
//
// Clause 1.2 puts orbit accuracy outside the standard, and this package
// follows. A message saying a spacecraft is one metre from the centre of the
// Earth is a well-formed message. What is checked is what the standard states:
// which keywords must be present, which blocks are all-or-nothing, and that
// values are written in the forms clause 7.5 allows.
//
// Reference frame and time system values are checked against the sets in
// clauses 3.2.3.2 and 3.2.3.3, but only as a warning would be — the clauses
// say values should be selected from those sets and that anything else
// "should be documented in an ICD", so an unrecognised value is carried
// through rather than refused.
package odm
