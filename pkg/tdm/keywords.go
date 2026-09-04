package tdm

import "strings"

// metadataKeywords is table 3-3, minus the two delimiters and COMMENT.
// Clause 3.3.1.7 says only these may appear in a metadata section, so an
// unknown one is refused rather than carried.
//
// The indexed families — PARTICIPANT_n, TRANSMIT_FREQ_n and the delays — are
// not in this set; isMetadataKeyword handles them by prefix, because the index
// is part of the keyword rather than of the value.
var metadataKeywords = map[string]bool{
	"TIME_SYSTEM": true, "START_TIME": true, "STOP_TIME": true,
	"TRACK_ID": true, "DATA_TYPES": true,
	"MODE": true, "PATH": true, "PATH_1": true, "PATH_2": true,
	"EPHEMERIS_NAME_1": true, "EPHEMERIS_NAME_2": true, "EPHEMERIS_NAME_3": true,
	"EPHEMERIS_NAME_4": true, "EPHEMERIS_NAME_5": true,
	"TRANSMIT_BAND": true, "RECEIVE_BAND": true,
	"TURNAROUND_NUMERATOR": true, "TURNAROUND_DENOMINATOR": true,
	"TIMETAG_REF":          true,
	"INTEGRATION_INTERVAL": true, "INTEGRATION_REF": true,
	"FREQ_OFFSET": true,
	"RANGE_MODE":  true, "RANGE_MODULUS": true, "RANGE_UNITS": true,
	"ANGLE_TYPE": true, "REFERENCE_FRAME": true,
	"INTERPOLATION": true, "INTERPOLATION_DEGREE": true,
	"DOPPLER_COUNT_BIAS": true, "DOPPLER_COUNT_SCALE": true,
	"DOPPLER_COUNT_ROLLOVER": true,
	"CORRECTION_ANGLE_1":     true, "CORRECTION_ANGLE_2": true,
	"CORRECTION_DOPPLER": true, "CORRECTION_MAG": true,
	"CORRECTION_RANGE": true, "CORRECTION_RCS": true,
	"CORRECTION_RECEIVE": true, "CORRECTION_TRANSMIT": true,
	"CORRECTION_ABERRATION_YEARLY": true, "CORRECTION_ABERRATION_DIURNAL": true,
	"CORRECTIONS_APPLIED": true,
	"DATA_QUALITY":        true,
}

// indexedMetadataPrefixes are the metadata families table 3-3 indexes by
// participant, 1 to 5.
var indexedMetadataPrefixes = []string{
	"PARTICIPANT_",
	"TRANSMIT_DELAY_",
	"RECEIVE_DELAY_",
}

// isMetadataKeyword reports whether table 3-3 allows this keyword.
func isMetadataKeyword(keyword string) bool {
	return metadataKeywords[keyword] || matchesIndexed(keyword, indexedMetadataPrefixes)
}

// dataKeywords is table 3-5, the data section keywords that are not indexed.
var dataKeywords = map[string]bool{
	"ANGLE_1": true, "ANGLE_2": true,
	"CARRIER_POWER": true,
	"CLOCK_BIAS":    true, "CLOCK_DRIFT": true,
	"DOPPLER_COUNT":         true,
	"DOPPLER_INSTANTANEOUS": true, "DOPPLER_INTEGRATED": true,
	"DOR": true, "VLBI_DELAY": true,
	"MAG": true, "RCS": true,
	"PC_N0": true, "PR_N0": true,
	"PRESSURE": true, "RHUMIDITY": true, "TEMPERATURE": true,
	"RANGE":        true,
	"RECEIVE_FREQ": true,
	"STEC":         true, "TROPO_DRY": true, "TROPO_WET": true,
}

// indexedDataPrefixes are the data families table 3-5 indexes 1 to 5.
//
// RECEIVE_FREQ is the odd one: table 3-5 lists both RECEIVE_FREQ and
// RECEIVE_FREQ_n, so the bare name is legal as well as the indexed one. The
// transmit and phase-count families are indexed only.
var indexedDataPrefixes = []string{
	"RECEIVE_FREQ_",
	"RECEIVE_PHASE_CT_",
	"TRANSMIT_FREQ_",
	"TRANSMIT_FREQ_RATE_",
	"TRANSMIT_PHASE_CT_",
}

// isDataKeyword reports whether table 3-5 allows this keyword.
func isDataKeyword(keyword string) bool {
	return dataKeywords[keyword] || matchesIndexed(keyword, indexedDataPrefixes)
}

// matchesIndexed reports whether a keyword is one of the indexed families.
//
// Every prefix is tried rather than the first that matches, because one of
// them is a prefix of another: TRANSMIT_FREQ_ and TRANSMIT_FREQ_RATE_ both
// start the same way, and stopping at the shorter one refuses
// TRANSMIT_FREQ_RATE_1 on the grounds that "RATE_1" is not an index.
func matchesIndexed(keyword string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if suffix, ok := strings.CutPrefix(keyword, prefix); ok && isIndex(suffix) {
			return true
		}
	}
	return false
}

// isIndex reports whether a keyword suffix is one of the indices 1 to 5 that
// tables 3-3 and 3-5 allow.
func isIndex(s string) bool {
	return len(s) == 1 && s[0] >= '1' && s[0] <= '5'
}
