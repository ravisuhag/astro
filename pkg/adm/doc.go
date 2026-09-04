// Package adm implements the CCSDS Attitude Data Messages, CCSDS 504.0-B-2.
//
// An attitude message says which way a spacecraft is pointing. Where an orbit
// message gives a position, these give an orientation: a quaternion, a set of
// Euler angles, or a spin axis, and the frames the rotation goes between.
//
// The standard defines three messages. Two are implemented:
//
//   - APM, the Attitude Parameter Message: one attitude at one epoch, with
//     optional angular velocity, spin, inertia and planned manoeuvres.
//   - AEM, the Attitude Ephemeris Message: a table of attitudes over a span,
//     with the interpolation a consumer should use between them.
//
// The ACM, the Attitude Comprehensive Message of section 5, is not
// implemented. It is roughly three times the size of the other two together,
// it arrived with this 2024 issue, and adoption is thin next to the APM and
// the AEM. That is the same call this project made about the ODM's OCM.
//
// # What this package does
//
// It reads and writes the messages, in 'keyword = value' notation. It does no
// attitude mathematics: nothing here normalises a quaternion, converts one to
// Euler angles, composes two rotations, or interpolates between attitudes.
// Getting those wrong is a different kind of mistake from getting a file
// format wrong, and the conventions they depend on are in annex F of the Blue
// Book rather than in the wire format.
//
// # The scalar component comes last
//
// A quaternion goes on the wire as Q1, Q2, Q3, QC. The first three are the
// vector part and QC is the scalar — cos(phi/2) in the table's own notation.
// Plenty of libraries put the scalar first, so reading these four numbers into
// a [4]float64 and handing it to one of them gives a rotation that is wrong
// and looks plausible. Quaternion names its fields rather than indexing them,
// for that reason.
//
// # An AEM data line has no fixed width
//
// The number of values on an attitude ephemeris line comes from the segment's
// ATTITUDE_TYPE, not from the line. Table 4-4 gives nine layouts, from three
// values for EULER_ANGLE to eight for QUATERNION/DERIVATIVE. A reader that
// assumes one of them silently misreads the rest, so AttitudeType.Fields
// reports the width and Decode checks every line against it.
package adm
