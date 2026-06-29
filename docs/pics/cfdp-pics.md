# PICS PROFORMA FOR CCSDS FILE DELIVERY PROTOCOL

## Conformance Statement for `pkg/cfdp` — CCSDS 727.0-B-5

---

## A1.1 GENERAL INFORMATION

### A1.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 23/08/2026 |
| PICS Serial Number | ASTRO-CFDP-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A1.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/cfdp |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CFDP Part 1. Full PDU codec set, modular and CRC-32 checksums, an abstract Filestore with in-memory and OS-backed implementations, and caller-pumped Class 1 and Class 2 transaction machines. The library owns no goroutines and no clock: timers and retransmission scheduling belong to the caller, matching the shape of `pkg/cop`'s FOP-1. |

### A1.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/cfdp (Go package) |
| System Name(s) | Astro |

### A1.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 727.0-B-5 (CCSDS File Delivery Protocol, Blue Book, Issue 5, July 2020) |
| Have any exceptions been required? | Yes [X] No [ ] — see A1.6 |

---

## A1.2 PROTOCOL DATA UNITS

| Feature | Reference | Status | Support |
|---|---|---|---|
| Fixed PDU header | §5.1.2, table 5-1 | M | Y |
| Version '001' | table 5-1 | M | Y — other versions rejected on decode |
| PDU type: File Directive / File Data | §5.1.6 | M | Y |
| Direction flag | table 5-1 | M | Y |
| Transmission mode flag | table 5-1 | M | Y — '0' acknowledged, '1' unacknowledged |
| CRC flag | table 5-1 | M | Y |
| Large File flag | table 5-1 | M | Y — widens every FSS field to 64 bits |
| PDU data field length | table 5-1 | M | Y — includes the CRC when present |
| Segmentation control | table 5-1 | M | Y |
| Length of entity IDs | table 5-1 | M | Y — 1 to 8 octets, encoded as width less one |
| Segment metadata flag | table 5-1 | M | Y |
| Length of transaction sequence number | table 5-1 | M | Y — 1 to 8 octets |
| Variable-width entity IDs | §5.1.4 | M | Y |
| LV objects | §5.1.8, table 5-2 | M | Y |
| TLV objects | §5.1.9, table 5-3 | M | Y |
| File-Size Sensitive fields | §5.1.10 | M | Y |

---

## A1.3 FILE DIRECTIVE PDUs

| Feature | Reference | Status | Support |
|---|---|---|---|
| Directive codes | §5.2.1.2, table 5-4 | M | Y — reserved codes rejected |
| Condition codes | §5.2.1.3, table 5-5 | M | Y — all fifteen |
| EOF PDU | §5.2.2, table 5-6 | M | Y — condition, checksum, file size, fault location |
| Fault location omitted for 'no error' | table 5-6 | M | Y |
| Finished PDU | §5.2.3, table 5-7 | M | Y — condition, delivery code, file status, responses |
| Fault location omitted for 'no error' and 'unsupported checksum type' | table 5-7 | M | Y |
| ACK PDU | §5.2.4, table 5-8 | M | Y — EOF and Finished only |
| ACK directive subtype rules | table 5-8 | M | Y — '0001' for Finished, '0000' otherwise |
| Transaction status | §5.2.4 | M | Y — all four values |
| Metadata PDU | §5.2.5, table 5-9 | M | Y |
| Closure requested | table 5-9 | M | Y |
| Checksum type field | table 5-9 | M | Y |
| Empty filenames for fileless transactions | table 5-9 | M | Y |
| NAK PDU | §5.2.6, table 5-10 | O | Y |
| Segment requests | §5.2.6.2, table 5-11 | O | Y — including the 0..0 metadata request |
| Prompt PDU | §5.2.7, table 5-12 | O | Y — answered with NAK or Keep Alive |
| Keep Alive PDU | §5.2.8, table 5-13 | O | Y |

---

## A1.4 FILE DATA PDUs

| Feature | Reference | Status | Support |
|---|---|---|---|
| File Data PDU | §5.3, table 5-14 | M | Y |
| Offset field | table 5-14 | M | Y — FSS |
| Record continuation state | §5.3 | O | Y — all four states, decoded when present |
| Segment metadata | table 5-14 | O | Y — up to 63 octets |

---

## A1.5 PROCEDURES

| Feature | Reference | Status | Support |
|---|---|---|---|
| CRC at transmitting entity | §4.1.1 | O | Y |
| CRC at receiving entity | §4.1.2 | O | Y — failing PDUs are discarded |
| CRC algorithm: CCSDS Telecommand CRC | §4.1.3.1 | M | Y — reuses `pkg/crc` |
| CRC placement and coverage | §4.1.3.2 | M | Y — final octets, counted in the data field length |
| Checksum 32 bits | §4.2.1.2 | M | Y |
| Modular checksum | §4.2.2.3 | M | Y — verified against the Annex F worked example |
| Null checksum | §4.2.2.4 | M | Y |
| Additional checksum algorithms | §4.2.2.5 | O | Y — CRC-32C (type 2), CRC-32 (type 3) |
| Class 1 unacknowledged transfer | §4.6 | M | Y |
| Class 2 acknowledged transfer | §4.6 | O | Y — NAK-driven gap recovery |
| Suspend and resume | §4.11 | O | Y — state flags; the caller owns the clock |
| Cancel | §4.11 | O | Y — EOF with the cancel condition code |
| Filestore requests | §5.4.1, table 5-16 | O | P — see A1.6 |
| Filestore responses | §5.4.2, table 5-17 | O | Y — one per request |
| Messages to user | §5.4.3 | O | Y — carried, not interpreted |
| Fault handler override TLV | §5.4.4 | O | P — decoded, not acted on |
| Flow label TLV | §5.4.5 | O | Y — carried, not interpreted |
| Entity ID TLV | §5.4.6 | M | Y — used for fault location |

---

## A1.6 EXCEPTIONS AND UNSUPPORTED FEATURES

| Feature | Reference | Support | Rationale |
|---|---|---|---|
| Filestore actions: append, replace | table 5-16 | N | Decoded; execution returns status 'not performed' (table 5-18 allows this). |
| Filestore actions: create/remove directory, deny directory | table 5-16 | N | Same. The `Filestore` interface is deliberately file-only. |
| Fault handler override execution | §5.4.4 | N | The TLV decodes; overriding handler behavior is left to the application. |
| Adaptive flow control from Keep Alive | §4.6 | N | Keep Alive and Prompt encode and decode; no rate adaptation. |
| Proxy and remote operations | CFDP Part 2 | N | A separate standard, out of scope for this package. |
| Timers and inactivity detection | §4.6 | N by design | The library owns no clock. Retransmission and timeout scheduling are the caller's, exposed as `ResendEOF`, `RequestNAK`, and `ResendFinished`. |

---

## A1.7 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| Entity ID width | 1 to 8 octets | 3-bit length field, table 5-1 |
| Transaction sequence number width | 1 to 8 octets | 3-bit length field, table 5-1 |
| PDU data field length | 65535 octets | 16-bit field, table 5-1 |
| LV / TLV value length | 255 octets | 8-bit length field, tables 5-2 and 5-3 |
| Segment metadata length | 63 octets | 6-bit length field, table 5-14 |
| File size, small file | 2^32 − 1 octets | 32-bit FSS, §5.1.10 |
| File size, large file | 2^64 − 1 octets | 64-bit FSS, §5.1.10 |
