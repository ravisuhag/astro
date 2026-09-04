package odm

import "errors"

// Sentinel errors from the message structure.
var (
	// ErrNotAnOPM indicates a file whose first keyword is not CCSDS_OPM_VERS.
	ErrNotAnOPM = errors.New("odm: file is not an Orbit Parameter Message")

	// ErrUnknownKeyword indicates a keyword none of the tables in section 3
	// lists. Clause 3.2.4.2 says only those keywords shall be used, so an
	// unknown one is refused rather than skipped.
	ErrUnknownKeyword = errors.New("odm: keyword is not one the tables allow")

	// ErrDuplicateKeyword indicates a keyword given twice within a block that
	// allows it once.
	ErrDuplicateKeyword = errors.New("odm: keyword appears more than once")

	// ErrMissingKeyword indicates a mandatory keyword that is not present.
	ErrMissingKeyword = errors.New("odm: mandatory keyword is missing")

	// ErrKeywordOutOfOrder indicates a keyword appearing somewhere the tables
	// do not put it. Clause 7.4.8 fixes the order of assignments.
	ErrKeywordOutOfOrder = errors.New("odm: keyword is out of the order the tables fix")
)

// Sentinel errors from the all-or-nothing blocks of table 3-3.
var (
	// ErrIncompleteKeplerian indicates some but not all of the osculating
	// Keplerian elements. Table 3-3 heads that block "none or all parameters
	// of this block must be given".
	ErrIncompleteKeplerian = errors.New("odm: the Keplerian elements block must be complete or absent")

	// ErrBothAnomalies indicates both TRUE_ANOMALY and MEAN_ANOMALY. Table 3-3
	// offers them as alternatives, so a message carries one.
	ErrBothAnomalies = errors.New("odm: give either TRUE_ANOMALY or MEAN_ANOMALY, not both")

	// ErrIncompleteCovariance indicates some but not all of the 21 elements of
	// the lower triangular covariance matrix. Table 3-3 makes that block
	// all-or-nothing too.
	ErrIncompleteCovariance = errors.New("odm: the covariance matrix must be complete or absent")

	// ErrIncompleteManeuver indicates a manoeuvre missing one of the seven
	// parameters clause 3.2.4.8 requires to be repeated for each one.
	ErrIncompleteManeuver = errors.New("odm: every maneuver must carry all of its parameters")

	// ErrManeuverWithoutMass indicates a manoeuvre in a message with no MASS.
	// Clause 3.2.4.9 makes the conditional spacecraft parameters mandatory
	// once a manoeuvre is defined, and a manoeuvre without a mass cannot be
	// modelled.
	ErrManeuverWithoutMass = errors.New("odm: a message with a maneuver must give MASS")

	// ErrDeltaMassNotNegative indicates a MAN_DELTA_MASS of zero or more.
	// Clause 3.2.4.7 says the value must be negative: a manoeuvre spends
	// propellant.
	ErrDeltaMassNotNegative = errors.New("odm: MAN_DELTA_MASS must be negative")
)

// Sentinel errors from the Orbit Ephemeris Message (section 5).
var (
	// ErrNotAnOEM indicates a file whose first keyword is not CCSDS_OEM_VERS.
	ErrNotAnOEM = errors.New("odm: file is not an Orbit Ephemeris Message")

	// ErrNoEphemerisBlock indicates an OEM with no metadata group at all.
	// Clause 5.2.3.3 requires one before each ephemeris data block, and an OEM
	// with no ephemeris is not an ephemeris message.
	ErrNoEphemerisBlock = errors.New("odm: an OEM must carry at least one metadata group and ephemeris block")

	// ErrUnterminatedBlock indicates a META_START with no META_STOP, or a
	// COVARIANCE_START with no COVARIANCE_STOP.
	ErrUnterminatedBlock = errors.New("odm: a delimited block was not closed")

	// ErrUnexpectedDelimiter indicates a block delimiter where the structure of
	// clause 5.2.3.3 does not allow one: a META_STOP with no META_START, a
	// nested META_START, or a covariance delimiter outside a covariance block.
	ErrUnexpectedDelimiter = errors.New("odm: block delimiter is out of place")

	// ErrEphemerisLineFields indicates an ephemeris data line without 7 or 10
	// fields. Clause 5.2.4.1 fixes the order and clause 5.2.4.2 makes position
	// and velocity mandatory and acceleration optional, so those are the only
	// two widths a line may have.
	ErrEphemerisLineFields = errors.New("odm: an ephemeris data line must have 7 or 10 fields")

	// ErrInterpolationDegreeMissing indicates an INTERPOLATION with no
	// INTERPOLATION_DEGREE. Table 5-3 makes the degree mandatory whenever the
	// method is given, because a method without a degree cannot be applied.
	ErrInterpolationDegreeMissing = errors.New("odm: INTERPOLATION requires INTERPOLATION_DEGREE")

	// ErrTimeSystemChanged indicates two metadata groups naming different time
	// systems. Clause 5.2.4.5 requires TIME_SYSTEM to remain fixed within an
	// OEM, so every epoch in the file is on one scale.
	ErrTimeSystemChanged = errors.New("odm: TIME_SYSTEM must stay the same throughout an OEM")

	// ErrCovarianceOutOfOrder indicates covariance matrices whose epochs do
	// not increase. Clause 5.2.5.7 requires them ordered by increasing time
	// tag.
	ErrCovarianceOutOfOrder = errors.New("odm: covariance matrices must be ordered by increasing epoch")

	// ErrCovarianceValueCount indicates a covariance matrix with a number of
	// values that is not a whole lower triangle. Clause 5.2.5.4 wants 21, from
	// [1,1] to [6,6] row by row.
	ErrCovarianceValueCount = errors.New("odm: a covariance matrix must hold 21 lower triangular values")
)
