package cdm

// relativeKeywords is table 3-2, the relative metadata and data that describe
// the conjunction itself rather than either object.
var relativeKeywords = map[string]bool{
	"TCA": true, "MISS_DISTANCE": true, "RELATIVE_SPEED": true,
	"RELATIVE_POSITION_R": true, "RELATIVE_POSITION_T": true, "RELATIVE_POSITION_N": true,
	"RELATIVE_VELOCITY_R": true, "RELATIVE_VELOCITY_T": true, "RELATIVE_VELOCITY_N": true,
	"START_SCREEN_PERIOD": true, "STOP_SCREEN_PERIOD": true,
	"SCREEN_VOLUME_FRAME": true, "SCREEN_VOLUME_SHAPE": true,
	"SCREEN_VOLUME_X": true, "SCREEN_VOLUME_Y": true, "SCREEN_VOLUME_Z": true,
	"SCREEN_ENTRY_TIME": true, "SCREEN_EXIT_TIME": true,
	"COLLISION_PROBABILITY": true, "COLLISION_PROBABILITY_METHOD": true,
}

// objectMetadataKeywords is table 3-3.
var objectMetadataKeywords = map[string]bool{
	"OBJECT": true, "OBJECT_DESIGNATOR": true, "CATALOG_NAME": true,
	"OBJECT_NAME": true, "INTERNATIONAL_DESIGNATOR": true,
	"OBJECT_TYPE":               true,
	"OPERATOR_CONTACT_POSITION": true, "OPERATOR_ORGANIZATION": true,
	"OPERATOR_PHONE": true, "OPERATOR_EMAIL": true,
	"EPHEMERIS_NAME": true, "COVARIANCE_METHOD": true,
	"MANEUVERABLE": true, "ORBIT_CENTER": true, "REF_FRAME": true,
	"GRAVITY_MODEL": true, "ATMOSPHERIC_MODEL": true,
	"N_BODY_PERTURBATIONS": true, "SOLAR_RAD_PRESSURE": true,
	"EARTH_TIDES": true, "INTRACK_THRUST": true,
}

// objectDataKeywords is tables 3-4 through 3-8: the OD parameters, the
// additional physical parameters, the state vector and the covariance.
var objectDataKeywords = map[string]bool{
	// Table 3-5, OD parameters.
	"TIME_LASTOB_START": true, "TIME_LASTOB_END": true,
	"RECOMMENDED_OD_SPAN": true, "ACTUAL_OD_SPAN": true,
	"OBS_AVAILABLE": true, "OBS_USED": true,
	"TRACKS_AVAILABLE": true, "TRACKS_USED": true,
	"RESIDUALS_ACCEPTED": true, "WEIGHTED_RMS": true,

	// Table 3-6, additional parameters.
	"AREA_PC": true, "AREA_DRG": true, "AREA_SRP": true, "MASS": true,
	"CD_AREA_OVER_MASS": true, "CR_AREA_OVER_MASS": true,
	"THRUST_ACCELERATION": true, "SEDR": true,

	// Table 3-7, the state vector.
	"X": true, "Y": true, "Z": true,
	"X_DOT": true, "Y_DOT": true, "Z_DOT": true,
}

func init() {
	// Table 3-8, the covariance matrix. Its keywords are generated from the
	// axis names rather than listed, because the naming is regular and a hand
	// list of forty-five is forty-five chances to mistype one.
	for keyword := range covarianceIndex {
		objectDataKeywords[keyword] = true
	}
}

// covarianceAxes are the nine rows of the covariance matrix, in the order
// table 3-8 puts them.
//
// The first six are position and velocity in the RTN frame. The last three are
// the uncertainties in drag, solar radiation pressure and thrust, and they are
// what makes this matrix 9x9 rather than the 6x6 the orbit messages carry.
var covarianceAxes = []string{"R", "T", "N", "RDOT", "TDOT", "NDOT", "DRG", "SRP", "THR"}

// covarianceIndex maps each covariance keyword to its place in the matrix.
//
// The keyword is the row axis prefixed with C, then the column axis: CR_R is
// [1,1], CT_R is [2,1], CTHR_THR is [9,9]. Only the lower triangle has
// keywords, because the matrix is symmetric.
var covarianceIndex = func() map[string][2]int {
	out := make(map[string][2]int)
	for row, rowAxis := range covarianceAxes {
		for col := 0; col <= row; col++ {
			out["C"+rowAxis+"_"+covarianceAxes[col]] = [2]int{row, col}
		}
	}
	return out
}()
