package tdm

import "github.com/ravisuhag/astro/internal/ndm"

// The XML form, CCSDS 503.0-B-2 section 5 with the structure of
// CCSDS 505.0-B-3.
//
// The TDM's two forms line up more neatly than any other message's. A
// Tracking Data Record is a keyword, a timetag and a measurement; in XML it is
// an <observation> element carrying an <EPOCH> and the data-type element:
//
//	RANGE = 2010-215T20:04:24.000 65249.6771931631
//
//	<observation>
//	  <EPOCH>2010-215T20:04:24.000</EPOCH>
//	  <RANGE>65249.6771931631</RANGE>
//	</observation>
//
// The metadata is a flat element list, since table 3-3 is a flat keyword list.
// What does not change is the thing that matters: the units a measurement is
// in still come from the segment's RANGE_UNITS and not from the record, in
// either form.

// xmlObservation is the element one Tracking Data Record becomes.
const xmlObservation = "observation"

// EncodeXML writes the message in the XML form.
func (m *TDM) EncodeXML() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	created, err := ndm.FormatEpoch(m.Header.CreationDate.UTC(), 3)
	if err != nil {
		return nil, err
	}
	header := ndm.Comments(m.Header.Comments)
	header = append(header,
		ndm.Leaf(ndm.KeywordCreationDate, created),
		ndm.Leaf(ndm.KeywordOriginator, m.Header.Originator),
	)
	if m.Header.MessageID != "" {
		header = append(header, ndm.Leaf(ndm.KeywordMessageID, m.Header.MessageID))
	}

	message := &ndm.XMLMessage{
		Root:    "tdm",
		ID:      "CCSDS_TDM_VERS",
		Version: m.Header.Version,
		Schema:  ndm.XMLSchemaTDM,
		Header:  header,
	}

	for i := range m.Segments {
		segment := &m.Segments[i]

		metadata := ndm.Comments(segment.Metadata.Comments)
		for _, f := range segment.Metadata.Fields {
			metadata = append(metadata, ndm.SplitLeaf(f.Keyword, f.Value))
		}

		data := ndm.Comments(segment.Comments)
		for _, obs := range segment.Observations {
			epoch, err := ndm.FormatEpoch(obs.Epoch, epochPrecision(obs.Epoch))
			if err != nil {
				return nil, err
			}
			data = append(data, ndm.Block(xmlObservation,
				ndm.Leaf("EPOCH", epoch),
				ndm.Leaf(obs.Keyword, formatValue(obs.Value)),
			))
		}

		message.Segments = append(message.Segments, ndm.Segment{
			Metadata: metadata,
			Data:     data,
		})
	}
	return message.EncodeXML()
}

// DecodeXML reads a Tracking Data Message in the XML form.
func DecodeXML(data []byte) (*TDM, error) {
	message, err := ndm.DecodeXML(data, "tdm")
	if err != nil {
		return nil, err
	}
	if message.ID != "CCSDS_TDM_VERS" {
		return nil, ErrNotATDM
	}

	m := &TDM{Header: Header{
		Version:  message.Version,
		Comments: ndm.CollectComments(message.Header),
	}}

	created, ok := ndm.Find(message.Header, ndm.KeywordCreationDate)
	if !ok {
		return nil, ndm.ErrMissingHeaderField
	}
	t, err := ndm.ParseEpoch(created)
	if err != nil {
		return nil, err
	}
	m.Header.CreationDate = t

	if m.Header.Originator, ok = ndm.Find(message.Header, ndm.KeywordOriginator); !ok {
		return nil, ndm.ErrMissingHeaderField
	}
	m.Header.MessageID, _ = ndm.Find(message.Header, ndm.KeywordMessageID)

	for _, xmlSegment := range message.Segments {
		segment := Segment{}
		if err := readXMLMetadata(&segment.Metadata, xmlSegment.Metadata); err != nil {
			return nil, err
		}
		if err := readXMLObservations(&segment, xmlSegment.Data); err != nil {
			return nil, err
		}
		m.Segments = append(m.Segments, segment)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func readXMLMetadata(md *Metadata, elements []ndm.Element) error {
	seen := make(map[string]bool)

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			md.Comments = append(md.Comments, e.Value)
			continue
		}
		if len(e.Children) > 0 {
			return ndm.ErrMalformedXML
		}
		if !isMetadataKeyword(e.Name) {
			return ErrUnknownKeyword
		}
		if seen[e.Name] {
			return ErrDuplicateKeyword
		}
		seen[e.Name] = true

		if index, ok := participantIndex(e.Name); ok && (index < 1 || index > 5) {
			return ErrParticipantIndex
		}
		// The units return to the value, so the accessors read the same thing
		// whichever form the message arrived in.
		md.Fields = append(md.Fields, Field{Keyword: e.Name, Value: e.JoinValue()})
	}
	return nil
}

// readXMLObservations reads the <observation> elements.
//
// Each carries exactly one measurement beside its epoch, which is the same
// rule clause 3.4.3 states for the key-value form: a record is a timetag and
// one observable, and either without the other is useless.
func readXMLObservations(segment *Segment, elements []ndm.Element) error {
	for _, e := range elements {
		if len(e.Children) == 0 {
			if e.Name == ndm.KeywordComment {
				segment.Comments = append(segment.Comments, e.Value)
				continue
			}
			return ErrUnknownKeyword
		}
		if e.Name != xmlObservation {
			return ErrUnknownKeyword
		}

		obs, err := readXMLObservation(e.Children)
		if err != nil {
			return err
		}
		segment.Observations = append(segment.Observations, obs)
	}
	return nil
}

func readXMLObservation(elements []ndm.Element) (Observation, error) {
	var (
		obs   Observation
		epoch string
		found bool
	)

	for _, e := range elements {
		if e.Name == ndm.KeywordComment || len(e.Children) > 0 {
			continue
		}
		if e.Name == "EPOCH" {
			epoch = e.Value
			continue
		}
		if !isDataKeyword(e.Name) {
			return obs, ErrUnknownKeyword
		}
		if found {
			// Two measurements in one observation. Clause 3.4.3 pairs a
			// timetag with one observable, so a second has no timetag of its
			// own and no way to say which epoch it belongs to.
			return obs, ErrMalformedRecord
		}
		v, err := ndm.ParseFloat(e.Value)
		if err != nil {
			return obs, err
		}
		obs.Keyword, obs.Value, found = e.Name, v, true
	}

	if epoch == "" || !found {
		return obs, ErrMalformedRecord
	}
	t, err := ndm.ParseEpoch(epoch)
	if err != nil {
		return obs, err
	}
	obs.Epoch = t
	return obs, nil
}
