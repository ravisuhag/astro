package pus

// ST[03] housekeeping, per ECSS-E-ST-70-41C clause 8.3.
//
// A housekeeping parameter report structure names a set of on-board parameters
// and how often to sample them. Once defined and enabled, the on-board system
// emits TM[3,25] reports carrying those parameters' values.
//
// This package handles the structure definitions and the report framing. It
// does not sample anything: parameter values are supplied by the caller, since
// only the flight software knows what they mean.
const (
	ServiceHousekeeping uint8 = 3

	SubtypeCreateHKStructure   uint8 = 1  // TC[3,1] clause 8.3.2.1
	SubtypeDeleteHKStructure   uint8 = 3  // TC[3,3] clause 8.3.2.3
	SubtypeEnableHKGeneration  uint8 = 5  // TC[3,5] clause 8.3.2.5
	SubtypeDisableHKGeneration uint8 = 6  // TC[3,6] clause 8.3.2.6
	SubtypeHKParameterReport   uint8 = 25 // TM[3,25] clause 8.3.2.25
)

// SuperCommutatedSet is one group of parameters sampled more than once per
// collection interval (Figure 8-21).
//
// Ordinary parameters are sampled once per interval. A super-commutated set
// repeats its sampling a fixed number of times, which is how a fast-changing
// value rides in a slow report.
type SuperCommutatedSet struct {
	// RepetitionNumber is how many times each parameter in this set is
	// sampled per collection interval.
	RepetitionNumber uint64
	// ParameterIDs names the parameters in the set.
	ParameterIDs []uint64
}

// HousekeepingStructure is TC[3,1]: create a housekeeping parameter report
// structure, per Figure 8-21.
type HousekeepingStructure struct {
	Profile MissionProfile

	// StructureID names this report structure.
	StructureID uint64
	// CollectionInterval is how often the report is generated, in the units
	// the mission declares.
	CollectionInterval uint64
	// ParameterIDs are the parameters sampled once per interval.
	ParameterIDs []uint64
	// SuperCommutated are the parameter groups sampled several times per
	// interval.
	SuperCommutated []SuperCommutatedSet
}

// Key returns the message type.
func (s *HousekeepingStructure) Key() MessageKey {
	return MessageKey{Service: ServiceHousekeeping, Subtype: SubtypeCreateHKStructure}
}

// Validate checks the structure definition.
func (s *HousekeepingStructure) Validate() error {
	return s.Profile.Validate()
}

// Encode serializes the application data field per Figure 8-21:
// structure ID, collection interval, N1, N1 parameter IDs, NFA, then for each
// of NFA groups a repetition number, N2, and N2 parameter IDs.
func (s *HousekeepingStructure) Encode() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	p := s.Profile

	out, err := putUint(nil, s.StructureID, p.HousekeepingStructureIDBytes)
	if err != nil {
		return nil, err
	}
	if out, err = putUint(out, s.CollectionInterval, p.CollectionIntervalBytes); err != nil {
		return nil, err
	}

	if out, err = putUint(out, uint64(len(s.ParameterIDs)), p.CountBytes); err != nil {
		return nil, err
	}
	for _, id := range s.ParameterIDs {
		if out, err = putUint(out, id, p.ParameterIDBytes); err != nil {
			return nil, err
		}
	}

	if out, err = putUint(out, uint64(len(s.SuperCommutated)), p.CountBytes); err != nil {
		return nil, err
	}
	for _, set := range s.SuperCommutated {
		if out, err = putUint(out, set.RepetitionNumber, p.CountBytes); err != nil {
			return nil, err
		}
		if out, err = putUint(out, uint64(len(set.ParameterIDs)), p.CountBytes); err != nil {
			return nil, err
		}
		for _, id := range set.ParameterIDs {
			if out, err = putUint(out, id, p.ParameterIDBytes); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// DecodeHousekeepingStructure parses TC[3,1].
func DecodeHousekeepingStructure(profile MissionProfile, data []byte) (*HousekeepingStructure, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	p := profile
	s := &HousekeepingStructure{Profile: p}
	offset := 0

	id, err := readUint(data[offset:], p.HousekeepingStructureIDBytes)
	if err != nil {
		return nil, err
	}
	s.StructureID = id
	offset += p.HousekeepingStructureIDBytes

	interval, err := readUint(data[offset:], p.CollectionIntervalBytes)
	if err != nil {
		return nil, err
	}
	s.CollectionInterval = interval
	offset += p.CollectionIntervalBytes

	ids, n, err := readUintList(data[offset:], p.CountBytes, p.ParameterIDBytes)
	if err != nil {
		return nil, err
	}
	s.ParameterIDs = ids
	offset += n

	nfa, err := readUint(data[offset:], p.CountBytes)
	if err != nil {
		return nil, err
	}
	offset += p.CountBytes

	for i := uint64(0); i < nfa; i++ {
		rep, err := readUint(data[offset:], p.CountBytes)
		if err != nil {
			return nil, err
		}
		offset += p.CountBytes

		ids, n, err := readUintList(data[offset:], p.CountBytes, p.ParameterIDBytes)
		if err != nil {
			return nil, err
		}
		offset += n
		s.SuperCommutated = append(s.SuperCommutated, SuperCommutatedSet{
			RepetitionNumber: rep,
			ParameterIDs:     ids,
		})
	}

	// The counts fully determine the body size; octets beyond it are a
	// malformed request, not padding.
	if offset != len(data) {
		return nil, ErrTrailingBytes
	}
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *HousekeepingStructure) Humanize() string {
	return "PUS TC[3,1] create housekeeping parameter report structure" +
		"\n  Structure ID ......... " + itoa(int(s.StructureID)) +
		"\n  Collection interval .. " + itoa(int(s.CollectionInterval)) +
		"\n  Parameters ........... " + itoa(len(s.ParameterIDs)) +
		"\n  Super-commutated ..... " + itoa(len(s.SuperCommutated))
}

// HousekeepingReport is TM[3,25]: a housekeeping parameter report.
//
// The report carries the structure ID and then the sampled parameter values
// back to back. Their layout is deduced from the structure definition, which
// both ends already share, so this package moves the values verbatim.
type HousekeepingReport struct {
	Profile MissionProfile

	// StructureID names the report structure these values belong to.
	StructureID uint64
	// ParameterValues holds the sampled values, laid out as the structure
	// definition dictates. The caller supplies and interprets them.
	ParameterValues []byte
}

// Key returns the message type.
func (r *HousekeepingReport) Key() MessageKey {
	return MessageKey{Service: ServiceHousekeeping, Subtype: SubtypeHKParameterReport}
}

// Encode serializes the source data field.
func (r *HousekeepingReport) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, r.StructureID, r.Profile.HousekeepingStructureIDBytes)
	if err != nil {
		return nil, err
	}
	return append(out, r.ParameterValues...), nil
}

