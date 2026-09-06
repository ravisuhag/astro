package cdm

import (
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// Header is the CDM header (table 3-1).
//
// Two things separate it from the other navigation headers. MESSAGE_ID is
// obligatory here and optional everywhere else, because a conjunction warning
// has to be referable when someone asks about it later. And MESSAGE_FOR exists
// only here, naming the spacecraft the warning was sent to. There is no
// CLASSIFICATION.
type Header struct {
	Version      string
	Comments     []string
	CreationDate time.Time
	Originator   string
	// MessageFor names the spacecraft the message is provided for. Optional.
	MessageFor string
	// MessageID is obligatory.
	MessageID string
}

var headerSpec = ndm.HeaderSpec{
	VersionKeyword: "CCSDS_CDM_VERS",
	Classification: ndm.Absent,
	MessageFor:     ndm.Optional,
	MessageID:      ndm.Mandatory,
}

// Field is one keyword assignment, kept in the order it arrived.
type Field struct {
	Keyword string
	Value   string
}

// Section is an ordered set of keyword assignments with the comments that
// introduced it.
//
// The CDM has over a hundred keywords across five tables, most of them
// optional and many describing the originator's orbit determination process
// rather than the conjunction. They are held as a list, with accessors for the
// ones a reader acts on, for the same reason the TDM's metadata is: a struct
// of a hundred pointers helps nobody, and a caller meeting an unfamiliar
// keyword could not see it.
type Section struct {
	Comments []string
	Fields   []Field
}

// Get returns the value of a keyword and whether the section carried it.
func (s Section) Get(keyword string) (string, bool) {
	for _, f := range s.Fields {
		if f.Keyword == keyword {
			return f.Value, true
		}
	}
	return "", false
}

// number returns a numeric value, ignoring any unit suffix.
func (s Section) number(keyword string) (float64, bool) {
	raw, ok := s.Get(keyword)
	if !ok {
		return 0, false
	}
	v, err := ndm.ParseValue(raw)
	if err != nil {
		return 0, false
	}
	return v, true
}

// epoch returns a time value.
func (s Section) epoch(keyword string) (time.Time, bool) {
	raw, ok := s.Get(keyword)
	if !ok {
		return time.Time{}, false
	}
	t, err := ndm.ParseEpoch(raw)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// Object is one of the two objects in the conjunction: its metadata and its
// data, which the KVN form does not separate.
type Object struct {
	Section
}

// Name is the object's name, such as a spacecraft name or a debris
// designation.
func (o Object) Name() string {
	v, _ := o.Get("OBJECT_NAME")
	return v
}

// Designator is the satellite catalogue designator, and CatalogName says which
// catalogue it is from.
func (o Object) Designator() string {
	v, _ := o.Get("OBJECT_DESIGNATOR")
	return v
}

// CatalogName is the catalogue the designator belongs to, such as SATCAT.
func (o Object) CatalogName() string {
	v, _ := o.Get("CATALOG_NAME")
	return v
}

// Maneuverable reports whether the object can move out of the way, and whether
// the message said. Table 3-3 makes this obligatory, and it is the first thing
// an operator looks at: a conjunction with an unmanoeuvrable object is a
// conjunction only one side can resolve.
func (o Object) Maneuverable() (bool, bool) {
	v, ok := o.Get("MANEUVERABLE")
	if !ok {
		return false, false
	}
	// Table 3-3's normative values are YES, NO and N/A.
	return v == "YES", true
}

// RefFrame is the frame the state vector is given in.
func (o Object) RefFrame() string {
	v, _ := o.Get("REF_FRAME")
	return v
}

// StateVector is the object's position in km and velocity in km/s at TCA, and
// whether all six components were present.
func (o Object) StateVector() (position, velocity [3]float64, ok bool) {
	names := [...]string{"X", "Y", "Z", "X_DOT", "Y_DOT", "Z_DOT"}
	var values [6]float64
	for i, name := range names {
		v, present := o.number(name)
		if !present {
			return position, velocity, false
		}
		values[i] = v
	}
	return [3]float64{values[0], values[1], values[2]},
		[3]float64{values[3], values[4], values[5]}, true
}

// Covariance is the object's covariance matrix, in the RTN frame.
//
// The matrix is returned at its full 9x9 with absent rows left at zero. Use
// CovarianceOrder to find how many rows were actually present: a zero row and
// an absent one mean different things to anyone computing a probability from
// this, and the difference is not recoverable from the numbers.
func (o Object) Covariance() [9][9]float64 {
	var m [9][9]float64
	for keyword, at := range covarianceIndex {
		if v, ok := o.number(keyword); ok {
			m[at[0]][at[1]] = v
			m[at[1]][at[0]] = v
		}
	}
	return m
}

// CovarianceOrder reports how many rows of the covariance matrix the object
// carried: 6 for the obligatory position and velocity block, and up to 9 when
// the optional drag, solar radiation and thrust rows are present.
func (o Object) CovarianceOrder() int {
	order := 0
	for row := 0; row < 9; row++ {
		// A row is present when its diagonal element is, since table 3-8
		// makes each row's elements conditional together.
		if _, ok := o.Get("C" + covarianceAxes[row] + "_" + covarianceAxes[row]); ok {
			order = row + 1
		}
	}
	return order
}

// CDM is a Conjunction Data Message: one conjunction between two objects
// (CCSDS 508.0-B-1).
type CDM struct {
	Header Header
	// Relative holds table 3-2: what the conjunction is, rather than what
	// either object is.
	Relative Section
	// Objects holds the two object sections, index 0 for OBJECT1.
	Objects [2]Object
}

// TCA is the time of closest approach, the moment the warning is about.
func (m *CDM) TCA() (time.Time, bool) { return m.Relative.epoch("TCA") }

// MissDistance is how close the two objects come, in metres.
func (m *CDM) MissDistance() (float64, bool) { return m.Relative.number("MISS_DISTANCE") }

// RelativeSpeed is the closing speed at TCA, in metres per second.
func (m *CDM) RelativeSpeed() (float64, bool) { return m.Relative.number("RELATIVE_SPEED") }

// CollisionProbability is the probability the originator computed, and
// CollisionProbabilityMethod names how.
//
// The method matters as much as the number. Table 3-2 makes the method
// obligatory whenever the probability is given, because the value is not
// comparable between methods and a bare probability cannot be acted on.
func (m *CDM) CollisionProbability() (float64, string, bool) {
	p, ok := m.Relative.number("COLLISION_PROBABILITY")
	if !ok {
		return 0, "", false
	}
	method, _ := m.Relative.Get("COLLISION_PROBABILITY_METHOD")
	return p, method, true
}

// Validate checks the message against the rules section 3 states.
func (m *CDM) Validate() error {
	if m.Header.Version == "" || m.Header.Originator == "" || m.Header.MessageID == "" {
		return ndm.ErrMissingHeaderField
	}

	// Table 3-2 makes these two obligatory: without them the message says
	// nothing about a conjunction.
	if _, ok := m.TCA(); !ok {
		return ErrMissingKeyword
	}
	if _, ok := m.MissDistance(); !ok {
		return ErrMissingKeyword
	}

	// Clause 3.1.2: a CDM carries data for a single conjunction event, which
	// is between two objects.
	for i := range m.Objects {
		o := &m.Objects[i]
		if len(o.Fields) == 0 {
			return ErrMissingObject
		}
		for _, keyword := range []string{
			"OBJECT_DESIGNATOR", "CATALOG_NAME", "OBJECT_NAME",
			"INTERNATIONAL_DESIGNATOR", "EPHEMERIS_NAME",
			"COVARIANCE_METHOD", "MANEUVERABLE", "REF_FRAME",
		} {
			if _, ok := o.Get(keyword); !ok {
				return ErrMissingKeyword
			}
		}
		if _, _, ok := o.StateVector(); !ok {
			return ErrMissingKeyword
		}
		// Table 3-8 makes the 6x6 block obligatory.
		if o.CovarianceOrder() < 6 {
			return ErrMissingKeyword
		}
	}

	// Table 3-2 pairs the probability with its method.
	if _, ok := m.Relative.Get("COLLISION_PROBABILITY"); ok {
		if _, ok := m.Relative.Get("COLLISION_PROBABILITY_METHOD"); !ok {
			return ErrMissingKeyword
		}
	}
	return nil
}
