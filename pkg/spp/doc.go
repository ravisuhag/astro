// Package spp implements CCSDS 133.0-B-2, the Space Packet Protocol.
//
// A Space Packet is the unit an application sends: a 6-octet header and a
// payload of 1 to 65,536 octets. The header names the application the
// packet came from or is going to (the APID) and counts packets per APID
// so a receiver can spot gaps. Packets do not travel alone — a data link
// protocol such as pkg/tmdl or pkg/tcdl packs them into transfer frames
// for the trip. See pkg/stack for how the layers fit together.
//
// # What it implements
//
// The full packet format, and both service interfaces the standard
// defines: the Packet Service (clause 3.3), which sends and receives
// pre-built packets, and the Octet String Service (clause 3.4), which
// wraps raw payload bytes automatically. Per-APID receive configuration
// (table 5-1), sequence counting with gap detection, and idle packet
// handling (APID 0x7FF) are all here.
//
// Astro also offers an optional 2-octet CRC-16-CCITT at the end of the
// data field. This is not part of CCSDS 133.0-B-2 — it is a mission and
// PUS-style convention that the standard leaves to the mission — but most
// missions want it, so it is built in rather than left to every caller to
// reimplement.
//
// # What it leaves to you
//
// Secondary header contents. The standard says these are mission-defined,
// so the SecondaryHeader interface sees only octets and moves them; it does
// not know what a mission's timestamp or ancillary data looks like. It also
// cannot enforce that a managed data path keeps one secondary header shape
// for its whole life, or that a relay's per-APID configuration matches what
// the standard requires — those are contracts about behavior over time,
// not properties of one packet.
//
// Segmentation and reassembly of large data units are not provided either:
// the sequence flags can be set, but splitting and rejoining is an
// application concern, and clause 4.1.3.4.2.3 forbids it entirely on a
// managed data path using the Octet String Service.
package spp
