---
title: Bundle Protocol
short: BP
description: "PICS proforma: what this package implements, clause by clause."
order: 50
---

## Conformance Statement for `pkg/bp`, CCSDS 734.2-B-1 / RFC 5050

---

## A1.1 GENERAL INFORMATION

### A1.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 23/08/2026 |
| PICS Serial Number | ASTRO-BP-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A1.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/bp, with astro/pkg/sdnv |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | `DecodeOptions` bounds block length and block count |
| Other Information | Go library implementing Bundle Protocol version 6 block formats: primary block with the RFC 5050 dictionary and RFC 6260 Compressed Bundle Header Encoding, canonical blocks, the CCSDS Extended Class of Service block, fragmentation and reassembly, and both administrative record types. Bundle agent behavior, routing, storage, custody timers, is out of scope. |

### A1.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub, github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/bp (Go package) |
| System Name(s) | Astro |

### A1.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 734.2-B-1 (CCSDS Bundle Protocol Specification, Blue Book, Issue 1, September 2015), profiling RFC 5050 and RFC 6260 |
| Have any exceptions been required? | Yes [X] No [ ], see A1.6 |

---

## A1.2 CCSDS PROFILE REQUIREMENTS

| Feature | Reference | Status | Support |
|---|---|---|---|
| Bundle Protocol version 6 per RFC 5050 | clause 3.1 | M | Y: not BPv7, which is wire-incompatible |
| IPN naming scheme | clause 3.2.1, RFC 6260 clause 2.1 | M | Y: `IPNEndpoint`, node 1 to 2^64-1, service 0 to 2^64-1 |
| Compressed Bundle Header Encoding | clause 3.2, RFC 6260 clause 2 | M | Y: when all four endpoints are ipn (dtn:none as node 0, service 0) the dictionary length encodes as zero and node/service numbers ride in the offset fields; a decoded dictionary length of zero is parsed as CBHE |
| Node number range enforced | clause 3.2.1 | M | Y: node 0 rejected; on CBHE decode, node 0 with a nonzero service rejected |
| Extended Class of Service block | clause 3.3, annex C | M | Y |
| DTN time precision relaxation | clause 3.4 | O | Y: nanoseconds carried, precision left to the caller |

---

## A1.3 BLOCK FORMATS

| Feature | Reference | Status | Support |
|---|---|---|---|
| Primary bundle block | RFC 5050 clause 4.5.1 | M | Y |
| Version field = 6 | clause 4.5.1 | M | Y: other versions rejected on decode |
| Bundle processing control flags | clause 4.2 | M | Y: SDNV, all defined bits |
| Class of service, bits 7 to 8 | clause 4.2 | M | Y: bulk, normal, expedited; the reserved value 3 rejected |
| Status report request flags, bits 14 to 18 | clause 4.2 | O | Y |
| Administrative record flag constraints | clause 4.2 | M | Y: custody and report flags rejected together with it |
| Anonymous-source constraints | clause 4.2 | M | Y: source dtn:none must not request custody and must set the no-fragment flag |
| Contradictory fragment flags rejected | clause 4.2 | M | Y: a fragment cannot also forbid fragmentation |
| Bundle ends at the last block | clause 4.1 | M | Y: trailing octets rejected (`ErrTrailingBytes`); `DecodeBundleN` returns consumed length for concatenated streams |
| Dictionary with endpoint offsets | clause 4.4, clause 4.5.1 | M | Y: repeated strings interned once |
| Creation timestamp and sequence number | clause 4.5.1 | M | Y |
| Lifetime | clause 4.5.1 | M | Y |
| Fragment offset and total ADU length | clause 4.5.1 | O | Y: present only with the fragment flag |
| Canonical block format | clause 4.5.2 | M | Y |
| Block type code | clause 4.5.2 | M | Y |
| Block processing control flags | clause 4.5.2 | M | Y: all seven defined bits |
| EID reference field | clause 4.5.2 | O | Y: present if and only if the flag is set |
| Block data length and body | clause 4.5.2 | M | Y |
| Payload block, type 1 | clause 4.5.2 | M | Y: exactly one per bundle |
| Last-block flag on the final block | clause 4.5.2 | M | Y: validated |

