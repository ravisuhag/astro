package adm

import "errors"

// Sentinel errors shared by both messages.
var (
	// ErrNotAnAPM indicates a file whose first keyword is not CCSDS_APM_VERS.
	ErrNotAnAPM = errors.New("adm: file is not an Attitude Parameter Message")

	// ErrNotAnAEM indicates a file whose first keyword is not CCSDS_AEM_VERS.
	ErrNotAnAEM = errors.New("adm: file is not an Attitude Ephemeris Message")

	// ErrUnknownKeyword indicates a keyword the message's tables do not list.
	// Clauses 3.2.3.2, 3.2.4.2 and their AEM counterparts each say only the
	// keywords in the table shall be used.
	ErrUnknownKeyword = errors.New("adm: keyword is not one the tables allow")

	// ErrDuplicateKeyword indicates a keyword given twice where one is allowed.
	ErrDuplicateKeyword = errors.New("adm: keyword appears more than once")

	// ErrMissingKeyword indicates a mandatory keyword that is not present.
	ErrMissingKeyword = errors.New("adm: mandatory keyword is missing")

	// ErrUnterminatedBlock indicates a *_START with no matching *_STOP.
	ErrUnterminatedBlock = errors.New("adm: a delimited block was not closed")

	// ErrUnexpectedDelimiter indicates a block delimiter out of place: a stop
	// with no start, a nested start, or a block closed by the wrong keyword.
	ErrUnexpectedDelimiter = errors.New("adm: block delimiter is out of place")
)

// Sentinel errors from the Attitude Parameter Message (section 3).
var (
	// ErrNoAttitude indicates an APM with none of the blocks that give an
	// attitude. Table 3-3's blocks are each optional on their own, but a
	// message with no quaternion, no Euler angles and no spin says nothing
	// about which way the spacecraft is pointing.
	ErrNoAttitude = errors.New("adm: an APM must carry a quaternion, Euler angles or spin data")

	// ErrIncompleteNutation indicates some but not all of the nutation
	// keywords in a spin block. Table 3-3 marks them conditional together:
	// a nutation angle without its period and phase cannot be applied.
	ErrIncompleteNutation = errors.New("adm: the spin block's nutation keywords must be complete or absent")
)

// Sentinel errors from the Attitude Ephemeris Message (section 4).
var (
	// ErrNoSegment indicates an AEM with no metadata group.
	ErrNoSegment = errors.New("adm: an AEM must carry at least one metadata group and data block")

	// ErrMissingDataSection indicates a metadata group with no data block
	// after it.
	ErrMissingDataSection = errors.New("adm: every AEM metadata group must be followed by a data block")

	// ErrUnknownAttitudeType indicates an ATTITUDE_TYPE outside the nine
	// values table 4-4 defines.
	ErrUnknownAttitudeType = errors.New("adm: unknown ATTITUDE_TYPE")

	// ErrAttitudeLineFields indicates a data line whose field count does not
	// match the segment's ATTITUDE_TYPE. Table 4-4 fixes the width per type,
	// and nothing in the line itself says which type it is.
	ErrAttitudeLineFields = errors.New("adm: attitude data line does not match the segment's ATTITUDE_TYPE")

	// ErrEulerRotSeqMissing indicates a Euler-based ATTITUDE_TYPE with no
	// EULER_ROT_SEQ. Three angles without a rotation sequence do not define a
	// rotation.
	ErrEulerRotSeqMissing = errors.New("adm: a Euler ATTITUDE_TYPE requires EULER_ROT_SEQ")

	// ErrInterpolationDegreeMissing indicates an INTERPOLATION_METHOD with no
	// INTERPOLATION_DEGREE, which table 4-3 makes mandatory alongside it.
	ErrInterpolationDegreeMissing = errors.New("adm: INTERPOLATION_METHOD requires INTERPOLATION_DEGREE")

	// ErrNoRecords indicates an AEM data block with no attitude lines.
	ErrNoRecords = errors.New("adm: an AEM data block must hold at least one attitude line")
)

