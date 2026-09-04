package odm

import (
	"strings"
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// The Orbit Comprehensive Message, CCSDS 502.0-B-3 section 6.
//
// The OCM is the largest message in the family by a wide margin: eight
// sections and something over two hundred keywords, most of them optional and
// most describing how an orbit was determined rather than what it is. It
// carries trajectories, physical properties, covariances, manoeuvres,
// perturbation models and orbit determination statistics in one file.
//
// Its sections are held as ordered keyword lists with typed accessors for the
// keywords that change how the data must be read, the same way pkg/tdm and
// pkg/cdm hold theirs. Two hundred named struct fields, almost all pointers,
// would help nobody, and a caller meeting an unfamiliar keyword could not see
// it. Get reaches anything.
//
// # Time tags may be relative
//
// This is what most distinguishes the OCM from the rest of the family. Clause
// 6.2.2.3 lets a time tag be an absolute time or a signed double in SI seconds
// from EPOCH_TZERO — 20157.26 rather than 2018-11-13T11:13:20.5Z. Clause
// 6.2.2.5 requires a block to pick one and keep it, so the two never mix
// inside a block, but two blocks in one message may disagree.
//
// A reader that assumes absolute times will parse nothing; one that assumes
// relative times will read an epoch as a malformed number. DataRow.TimeTag
// resolves either against the message's EPOCH_TZERO.
//
// # A message with no data at all is valid
//
// Clause 6.2.1.1's note calls it a "degenerate case" and says it was an
// intentional choice: the metadata alone is useful for carrying contact
// details, linking messages together and conveying timing sources. So an OCM
// with a header and a metadata section and nothing else is well formed.

// OCM section delimiters (table 6-1). Every data section is wrapped, and the
// order they may appear in is fixed.
const (
	ocmMeta = "META"
	ocmTraj = "TRAJ"
	ocmPhys = "PHYS"
	ocmCov  = "COV"
	ocmMan  = "MAN"
	ocmPert = "PERT"
	ocmOD   = "OD"
	ocmUser = "USER"
)

// Field is one keyword assignment, kept in the order it arrived.
type Field struct {
	Keyword string
	Value   string
}

// DataRow is one positional data line from a trajectory, covariance or
// manoeuvre block.
//
// The fields are kept as text. The first is a time tag that may be absolute or
// relative, and how many follow it depends on the block: on TRAJ_TYPE, on
// COV_TYPE with COV_ORDERING, or on MAN_COMPOSITION.
//
// Only the last of those three can be checked here. Clause 6.2.8.15 draws the
// manoeuvre fields from tables 6-8 and 6-9, which are printed in the Blue
// Book, so a manoeuvre row's width and column names are known. TRAJ_TYPE and
// COV_TYPE are drawn from the SANA registry instead (clauses 6.2.5.11 and
// 6.2.7.12.1), so a trajectory row's width is carried rather than checked.
type DataRow struct {
	Fields []string
}

// TimeTag resolves the row's time tag against the message's EPOCH_TZERO.
//
// An absolute tag is returned as it stands. A relative one is seconds from
// tzero, which may be negative: clause 6.2.2.3 calls it a signed value, and a
// trajectory may begin before the epoch it is measured from.
func (r DataRow) TimeTag(tzero time.Time) (time.Time, error) {
	if len(r.Fields) == 0 {
		return time.Time{}, ErrMalformedDataRow
	}
	raw := r.Fields[0]

	// A CCSDS time string always has a 'T' between its date and its time, and
	// a relative offset is a bare number, so the two cannot be confused.
	if strings.ContainsRune(raw, 'T') {
		return ndm.ParseEpoch(raw)
	}

	seconds, err := ndm.ParseFloat(raw)
	if err != nil {
		return time.Time{}, err
	}
	if tzero.IsZero() {
		// A relative tag with nothing to be relative to. Clause 6.2.4 makes
		// EPOCH_TZERO mandatory in the metadata for exactly this reason.
		return time.Time{}, ErrNoEpochTZero
	}
	return tzero.Add(time.Duration(seconds * float64(time.Second))), nil
}

// IsRelative reports whether the row's time tag is an offset rather than an
// absolute time.
func (r DataRow) IsRelative() bool {
	return len(r.Fields) > 0 && !strings.ContainsRune(r.Fields[0], 'T')
}

// Values returns the row's measurements, everything after the time tag.
func (r DataRow) Values() ([]float64, error) {
	if len(r.Fields) == 0 {
		return nil, ErrMalformedDataRow
	}
	out := make([]float64, 0, len(r.Fields)-1)
	for _, field := range r.Fields[1:] {
		v, err := ndm.ParseFloat(field)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// OCMSection is one of the OCM's sections: its comments, its keywords in wire
// order, and the positional data rows that follow them.
type OCMSection struct {
	Comments []string
	Fields   []Field
	Rows     []DataRow
}

// Get returns the value of a keyword and whether the section carried it.
func (s OCMSection) Get(keyword string) (string, bool) {
	for _, f := range s.Fields {
		if f.Keyword == keyword {
			return f.Value, true
		}
	}
	return "", false
}

// GetOr returns a keyword's value, or the default the tables give it when the
// section left it out. Clause 6.2.1.3 says a default that matches the intent
// need not be written, and the recipient adopts it.
func (s OCMSection) GetOr(keyword, fallback string) string {
	if v, ok := s.Get(keyword); ok {
		return v
	}
	return fallback
}

// OCM is an Orbit Comprehensive Message (CCSDS 502.0-B-3 section 6).
type OCM struct {
	Header Header
	// Metadata is the single metadata section clause 6.2.4.3 allows.
	Metadata OCMSection

	// Trajectories, Covariances and Maneuvers may each appear any number of
	// times; the rest at most once.
	Trajectories       []OCMSection
	Physical           *OCMSection
	Covariances        []OCMSection
	Maneuvers          []OCMSection
	Perturbations      *OCMSection
	OrbitDetermination *OCMSection

	// UserDefined holds the USER_DEFINED_x parameters of table 6-12, and
	// UserComments the comments in that section.
	UserDefined  []UserDefined
	UserComments []string
}

// EpochTZero is the time a relative time tag is measured from, and whether the
// metadata gave one.
func (m *OCM) EpochTZero() (time.Time, bool) {
	raw, ok := m.Metadata.Get("EPOCH_TZERO")
	if !ok {
		return time.Time{}, false
	}
	t, err := ndm.ParseEpoch(raw)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// TimeSystem is the scale the message's times are on. Table 6-3 defaults it
// to UTC, so a message that leaves it out is on UTC rather than on nothing.
func (m *OCM) TimeSystem() string {
	return m.Metadata.GetOr("TIME_SYSTEM", ocmDefaultTimeSystem)
}

// ObjectName and ObjectDesignator identify the object.
func (m *OCM) ObjectName() string {
	return ndm.ParseText(m.Metadata.GetOr("OBJECT_NAME", ""))
}

// ObjectDesignator is the catalogue designator.
func (m *OCM) ObjectDesignator() string {
	return m.Metadata.GetOr("OBJECT_DESIGNATOR", "")
}

// TrajType is the orbit element set a trajectory block's rows carry, which is
// what says how wide a row is and what each column means. Clause 6.2.5.11
// draws the values from the SANA registry, so they are carried rather than
// checked. Its default is CARTPV: time, position, velocity.
func (s OCMSection) TrajType() string { return s.GetOr("TRAJ_TYPE", ocmDefaultTrajType) }

// TrajUnits is the units of a trajectory block's rows. Clause 7.7.1.1 makes
// them documentation, so a block that leaves them out is not missing anything
// a reader needs; the units for a TRAJ_TYPE are fixed by its SANA entry.
func (s OCMSection) TrajUnits() string { return s.GetOr("TRAJ_UNITS", "") }

// CenterName is the origin of a trajectory block's reference frame, EARTH
// unless the block says otherwise.
func (s OCMSection) CenterName() string { return s.GetOr("CENTER_NAME", ocmDefaultCenterName) }

// CovType is the covariance element set a covariance block's rows carry.
func (s OCMSection) CovType() string { return s.GetOr("COV_TYPE", ocmDefaultCovType) }

// CovOrdering is how the numbers in a covariance row are laid out. Without it
// they cannot be put back into a matrix, so clause 6.2.7.12.3 defines five
// orderings and table 6-6 defaults to LTM.
func (s OCMSection) CovOrdering() string { return s.GetOr("COV_ORDERING", ocmDefaultCovOrdering) }

// ManComposition lists, in order, what each field of a manoeuvre row holds,
// starting with its time tag. Clause 6.2.8.14 makes it the row layout.
func (s OCMSection) ManComposition() []string {
	raw, ok := s.Get("MAN_COMPOSITION")
	if !ok {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// DutyCycle is how a manoeuvre block's thrust is switched on and off,
// CONTINUOUS unless the block says otherwise.
func (s OCMSection) DutyCycle() string { return s.GetOr("DC_TYPE", ocmDefaultDutyCycle) }

// RefFrame returns the reference frame a block names, whichever of the three
// frame keywords it uses, and the default for that keyword when it is absent.
func (s OCMSection) RefFrame() string {
	for _, f := range []struct{ keyword, fallback string }{
		{"TRAJ_REF_FRAME", ocmDefaultTrajFrame},
		{"COV_REF_FRAME", ocmDefaultCovFrame},
		{"MAN_REF_FRAME", ocmDefaultManFrame},
	} {
		if v, ok := s.Get(f.keyword); ok {
			return v
		}
	}
	return ""
}

// The covariance orderings of clause 6.2.7.12.3.
const (
	CovLTM    = "LTM"    // lower triangle, row by row
	CovUTM    = "UTM"    // upper triangle, row by row
	CovFull   = "FULL"   // the whole symmetric matrix
	CovLTMWCC = "LTMWCC" // lower triangle, correlations in the upper one
	CovUTMWCC = "UTMWCC" // upper triangle, correlations in the lower one
)

// CovMatrix rebuilds one covariance row into a square matrix.
//
// The row's numbers are a flattened matrix whose shape COV_ORDERING gives, and
// the four figures of clause 6.2.7.12.3 are the only statement of what order
// they arrive in. Two of the orderings, LTMWCC and UTMWCC, carry correlations
// rather than covariances in their off-diagonal half — the correlation of two
// variables is their covariance divided by the product of their standard
// deviations — so the matrix they return is not symmetric, and reading it as
// though it were would silently scale half the entries by the wrong amount.
// Those two are returned as they were written.
//
// The size of the matrix comes from how many numbers the row holds: a triangle
// of N(N+1)/2 values, or a full N*N.
func (s OCMSection) CovMatrix(row DataRow) ([][]float64, error) {
	values, err := row.Values()
	if err != nil {
		return nil, err
	}

	ordering := s.CovOrdering()
	triangular := ordering == CovLTM || ordering == CovUTM
	switch ordering {
	case CovLTM, CovUTM, CovFull, CovLTMWCC, CovUTMWCC:
	default:
		return nil, ErrUnknownCovOrdering
	}

	n, ok := covDimension(len(values), triangular)
	if !ok {
		return nil, ErrCovRowWidth
	}

	m := make([][]float64, n)
	for i := range m {
		m[i] = make([]float64, n)
	}

	at := 0
	take := func(row, col int) {
		m[row][col] = values[at]
		at++
	}

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			switch ordering {
			case CovLTM:
				// Lower triangle only, and the matrix is symmetric, so the
				// upper one is filled from it.
				if j <= i {
					m[i][j] = values[at]
					m[j][i] = values[at]
					at++
				}
			case CovUTM:
				if j >= i {
					m[i][j] = values[at]
					m[j][i] = values[at]
					at++
				}
			default:
				// FULL, LTMWCC and UTMWCC are all written row by row across
				// the whole matrix.
				take(i, j)
			}
		}
	}
	return m, nil
}

// covDimension works out the side of a square matrix from how many numbers
// were written for it, or reports that no whole matrix fits.
func covDimension(count int, triangular bool) (int, bool) {
	for n := 1; n <= count; n++ {
		size := n * n
		if triangular {
			size = n * (n + 1) / 2
		}
		if size == count {
			return n, true
		}
		if size > count {
			break
		}
	}
	return 0, false
}

// ManFields names each column of a manoeuvre row, or reports why
// MAN_COMPOSITION cannot be read.
//
// Clause 6.2.8.15 says the fields come from table 6-8 or table 6-9 and that
// the two must not be mixed within one block; clause 6.2.8.16 fixes their
// order; clause 6.2.8.18 requires the first to be exactly one of
// TIME_ABSOLUTE and TIME_RELATIVE.
func (s OCMSection) ManFields() ([]string, error) {
	fields := s.ManComposition()
	if len(fields) < 2 {
		return nil, ErrMalformedManComposition
	}
	if fields[0] != "TIME_ABSOLUTE" && fields[0] != "TIME_RELATIVE" {
		return nil, ErrMalformedManComposition
	}
	// The time tag is in both tables, so it says nothing about which one this
	// block draws from; the second field is the first that does.
	table := ocmManPropulsiveFields
	if indexOf(ocmManDeploymentFields, fields[1]) >= 0 {
		table = ocmManDeploymentFields
	}

	previous := -1
	for _, field := range fields {
		at := indexOf(table, field)
		if at < 0 {
			// Either not a manoeuvre field at all, or one from the other
			// table. Clause 6.2.8.15 refuses both.
			return nil, ErrMalformedManComposition
		}
		if at <= previous {
			return nil, ErrMalformedManComposition
		}
		previous = at
	}
	return fields, nil
}

// indexOf returns where a value sits in a list, or -1.
func indexOf(list []string, want string) int {
	for i, v := range list {
		if v == want {
			return i
		}
	}
	return -1
}

// Validate checks the message against the rules section 6 states.
//
// What it does not check is worth saying plainly. Most OCM keywords are
// optional, most of their values are drawn from the SANA registry rather than
// from the Blue Book, and clause 6.2.1.1 makes a message with no data blocks
// at all valid on purpose. So this checks the header, the keywords a section
// must carry, the keyword and section ordering the standard fixes, and the
// rules about time tags that a reader cannot recover from if they are broken.
func (m *OCM) Validate() error {
	if m.Header.Version == "" || m.Header.Originator == "" {
		return ndm.ErrMissingHeaderField
	}
	if err := validateSection(ocmMeta, m.Metadata); err != nil {
		return err
	}

	// Table 6-3 makes the two spacecraft clock keywords conditional on the
	// time system. Without them an SCLK time cannot be converted at all.
	if m.TimeSystem() == "SCLK" {
		for _, keyword := range []string{"SCLK_OFFSET_AT_EPOCH", "SCLK_SEC_PER_SI_SEC"} {
			if _, ok := m.Metadata.Get(keyword); !ok {
				return ErrMissingSCLKFields
			}
		}
	}

	// Clause 6.2.10.5: an orbit determination section says how an orbit was
	// determined, which is not meaningful without the force models it used.
	if m.OrbitDetermination != nil && m.Perturbations == nil {
		return ErrMissingPerturbations
	}

	for name, sections := range map[string][]OCMSection{
		ocmTraj: m.Trajectories,
		ocmCov:  m.Covariances,
		ocmMan:  m.Maneuvers,
	} {
		for i := range sections {
			if err := validateSection(name, sections[i]); err != nil {
				return err
			}
			if err := checkRowTimes(sections[i].Rows); err != nil {
				return err
			}
		}
	}

	// Clauses 6.2.5.6 and 6.2.7.6 order the trajectory and covariance blocks
	// in time. The manoeuvre section has no such rule: clause 6.2.8.5 lets
	// manoeuvre blocks overlap or repeat in time.
	tzero, _ := m.EpochTZero()
	for _, group := range [][]OCMSection{m.Trajectories, m.Covariances} {
		for i := range group {
			if err := checkRowOrder(group[i].Rows, tzero); err != nil {
				return err
			}
		}
	}

	for _, section := range m.Maneuvers {
		if err := validateManeuverRows(section); err != nil {
			return err
		}
	}

	for _, single := range []struct {
		name    string
		section *OCMSection
	}{
		{ocmPhys, m.Physical},
		{ocmPert, m.Perturbations},
		{ocmOD, m.OrbitDetermination},
	} {
		if single.section == nil {
			continue
		}
		if err := validateSection(single.name, *single.section); err != nil {
			return err
		}
	}
	return nil
}

// validateSection checks one section against its table: that every keyword
// belongs to it, that none repeats, that they arrive in the table's order, and
// that the ones with no default are present.
func validateSection(name string, section OCMSection) error {
	positions, ok := ocmSectionIndex[name]
	if !ok {
		return nil
	}

	previous := -1
	seen := make(map[string]bool, len(section.Fields))

	for _, f := range section.Fields {
		at, known := positions[f.Keyword]
		if !known {
			return ErrUnknownKeyword
		}
		if seen[f.Keyword] {
			return ErrDuplicateKeyword
		}
		seen[f.Keyword] = true

		// Clause 6.2.2.1 fixes the order of occurrence.
		if at < previous {
			return ErrKeywordsOutOfOrder
		}
		previous = at
	}

	for _, keyword := range ocmRequired[name] {
		if !seen[keyword] {
			return ErrMissingKeyword
		}
	}
	return nil
}

// checkRowTimes enforces the two rules about a block's time tags.
func checkRowTimes(rows []DataRow) error {
	seen := make(map[string]bool, len(rows))

	for i, row := range rows {
		if len(row.Fields) == 0 {
			return ErrMalformedDataRow
		}
		if i > 0 && row.IsRelative() != rows[0].IsRelative() {
			return ErrMixedTimeTags
		}
		if seen[row.Fields[0]] {
			return ErrDuplicateTimeTag
		}
		seen[row.Fields[0]] = true
	}
	return nil
}

// checkRowOrder enforces that a block's time tags run forward.
//
// The comparison is on resolved times rather than on the text, because two
// relative tags may be written to different precisions and a negative one
// sorts before a positive one whatever the strings say.
func checkRowOrder(rows []DataRow, tzero time.Time) error {
	var previous time.Time

	for i, row := range rows {
		at, err := row.TimeTag(tzero)
		if err != nil {
			return err
		}
		if i > 0 && !at.After(previous) {
			return ErrTimeTagsOutOfOrder
		}
		previous = at
	}
	return nil
}

// validateManeuverRows checks a manoeuvre block's rows against the layout its
// MAN_COMPOSITION names.
//
// This is the one row width the package can check. Clause 6.2.8.15 draws the
// field names from tables 6-8 and 6-9, both printed in the Blue Book, so the
// number of columns a row must have is known here in a way a trajectory row's
// never is.
func validateManeuverRows(section OCMSection) error {
	fields, err := section.ManFields()
	if err != nil {
		return err
	}
	for _, row := range section.Rows {
		if len(row.Fields) != len(fields) {
			return ErrManRowWidth
		}
	}

	// Clause 6.2.8.18 puts the time tag first, and its name has to agree with
	// what the rows actually carry.
	wantRelative := fields[0] == "TIME_RELATIVE"
	for _, row := range section.Rows {
		if row.IsRelative() != wantRelative {
			return ErrMixedTimeTags
		}
	}
	return nil
}
