package tdm

import (
	"fmt"
	"sort"
	"strings"
)

// Humanize returns a human-readable summary of the message.
//
// The observations are counted by keyword rather than listed: a tracking pass
// runs to thousands of records, and what a reader wants is which data types
// are present and under what configuration.
func (m *TDM) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "CCSDS Tracking Data Message %s\n", m.Header.Version)
	fmt.Fprintf(&sb, "  Originator ...... %s\n", m.Header.Originator)
	fmt.Fprintf(&sb, "  Created ......... %s UTC\n", m.Header.CreationDate.Format("2006-01-02T15:04:05"))
	fmt.Fprintf(&sb, "  Records ......... %d in %d segment(s)\n", m.Observations(), len(m.Segments))

	for i := range m.Segments {
		s := &m.Segments[i]
		fmt.Fprintf(&sb, "  Segment %d\n", i+1)
		sb.WriteString(s.Metadata.Humanize())

		counts := make(map[string]int)
		for _, obs := range s.Observations {
			counts[obs.Keyword]++
		}
		types := make([]string, 0, len(counts))
		for keyword := range counts {
			types = append(types, keyword)
		}
		sort.Strings(types)

		fmt.Fprintf(&sb, "    Data types .... %d record(s)\n", len(s.Observations))
		for _, keyword := range types {
			fmt.Fprintf(&sb, "      %-24s %d\n", keyword, counts[keyword])
		}
	}
	return sb.String()
}

// Humanize returns a human-readable summary of a metadata section.
//
// The units a measurement must be read in come first, because they are what a
// reader is most likely to get wrong: RANGE_UNITS may be km, s or RU, and a
// non-zero RANGE_MODULUS means the range is ambiguous and not yet the range.
func (md Metadata) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "    Time system ... %s\n", md.TimeSystem())

	participants := md.Participants()
	indices := make([]int, 0, len(participants))
	for index := range participants {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	for _, index := range indices {
		fmt.Fprintf(&sb, "    Participant %d . %s\n", index, participants[index])
	}

	if mode := md.Mode(); mode != "" {
		fmt.Fprintf(&sb, "    Mode .......... %s", mode)
		if path := md.Path(); path != "" {
			fmt.Fprintf(&sb, ", path %s", path)
		}
		sb.WriteString("\n")
	}
	if start, ok := md.StartTime(); ok {
		stop, _ := md.StopTime()
		fmt.Fprintf(&sb, "    Span .......... %s to %s\n",
			start.Format("2006-01-02T15:04:05.999"), stop.Format("2006-01-02T15:04:05.999"))
	}

	units := md.RangeUnits()
	if _, given := md.Get(KeywordRangeUnits); !given {
		// Worth saying out loud. Clause 3.5.2.7 defaults to km, and a segment
		// that meant RU and forgot to say so reads as km without complaint.
		units += " (defaulted, the segment does not say)"
	}
	fmt.Fprintf(&sb, "    Range units ... %s\n", units)

	if modulus, ok := md.RangeModulus(); ok && modulus != 0 {
		fmt.Fprintf(&sb, "    Range modulus . %g — RANGE is ambiguous and is not the range\n", modulus)
	}
	if angle := md.AngleType(); angle != "" {
		fmt.Fprintf(&sb, "    Angle type .... %s\n", angle)
	}
	return sb.String()
}
