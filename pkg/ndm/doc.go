// Package ndm implements the NDM combined instantiation of CCSDS 505.0-B-3,
// the XML file that carries several navigation data messages at once.
//
// The other navigation packages each read and write one standard's messages:
// pkg/odm the orbit messages, pkg/adm the attitude messages, pkg/tdm tracking
// data and pkg/cdm conjunctions. Clause 4.11 of the XML specification allows
// any number of those, of any types, in one file under an <ndm> root. This
// package is that file.
//
//	combined, err := ndm.DecodeCombined(data)
//	for _, m := range combined.Messages {
//	    fmt.Println(ndm.Kind(m), m.Humanize())
//	}
//
// # It is not a concatenation
//
// The rules are all about attributes. Clause 4.11.4 gives the <ndm> root the
// namespace and schema attributes but neither 'id' nor 'version', because it
// is not a message and has no version of its own. Clause 4.11.5 then allows a
// constituent message tag 'id' and 'version' and nothing else: the attributes
// it would carry as a standalone document move up to the root.
//
// So joining several files by hand produces something that is not a combined
// instantiation. It would leave each message's namespace and schema attributes
// where clause 4.11.5 forbids them, and several XML declarations in one file.
//
// # Which schema
//
// A combined instantiation names one master schema for the whole file, and
// each navigation standard names a different one — CCSDS 502.0-B-3 gives 3.0
// and CCSDS 504.0-B-2 gives 4.0. A file mixing their messages can only name
// one of them.
//
// The documents show the difficulty rather than resolving it: figure 7-3 of
// CCSDS 504.0-B-2 writes ndmxml-4.0.0-master-4.0.xsd over a file of ADM
// messages, and figure G-12 of the same document writes
// ndmxml-3.0.0-master-3.0.xsd over another. This package carries whatever the
// file had, and defaults a new one to the schema its first message names.
//
// # The key-value form has no equivalent
//
// Aggregation is an XML feature. Clause 5.2.2 of CCSDS 504.0-B-2 says a
// sequence of ACMs "may be aggregated into a single Navigation Data Message
// (NDM) XML file", and neither standard defines a way to do it in the
// 'keyword = value' notation. There is nothing here for the key-value form
// because there is nothing in the standards for it.
package ndm
