---
title: Bundle Protocol
short: BP
description: "PICS proforma: what this package implements, clause by clause."
order: 50
---

## Conformance Statement for `pkg/bp`, RFC 9171

---

## A1.1 GENERAL INFORMATION

### A1.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 03/09/2026 |
| PICS Serial Number | ASTRO-BP-PICS-002 |
| System Conformance Statement Cross-Reference | This document |

### A1.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/bp |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing Bundle Protocol version 7 block formats: the primary block, canonical blocks, the three extension blocks RFC 9171 defines, fragmentation and reassembly, and bundle status reports. Bundle agent behaviour — routing, storage, forwarding, convergence layers — is out of scope. |

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
| Specification | RFC 9171 (Bundle Protocol Version 7, Standards Track, January 2022), with the `ipn` URI scheme per RFC 9758 (May 2025) |
| Have any exceptions been required? | Yes [X] No [ ], see A1.6 |

---

## A1.2 ENCODING

| Feature | Reference | Status | Support |
|---|---|---|---|
| Bundles conform to CBOR | clause 4.1 | M | Y: a hand-rolled subset codec, no external dependency |
| Core deterministic encoding, indefinite-length items excepted | clause 4.1 | M | Y: arguments written in shortest form; longer forms refused on decode |
| Bundle is a CBOR indefinite-length array closed by a break | clause 4.1 | M | Y: a definite-length array is refused. The appendix B grammar reads as definite; clause 4.1 governs and RFC 9173 appendix A.1.1.3 confirms |
| Non-conformant input accepted and transformed | clause 4.1 | O | N: refused instead, see A1.6 |

---

## A1.3 FUNDAMENTAL DATA STRUCTURES

| Feature | Reference | Status | Support |
|---|---|---|---|
| CRC type codes 0, 1, 2 and no others | clause 4.2.1 | M | Y: code 3 and above refused |
| X-25 CRC-16 | clause 4.2.1 | M | Y: pinned to the published check value 0x906E |
| CRC-32C, Castagnoli | clause 4.2.1 | M | Y: stdlib `hash/crc32` |
| CRC as a byte string of 2 or 4 octets, network byte order | clause 4.2.2 | M | Y |
| CRC computed with its own field zeroed | clauses 4.3.1, 4.3.2 | M | Y |
| Bundle processing control flags | clause 4.2.3 | M | Y: all nine version 7 bits |
| Unrecognised bundle flags ignored, not refused | clause 4.2.3 | M | Y |
| Administrative record must not request status reports | clause 4.2.3 | M | Y: validated |
| Anonymous source must set must-not-fragment and request no reports | clause 4.2.3 | M | Y: validated |
| Block processing control flags | clause 4.2.4 | M | Y: all four version 7 bits |
| Endpoint ID as a two-item array | clause 4.2.5.1 | M | Y |
| `dtn` URI scheme, with `dtn:none` as an integer zero | clause 4.2.5.1.1 | M | Y |
| `ipn` URI scheme | clause 4.2.5.1.2 | M | Y: per RFC 9758, see A1.6 |
| DTN time, milliseconds since 2000-01-01, no leap seconds | clause 4.2.6 | M | Y |
| Creation timestamp as time plus sequence number | clause 4.2.7 | M | Y |

---

## A1.4 BLOCK FORMATS

| Feature | Reference | Status | Support |
|---|---|---|---|
| Primary block as an array of 8, 9, 10 or 11 items | clause 4.3.1 | M | Y: a length disagreeing with the flags and CRC type is refused |
| Version field = 7 | clause 4.3.1 | M | Y: other versions refused on decode |
| Three endpoint IDs, then timestamp and lifetime | clause 4.3.1 | M | Y |
| Fragment offset and total ADU length, iff the fragment flag is set | clause 4.3.1 | M | Y |
| Primary block CRC present unless a BPSec BIB targets it | clause 4.3.1 | M | Partial: astro permits CRC type 0, since it cannot see a BIB it does not implement. See A1.6 |
| Canonical block as an array of 5 or 6 items | clause 4.3.2 | M | Y: a length disagreeing with the CRC type is refused |
| Block type code, number, flags, CRC type, type-specific data | clause 4.3.2 | M | Y |
| Block-type-specific data as a definite-length byte string | clause 4.3.2 | M | Y: the indefinite form is refused |
| Payload block is type 1, number 1, and last | clauses 4.1, 4.3.2 | M | Y: validated |
| Exactly one primary and one payload block | clause 4.1 | M | Y: validated |
| Block numbers unique within a bundle | clause 4.1 | M | Y: validated |
| Bundle ends at the break stop code | clause 4.1 | M | Y: trailing octets refused |

