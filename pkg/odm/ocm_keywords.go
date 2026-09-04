package odm

// The OCM keyword tables, CCSDS 502.0-B-3 section 6.
//
// Each table is kept in the order the Blue Book prints it, because the order
// is itself a rule: clause 6.2.2.1 says the order of occurrence of OCM
// keywords is fixed as listed in the keyword value tables. So one slice per
// section serves twice, as the set of keywords the section may carry
// (clauses 6.2.4.1, 6.2.5.1 and their siblings) and as the order they must
// arrive in.
//
// The delimiters and COMMENT are left out; they are handled by the reader
// before it reaches the table.

// ocmMetadataOrder is table 6-3.
var ocmMetadataOrder = []string{
	"OBJECT_NAME", "INTERNATIONAL_DESIGNATOR", "CATALOG_NAME",
	"OBJECT_DESIGNATOR", "ALTERNATE_NAMES", "ORIGINATOR_POC",
	"ORIGINATOR_POSITION", "ORIGINATOR_PHONE", "ORIGINATOR_EMAIL",
	"ORIGINATOR_ADDRESS", "TECH_ORG", "TECH_POC", "TECH_POSITION",
	"TECH_PHONE", "TECH_EMAIL", "TECH_ADDRESS", "PREVIOUS_MESSAGE_ID",
	"NEXT_MESSAGE_ID", "ADM_MSG_LINK", "CDM_MSG_LINK", "PRM_MSG_LINK",
	"RDM_MSG_LINK", "TDM_MSG_LINK", "OPERATOR", "OWNER", "COUNTRY",
	"CONSTELLATION", "OBJECT_TYPE", "TIME_SYSTEM", "EPOCH_TZERO",
	"OPS_STATUS", "ORBIT_CATEGORY", "OCM_DATA_ELEMENTS",
	"SCLK_OFFSET_AT_EPOCH", "SCLK_SEC_PER_SI_SEC", "PREVIOUS_MESSAGE_EPOCH",
	"NEXT_MESSAGE_EPOCH", "START_TIME", "STOP_TIME", "TIME_SPAN",
	"TAIMUTC_AT_TZERO", "NEXT_LEAP_EPOCH", "NEXT_LEAP_TAIMUTC",
	"UT1MUTC_AT_TZERO", "EOP_SOURCE", "INTERP_METHOD_EOP", "CELESTIAL_SOURCE",
}

// ocmTrajOrder is table 6-4.
var ocmTrajOrder = []string{
	"TRAJ_ID", "TRAJ_PREV_ID", "TRAJ_NEXT_ID", "TRAJ_BASIS", "TRAJ_BASIS_ID",
	"INTERPOLATION", "INTERPOLATION_DEGREE", "PROPAGATOR", "CENTER_NAME",
	"TRAJ_REF_FRAME", "TRAJ_FRAME_EPOCH", "USEABLE_START_TIME",
	"USEABLE_STOP_TIME", "ORB_REVNUM", "ORB_REVNUM_BASIS", "TRAJ_TYPE",
	"ORB_AVERAGING", "TRAJ_UNITS",
}

// ocmPhysOrder is table 6-5.
var ocmPhysOrder = []string{
	"MANUFACTURER", "BUS_MODEL", "DOCKED_WITH", "DRAG_CONST_AREA",
	"DRAG_COEFF_NOM", "DRAG_UNCERTAINTY", "INITIAL_WET_MASS", "WET_MASS",
	"DRY_MASS", "OEB_PARENT_FRAME", "OEB_PARENT_FRAME_EPOCH", "OEB_Q1",
	"OEB_Q2", "OEB_Q3", "OEB_QC", "OEB_MAX", "OEB_INT", "OEB_MIN",
	"AREA_ALONG_OEB_MAX", "AREA_ALONG_OEB_INT", "AREA_ALONG_OEB_MIN",
	"AREA_MIN_FOR_PC", "AREA_MAX_FOR_PC", "AREA_TYP_FOR_PC", "RCS", "RCS_MIN",
	"RCS_MAX", "SRP_CONST_AREA", "SOLAR_RAD_COEFF", "SOLAR_RAD_UNCERTAINTY",
	"VM_ABSOLUTE", "VM_APPARENT_MIN", "VM_APPARENT", "VM_APPARENT_MAX",
	"REFLECTANCE", "ATT_CONTROL_MODE", "ATT_ACTUATOR_TYPE", "ATT_KNOWLEDGE",
	"ATT_CONTROL", "ATT_POINTING", "AVG_MANEUVER_FREQ", "MAX_THRUST",
	"DV_BOL", "DV_REMAINING", "IXX", "IYY", "IZZ", "IXY", "IXZ", "IYZ",
}

