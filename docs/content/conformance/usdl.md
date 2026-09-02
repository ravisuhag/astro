---
title: Unified Space Data Link Protocol
short: USDL
description: "PICS proforma: what this package implements, clause by clause."
order: 90
---

## Conformance Statement for `pkg/usdl` — CCSDS 732.1-B-3

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 25/08/2026 |
| PICS Serial Number | ASTRO-USDL-PICS-003 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/usdl |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS Unified Space Data Link Protocol. Full pipeline: PhysicalChannel (MC mux/demux) -> MasterChannel (VC mux, gap detection via the VCF Count) -> VirtualChannel (frame buffer with a per-MAP receive demultiplexer) -> MAP services (MAPP packets under rules '000'/'111', MAPA SDUs under rules '001'/'010'/'111', MAPO octet stream under rule '011'). Non-truncated and truncated (annex D) headers. Partial fixed-length packet zones completed with Encapsulation Idle Packets per clause 4.1.4.3.4; OID TFDZs filled from the mandatory clause 4.1.4.1.10 PN sequence. |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/usdl (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 732.1-B-3 (Unified Space Data Link Protocol, Blue Book, Issue 3, June 2024) |
| Have any exceptions been required? | Yes [X] No [ ] — see notes: COP-1/COP-P frame acceptance procedures are out of scope (the Bypass/Sequence Control and Protocol Control Command flags are carried but no FARM/FOP runs on them); SDLS integration is out of scope. |

---

## A2.2 TRANSFER FRAME FIELDS

| Feature | Reference | Status | Support |
|---|---|---|---|
| Transfer Frame Version Number | clause 4.1.2.2.2 | M | Y — TFVN = '1100' (12) |
| Spacecraft Identifier | clause 4.1.2.2.3 | M | Y — 16-bit SCID |
| Source-or-Destination Identifier | clause 4.1.2.3 | M | Y — 1-bit flag |
| Virtual Channel Identifier | clause 4.1.2.4 | M | Y — 6-bit VCID (0-63; 63 reserved for OID) |
| MAP Identifier | clause 4.1.2.5 | M | Y — 4-bit MAP ID (0-15) |
| End of Frame Primary Header Flag | clause 4.1.2.6 | M | Y — selects the truncated 4-octet header |
| Frame Length | clause 4.1.2.7 | M | Y — 16-bit, total octets - 1; cross-checked against the delivered buffer on decode |
| Bypass/Sequence Control Flag | clause 4.1.2.8.1 | M | Y — carried and round-tripped (COP procedures out of scope) |
| Protocol Control Command Flag | clause 4.1.2.8.2 | M | Y — carried and round-tripped |
| Reserve Spares | clause 4.1.2.9 | M | Y — encoded '00'; validated on decode |
| OCF Flag | clause 4.1.2.10 | M | Y — set from OCF presence; drives OCF extraction on decode |
| VCF Count Length | clause 4.1.2.11 | M | Y — 3-bit field, 0-7 octet counts |
| VCF Count | clause 4.1.2.12 | C | Y — 0-56 bit count, big-endian; independent sequence-controlled and expedited counters per VC (clause 4.1.2.12.4-12.5) |
| Insert Zone | clause 4.1.3 | O | Y — configurable length via ChannelConfig |
| TFDF Header | clause 4.1.4.2 | M | Y — 1 octet (rules + UPID) plus 16-bit pointer only for rules '000'/'001'/'010' |
| TFDZ Construction Rules | clause 4.1.4.2.2 | M | Y — all eight values defined; services emit '000', '001', '010', '011', '111' |
| USLP Protocol Identifier | clause 4.1.4.2.3 | M | Y — SANA registry values (0-8, 31) provided as constants; set per service |
| First Header / Last Valid Octet Pointer | clause 4.1.4.2.4 | C | Y — FHP for rule '000', LVOP for '001'/'010'; 'all ones' specials |
| Only Idle Data frames | clause 4.1.4.1.5-1.9 | M | Y — VCID 63, MAP 0, rule '001', UPID 'Idle Data', LVOP = last TFDZ octet |
| OID TFDZ fill | clause 4.1.4.1.10 | M | Y — 32-cell Fibonacci LFSR PN sequence (polynomial D0+D1+D2+D22+D32, all-ones seed, never restarted), known-answer tested against annex H |
| Idle fill of partial fixed TFDZs | clause 4.1.4.3.4 | M | Y — Encapsulation Idle Packet completes the TFDZ; discarded on extraction |
| Operational Control Field | clause 4.1.5 | O | Y — 4 octets, presence signaled by the OCF Flag; content comes from an OCF supplier hook (SetOCFSupplier) — channels with HasOCF and no supplier refuse to emit rather than fabricate an all-zero Type-1 report |
| Frame Error Control Field | clause 4.1.6, annex B | O | Y — 16-bit CRC-16-CCITT with known-answer tests (clause 4.1.6.2.2: the FECF, when present, is the last 16 bits; USLP defines no other FECF size) |
| Truncated Transfer Frame | Annex D | O | Y — 4-octet header + 1-octet TFDF header (rule '111'); no insert zone, OCF, FECF, or pointer; length bounds enforced (6-32 octets, D1.3.2 notes 2-3, D1.4.2.4) |

---

## A2.3 SERVICES

| Feature | Reference | Status | Support |
|---|---|---|---|
| MAP Packet Service (MAPP) | clause 3.4, clause 4.2.2 | O | Y — rule '000' with FHP on fixed-length channels (EPP idle fill, FHP resync after loss); rule '111' on variable-length channels |
| MAP Access Service (MAPA) | clause 3.4, clause 4.2.3 | O | Y — constant-length MAPA_SDUs under rules '001'/'010' with LVOP delimiting; rule '111' on variable-length channels |
| MAP Octet Stream Service (MAPO) | clause 3.4, clause 4.2.4 | O | Y — rule '011'; variable-length frames only (fixed-length rejected per clause 4.2.4.1) |
| MAP Multiplexing / Demultiplexing | clause 2.2.1, clause 3.2.4 | M | Y — the VC owns a per-MAP demultiplexer: frames of other MAPs on the shared VC are queued for their own services, never discarded or misdelivered |
| Idle frame generation | clause 4.1.4.1 | M | Y — GetNextFrameOrIdle on fixed-length channels; OID frames keep their own VC 63 count and draw from a persistent, never-restarted PN generator |
| COP-1 / COP-P procedures | refs [9], [10] | O | N — flags carried; FOP/FARM not implemented |
| SDLS | ref [15] | O | N |

---

## A2.4 CHANNEL MANAGEMENT

| Feature | Reference | Status | Support |
|---|---|---|---|
| Physical Channel | clause 2.1.3 | M | Y — PhysicalChannel with MC multiplexing |
| Master Channel | clause 2.1.3 | M | Y — MasterChannel with VC multiplexing |
| Virtual Channel | clause 2.1.3 | M | Y — VirtualChannel with frame buffering |
| MAP Channel | clause 2.1.3 | M | Y — per-MAP service instances with Receive filtering |
| Frame Gap Detection | clause 4.1.2.12 | M | Y — per-VC, per-QoS tracking of the VCF Count at the managed field width |
| VC Multiplexing | clause 4.2.6 | M | Y — weighted round-robin via SDL |
| MC Multiplexing | clause 4.2.8 | M | Y — weighted round-robin via SDL |

---

**Legend**: M = Mandatory, O = Optional, C = Conditional, Y = Yes (supported), N = No (not supported)
