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

// Sentinel errors from the Orbit Mean-Elements Message (section 4).
var (
	// ErrNotAnOMM indicates a file whose first keyword is not CCSDS_OMM_VERS.
	ErrNotAnOMM = errors.New("odm: file is not an Orbit Mean-Elements Message")

	// ErrSizeKeywordMissing indicates an OMM with neither SEMI_MAJOR_AXIS nor
	// MEAN_MOTION. Table 4-3 offers them as alternatives and makes the pair
	// mandatory, because without one of them the orbit has no size.
	ErrSizeKeywordMissing = errors.New("odm: give either SEMI_MAJOR_AXIS or MEAN_MOTION")

	// ErrBothSizeKeywords indicates both SEMI_MAJOR_AXIS and MEAN_MOTION.
	// They are alternatives, and a message carrying both has not said which
	// one the receiver should believe.
	ErrBothSizeKeywords = errors.New("odm: give SEMI_MAJOR_AXIS or MEAN_MOTION, not both")

	// ErrBothDragKeywords indicates both BSTAR and BTERM, or both
	// MEAN_MOTION_DDOT and AGOM. Each pair shares one slot in table 4-3 and
	// which name applies is decided by MEAN_ELEMENT_THEORY.
	ErrBothDragKeywords = errors.New("odm: BSTAR and BTERM, and MEAN_MOTION_DDOT and AGOM, are alternatives")

	// ErrTLEConventions indicates a TLE-based OMM breaking one of the four
	// conventions clause 4.2.4.6 fixes: EARTH as the center, TEME as the
	// frame, UTC as the time system, and MEAN_MOTION rather than
	// SEMI_MAJOR_AXIS.
	ErrTLEConventions = errors.New("odm: a TLE-based OMM must use EARTH, TEME, UTC and MEAN_MOTION")

	// ErrTEMEWithoutTLE indicates the TEME reference frame on an OMM that is
	// not TLE-based. Clause 4.2.4.9 allows TEME "only for OMMs based on NORAD
	// Two Line Element sets, and in no other circumstances", because the frame
	// is not well defined by any international convention.
	ErrTEMEWithoutTLE = errors.New("odm: TEME may be used only for a TLE-based OMM")
)

// Sentinel errors from the Orbit Comprehensive Message (section 6).
var (
	// ErrNotAnOCM indicates a file whose first keyword is not CCSDS_OCM_VERS.
	ErrNotAnOCM = errors.New("odm: file is not an Orbit Comprehensive Message")

	// ErrMalformedDataRow indicates a positional data row with no fields.
	ErrMalformedDataRow = errors.New("odm: a data row must carry at least a time tag")

	// ErrMixedTimeTags indicates a block whose rows mix relative and absolute
	// time tags. Clause 6.2.2.5 requires a block to pick one for the whole of
	// it, because a reader cannot tell from a bare number whether it is
	// seconds or a malformed date.
	ErrMixedTimeTags = errors.New("odm: a data block must use relative or absolute time tags throughout")

	// ErrDuplicateTimeTag indicates two rows in a block with the same time
	// tag, which clause 6.2.2.4 forbids.
	ErrDuplicateTimeTag = errors.New("odm: duplicate time tags in one data block")

	// ErrNoEpochTZero indicates a relative time tag in a message whose
	// metadata gives no EPOCH_TZERO to measure it from.
	ErrNoEpochTZero = errors.New("odm: a relative time tag needs EPOCH_TZERO")

	// ErrTimeTagsOutOfOrder indicates a trajectory or covariance block whose
	// rows do not run forward in time. Clauses 6.2.5.6 and 6.2.7.6 require
	// each to be monotonically increasing, which is what lets a consumer
	// interpolate without sorting the file first.
	ErrTimeTagsOutOfOrder = errors.New("odm: time tags in a data block must increase")

	// ErrKeywordsOutOfOrder indicates a section whose keywords do not follow
	// the order of the table they come from, which clause 6.2.2.1 fixes.
	ErrKeywordsOutOfOrder = errors.New("odm: keywords must follow the order of their table")

	// ErrUnterminatedSection indicates a section opened with a *_START
	// delimiter that the file ends before closing.
	ErrUnterminatedSection = errors.New("odm: a data section was not closed")

	// ErrSectionsOutOfOrder indicates data sections in an order other than the
	// one table 6-1 fixes.
	ErrSectionsOutOfOrder = errors.New("odm: data sections must follow the order of table 6-1")

	// ErrDuplicateSection indicates a second copy of a section the standard
	// allows only once: metadata (clause 6.2.4.3), physical characteristics
	// (6.2.6.2), perturbations (6.2.9.2), orbit determination (6.2.10.2) and
	// user-defined parameters (6.2.11.2).
	ErrDuplicateSection = errors.New("odm: this section may appear only once")

	// ErrMissingPerturbations indicates an orbit determination section with no
	// perturbations section beside it, which clause 6.2.10.5 requires: the
	// orbit determination is only meaningful alongside the force models it
	// used.
	ErrMissingPerturbations = errors.New("odm: an orbit determination section requires a perturbations section")

	// ErrMalformedManComposition indicates a MAN_COMPOSITION that names a
	// field no manoeuvre table holds, mixes the propulsive fields of
	// table 6-8 with the deployment fields of table 6-9 (clause 6.2.8.15),
	// lists them out of order (clause 6.2.8.16), or does not begin with
	// exactly one of TIME_ABSOLUTE and TIME_RELATIVE (clause 6.2.8.18).
	ErrMalformedManComposition = errors.New("odm: malformed MAN_COMPOSITION")

	// ErrManRowWidth indicates a manoeuvre row whose field count does not
	// match the number of fields MAN_COMPOSITION names.
	ErrManRowWidth = errors.New("odm: manoeuvre row does not match MAN_COMPOSITION")

	// ErrUnknownCovOrdering indicates a COV_ORDERING that is not one of the
	// five clause 6.2.7.12.3 defines.
	ErrUnknownCovOrdering = errors.New("odm: COV_ORDERING must be LTM, UTM, FULL, LTMWCC or UTMWCC")

	// ErrCovRowWidth indicates a covariance row holding a number of values
	// that no square matrix can be built from under its COV_ORDERING.
	ErrCovRowWidth = errors.New("odm: covariance row does not hold a whole matrix")

	// ErrMissingSCLKFields indicates TIME_SYSTEM = SCLK without the two
	// keywords table 6-3 makes conditional on it.
	ErrMissingSCLKFields = errors.New("odm: TIME_SYSTEM = SCLK requires SCLK_OFFSET_AT_EPOCH and SCLK_SEC_PER_SI_SEC")
)
