package ndm_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/ndm"
)

// The vectors for this package live in vectors/ndm/.
//
// What they assert is the structure a combined instantiation adds: how many
// messages a file holds, of what types, in what order, and which schema the
// root names. The constituents themselves are asserted by their own packages'
// vectors, because a message inside a combined file is the same message.
func TestCombinedVectors(t *testing.T) {
	vectors.RunFile(t, "ndm/combined.json", vectors.Impl{
		DecodeFn: decodeVector,
	})
}

func decodeVector(input []byte, _ vectors.Fields) (vectors.Fields, error) {
	c, err := ndm.DecodeCombined(input)
	if err != nil {
		return nil, err
	}
	return combinedFields(c), nil
}

// combinedFields reports what a vector can compare.
func combinedFields(c *ndm.Combined) vectors.Fields {
	kinds := make([]string, 0, len(c.Messages))
	for _, m := range c.Messages {
		kinds = append(kinds, ndm.Kind(m))
	}

	f := vectors.Fields{
		"schema":        c.Schema,
		"message_count": uint64(len(c.Messages)),
		"comment_count": uint64(len(c.Comments)),
		"kinds":         joinKinds(kinds),
	}
	if len(c.Messages) > 0 {
		f["first_kind"] = kinds[0]
	}
	return f
}

// joinKinds renders the constituent types as one comma-separated string,
// because a vector field holds a scalar rather than a list. The order is the
// file's, which is the part worth pinning.
func joinKinds(kinds []string) string {
	out := ""
	for i, k := range kinds {
		if i > 0 {
			out += ","
		}
		out += k
	}
	return out
}