---

## A1.5 EXTENSION BLOCKS AND PROCEDURES

| Feature | Reference | Status | Support |
|---|---|---|---|
| Recognise, parse and act on the three defined extension blocks | clause 4.4 | M | Y |
| Unknown block types forwarded intact | clause 4.4 | M | Y: data and flags round-trip byte for byte |
| Previous Node block, type 6 | clause 4.4.1 | M | Y: at most one per bundle, validated |
| Bundle Age block, type 7 | clause 4.4.2 | M | Y: at most one per bundle; required when the creation time is zero |
| Hop Count block, type 10 | clause 4.4.3 | M | Y: hop limit bounded to 1 through 255 |
| Fragmentation | clause 5.8 | O | Y |
| Fragment payloads concatenate to the original | clause 5.8 | M | Y |
| Fragment carries the offset, total length and a fresh CRC | clause 5.8 | M | Y |
| Replicate-in-every-fragment blocks copied to all fragments | clause 5.8 | M | Y |
| Offset-zero fragment carries the remaining extension blocks | clause 5.8 | M | Y |
| Must-not-fragment respected | clause 5.8 | M | Y |
| Reassembly by material extents | clause 5.9 | O | Y: any order, overlaps tolerated, gaps refused |
| Administrative record as a two-item array | clause 6.1 | M | Y |
| Bundle status report, record type 1 | clause 6.1.1 | O | Y |
| Report array of 4 or 6 elements by subject fragmentation | clause 6.1.1 | M | Y |
| Status item of 2 elements only when asserted and times requested | clause 6.1.1 | M | Y: a time on an unasserted status is refused |
| At least four status assertions, extras skipped | clause 6.1.1 | M | Y |
| Reason codes | clause 6.1.1, table 1 | M | Y: the twelve RFC 9171 defines; unknown codes decode rather than fail |

---

## A1.6 EXCEPTIONS AND UNSUPPORTED FEATURES

| Feature | Reference | Support | Rationale |
|---|---|---|---|
| Bundle protocol agent: routing, storage, forwarding, dispatch | clause 5 | N | This package is the wire format and block structure. Forwarding policy, contact graphs and storage belong to a layer above, and need timers and sockets this library does not own. |
| Convergence layer adapters | clause 7 | N | A transport, not a format. Compose with `pkg/ltp` from the outside. |
| Bundle Protocol Security | RFC 9172, RFC 9173 | N | A separate specification, and a separate package if it is written. RFC 9173's appendix is used here only as a source of published bundle octets. |
| Primary block CRC mandatory unless a BIB targets it | clause 4.3.1 | Partial | The rule is conditional on a BPSec block astro does not implement. Refusing CRC type 0 outright would reject valid bundles; astro therefore permits it and leaves the check to a node that can see the BIB. |
| Accepting non-conformant CBOR and repairing it | clause 4.1 | N | Clause 4.1 permits this and astro declines. Quietly accepting a malformed bundle is how two implementations come to disagree about what they are exchanging. |
| `ipn` scheme exactly as RFC 9171 prints it | clause 4.2.5.1.2 | Superseded | astro implements RFC 9758, which packs an allocator identifier into the node number. With the default allocator the octets are identical to RFC 9171's, so this is a superset rather than a departure. |
| Bundle Protocol version 6 | RFC 5050, CCSDS 734.2-B-1 | N | A different wire format, not an earlier revision. Nothing current speaks it. astro implemented it until `v0.4.0`, where it remains in git history. |

---

## A1.7 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| Unsigned integer range | 0 to 2^64 - 1 | CBOR argument width, RFC 8949 clause 3 |
| `ipn` allocator identifier and node number | below 2^32 each | RFC 9758 clause 6.3 |
| Hop limit | 1 to 255 | RFC 9171 clause 4.4.3 |
| Bundle and block size | bounded by the input slice | No ceiling is imposed: a decoder reads from a caller-supplied slice, so the caller's own bound applies |

---

## Wire test vectors

The octets backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/bp) — 18 vectors. Each vector names the clause it comes from and carries the derivation that produced it.

| File | |
|---|---|
| [`bp/bundle.json`](https://github.com/ravisuhag/astro/blob/main/vectors/bp/bundle.json) | 18 vectors |

Four of them are **published octets rather than derived values**. RFC 9173 appendix A prints worked example bundles beside their hex, and the primary block, payload block, Bundle Age block and whole-bundle vectors are those bytes. A different working group wrote them, which is corroboration almost nothing else in this corpus has.

These are data files, so any implementation can check itself against the same octets. See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
