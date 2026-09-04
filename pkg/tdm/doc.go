// Package tdm implements the CCSDS Tracking Data Message, CCSDS 503.0-B-2.
//
// A TDM carries what a ground station measured while it was watching a
// spacecraft: range, Doppler, angles, signal levels, and the weather and
// clock corrections that go with them. It is what one agency sends another
// when it has tracked something on their behalf, and it is normally one
// tracking pass per file.
//
// The shape is a header and then one or more segments. Each segment is a
// metadata section describing how the measurements were taken, followed by a
// data section of Tracking Data Records. A new segment starts whenever any
// metadata value changes — clause 3.3.1.4 requires it, so a switch from
// one-way to two-way tracking is a segment boundary rather than a keyword
// buried in the data.
//
// # What this package does
//
// It reads and writes the message, in 'keyword = value' notation. It does no
// tracking mathematics: nothing here differences a range, unwraps an
// ambiguous one, applies a media correction, or converts an angle between
// frames. Those need the interface control document the standard keeps
// deferring to, and clause 3.1.7 puts even the exchange method outside its
// own scope.
//
//	message, err := tdm.Decode(data)
//	for _, segment := range message.Segments {
//	    units := segment.Metadata.RangeUnits()
//	    for _, obs := range segment.Observations {
//	        fmt.Println(obs.Keyword, obs.Epoch, obs.Value, units)
//	    }
//	}
//
// # A measurement means nothing without its metadata
//
// This is the part of the TDM that catches people. A data record is a
// keyword, a timetag and a number:
//
//	RANGE = 2010-215T20:04:24.000   65249.6771931631
//
// Nothing in that line says what the number is in. Clause 3.5.2.7 puts the
// units in the segment's RANGE_UNITS keyword, which may be km, s or RU — and
// if it is absent the default is km. The example above came from a segment
// declaring RU, so reading it as kilometres is wrong by orders of magnitude
// and nothing in the record itself would tell you.
//
// The same holds for angles, whose meaning comes from ANGLE_TYPE, and for a
// range that is ambiguous because RANGE_MODULUS is non-zero: clause 3.5.2.7
// says such a value "does not represent the actual range to the spacecraft"
// until a calculation using the modulus has been done.
//
// So the metadata is not decoration to skip past. Segment.Metadata carries
// every keyword the segment had, in order, with accessors for the ones that
// change how a number must be read.
package tdm
