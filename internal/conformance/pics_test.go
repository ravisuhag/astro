// Package conformance pins the "Numbered PICS items" figure published on
// docs/content/docs/reference/verification.md against the conformance
// pages it is derived from, so the two cannot drift apart silently.
package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// root returns the repository root, located relative to this source file
// so the test works regardless of the caller's working directory.
func root() string {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		panic("conformance: cannot locate the repository root")
	}
	return filepath.Join(filepath.Dir(self), "..", "..")
}

var (
	itemHeaderRE = regexp.MustCompile(`^\|\s*Item\s*\|`)
	separatorRE  = regexp.MustCompile(`^\|[\s:-]+\|`)

	// idRE matches an explicit PICS identifier in an Item cell: a run of
	// letters, an optional hyphen, a run of digits, and an optional
	// trailing letter for a sub-item (COP-1, SLE-40, TC-20a, or the
	// standard's own terser R1 / P1 form with no hyphen at all).
	idRE = regexp.MustCompile(`^([A-Za-z]+)(-?)(\d+)([A-Za-z]?)$`)

	// bareRE matches a bare number (optionally sub-lettered) that
	// continues a comma- or slash-separated list started by an idRE
	// match earlier in the same cell, e.g. the "48" in "SLE-40, 48, 55,
	// 65" or the "27" in "TMSC-26/27".
	bareRE = regexp.MustCompile(`^(\d+)([A-Za-z]?)$`)

	// rangeRE matches an inclusive range shorthand, e.g.
	// "SLE-66 to SLE-71".
	rangeRE = regexp.MustCompile(`^([A-Za-z]+)(-?)(\d+)\s+to\s+(?:([A-Za-z]+)(-?))?(\d+)$`)
)

// anchor carries the prefix of the last explicit identifier seen in the
// current table, so a bare continuation number can be resolved. It resets
// at the start of every Item-headed table.
type anchor struct {
	prefix string
	sep    string
	set    bool
}

// parseCell extracts every PICS identifier named in one Item-column cell.
// A cell that names no identifier at all — blank, an em dash, a bare
// hyphen, or prose describing an area rather than a single requirement —
// contributes nothing. That is the honest answer for a page such as
// tcf.md, whose "Not Implemented" table has no numbering scheme to force.
func parseCell(cell string, a anchor) ([]string, anchor) {
	cell = strings.TrimSpace(cell)
	if cell == "" || cell == "—" || cell == "-" {
		return nil, a
	}
	if m := rangeRE.FindStringSubmatch(cell); m != nil {
		prefix, sep := m[1], m[2]
		if sep == "" {
			sep = "-"
		}
		start, errStart := strconv.Atoi(m[3])
		end, errEnd := strconv.Atoi(m[6])
		if errStart != nil || errEnd != nil || end < start {
			return nil, a
		}
		ids := make([]string, 0, end-start+1)
		for n := start; n <= end; n++ {
			ids = append(ids, fmt.Sprintf("%s%s%d", prefix, sep, n))
		}
		return ids, anchor{prefix: prefix, sep: sep, set: true}
	}

	var ids []string
	for _, part := range strings.FieldsFunc(cell, func(r rune) bool { return r == ',' || r == '/' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if m := idRE.FindStringSubmatch(part); m != nil {
			prefix, sep, num, suf := m[1], m[2], m[3], m[4]
			ids = append(ids, prefix+sep+num+suf)
			a = anchor{prefix: prefix, sep: sep, set: true}
			continue
		}
		if m := bareRE.FindStringSubmatch(part); m != nil && a.set {
			ids = append(ids, a.prefix+a.sep+m[1]+m[2])
			continue
		}
		// Prose, not an identifier: contributes nothing.
	}
	return ids, a
}

// pageIdentifiers returns the set of distinct PICS identifiers named
// anywhere in an Item-headed conformance table on one page. An identifier
// is counted once even if it is named in several tables on the page (a
// primary table, a "Non-Conformances" list, a "Fully Supported Items"
// summary, and so on).
func pageIdentifiers(t *testing.T, path string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	lines := strings.Split(string(raw), "\n")
	ids := map[string]bool{}
	i := 0
	for i < len(lines) {
		if !itemHeaderRE.MatchString(lines[i]) {
			i++
			continue
		}
		j := i + 1
		if j < len(lines) && separatorRE.MatchString(lines[j]) {
			j++
		}
		var a anchor
		for j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "|") {
			cells := strings.Split(lines[j], "|")
			first := ""
			if len(cells) > 1 {
				first = cells[1]
			}
			var found []string
			found, a = parseCell(first, a)
			for _, id := range found {
				ids[id] = true
			}
			j++
		}
		i = j
	}
	return ids
}

// TestNumberedPICSItemsMatchesTheCorpus keeps the "Numbered PICS items" row
// on verification.md honest. A numbered PICS item is a distinct identifier
// named in the Item column of a conformance page, counted once per page no
// matter how many tables on that page name it; see verification.md for the
// full statement of the rule.
func TestNumberedPICSItemsMatchesTheCorpus(t *testing.T) {
	confDir := filepath.Join(root(), "docs", "content", "conformance")
	entries, err := os.ReadDir(confDir)
	if err != nil {
		t.Fatalf("conformance dir: %v", err)
	}

	perPage := map[string]int{}
	total := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		ids := pageIdentifiers(t, filepath.Join(confDir, e.Name()))
		if len(ids) == 0 {
			continue
		}
		page := strings.TrimSuffix(e.Name(), ".md")
		perPage[page] = len(ids)
		total += len(ids)
	}

	verificationPath := filepath.Join(root(), "docs", "content", "docs", "reference", "verification.md")
	raw, err := os.ReadFile(verificationPath)
	if err != nil {
		t.Fatalf("%s: %v", verificationPath, err)
	}
	row := regexp.MustCompile(`\|\s*Numbered PICS items\s*\|\s*(\d+)\s*\|`)
	m := row.FindSubmatch(raw)
	if m == nil {
		t.Fatal(`verification.md has no "Numbered PICS items" row`)
	}
	published, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatalf("verification.md's Numbered PICS items value does not parse: %v", err)
	}

	if published != total {
		pages := make([]string, 0, len(perPage))
		for p := range perPage {
			pages = append(pages, p)
		}
		sort.Strings(pages)
		var b strings.Builder
		fmt.Fprintf(&b, "verification.md says %d numbered PICS items, the conformance pages have %d. Per-page counts:\n", published, total)
		for _, p := range pages {
			fmt.Fprintf(&b, "  %-12s %d\n", p, perPage[p])
		}
		t.Error(b.String())
	}
}
