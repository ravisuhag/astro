// Package sle implements the foundation of the CCSDS Space Link Extension
// transfer services: the Internet SLE Protocol One transport, a BER codec for
// SLE protocol data units, ISP1 credentials, and the association handshake.
//
// SLE is how ground systems move space link data between each other. A mission
// control centre opens a TCP connection to a ground station and either
// receives telemetry — frames, channel frames, operational control fields — or
// sends telecommand transmission units. This library already produces and
// consumes exactly those payloads: CADUs from pkg/tmsc, CLTUs from pkg/tcsc,
// CLCWs from pkg/cop. SLE is the wire between ground systems.
//
// # The standard numbers
//
// The SLE suite is easy to misattribute, so for the record:
//
//	CCSDS 913.1-B-2   ISP1, the transport this package implements
//	CCSDS 911.1-B-5   Return All Frames
//	CCSDS 911.2-B-4   Return Channel Frames
//	CCSDS 911.5-B-4   Return Operational Control Fields
//	CCSDS 912.1-B-5   Forward CLTU
//
// CCSDS 914.0-M-2 is the SLE Application Program Interface, a Recommended
// Practice rather than a Blue Book, and is not a wire specification.
//
// # Three design decisions
//
// First, this package owns no goroutines and no timers. Every codec is pure
// and the association state machine is caller-pumped, the same contract as
// pkg/cop's FOP-1: you feed it inbound messages and ask what to send. ISP1 has
// a heartbeat and a dead factor; this package tells you when a heartbeat is
// due and when a peer has gone silent, and your scheduler acts on it. Actual
// TCP I/O is yours.
//
// Second, the BER codec here is a deliberate subset. Go's encoding/asn1 is
// DER-oriented and cannot round-trip SLE PDUs: it rejects the context-specific
// CHOICE tagging the SLE modules rely on. Rather than take a dependency, this
// package carries just enough BER to encode and decode what SLE actually
// sends, developed against the ASN.1 modules in the service specifications.
// It emits definite lengths only, and accepts the indefinite form on decode,
// because real providers emit it.
//
// Third, ISP1 credentials hash with SHA-256, per CCSDS 913.1-B-2 §3.1.2.3.
// SHA-1 belonged to the previous issue of the standard. A 20-octet legacy
// digest still decodes — no other length but 20 or 32 does — but it cannot be
// verified, because this package does not implement the superseded scheme:
// verification requires SHA-256, and only SHA-256 is ever generated.
//
// # What is here
//
// The transport and the handshake: TML message framing, the context and
// heartbeat messages, BER, credentials, and BIND, UNBIND and PEER-ABORT —
// including the OBJECT IDENTIFIER form of the service instance identifier
// and the primitive [104] PEER-ABORT encoding. On top of that, the four
// transfer services themselves: RAF, RCF, ROCF and FCLTU, each as a
// caller-pumped user machine and a partial provider. GET-PARAMETER decodes
// and answers cleanly, and the per-service parameter sets are named: all 50
// alternatives across the four services, with integer values read and
// structured ones left as raw BER. See docs/content/conformance/sle.md
// for the row-by-row conformance picture.
package sle
