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
