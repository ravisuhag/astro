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
