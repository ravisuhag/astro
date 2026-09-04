package odm

import "github.com/ravisuhag/astro/internal/ndm"

// Validate checks the message against the rules CCSDS 502.0-B-3 states about
// an OPM.
//
// It checks structure, not physics. Clause 1.2 puts orbit accuracy outside the
// standard, so a state vector inside the Earth passes here; what does not pass
// is a Keplerian block with four of its seven parameters, or a manoeuvre in a
// message with no mass.
func (m *OPM) Validate() error {
	if m.Header.Version == "" || m.Header.Originator == "" {
		return ndm.ErrMissingHeaderField
	}

	// Table 3-2 makes these five mandatory. REF_FRAME_EPOCH is conditional on
	// the frame needing one, which only the frame's definition knows.
	for _, field := range []string{
		m.Metadata.ObjectName,
		m.Metadata.ObjectID,
		m.Metadata.CenterName,
		m.Metadata.RefFrame,
		m.Metadata.TimeSystem,
	} {
		if field == "" {
			return ErrMissingKeyword
		}
	}

	if m.Data.StateVector.Epoch.IsZero() {
		return ErrMissingKeyword
	}

	if k := m.Data.Keplerian; k != nil {
		if k.GM == 0 {
			// Table 3-3 makes GM part of the all-or-nothing block, and a
			// gravitational coefficient of zero is not a value anyone means.
			return ErrIncompleteKeplerian
		}
	}

	// Clause 3.2.4.9: a manoeuvre makes the conditional spacecraft parameters
	// mandatory, and the one a manoeuvre cannot be modelled without is mass.
	if len(m.Data.Maneuvers) > 0 {
		if m.Data.Spacecraft == nil || !m.Data.Spacecraft.hasMass {
			return ErrManeuverWithoutMass
		}
	}

	for _, man := range m.Data.Maneuvers {
		if man.EpochIgnition.IsZero() || man.RefFrame == "" {
			return ErrIncompleteManeuver
		}
		// Clause 3.2.4.7: the value must be negative. A manoeuvre that gains
		// mass is not something this format can express.
		if man.DeltaMass >= 0 {
			return ErrDeltaMassNotNegative
		}
	}
	return nil
}

// SetMass records a spacecraft mass, which is the one spacecraft parameter
// whose absence and whose zero value mean different things. Clause 3.2.4.9
// makes it mandatory once a manoeuvre is present, so a message has to be able
// to say "no mass given" as well as "mass is zero".
func (s *SpacecraftParameters) SetMass(kg float64) {
	s.Mass = kg
	s.hasMass = true
}

// HasMass reports whether a mass was given.
func (s *SpacecraftParameters) HasMass() bool { return s.hasMass }