---

## A1.4 EXTENDED CLASS OF SERVICE

| Feature | Reference | Status | Support |
|---|---|---|---|
| ECOS block conforms to clause 4.5.2 and clause 4.6 | annex C, C2 | M | Y |
| Replicate-in-every-fragment flag set | C2 b) | M | Y: enforced by bundle validation and decode, not just the construction helper |
| No EID references | C2 c) | M | Y: enforced by bundle validation and decode |
| Block data length 2 + N | C2 d) | M | Y |
| Flags byte: critical (0x01) | C2 f) 1) | M | Y |
| Flags byte: streaming (0x02) | C2 f) 2) | M | Y |
| Flags byte: flow label present (0x04) | C2 f) 3) | M | Y |
| Flags byte: reliable (0x08) | C2 f) 4) | M | Y |
| Ordinal byte, 0 to 255 | C2 g) | M | Y |
| Ordinal 255 reserved for custody signals | C3.1.4 | M | Y: rejected unless the bundle is an administrative record |
| Flow label as SDNV | C2 h) | O | Y |
| ECOS precedes the payload block | C3.1.1 | M | Y: validated |
| At most one ECOS block per bundle | C3.1.2 | M | Y: validated |
| Flow-label flag matches the field | C3.1.3 | M | Y: validated |

---

## A1.5 PROCEDURES

| Feature | Reference | Status | Support |
|---|---|---|---|
| Fragmentation | RFC 5050 clause 5.8 | O | Y |
| Replicated blocks copied to every fragment | clause 5.8 | M | Y |
| Blocks preceding the payload replicated in the first fragment; blocks following the payload in the last | clause 5.8 | M | Y |
| "Must not be fragmented" respected | clause 4.2 | M | Y |
| Reassembly | clause 5.9 | O | Y: any order, overlaps tolerated, gaps rejected |
| Administrative record framing | clause 6.1 | M | Y: 4-bit type, 4-bit flags |
| DTN time representation | clause 6.1 | M | Y: seconds and nanoseconds as SDNVs |
| Bundle status report | clause 6.1.1 | O | Y: times present only for set status flags |
| Status flags | clause 6.1.1, figure 11 | M | Y: all five |
| Reason codes | clause 6.1.1 | M | Y: the nine RFC 5050 defines |
| Custody signal | clause 6.1.2 | O | Y: succeeded bit plus 7-bit reason |
| Fragment fields in administrative records | clause 6.1, figure 9 | O | Y |

---

## A1.6 EXCEPTIONS AND UNSUPPORTED FEATURES

| Feature | Reference | Support | Rationale |
|---|---|---|---|
| Bundle protocol agent: routing, storage, forwarding | RFC 5050 clause 5 | N | This package is the wire format and block structure. Forwarding policy, contact graphs and storage belong to a layer above. |
| Custody transfer timers and retransmission | clause 5.10, clause 6.3 | N | Requires an agent and a clock; the same reasoning as `pkg/ltp`. |
| Aggregate Custody Signals | CCSDS 734.2-B-1 annex D | N | A separate normative annex; a follow-up. |
| Delay-Tolerant Payload Conditioning | annex E | N | A separate normative annex; a follow-up. |
| Bundle Security Protocol blocks | [BSP] | N | A separate specification. |
| BP managed information | annex F | N | Management data model, not wire format. |
| Bundle Protocol version 7 | RFC 9171 | N | A CBOR encoding, wire-incompatible with v6. CCSDS 734.2-B-1 profiles v6. |
| CLI subcommands | - | N | A follow-up once the API settles. |

---

## A1.7 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| SDNV value range | 0 to 2^64 - 1 | `pkg/sdnv` |
| Block body length | `MaxBlockLength`, default 16 MiB | Implementation choice; RFC 5050 states no ceiling, but a block length is an SDNV reaching 2^64 and would otherwise size an allocation |
| Blocks per bundle | `MaxBlocks`, default 64 | Same reasoning |
| Reassembled application data unit | 16 MiB | Bounded by the same block-length cap |