// ocmCovOrder is table 6-6.
var ocmCovOrder = []string{
	"COV_ID", "COV_PREV_ID", "COV_NEXT_ID", "COV_BASIS", "COV_BASIS_ID",
	"COV_REF_FRAME", "COV_FRAME_EPOCH", "COV_SCALE_MIN", "COV_SCALE_MAX",
	"COV_CONFIDENCE", "COV_TYPE", "COV_ORDERING", "COV_UNITS",
}

// ocmManOrder is table 6-7.
var ocmManOrder = []string{
	"MAN_ID", "MAN_PREV_ID", "MAN_NEXT_ID", "MAN_BASIS", "MAN_BASIS_ID",
	"MAN_DEVICE_ID", "MAN_PREV_EPOCH", "MAN_NEXT_EPOCH", "MAN_PURPOSE",
	"MAN_PRED_SOURCE", "MAN_REF_FRAME", "MAN_FRAME_EPOCH", "GRAV_ASSIST_NAME",
	"DC_TYPE", "DC_WIN_OPEN", "DC_WIN_CLOSE", "DC_MIN_CYCLES",
	"DC_MAX_CYCLES", "DC_EXEC_START", "DC_EXEC_STOP", "DC_REF_TIME",
	"DC_TIME_PULSE_DURATION", "DC_TIME_PULSE_PERIOD", "DC_REF_DIR",
	"DC_BODY_FRAME", "DC_BODY_TRIGGER", "DC_PA_START_ANGLE",
	"DC_PA_STOP_ANGLE", "MAN_COMPOSITION", "MAN_UNITS",
}

// ocmPertOrder is table 6-10.
var ocmPertOrder = []string{
	"ATMOSPHERIC_MODEL", "GRAVITY_MODEL", "EQUATORIAL_RADIUS", "GM",
	"N_BODY_PERTURBATIONS", "CENTRAL_BODY_ROTATION", "OBLATE_FLATTENING",
	"OCEAN_TIDES_MODEL", "SOLID_TIDES_MODEL", "REDUCTION_THEORY",
	"ALBEDO_MODEL", "ALBEDO_GRID_SIZE", "SHADOW_MODEL", "SHADOW_BODIES",
	"SRP_MODEL", "SW_DATA_SOURCE", "SW_DATA_EPOCH", "SW_INTERP_METHOD",
	"FIXED_GEOMAG_KP", "FIXED_GEOMAG_AP", "FIXED_GEOMAG_DST", "FIXED_F10P7",
	"FIXED_F10P7_MEAN", "FIXED_M10P7", "FIXED_M10P7_MEAN", "FIXED_S10P7",
	"FIXED_S10P7_MEAN", "FIXED_Y10P7", "FIXED_Y10P7_MEAN",
}

// ocmODOrder is table 6-11.
var ocmODOrder = []string{
	"OD_ID", "OD_PREV_ID", "OD_METHOD", "OD_EPOCH", "DAYS_SINCE_FIRST_OBS",
	"DAYS_SINCE_LAST_OBS", "RECOMMENDED_OD_SPAN", "ACTUAL_OD_SPAN",
	"OBS_AVAILABLE", "OBS_USED", "TRACKS_AVAILABLE", "TRACKS_USED",
	"MAXIMUM_OBS_GAP", "OD_EPOCH_EIGMAJ", "OD_EPOCH_EIGINT",
	"OD_EPOCH_EIGMIN", "OD_MAX_PRED_EIGMAJ", "OD_MIN_PRED_EIGMIN",
	"OD_CONFIDENCE", "GDOP", "SOLVE_N", "SOLVE_STATES", "CONSIDER_N",
	"CONSIDER_PARAMS", "SEDR", "SENSORS_N", "SENSORS", "WEIGHTED_RMS",
	"DATA_TYPES",
}

