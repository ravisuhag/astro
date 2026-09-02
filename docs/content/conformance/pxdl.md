---
title: Proximity-1 Data Link Layer
description: "PICS proforma: what this package implements, clause by clause."
order: 100
---

## Conformance Statement for `pkg/pxdl` — CCSDS 211.0-B-6

---

## A1.1 GENERAL INFORMATION

### A1.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 23/08/2026 |
| PICS Serial Number | ASTRO-PXDL-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A1.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/pxdl |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | `Reassembler.MaxPacketSize` bounds an accumulating packet |
| Other Information | Go library implementing the Version-3 Transfer Frame, its data field constructions, packet segmentation and reassembly keyed on routing ID, and both supervisory PDU formats including the Proximity Link Control Word. |

### A1.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/pxdl (Go package) |
| System Name(s) | Astro |

### A1.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 211.0-B-6 (Proximity-1 Space Link Protocol — Data Link Layer, Blue Book, Issue 6, July 2020) |
| Have any exceptions been required? | Yes [X] No [ ] — see A1.6 |

---

## A1.2 TRANSFER FRAME

| Feature | Reference | Status | Support |
|---|---|---|---|
| Version-3 Transfer Frame structure | §3.2.1 | M | Y — 5-octet header, up to 2043 octets of data |
| Transfer Frame Version Number = binary '10' | §3.2.2.2.2 | M | Y — other values rejected on decode |
| Quality of Service Indicator | §3.2.2.3 | M | Y — sequence controlled and expedited |
| PDU Type ID: U-frame and P-frame | §3.2.2.4 | M | Y |
| Data Field Construction ID | §3.2.2.5 | M | Y — all four values of table 3-1 |
| DFC ID zero in a P-frame | §3.2.2.5.2 | M | Y — enforced |
| Spacecraft Identifier, 10 bits | §3.2.2.6 | M | Y |
| Physical Channel Identifier | §3.2.2.7 | M | Y |
| Port Identifier, 3 bits | §3.2.2.8 | M | Y |
| Port ID zero in a P-frame | §3.2.2.8.2 | M | Y — enforced |
| Source-or-Destination Identifier | §3.2.2.9 | M | Y — '0' = source, '1' = destination, per table 3-2 |
| Frame Length as a count less one | §3.2.2.10.2 | M | Y — 5 to 2048 octets enforced |
| Frame Sequence Number | §3.2.2.11 | M | Y — carried; COP-P is out of scope |

---

## A1.3 DATA FIELD CONSTRUCTIONS

| Feature | Reference | Status | Support |
|---|---|---|---|
| Packets, DFC ID '00' | §3.2.3.2 | O | Y — carried verbatim |
| Segment data units, DFC ID '01' | §3.2.3.3 | O | Y |
| Segment header: sequence flags and pseudo packet ID | §3.2.3.3.2 | M | Y |
| Sequence flag values of table 3-4 | §3.2.3.3.2 a) | M | Y |
| Reassembly by routing ID | §3.2.3.3.3 | M | Y — PCID, Port ID and pseudo packet ID |
| Only complete packets delivered | §3.2.3.3.4 | M | Y |
| Discard on a segment before the start segment | §3.2.3.3.5 b) | M | Y — `ErrSegmentOutOfOrder` |
| User defined data, DFC ID '11' | §3.2.3.5 | O | Y — carried verbatim |

---

## A1.4 SUPERVISORY PDUs

| Feature | Reference | Status | Support |
|---|---|---|---|
| Fixed-length SPDU format | §3.2.4.2.1 | M | Y — 2 octets |
| Variable-length SPDU format | §3.2.4.2.2 | M | Y — 1 octet header, 0 to 15 octets of data |
| Data field length is the actual count | §3.2.4.2.2 a) 3) note | M | Y |
| SPDUs are self-delimiting | §3.2.4.1 | M | Y — a mixed run decodes without a count |
| SPDUs only on the Expedited service | §3.2.4.1 | M | Y — enforced by the constructor |
| Type F1: Proximity Link Control Word | §3.2.4.3.2 | M | Y |
| PLCW field layout of figure 3-5 | §3.2.4.3.2.1.1 | M | Y — all seven fields, verified against the published figure |
| PLCW Report Value = V(R) | §3.2.4.3.2.2.2 | M | Y |
| Type F2 fixed-length SPDU | table 3-5 | — | N — reserved for future CCSDS use |

---

## A1.5 MANAGED PARAMETERS

| Feature | Reference | Status | Support |
|---|---|---|---|
| Managed parameter representation | §4, annex C | M | P — `ManagedParameters` holds the frame-layer subset |
| Local_Spacecraft_ID, Remote_Spacecraft_ID | annex C | M | Y |
| Maximum_Frame_Length, per direction | annex C | M | Y — send and receive maxima held separately |
| Maximum_Packet_Size | §4.4.2.1 | M | Y |
| Synch_Timeout, PLCW_Repeat_Interval | annex C | M | Y — carried; nothing in this package acts on timers |
| MAC, hailing, and COP-P parameters | annex C | M | N — out of scope with those sublayers, see A1.6 |

---

## A1.6 EXCEPTIONS AND UNSUPPORTED FEATURES

| Feature | Reference | Support | Rationale |
|---|---|---|---|
| COP-P retransmission procedure | §7 | N | Sequence numbers and PLCWs are carried; the state machine that acts on them is a follow-up, modelled on `pkg/cop`. |
| Directive and status report contents | annex B | P | Variable-length SPDUs encode and decode; their payloads are carried without interpretation. |
| MAC sublayer, session establishment | §5, §6 | N | Out of scope for the frame layer. |
| Physical layer, transceiver control | CCSDS 211.1-B | N | A separate specification. |
| Coding and synchronization | CCSDS 211.2-B-3 | N | A separate specification; `pkg/pxsc`. |
| CLI subcommands | — | N | A follow-up once the API settles. |

---

## A1.7 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| Transfer Frame length | 5 to 2048 octets | 11-bit length field, §3.2.2.10.2 |
| Transfer Frame Data field | 2043 octets | §3.2.1 b) |
| Spacecraft Identifier | 0 to 1023 | 10-bit field, §3.2.2.6.1 |
| Port Identifier | 0 to 7 | 3-bit field, §3.2.2.8 |
| Pseudo packet identifier | 0 to 63 | 6-bit field, §3.2.3.3.2 b) |
| Variable-length SPDU data | 0 to 15 octets | 4-bit length field, §3.2.4.2.2 |
| Reassembled packet | `MaxPacketSize`, default 64 KiB | Implementation choice; the standard sets no ceiling |
