// Package vectors loads the wire test vectors under vectors/ and runs
// them against this implementation.
//
// The vectors, not this package, are the reference: they carry the octets
// a standard requires and the derivation behind them. See
// vectors/README.md for the format and vectors/CONTRACT.md for the
// consumer contract.
//
// This package is a thin adapter. Each protocol package supplies closures
// mapping a vector's fields to its own API, and the runners here compare
// the result. Keep it small: logic that belongs to a protocol belongs in
// that protocol's package.
package vectors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

// SchemaVersion is the only file version this loader accepts.
const SchemaVersion = 1

// File is one vector file: every vector for one concern of one package.
type File struct {
	SchemaVersion int      `json:"schema_version"`
	Standard      string   `json:"standard"`
	Package       string   `json:"package"`
	Source        string   `json:"source"`
	Note          string   `json:"note,omitempty"`
	Corpus        []string `json:"corpus,omitempty"`

	Encode   []Encode   `json:"encode,omitempty"`
	Decode   []Decode   `json:"decode,omitempty"`
	Reject   []Reject   `json:"reject,omitempty"`
	Sequence []Sequence `json:"sequence,omitempty"`

	// Path is the file this was read from, for test failure messages.
	Path string `json:"-"`
}

// Encode asserts that a set of fields produces an exact octet string.
type Encode struct {
	Name     string   `json:"name"`
	Clause   string   `json:"clause,omitempty"`
	Source   string   `json:"source,omitempty"`
	Note     string   `json:"note"`
	Requires []string `json:"requires,omitempty"`
	Config   Fields   `json:"config,omitempty"`
	Fields   Fields   `json:"fields"`
	Want     string   `json:"want"`
}

// Decode asserts that an octet string produces a set of fields. Only the
// fields listed are compared; anything else the decoder exposes is
// unconstrained.
type Decode struct {
	Name     string   `json:"name"`
	Clause   string   `json:"clause,omitempty"`
	Source   string   `json:"source,omitempty"`
	Note     string   `json:"note"`
	Requires []string `json:"requires,omitempty"`
	Config   Fields   `json:"config,omitempty"`
	Input    string   `json:"input"`
	Fields   Fields   `json:"fields"`
}

// Reject asserts a failure with a named error. Exactly one of Input (bad
// octets, refused at decode) or Fields (bad values, refused at
// construction) is set.
type Reject struct {
	Name      string   `json:"name"`
	Clause    string   `json:"clause,omitempty"`
	Source    string   `json:"source,omitempty"`
	Note      string   `json:"note,omitempty"`
	Requires  []string `json:"requires,omitempty"`
	Config    Fields   `json:"config,omitempty"`
	Input     *string  `json:"input,omitempty"`
	Fields    Fields   `json:"fields,omitempty"`
	BufferLen *int     `json:"buffer_len,omitempty"`
	Error     string   `json:"error"`
}

// Sequence is a scripted run against a state machine. The schema defines
// it and no file carries one yet. The loader validates it so a fixture
// cannot be added that the runners would later reject.
type Sequence struct {
	Name   string `json:"name"`
	Clause string `json:"clause,omitempty"`
	Source string `json:"source,omitempty"`
	Note   string `json:"note"`
	Config Fields `json:"config,omitempty"`
	Init   Fields `json:"init,omitempty"`
	Steps  []Step `json:"steps"`
}

// Step is one call in a Sequence.
type Step struct {
	Call      string `json:"call"`
	Fields    Fields `json:"fields,omitempty"`
	Want      string `json:"want,omitempty"`
	WantState Fields `json:"want_state,omitempty"`
	Error     string `json:"error,omitempty"`
}

var (
	hexRE    = regexp.MustCompile(`^([0-9a-f]{2})*$`)
	nameRE   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	fieldRE  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	callRE   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	pkgRE    = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	minNote  = 10
	capNames = map[string]bool{"encode_into": true}
)

// Root returns the absolute path of the vectors directory, located
// relative to this source file so tests work from any package directory.
func Root() string {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		panic("vectors: cannot locate the vectors directory")
	}
	return filepath.Join(filepath.Dir(self), "..", "..", "vectors")
}

// Load reads one vector file. rel is relative to the vectors directory,
// for example "spp/packet.json".
func Load(rel string) (*File, error) {
	path := filepath.Join(Root(), rel)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("vectors: %w", err)
	}

	return Parse(raw, rel)
}

// Parse reads one vector file from memory. name is used in error
// messages. It applies exactly the rules vectors/schema.json states, so
// the loader and the schema stay interchangeable.
func Parse(raw []byte, name string) (*File, error) {
	var f File
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("vectors %s: %w", name, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("vectors %s: trailing content after the top-level object", name)
	}
	f.Path = name

	if err := f.validate(); err != nil {
		return nil, fmt.Errorf("vectors %s: %w", name, err)
	}
	return &f, nil
}

// LoadAll reads every vector file under the vectors directory.
func LoadAll() ([]*File, error) {
	var out []*File
	err := filepath.WalkDir(Root(), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		// schema.json describes the format; it is not a vector file.
		if filepath.Base(path) == "schema.json" {
			return nil
		}
		rel, err := filepath.Rel(Root(), path)
		if err != nil {
			return err
		}
		f, err := Load(rel)
		if err != nil {
			return err
		}
		out = append(out, f)
		return nil
	})
	return out, err
}
