package adm

import (
	"strings"
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// The Attitude Comprehensive Message, CCSDS 504.0-B-2 section 5.
//
// The ACM is to the APM and the AEM what the ODM's OCM is to the OPM and the
// OEM: one file carrying everything about an object's attitude rather than one
// aspect of it. Attitude state histories, physical properties, covariance
// histories, manoeuvres and how the attitude was determined, all in six
// delimited sections after a single metadata section.
//
// Its sections are held as ordered keyword lists with typed accessors for the
// keywords that change how the data must be read, the same way pkg/odm holds
// the OCM's. A caller meeting an unfamiliar keyword can still see it, and Get
// reaches anything.
//
// # A row's width is checkable here, unlike the OCM's
//
// Every element count the ACM needs is printed in the Blue Book. Annex B4
// gives the number of components for each ATT_TYPE and RATE_TYPE, annex B6
// does the same for each COV_TYPE, and table 5-4 makes NUMBER_STATES mandatory
// besides. So an attitude row is checked twice over: against the count the
// types imply, and against the count the producer declared.
//
// The OCM cannot do this. Clause 6.2.5.11 of CCSDS 502.0-B-3 draws its
// TRAJ_TYPE values from the SANA registry, so nothing in that document says
// how many numbers a row should hold.
//
// # Time tags may be relative
//
// Clause 5.3.4.3 lets a time tag be an absolute time or a signed count of SI
// seconds from EPOCH_TZERO, exactly as the OCM does, and clause 5.3.4.5
// requires a block to pick one and keep it. Every example in annex G uses
// relative tags.

// ACM section delimiters (table 5-1). Every data section is wrapped, and the
// order they may appear in is fixed by clause 5.3.1.2.
const (
	acmMeta = "META"
	acmAtt  = "ATT"
	acmPhys = "PHYS"
	acmCov  = "COV"
	acmMan  = "MAN"
	acmAD   = "AD"
	acmUser = "USER"

	// acmSensor is nested inside the attitude determination section rather
	// than being a section of its own (clause 5.3.9.6).
	acmSensor = "SENSOR"
)

// Field is one keyword assignment, kept in the order it arrived.
type Field struct {
	Keyword string
	Value   string
}

// DataRow is one positional data line from an attitude or covariance block.
//
// The fields are kept as text. The first is a time tag that may be absolute or
// relative; how many follow it comes from the block's ATT_TYPE and RATE_TYPE,
// or from its COV_TYPE.
type DataRow struct {
	Fields []string
}

// TimeTag resolves the row's time tag against the message's EPOCH_TZERO.
//
// An absolute tag is returned as it stands. A relative one is seconds from
// tzero and may be negative, since clause 5.3.4.3 calls it a signed value.
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

// ACMSensor is one sensor sub-block of the attitude determination section
// (clause 5.3.9.6).
type ACMSensor struct {
	Comments []string
	Fields   []Field
}

// Get returns the value of a keyword and whether the sensor block carried it.
func (s ACMSensor) Get(keyword string) (string, bool) { return findField(s.Fields, keyword) }

// ACMSection is one of the ACM's sections: its comments, its keywords in wire
// order, and the positional data rows that follow them.
type ACMSection struct {
	Comments []string
	Fields   []Field
	Rows     []DataRow
	// Sensors is populated only for the attitude determination section.
	Sensors []ACMSensor
}

// Get returns the value of a keyword and whether the section carried it.
func (s ACMSection) Get(keyword string) (string, bool) { return findField(s.Fields, keyword) }

// GetOr returns a keyword's value, or a default when the section left it out.
func (s ACMSection) GetOr(keyword, fallback string) string {
	if v, ok := s.Get(keyword); ok {
		return v
	}
	return fallback
}

func findField(fields []Field, keyword string) (string, bool) {
	for _, f := range fields {
		if f.Keyword == keyword {
			return f.Value, true
		}
	}
	return "", false
}

// AttitudeType is the attitude representation an attitude block's rows carry:
// QUATERNION, EULER_ANGLES or DCM (annex B4).
func (s ACMSection) AttitudeType() string { return s.GetOr("ATT_TYPE", "") }

// RateType is the rate representation that follows the attitude on each row,
// or NONE when the block carries no rate data.
func (s ACMSection) RateType() string { return s.GetOr("RATE_TYPE", "NONE") }

// CovarianceType is the covariance composition a covariance block's rows
// carry (annex B6).
func (s ACMSection) CovarianceType() string { return s.GetOr("COV_TYPE", "") }

// Frames returns the two frames the block's rotation goes between: from
// REF_FRAME_A to REF_FRAME_B.
func (s ACMSection) Frames() (from, to string) {
	return s.GetOr("REF_FRAME_A", ""), s.GetOr("REF_FRAME_B", "")
}

// StateCount returns how many numbers follow the time tag on each row of an
// attitude block, and whether the block's types are ones annex B4 defines.
//
// This is the sum of the attitude elements and the rate elements. Table 5-4
// also requires NUMBER_STATES to be written, so a well-formed block says the
// same thing twice; Validate checks that the two agree.
func (s ACMSection) StateCount() (int, bool) {
	attitude, ok := acmAttitudeElements[s.AttitudeType()]
	if !ok {
		return 0, false
	}
	rate, ok := acmRateElements[s.RateType()]
	if !ok {
		return 0, false
	}
	return attitude + rate, true
}

// CovarianceCount returns how many numbers follow the time tag on each row of
// a covariance block.
//
// Clause 5.3.7.6 puts only the main diagonal on the line, so this is the
// dimension of the matrix rather than the size of a triangle. An ACM
// covariance row is therefore much narrower than an OCM one, and carries less:
// clause 5.3.7.7 sends anyone who needs the off-diagonal terms to a
// user-defined block.
func (s ACMSection) CovarianceCount() (int, bool) {
	n, ok := acmCovarianceElements[s.CovarianceType()]
	return n, ok
}

// ACM is an Attitude Comprehensive Message (CCSDS 504.0-B-2 section 5).
type ACM struct {
	Header Header
	// Metadata is the single metadata section clause 5.3.3.4 allows.
	Metadata ACMSection

	// Attitudes, Covariances and Maneuvers may each appear any number of
	// times; the rest at most once.
	Attitudes             []ACMSection
	Physical              *ACMSection
	Covariances           []ACMSection
	Maneuvers             []ACMSection
	AttitudeDetermination *ACMSection

	// UserDefined holds the USER_DEFINED_x parameters of table 5-9, and
	// UserComments the comments in that section.
	UserDefined  []UserDefined
	UserComments []string
}

// UserDefined is one USER_DEFINED_x parameter (table 5-9). The name is what
// follows the prefix in the key-value form, and an attribute in XML.
type UserDefined struct {
	Name  string
	Value string
}

// EpochTZero is the time a relative time tag is measured from, and whether the
// metadata gave one.
func (m *ACM) EpochTZero() (time.Time, bool) {
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

// TimeSystem is the scale the message's absolute times are on. Table 5-3
// makes it mandatory and gives it no default.
func (m *ACM) TimeSystem() string { return m.Metadata.GetOr("TIME_SYSTEM", "") }

// ObjectName is the name of the object the message describes.
func (m *ACM) ObjectName() string { return ndm.ParseText(m.Metadata.GetOr("OBJECT_NAME", "")) }

// Validate checks the message against the rules section 5 states.
func (m *ACM) Validate() error {
	if m.Header.Version == "" || m.Header.Originator == "" {
		return ndm.ErrMissingHeaderField
	}
	if err := validateACMSection(acmMeta, m.Metadata); err != nil {
		return err
	}

	tzero, _ := m.EpochTZero()

	for i := range m.Attitudes {
		if err := validateAttitudeBlock(m.Attitudes[i], tzero); err != nil {
			return err
		}
	}
	for i := range m.Covariances {
		if err := validateCovarianceBlock(m.Covariances[i], tzero); err != nil {
			return err
		}
	}
	for i := range m.Maneuvers {
		if err := validateManeuverBlock(m.Maneuvers[i]); err != nil {
			return err
		}
	}

	if m.Physical != nil {
		if err := validateACMSection(acmPhys, *m.Physical); err != nil {
			return err
		}
		// Table 5-5: CP_REF_FRAME shall be present if CP is present. A centre
		// of pressure is three numbers in some frame, and without the frame
		// they are three numbers.
		if _, ok := m.Physical.Get("CP"); ok {
			if _, ok := m.Physical.Get("CP_REF_FRAME"); !ok {
				return ErrMissingFrame
			}
		}
		if err := checkVectorWidths(m.Physical.Fields); err != nil {
			return err
		}
	}

	if ad := m.AttitudeDetermination; ad != nil {
		if err := validateDeterminationBlock(*ad); err != nil {
			return err
		}
	}
	return nil
}

// validateAttitudeBlock checks one attitude state time history.
func validateAttitudeBlock(section ACMSection, tzero time.Time) error {
	if err := validateACMSection(acmAtt, section); err != nil {
		return err
	}

	states, ok := section.StateCount()
	if !ok {
		// An ATT_TYPE or RATE_TYPE annex B4 does not define. Nothing says how
		// wide a row is, so the block cannot be read.
		return ErrUnknownAttitudeType
	}

	// Table 5-4: EULER_ROT_SEQ is applicable only if the type is Euler angles,
	// and three angles with no rotation sequence do not define a rotation.
	if section.AttitudeType() == "EULER_ANGLES" {
		if _, ok := section.Get("EULER_ROT_SEQ"); !ok {
			return ErrEulerRotSeqMissing
		}
	}

	// NUMBER_STATES is mandatory, and it must agree with what the types imply.
	// A message where the two disagree is one where the producer and the
	// consumer would read different columns.
	declared, err := ndm.ParseInt(section.GetOr("NUMBER_STATES", ""))
	if err != nil {
		return err
	}
	if int(declared) != states {
		return ErrStateCountMismatch
	}

	for _, row := range section.Rows {
		if len(row.Fields) != states+1 {
			return ErrAttitudeLineFields
		}
	}

	// The attitude section is not required to run forward in time. Clause
	// 5.3.7.5 says a covariance time history "shall be time-ordered to be
	// monotonically increasing" and section 5.3.5 says no such thing about
	// attitude states — it only calls them an ordered sequence (5.3.5.7).
	// Refusing an out-of-order attitude block would refuse a file the
	// standard permits.
	return checkACMRowTimes(section.Rows, tzero, false)
}

// validateCovarianceBlock checks one covariance time history.
func validateCovarianceBlock(section ACMSection, tzero time.Time) error {
	if err := validateACMSection(acmCov, section); err != nil {
		return err
	}
	elements, ok := section.CovarianceCount()
	if !ok {
		return ErrUnknownCovarianceType
	}
	for _, row := range section.Rows {
		if len(row.Fields) != elements+1 {
			return ErrCovarianceLineFields
		}
	}
	// Clause 5.3.7.5: monotonically increasing. The attitude section has no
	// such rule, which is why only this one and the attitude blocks differ.
	return checkACMRowTimes(section.Rows, tzero, true)
}

// validateManeuverBlock checks one manoeuvre specification.
func validateManeuverBlock(section ACMSection) error {
	if err := validateACMSection(acmMan, section); err != nil {
		return err
	}

	// Table 5-7: MAN_END_TIME or MAN_DURATION, not both. They say the same
	// thing two ways, and a message giving both can contradict itself.
	_, hasEnd := section.Get("MAN_END_TIME")
	_, hasDuration := section.Get("MAN_DURATION")
	if hasEnd && hasDuration {
		return ErrBothManeuverEnds
	}

	// Table 5-7: TARGET_MOM_FRAME shall be present if TARGET_MOMENTUM is.
	if _, ok := section.Get("TARGET_MOMENTUM"); ok {
		if _, ok := section.Get("TARGET_MOM_FRAME"); !ok {
			return ErrMissingFrame
		}
	}
	return checkVectorWidths(section.Fields)
}

// validateDeterminationBlock checks the attitude determination section and its
// sensor sub-blocks.
func validateDeterminationBlock(section ACMSection) error {
	if err := validateACMSection(acmAD, section); err != nil {
		return err
	}

	// Annex B5 enumerates the estimator types outright, so an AD_METHOD
	// outside the list is one nothing in the standard describes.
	if method, ok := section.Get("AD_METHOD"); ok && !acmEstimators[method] {
		return ErrUnknownEstimator
	}
	if states, ok := section.Get("ATTITUDE_STATES"); ok {
		if _, known := acmAttitudeElements[states]; !known {
			return ErrUnknownAttitudeType
		}
		if states == "EULER_ANGLES" {
			if _, ok := section.Get("EULER_ROT_SEQ"); !ok {
				return ErrEulerRotSeqMissing
			}
		}
	}
	if covType, ok := section.Get("COV_TYPE"); ok && covType != "NONE" {
		if _, known := acmCovarianceElements[covType]; !known {
			return ErrUnknownCovarianceType
		}
	}

	seen := make(map[string]bool, len(section.Sensors))
	for _, sensor := range section.Sensors {
		if err := validateACMSection(acmSensor, ACMSection{Fields: sensor.Fields}); err != nil {
			return err
		}
		// Table 5-8: each sensor has a unique number.
		number, ok := sensor.Get("SENSOR_NUMBER")
		if !ok {
			continue
		}
		if seen[number] {
			return ErrDuplicateSensorNumber
		}
		seen[number] = true

		if err := checkSensorNoise(sensor); err != nil {
			return err
		}
	}
	return nil
}

// checkSensorNoise checks a sensor's noise vector against the count beside it.
//
// Table 5-8 says the size of SENSOR_NOISE_STDDEV "will be the same as
// NUMBER_SENSOR_NOISE_COVARIANCE". They are written as two keywords, so
// nothing but this stops them disagreeing.
func checkSensorNoise(sensor ACMSensor) error {
	raw, ok := sensor.Get("SENSOR_NOISE_STDDEV")
	if !ok {
		return nil
	}
	declared, ok := sensor.Get("NUMBER_SENSOR_NOISE_COVARIANCE")
	if !ok {
		return nil
	}
	count, err := ndm.ParseInt(declared)
	if err != nil {
		return err
	}
	number, _, err := ndm.SplitUnits(raw)
	if err != nil {
		number = raw
	}
	if len(strings.Fields(number)) != int(count) {
		return ErrSensorNoiseCount
	}
	return nil
}

// checkVectorWidths checks the keywords whose value is several numbers.
func checkVectorWidths(fields []Field) error {
	for _, f := range fields {
		want, ok := acmVectorWidths[f.Keyword]
		if !ok {
			continue
		}
		number, _, err := ndm.SplitUnits(f.Value)
		if err != nil {
			number = f.Value
		}
		if len(strings.Fields(number)) != want {
			return ErrVectorWidth
		}
	}
	return nil
}

// validateACMSection checks one section against its table: that every keyword
// belongs to it, that none repeats, that they arrive in the table's order, and
// that the mandatory ones are present.
func validateACMSection(name string, section ACMSection) error {
	positions, ok := acmSectionIndex[name]
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

		// Clauses 5.3.3.5 and 5.3.4.1 fix the order of occurrence.
		if at < previous {
			return ErrKeywordsOutOfOrder
		}
		previous = at
	}

	for _, keyword := range acmRequired[name] {
		if !seen[keyword] {
			return ErrMissingKeyword
		}
	}
	return nil
}

// checkACMRowTimes enforces the rules about a block's time tags: one kind
// throughout (clause 5.3.4.5), no duplicates (clause 5.3.4.4), and increasing
// where the section requires it.
func checkACMRowTimes(rows []DataRow, tzero time.Time, increasing bool) error {
	seen := make(map[string]bool, len(rows))
	var previous time.Time

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

		if !increasing {
			continue
		}
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
