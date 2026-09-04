// Package csts implements the CCSDS Cross Support Transfer Service
// specification framework, CCSDS 921.1-B-2.
//
// CSTS is the successor to Space Link Extension. Where SLE defines four
// services as four separate standards with four sets of protocol data units,
// CSTS defines a framework of reusable procedures and operations, and a
// service is assembled from them. The Monitored Data service of
// CCSDS 922.1-B-2 and the Tracking Data service of CCSDS 922.2-B-2 are the
// first two built this way.
//
// This package implements the framework's spine: the object identifier tree,
// the common types, the standard operation headers, the Association Control
// procedure, and the framework protocol data unit that carries them all.
//
//	pdu, err := csts.Decode(data)
//	fmt.Println(pdu.Type)                    // "BIND invocation"
//	if header, ok := pdu.Header(); ok {
//	    fmt.Println(header.Procedure.Type)    // which procedure it belongs to
//	}
//
// # A CSTS PDU says what it is; an SLE PDU does not
//
// This is the difference that matters most in practice. An SLE PDU's wire tag
// means one operation in Return All Frames and another in Forward CLTU, and
// nothing in the octets says which service they came from — which is why
// pkg/sle needs the service told to it out of band and refuses to guess.
//
// A framework PDU's tag means the same operation everywhere (annex F3.15), and
// the message carries the name of the procedure instance it belongs to
// (clause 3.3.2.5). So the same octets mean the same thing wherever they
// arrive, and Decode needs nothing but the octets.
//
// # The transport is the same
//
// Clause 2.6 makes ISP1, CCSDS 913.1-B-2, the default underlying protocol, and
// says an implementation using it uses that document's credentials algorithm.
// So the transport and the credential octets are what pkg/sle already builds;
// this package carries credentials rather than interpreting them, and a caller
// that needs them can use pkg/sle for the algorithm.
//
// # What is not here
//
// The twelve procedures of section 4 are not implemented as state machines.
// This package reads and writes their messages; it does not run Buffered Data
// Delivery or Sequence-Controlled Data Processing. That is the same split
// pkg/sle makes, where the codecs are pure and the association machine is
// caller-pumped.
//
// Three of the twenty PDU alternatives are carried as raw content rather than
// modelled: the EXECUTE-DIRECTIVE invocation, whose directive qualifier is a
// four-way CHOICE over SANA-registered identifiers, and the two buffer
// messages, which belong to the Buffered Data Delivery and Buffered Data
// Processing procedures rather than to the common operations of annex F3.4.
// Their octets are kept, so nothing is lost by decoding one.
//
// Several fields of the modelled operations are likewise carried as encoded
// octets: a Time, a Name, an EventValue, a ListOfParametersEvents. Each is
// built from identifiers registered with SANA rather than fixed by this
// document, so a Go type for them would be a type for a registry that changes
// without this package.
//
// # The standard numbers
//
// The CSTS suite, for the record:
//
//	CCSDS 921.1-B-2   the specification framework, which this package implements
//	CCSDS 922.1-B-2   Monitored Data
//	CCSDS 922.2-B-2   Tracking Data
//	CCSDS 922.3-B-1   Forward Frame
//	CCSDS 913.1-B-2   ISP1, the transport, implemented by pkg/sle
//
// CCSDS 920.0-G-1 is the CSTS Green Book, informative rather than normative.
package csts
