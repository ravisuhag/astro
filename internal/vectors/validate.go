package vectors

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validate enforces in code every rule vectors/schema.json states. The
// standard library has no JSON Schema engine and this module adds no
// dependencies, so the two are kept in agreement by feeding both the same
// known-bad fixtures (see TestLoaderRejects and the vectors target).
func (f *File) validate() error {
	var errs []string
	add := func(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }

	if f.SchemaVersion != SchemaVersion {
		add("schema_version is %d, this loader accepts %d", f.SchemaVersion, SchemaVersion)
	}
	if len(f.Standard) < 3 {
		add("standard is missing or too short")
	}
	if !pkgRE.MatchString(f.Package) {
		add("package %q is not a lowercase package name", f.Package)
	}
	if len(f.Source) < 3 {
		add("source is missing: say where these vectors came from")
	}

	// A file either carries vectors or points at a published corpus. The
	// corpus form exists so the 121.0 data set and the shared XTCE
	// documents are referenced where they lie rather than transcribed.
	total := len(f.Encode) + len(f.Decode) + len(f.Reject) + len(f.Sequence)
	if total == 0 && len(f.Corpus) == 0 {
		add("file holds no vectors and names no corpus")
	}
	for _, rel := range f.Corpus {
		if strings.HasPrefix(rel, "/") || strings.Contains(rel, "..") {
			add("corpus path %q must be relative to vectors/ and may not escape it", rel)
			continue
		}
		if _, err := os.Stat(filepath.Join(Root(), rel)); err != nil {
			add("corpus path %q does not exist", rel)
		}
	}

	// Names are unique across every kind in a file, so a failure message
	// names one vector unambiguously.
	seen := map[string]string{}
	claim := func(kind, name string) {
		if !nameRE.MatchString(name) {
			add("%s %q: name must be lowercase words joined by hyphens", kind, name)
			return
		}
		if prev, dup := seen[name]; dup {
			add("%s %q: name already used by %s in this file", kind, name, prev)
			return
		}
		seen[name] = kind
	}
	checkNote := func(kind, name, note string) {
		if len(strings.TrimSpace(note)) < minNote {
			add("%s %q: note is required and must say how the value was derived", kind, name)
		}
	}
	checkCaps := func(kind, name string, reqs []string) {
		for _, r := range reqs {
			if !capNames[r] {
				add("%s %q: unknown capability %q in requires", kind, name, r)
			}
		}
	}
	checkFieldNames := func(kind, name, what string, fs Fields) {
		for _, k := range fs.Names() {
			if !fieldRE.MatchString(k) {
				add("%s %q: %s name %q must be snake_case", kind, name, what, k)
			}
		}
	}

	for _, v := range f.Encode {
		claim("encode", v.Name)
		checkNote("encode", v.Name, v.Note)
		checkCaps("encode", v.Name, v.Requires)
		checkFieldNames("encode", v.Name, "field", v.Fields)
		checkFieldNames("encode", v.Name, "config", v.Config)
		if len(v.Fields) == 0 {
			add("encode %q: fields is required", v.Name)
		}
		if !hexRE.MatchString(v.Want) {
			add("encode %q: want %q is not lowercase hex octet pairs", v.Name, v.Want)
		}
	}

	for _, v := range f.Decode {
		claim("decode", v.Name)
		checkNote("decode", v.Name, v.Note)
		checkCaps("decode", v.Name, v.Requires)
		checkFieldNames("decode", v.Name, "field", v.Fields)
		checkFieldNames("decode", v.Name, "config", v.Config)
		if !hexRE.MatchString(v.Input) {
			add("decode %q: input %q is not lowercase hex octet pairs", v.Name, v.Input)
		}
		if len(v.Fields) == 0 {
			add("decode %q: fields is required, and names what this vector pins", v.Name)
		}
	}

	for _, v := range f.Reject {
		claim("reject", v.Name)
		checkCaps("reject", v.Name, v.Requires)
		checkFieldNames("reject", v.Name, "field", v.Fields)
		checkFieldNames("reject", v.Name, "config", v.Config)
		if !ErrorNames[v.Error] {
			add("reject %q: error %q is not in the vocabulary (%s)",
				v.Name, v.Error, strings.Join(errorVocabulary(), ", "))
		}
		hasInput, hasFields := v.Input != nil, len(v.Fields) > 0
		switch {
		case hasInput && hasFields:
			add("reject %q: set input or fields, not both — bytes refused at decode, or values refused at construction", v.Name)
		case !hasInput && !hasFields:
			add("reject %q: set exactly one of input or fields", v.Name)
		}
		if hasInput && !hexRE.MatchString(*v.Input) {
			add("reject %q: input %q is not lowercase hex octet pairs", v.Name, *v.Input)
		}
		if v.BufferLen != nil && !hasCap(v.Requires, "encode_into") {
			add("reject %q: buffer_len only means something with requires: [\"encode_into\"]", v.Name)
		}
		if v.Error == "buffer_too_small" && !hasCap(v.Requires, "encode_into") {
			add("reject %q: buffer_too_small is only reachable with requires: [\"encode_into\"]", v.Name)
		}
	}

	for _, v := range f.Sequence {
		claim("sequence", v.Name)
		checkNote("sequence", v.Name, v.Note)
		checkFieldNames("sequence", v.Name, "config", v.Config)
		if len(v.Steps) == 0 {
			add("sequence %q: steps is required", v.Name)
		}
		for i, s := range v.Steps {
			where := fmt.Sprintf("sequence %q step %d", v.Name, i)
			if !callRE.MatchString(s.Call) {
				add("%s: call %q must be snake_case", where, s.Call)
			}
			if s.Want != "" && !hexRE.MatchString(s.Want) {
				add("%s: want %q is not lowercase hex octet pairs", where, s.Want)
			}
			if s.Error != "" && !ErrorNames[s.Error] {
				add("%s: error %q is not in the vocabulary", where, s.Error)
			}
			if s.Want != "" && s.Error != "" {
				add("%s: a step either produces bytes or fails, not both", where)
			}
			checkFieldNames(where, "", "field", s.Fields)
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func hasCap(reqs []string, want string) bool {
	for _, r := range reqs {
		if r == want {
			return true
		}
	}
	return false
}
