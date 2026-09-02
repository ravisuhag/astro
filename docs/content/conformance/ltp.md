---
title: Licklider Transmission Protocol
short: LTP
description: "PICS proforma: what this package implements, clause by clause."
order: 40
---

## Conformance Statement for `pkg/ltp` — RFC 5326

Conformance is claimed against RFC 5326 only. CCSDS 734.1-B-1 profiles that
RFC for space links; this implementation contains no profile-specific
behavior, so no claim is made against the profile itself — see A1.5.

---

## A1.1 GENERAL INFORMATION

### A1.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 23/08/2026 |
| PICS Serial Number | ASTRO-LTP-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A1.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/ltp, with astro/pkg/sdnv |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | `ReceiverConfig.MaxBlockSize` bounds an assembled block; serial number seeds are caller-supplied |
| Other Information | Go library implementing the LTP segment codecs and caller-pumped sender and receiver session machines. The library owns no goroutines and no clock: every timer is the caller's, matching the shape of `pkg/cop`'s FOP-1. |

### A1.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/ltp, astro/pkg/sdnv (Go packages) |
| System Name(s) | Astro |

### A1.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | RFC 5326 (Licklider Transmission Protocol — Specification, September 2008). CCSDS 734.1-B-1 profiles this RFC; no conformance to the profile's own requirements is claimed — see A1.5 |
| Have any exceptions been required? | Yes [X] No [ ] — see A1.5 |

---

## A1.2 SEGMENT STRUCTURE

| Feature | Reference | Status | Support |
|---|---|---|---|
| Segment header | §3.1 | M | Y |
| Version number 0 | §3.1 | M | Y — other versions rejected on decode |
| Segment type flags: CTRL, EXC, Flag 1, Flag 0 | §3.1.1 | M | Y |
| Segment type codes 0 to 15 | §3.1.2 | M | Y — codes 5, 6, 10, 11 rejected as undefined |
| Segment class masks: CP, EORP, EOB, RS, RA, CS, CAS, CR, CAR | §3.1.3 | M | Y — exposed as predicates on `SegmentType` |
| Session ID: engine ID and session number | §3.1 | M | Y — both SDNV |
| Extensions field, counts octet | §3.1.4 | M | Y — 0 to 15 header, 0 to 15 trailer |
| Extension TLV: tag, SDNV length, value | §3.1.4 | O | Y |
| Header extensions before content | §3.1.4 | M | Y |
| Trailer extensions after content | §3.1.4 | M | Y |
| Self-Delimiting Numeric Values | §1.6 item 20 | M | Y — `pkg/sdnv`, rejects values past 64 bits (`ErrOverflow`) and encodings past ten octets (`ErrTooLong`, RFC 6256 §3.2 padding included) |

---

## A1.3 SEGMENT CONTENT

| Feature | Reference | Status | Support |
|---|---|---|---|
| Data segment: client service ID, offset, length | §3.2.1 | M | Y — all SDNV |
| Checkpoint serial number on checkpoints only | §3.2.1 | M | Y — non-checkpoints carry neither serial |
| Report serial number on checkpoints | §3.2.1 | M | Y — a checkpoint closing a retransmission cycle carries the prompting report's serial; zero when no report prompted it |
| Checkpoint serial number never zero | §3.2.1 | M | Y — enforced on encode and decode |
| Report segment: serials, bounds, claim count, claims | §3.2.2 | O | Y |
| Report serial number never zero | §3.2.2 | M | Y |
| Claim offsets relative to the lower bound | §3.2.2 | M | Y — `ClaimedRanges()` converts to block offsets |
| Claim length at least 1, within the bounds | §3.2.2 | M | Y — validated on encode and decode |
| Claims in ascending order, non-overlapping, at least one | §3.2.2 | M | Y — validated on encode and decode; zero-claim reports are neither emitted nor accepted |
| Report acknowledgment segment | §3.2.3 | O | Y |
| Cancel segment reason code | §3.2.4 | O | Y — codes 0 to 5; 6 to 255 rejected as reserved |
| Cancel acknowledgment has no content | §3.2.5 | O | Y |

---

## A1.4 SESSION PROCEDURES

| Feature | Reference | Status | Support |
|---|---|---|---|
| Red-part reliable delivery | §6 | M | Y |
| Green-part best-effort delivery | §6 | O | Y |
| Mixed red and green blocks | §3.1.1 | O | Y — no segment straddles the boundary |
| Checkpoint at end of red part | §3.1.1 | M | Y |
| End-of-block flagging | §3.1.3 | M | Y |
| Reception reporting on checkpoint | §6.13 | M | Y — report upper bound covers the checkpoint's own end even before the EORP arrives |
| Duplicate checkpoints answered with the same report | §6.11 | M | Y — deduplicated on checkpoint serial; the original report is resent |
| Asynchronous reception reporting | §6.2 | O | Y — `RequestReport`, checkpoint serial zero |
| Retransmission of unclaimed ranges | §6.13 | M | Y |
| Retransmission cycle ends with a checkpoint | §6.9 | M | Y — the cycle's last segment is a checkpoint wherever it sits in the block |
| Report acknowledgment | §6.14 | M | Y — every report acknowledged, including the final one after the sender's session closes |
| Receiver closure gated on report acknowledgment | §6.11, §6.16 | M | Y — the session stays open until every report is acknowledged; an all-green session closes on the green EOB |
| Miscolored block detection | §3.2.4 | M | Y — cancels with reason MISCOLORED |
| Session cancellation from either end | §6.19, §6.20 | O | Y |
| Cancel acknowledgment | §6.21 | M | Y — the sender acknowledges a cancel from the receiver, and the receiver one from the sender |
| Segments for other sessions ignored | §6 | M | Y — filtered on session ID |

---

## A1.5 EXCEPTIONS AND UNSUPPORTED FEATURES

| Feature | Reference | Support | Rationale |
|---|---|---|---|
| CCSDS 734.1-B-1 profile requirements | 734.1-B-1 | N | The implementation follows RFC 5326 as written and contains no profile-specific behavior (managed parameters, profile PICS items). Conformance to the Blue Book's own requirements is therefore not claimed. |
| Timers: checkpoint, report, cancel retransmission | §6.7, §6.8 | N by design | The library owns no clock. Exposed as `ResendCheckpoint` and `RequestReport` for the caller's scheduler to drive. On a light-hour link only the mission can pick a timeout. |
| Session multiplexing | §6 | N | One `Sender` and one `Receiver` per session; managing many is the caller's. |
| Authentication extension | §3.1.4, [LTPEXT] | P | The TLV encodes and decodes; no cryptographic processing. |
| Cookie extension | §3.1.4, [LTPEXT] | P | Same. |
| Random serial number generation | §3.2.1, §3.2.2 | N by design | The spec says the first serials must be random. The caller supplies them; a zero is rejected. A library should not pick a mission's randomness source. |
| Deferred transmission and link-state cues | §6.5 | N | Scheduling policy the caller owns. |
| CLI subcommands | — | N | A follow-up once the API settles. |

---

## A1.6 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| SDNV value range | 0 to 2^64 − 1 | `pkg/sdnv`, 10 octets maximum |
| Header extensions per segment | 15 | 4-bit count, §3.1.4 |
| Trailer extensions per segment | 15 | 4-bit count, §3.1.4 |
| Assembled block size | `MaxBlockSize`, default 64 MiB | Implementation choice; RFC 5326 states no ceiling, but a segment offset is an SDNV reaching 2^64 and would otherwise size an allocation |
| Cancel reason codes accepted | 0 to 5 | §3.2.4; 6 to 255 reserved |
