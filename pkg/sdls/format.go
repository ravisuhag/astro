package sdls

import (
	"encoding/hex"
	"strconv"
)

func itoa(n int) string { return strconv.Itoa(n) }

// hexOrNone renders a field as hex, or "(absent)" when it has zero width.
func hexOrNone(b []byte) string {
	if len(b) == 0 {
		return "(absent)"
	}
	return hex.EncodeToString(b)
}