// Sentinel errors from the Attitude Comprehensive Message (section 5).
var (
	// ErrNotAnACM indicates a file whose first keyword is not CCSDS_ACM_VERS.
	ErrNotAnACM = errors.New("adm: file is not an Attitude Comprehensive Message")

	// ErrMalformedDataRow indicates a positional data row with no fields.
	ErrMalformedDataRow = errors.New("adm: a data row must carry at least a time tag")

	// ErrMixedTimeTags indicates a block whose rows mix relative and absolute
	// time tags. Clause 5.3.4.5 requires a block to pick one for the whole of
	// it, because a reader cannot tell from a bare number whether it is
	// seconds or a malformed date.
	ErrMixedTimeTags = errors.New("adm: a data block must use relative or absolute time tags throughout")

	// ErrDuplicateTimeTag indicates two rows in a block with the same time
	// tag, which clause 5.3.4.4 forbids.
	ErrDuplicateTimeTag = errors.New("adm: duplicate time tags in one data block")

	// ErrTimeTagsOutOfOrder indicates a covariance block whose rows do not run
	// forward in time, which clause 5.3.7.5 requires.
	ErrTimeTagsOutOfOrder = errors.New("adm: covariance time tags must increase")

	// ErrNoEpochTZero indicates a relative time tag in a message whose
	// metadata gives no EPOCH_TZERO to measure it from.
	ErrNoEpochTZero = errors.New("adm: a relative time tag needs EPOCH_TZERO")

	// ErrKeywordsOutOfOrder indicates a section whose keywords do not follow
	// the order of the table they come from, which clauses 5.3.3.5 and 5.3.4.1
	// fix.
	ErrKeywordsOutOfOrder = errors.New("adm: keywords must follow the order of their table")

	// ErrSectionsOutOfOrder indicates data sections in an order other than the
	// one table 5-1 fixes, which clause 5.3.1.2 makes mandatory.
	ErrSectionsOutOfOrder = errors.New("adm: data sections must follow the order of table 5-1")

	// ErrDuplicateSection indicates a second copy of a section the standard
	// allows only once: metadata (clause 5.3.3.4), physical characteristics
	// (5.3.6.3), attitude determination (5.3.9.2) and user-defined parameters
	// (5.3.10.4).
	ErrDuplicateSection = errors.New("adm: this section may appear only once")

	// ErrStateCountMismatch indicates an attitude block whose NUMBER_STATES
	// disagrees with the element counts annex B4 gives its ATT_TYPE and
	// RATE_TYPE. The block says how wide a row is twice over, and a producer
	// and a consumer following different halves read different columns.
	ErrStateCountMismatch = errors.New("adm: NUMBER_STATES disagrees with ATT_TYPE and RATE_TYPE")

	// ErrUnknownCovarianceType indicates a COV_TYPE outside the six values
	// annex B6 defines. Without one there is no way to know how many numbers a
	// covariance row holds.
	ErrUnknownCovarianceType = errors.New("adm: unknown COV_TYPE")

	// ErrCovarianceLineFields indicates a covariance row whose field count
	// does not match the matrix dimension its COV_TYPE names. Clause 5.3.7.6
	// puts the main diagonal on the line and nothing else.
	ErrCovarianceLineFields = errors.New("adm: covariance data line does not match COV_TYPE")

	// ErrUnknownEstimator indicates an AD_METHOD outside the six values
	// annex B5 enumerates.
	ErrUnknownEstimator = errors.New("adm: unknown AD_METHOD")

	// ErrBothManeuverEnds indicates a manoeuvre giving both MAN_END_TIME and
	// MAN_DURATION, which table 5-7 says to give one of.
	ErrBothManeuverEnds = errors.New("adm: give MAN_END_TIME or MAN_DURATION, not both")

	// ErrMissingFrame indicates a vector with no reference frame beside it:
	// CP without CP_REF_FRAME, or TARGET_MOMENTUM without TARGET_MOM_FRAME.
	// Tables 5-5 and 5-7 make each frame conditional on its vector.
	ErrMissingFrame = errors.New("adm: a vector keyword requires its reference frame")

	// ErrVectorWidth indicates a keyword whose value holds the wrong number of
	// components: CP and TARGET_MOMENTUM take three, TARGET_ATTITUDE four.
	ErrVectorWidth = errors.New("adm: vector keyword has the wrong number of components")

	// ErrSensorNoiseCount indicates a SENSOR_NOISE_STDDEV whose component
	// count disagrees with NUMBER_SENSOR_NOISE_COVARIANCE beside it.
	ErrSensorNoiseCount = errors.New("adm: SENSOR_NOISE_STDDEV does not match NUMBER_SENSOR_NOISE_COVARIANCE")

	// ErrDuplicateSensorNumber indicates two sensor blocks claiming the same
	// SENSOR_NUMBER, which table 5-8 requires to be unique.
	ErrDuplicateSensorNumber = errors.New("adm: two sensor blocks share a SENSOR_NUMBER")
)
