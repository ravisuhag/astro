// Package adm implements the CCSDS Attitude Data Messages, CCSDS 504.0-B-2.
//
// An attitude message says which way a spacecraft is pointing. Where an orbit
// message gives a position, these give an orientation: a quaternion, a set of
// Euler angles, or a spin axis, and the frames the rotation goes between.
//
// The standard defines three messages, and all three are implemented:
//
//   - APM, the Attitude Parameter Message: one attitude at one epoch, with
//     optional angular velocity, spin, inertia and planned manoeuvres.
//   - AEM, the Attitude Ephemeris Message: a table of attitudes over a span,
//     with the interpolation a consumer should use between them.
//   - ACM, the Attitude Comprehensive Message: attitude histories, physical
//     properties, covariance histories, manoeuvres and how the attitude was
//     determined, in six delimited sections after a single metadata section.
//
// # What this package does
//
// It reads and writes the messages, in the 'keyword = value' notation of
// section 6 and the XML form of section 7. It does no attitude mathematics:
// nothing here normalises a quaternion, converts one to Euler angles,
// composes two rotations, or interpolates between attitudes.
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
//
// # The ACM is held differently from the other two
//
// The APM and the AEM have typed fields. The ACM's keyword tables are too
// large for that — six sections and something over a hundred keywords, most of
// them optional — so its sections are ordered keyword lists with typed
// accessors for the keywords that change how the data must be read.
//
// Where the ODM's OCM has to carry a row's width unchecked, the ACM's is
// checkable. Annex B4 prints the component count of every ATT_TYPE and
// RATE_TYPE, annex B6 does the same for every COV_TYPE, and table 5-4 makes
// NUMBER_STATES mandatory besides. A block therefore says how wide its rows
// are twice over, and DecodeACM refuses one where the two disagree.
//
// Its time tags may also be a signed count of seconds from EPOCH_TZERO rather
// than a date (clause 5.3.4.3); DataRow.TimeTag resolves either.
package adm