// ocmSectionOrder maps a section's delimiter prefix to its keyword table.
var ocmSectionOrder = map[string][]string{
	ocmMeta: ocmMetadataOrder,
	ocmTraj: ocmTrajOrder,
	ocmPhys: ocmPhysOrder,
	ocmCov:  ocmCovOrder,
	ocmMan:  ocmManOrder,
	ocmPert: ocmPertOrder,
	ocmOD:   ocmODOrder,
}

// ocmSectionIndex is ocmSectionOrder as lookup tables, built once.
var ocmSectionIndex = func() map[string]map[string]int {
	out := make(map[string]map[string]int, len(ocmSectionOrder))
	for section, keywords := range ocmSectionOrder {
		positions := make(map[string]int, len(keywords))
		for i, keyword := range keywords {
			positions[keyword] = i
		}
		out[section] = positions
	}
	return out
}()

// ocmRequired lists, per section, the keywords a message must carry when that
// section is present.
//
// It is much shorter than the count of 'M' rows in the tables, and clause
// 6.2.1.3 is why: where a mandatory keyword has a default and the default is
// what the producer meant, the keyword may be left out and the recipient
// adopts the default. Only the mandatory keywords with no default are
// genuinely required. TRAJ has none — its three mandatory keywords all default
// (CENTER_NAME to EARTH, TRAJ_REF_FRAME to ICRF3, TRAJ_TYPE to CARTPV) —
// which is what lets figure G-15 print a trajectory block holding a frame and
// six numbers.
var ocmRequired = map[string][]string{
	ocmMeta: {"EPOCH_TZERO"},
	ocmMan:  {"MAN_ID", "MAN_DEVICE_ID", "MAN_COMPOSITION"},
	ocmOD:   {"OD_ID", "OD_METHOD", "OD_EPOCH"},
}

// The defaults clause 6.2.1.3 lets a producer leave out. Only the ones a
// reader needs to make sense of the data are named here; the rest are
// descriptive and a caller that wants them can ask GetOr for them directly.
const (
	ocmDefaultTimeSystem  = "UTC"
	ocmDefaultCenterName  = "EARTH"
	ocmDefaultTrajFrame   = "ICRF3"
	ocmDefaultTrajType    = "CARTPV"
	ocmDefaultCovFrame    = "TNW_INERTIAL"
	ocmDefaultCovType     = "CARTPV"
	ocmDefaultCovOrdering = "LTM"
	ocmDefaultManFrame    = "TNW_INERTIAL"
	ocmDefaultDutyCycle   = "CONTINUOUS"
	ocmDefaultOEBFrame    = "RSW_ROTATING"
)

// The propulsive manoeuvre fields of table 6-8, in the order clause 6.2.8.16
// fixes. MAN_COMPOSITION names a subset of these, or of table 6-9's
// deployment fields, and clause 6.2.8.15 forbids mixing the two.
//
// This is the OCM's answer to the AEM's ATTITUDE_TYPE: the row layout is
// named rather than fixed. Unlike TRAJ_TYPE and COV_TYPE, whose values live in
// the SANA registry, these are printed in the Blue Book, so a manoeuvre row
// can be checked and its columns named.
var ocmManPropulsiveFields = []string{
	"TIME_ABSOLUTE", "TIME_RELATIVE", "MAN_DURA", "DELTA_MASS",
	"ACC_X", "ACC_Y", "ACC_Z", "ACC_INTERP", "ACC_MAG_SIGMA", "ACC_DIR_SIGMA",
	"DV_X", "DV_Y", "DV_Z", "DV_MAG_SIGMA", "DV_DIR_SIGMA",
	"THR_X", "THR_Y", "THR_Z", "THR_EFFIC", "THR_INTERP", "THR_ISP",
	"THR_MAG_SIGMA", "THR_DIR_SIGMA",
}

// The deployment manoeuvre fields of table 6-9, in the order clause 6.2.8.16
// fixes.
var ocmManDeploymentFields = []string{
	"TIME_ABSOLUTE", "TIME_RELATIVE", "DEPLOY_ID",
	"DEPLOY_DV_X", "DEPLOY_DV_Y", "DEPLOY_DV_Z", "DEPLOY_MASS",
	"DEPLOY_DV_SIGMA", "DEPLOY_DIR_SIGMA", "DEPLOY_DV_RATIO", "DEPLOY_DV_CDA",
}