// DecodeHousekeepingReport parses TM[3,25].
func DecodeHousekeepingReport(profile MissionProfile, data []byte) (*HousekeepingReport, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	id, err := readUint(data, profile.HousekeepingStructureIDBytes)
	if err != nil {
		return nil, err
	}

	r := &HousekeepingReport{Profile: profile, StructureID: id}
	if len(data) > profile.HousekeepingStructureIDBytes {
		r.ParameterValues = make([]byte, len(data)-profile.HousekeepingStructureIDBytes)
		copy(r.ParameterValues, data[profile.HousekeepingStructureIDBytes:])
	}
	return r, nil
}

// Humanize returns a human-readable summary.
func (r *HousekeepingReport) Humanize() string {
	return "PUS TM[3,25] housekeeping parameter report" +
		"\n  Structure ID .. " + itoa(int(r.StructureID)) +
		"\n  Values ........ " + itoa(len(r.ParameterValues)) + " octets"
}

// HousekeepingControlRequest is TC[3,3], TC[3,5] and TC[3,6]: delete report
// structures, or enable and disable their periodic generation. All three carry
// a count followed by that many structure IDs.
type HousekeepingControlRequest struct {
	Profile MissionProfile
	// Subtype selects which of the three requests this is.
	Subtype uint8
	// StructureIDs names the report structures to act on.
	StructureIDs []uint64
}

// Key returns the message type.
func (r *HousekeepingControlRequest) Key() MessageKey {
	return MessageKey{Service: ServiceHousekeeping, Subtype: r.Subtype}
}

// Validate checks the request.
func (r *HousekeepingControlRequest) Validate() error {
	switch r.Subtype {
	case SubtypeDeleteHKStructure, SubtypeEnableHKGeneration, SubtypeDisableHKGeneration:
	default:
		return ErrWrongMessageType
	}
	return r.Profile.Validate()
}

// Encode serializes the application data field.
func (r *HousekeepingControlRequest) Encode() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, uint64(len(r.StructureIDs)), r.Profile.CountBytes)
	if err != nil {
		return nil, err
	}
	for _, id := range r.StructureIDs {
		if out, err = putUint(out, id, r.Profile.HousekeepingStructureIDBytes); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodeHousekeepingControlRequest parses TC[3,3], TC[3,5] or TC[3,6].
func DecodeHousekeepingControlRequest(profile MissionProfile, subtype uint8, data []byte) (*HousekeepingControlRequest, error) {
	r := &HousekeepingControlRequest{Profile: profile, Subtype: subtype}
	if err := r.Validate(); err != nil {
		return nil, err
	}

	ids, n, err := readUintList(data, profile.CountBytes, profile.HousekeepingStructureIDBytes)
	if err != nil {
		return nil, err
	}
	if n != len(data) {
		return nil, ErrTrailingBytes
	}
	r.StructureIDs = ids
	return r, nil
}

// Humanize returns a human-readable summary.
func (r *HousekeepingControlRequest) Humanize() string {
	verb := map[uint8]string{
		SubtypeDeleteHKStructure:   "delete",
		SubtypeEnableHKGeneration:  "enable periodic generation of",
		SubtypeDisableHKGeneration: "disable periodic generation of",
	}[r.Subtype]
	return "PUS TC[3," + itoa(int(r.Subtype)) + "] " + verb + " housekeeping report structures" +
		"\n  Structures ... " + itoa(len(r.StructureIDs))
}

// registerST03 adds the ST[03] codecs to a registry.
func registerST03(r *Registry) error {
	if err := r.RegisterRequest(
		MessageKey{Service: ServiceHousekeeping, Subtype: SubtypeCreateHKStructure},
		func(p MissionProfile, data []byte) (Request, error) {
			return DecodeHousekeepingStructure(p, data)
		},
	); err != nil {
		return err
	}

	for _, subtype := range []uint8{SubtypeDeleteHKStructure, SubtypeEnableHKGeneration, SubtypeDisableHKGeneration} {
		sub := subtype
		err := r.RegisterRequest(
			MessageKey{Service: ServiceHousekeeping, Subtype: sub},
			func(p MissionProfile, data []byte) (Request, error) {
				return DecodeHousekeepingControlRequest(p, sub, data)
			},
		)
		if err != nil {
			return err
		}
	}

	return r.RegisterReport(
		MessageKey{Service: ServiceHousekeeping, Subtype: SubtypeHKParameterReport},
		func(p MissionProfile, data []byte) (Report, error) {
			return DecodeHousekeepingReport(p, data)
		},
	)
}
