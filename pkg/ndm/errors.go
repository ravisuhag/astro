package ndm

import "errors"

// Sentinel errors from the NDM combined instantiation (CCSDS 505.0-B-3
// clause 4.11).
var (
	// ErrUnknownMessageType indicates a constituent whose element name is not
	// one of the nine navigation messages this repository implements. Clause
	// 4.11.6 draws them from table 3-1, which also lists the Re-entry Data
	// Message; that standard has no package here.
	ErrUnknownMessageType = errors.New("ndm: not a navigation message type this package reads")

	// ErrNoMessage indicates a nil constituent, or an empty file with no
	// schema to fall back on.
	ErrNoMessage = errors.New("ndm: a combined instantiation needs a message to take its schema from")
)
