package adm

// The ACM keyword tables, CCSDS 504.0-B-2 section 5.
//
// Each table is kept in the order the Blue Book prints it, because the order
// is itself a rule: clauses 5.3.3.5 and 5.3.4.1 fix the order of occurrence of
// the metadata and data keywords as listed in the tables. So one slice per
// section serves twice, as the set of keywords the section may carry and as
// the order they must arrive in.
//
// The delimiters and COMMENT are left out; they are handled by the reader
// before it reaches the table.

// acmMetadataOrder is table 5-3.
var acmMetadataOrder = []string{
	"CLASSIFICATION", "OBJECT_NAME", "INTERNATIONAL_DESIGNATOR",
	"CATALOG_NAME", "OBJECT_DESIGNATOR", "ORIGINATOR_POC",
	"ORIGINATOR_POSITION", "ORIGINATOR_PHONE", "ORIGINATOR_EMAIL",
	"ORIGINATOR_ADDRESS", "ODM_MSG_LINK", "CENTER_NAME", "TIME_SYSTEM",
	"EPOCH_TZERO", "ACM_DATA_ELEMENTS", "START_TIME", "STOP_TIME",
	"TAIMUTC_AT_TZERO", "NEXT_LEAP_EPOCH", "NEXT_LEAP_TAIMUTC",
}

// acmAttitudeOrder is table 5-4.
var acmAttitudeOrder = []string{
	"ATT_ID", "ATT_PREV_ID", "ATT_BASIS", "ATT_BASIS_ID",
	"REF_FRAME_A", "REF_FRAME_B", "NUMBER_STATES", "ATT_TYPE",
	"EULER_ROT_SEQ", "RATE_TYPE",
}

// acmPhysicalOrder is table 5-5.
var acmPhysicalOrder = []string{
	"DRAG_COEFF", "WET_MASS", "DRY_MASS", "CP_REF_FRAME", "CP",
	"INERTIA_REF_FRAME", "IXX", "IYY", "IZZ", "IXY", "IXZ", "IYZ",
}

// acmCovarianceOrder is table 5-6.
var acmCovarianceOrder = []string{
	"COV_ID", "COV_PREV_ID", "COV_BASIS", "COV_BASIS_ID",
	"COV_REF_FRAME", "COV_TYPE",
}

// acmManeuverOrder is table 5-7.
var acmManeuverOrder = []string{
	"MAN_ID", "MAN_PREV_ID", "MAN_PURPOSE", "MAN_BEGIN_TIME",
	"MAN_END_TIME", "MAN_DURATION", "ACTUATOR_USED", "TARGET_MOMENTUM",
	"TARGET_MOM_FRAME", "TARGET_ATTITUDE", "TARGET_SPINRATE",
}

// acmDeterminationOrder is table 5-8, without the sensor sub-block.
var acmDeterminationOrder = []string{
	"AD_ID", "AD_PREV_ID", "AD_METHOD", "ATTITUDE_SOURCE", "NUMBER_STATES",
	"ATTITUDE_STATES", "EULER_ROT_SEQ", "COV_TYPE", "REF_FRAME_A",
	"REF_FRAME_B", "RATE_STATES", "SIGMA_U", "SIGMA_V",
	"RATE_PROCESS_NOISE_STDDEV",
}

// acmSensorOrder is the sensor sub-block of table 5-8, which clause 5.3.9.6
// delimits with its own SENSOR_START and SENSOR_STOP inside the attitude
// determination section. It is the only nested block in either standard this
// repository implements.
var acmSensorOrder = []string{
	"SENSOR_NUMBER", "SENSOR_USED", "NUMBER_SENSOR_NOISE_COVARIANCE",
	"SENSOR_NOISE_STDDEV", "SENSOR_FREQUENCY",
}

