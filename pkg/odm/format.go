package odm

import "math"

// sqrt returns the square root, and zero for a negative input.
//
// A covariance diagonal cannot be negative in a well-formed matrix, but this
// package does not check that a message's numbers are physically sensible —
// clause 1.2 puts that outside the standard — so a dump has to survive one
// that is.
func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	return math.Sqrt(v)
}