// acmSectionOrder maps a section's delimiter prefix to its keyword table.
var acmSectionOrder = map[string][]string{
	acmMeta:   acmMetadataOrder,
	acmAtt:    acmAttitudeOrder,
	acmPhys:   acmPhysicalOrder,
	acmCov:    acmCovarianceOrder,
	acmMan:    acmManeuverOrder,
	acmAD:     acmDeterminationOrder,
	acmSensor: acmSensorOrder,
}

// acmSectionIndex is acmSectionOrder as lookup tables, built once.
var acmSectionIndex = func() map[string]map[string]int {
	out := make(map[string]map[string]int, len(acmSectionOrder))
	for section, keywords := range acmSectionOrder {
		positions := make(map[string]int, len(keywords))
		for i, keyword := range keywords {
			positions[keyword] = i
		}
		out[section] = positions
	}
	return out
}()

// acmRequired lists, per section, the keywords marked mandatory in that
// section's table.
//
// Unlike the OCM's, none of these have a default to fall back on: the ACM
// tables carry no 'Default' column at all, so a mandatory keyword really must
// be written.
var acmRequired = map[string][]string{
	acmMeta: {"OBJECT_NAME", "TIME_SYSTEM", "EPOCH_TZERO"},
	acmAtt:  {"REF_FRAME_A", "REF_FRAME_B", "NUMBER_STATES", "ATT_TYPE"},
	acmCov:  {"COV_TYPE"},
	acmMan:  {"MAN_PURPOSE", "MAN_BEGIN_TIME"},
	acmAD:   {"ATTITUDE_STATES", "REF_FRAME_A", "REF_FRAME_B"},
}

// The attitude and rate types of annex B4, with how many numbers each one
// puts on a data line.
//
// This is what most separates the ACM from the ODM's OCM. The OCM's TRAJ_TYPE
// and COV_TYPE come from the SANA registry, so nothing in that Blue Book says
// how wide a row should be. Annex B4 prints these counts, and table 5-4 makes
// NUMBER_STATES mandatory besides, so an ACM attitude row can be checked twice
// over: against the types, and against the count the producer declared.
var acmAttitudeElements = map[string]int{
	"QUATERNION":   4, // vector part first, scalar last
	"EULER_ANGLES": 3, // needs EULER_ROT_SEQ to mean anything
	"DCM":          9, // by columns: the first three are column one
}

// acmRateElements is the rate half of annex B4. NONE is not in the table; it
// is in table 5-4's examples for RATE_TYPE, and means no rate data follows.
var acmRateElements = map[string]int{
	"ANGVEL":     3,
	"Q_DOT":      4,
	"EULER_RATE": 3,
	"GYRO_BIAS":  3,
	"NONE":       0,
}

// acmCovarianceElements is annex B6: the dimension of the matrix each
// covariance type describes.
//
// Clause 5.3.7.6 puts only the main diagonal on the line, so a covariance row
// holds a time tag and this many numbers — not the triangle the OCM writes.
// Clause 5.3.7.7 says the off-diagonal terms, if anyone needs them, go in a
// user-defined block.
var acmCovarianceElements = map[string]int{
	"ANGLE":               3,
	"ANGLE_GYROBIAS":      6,
	"ANGLE_ANGVEL":        6,
	"QUATERNION":          4,
	"QUATERNION_GYROBIAS": 7,
	"QUATERNION_ANGVEL":   7,
}

// acmEstimators is annex B5, the values AD_METHOD may take.
var acmEstimators = map[string]bool{
	"EKF": true, "TRIAD": true, "QUEST": true,
	"BATCH": true, "Q_METHOD": true, "FILTER_SMOOTHER": true,
}

// acmVectorWidths gives the keywords whose value is several numbers rather
// than one, and how many. Tables 5-5 and 5-7 state each count in words.
var acmVectorWidths = map[string]int{
	"CP":              3, // one per axis of CP_REF_FRAME
	"TARGET_MOMENTUM": 3, // one per axis of TARGET_MOM_FRAME
	"TARGET_ATTITUDE": 4, // a quaternion, in the order Q1, Q2, Q3, QC
}
