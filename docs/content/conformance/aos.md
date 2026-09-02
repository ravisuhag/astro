---
title: AOS Space Data Link Protocol
description: "PICS proforma: what this package implements, clause by clause."
order: 80
---

## Conformance Statement for `pkg/aos` — CCSDS 732.0-B-4

---

## A1.1 GENERAL INFORMATION

### A1.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 08/05/2026 |
| PICS Serial Number | ASTRO-AOS-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A1.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/aos |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS AOS Space Data Link Protocol. Full pipeline: PhysicalChannel (MC mux/demux) → MasterChannel (VC mux, 24-bit gap detection) → VirtualChannel (frame buffer) → Services (M_PDU with packet spanning via FHP and SPP idle-packet fill, B_PDU with bitstream pointer, VCA with exact-fill SDUs, VCF for raw frame ingest). Partial M_PDU packet zones are completed with SPP idle packets (APID 0x7FF) per §4.1.4.2.2; oversized partial B_PDU payloads split across frames rather than clamping the pointer; the idle fill pattern is configurable per channel. |

### A1.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/aos (Go package) |
| System Name(s) | Astro |

### A1.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 732.0-B-4 (AOS Space Data Link Protocol, Blue Book, Issue 4, August 2021) |
| Have any exceptions been required? | Yes [ ] No [X] |

---

## A1.2 PROTOCOL FEATURES

| Feature | Reference | Status | Support |
|---|---|---|---|
| Transfer Frame Version Number | §4.1.2.2 | M | Y — TFVN = 1 (0b01) |
| Spacecraft Identifier | §4.1.2.3 | M | Y — 8-bit SCID (0-255) |
| Virtual Channel Identifier | §4.1.2.4 | M | Y — 6-bit VCID (0-63) |
| Virtual Channel Frame Count | §4.1.2.5 | M | Y — 24-bit per-VC counter with wrap |
| Replay Flag | §4.1.2.5.2 | M | Y — 1-bit signaling field |
| VC Frame Count Usage Flag | §4.1.2.5.3 | M | Y — 1-bit signaling field |
| Signaling Reserved Spares | §4.1.2.5.4 | M | Y — encoded as '00'; validated on decode |
| VC Frame Count Cycle | §4.1.2.5.5 | M | Y — 4-bit signaling field |
| Frame Header Error Control | §4.1.2.6 | O | Y — RS(10,6) over GF(2^4); check symbols generated on encode, verified on decode (error detection; correction of up to E=2 symbols not performed — failing frames are rejected) |
| Insert Zone | §4.1.3 | O | Y — Fixed length per physical channel |
| Transfer Frame Data Field | §4.1.4 | M | Y — Carries M_PDU, B_PDU, or VCA |
| Operational Control Field | §4.1.5 | O | Y — Optional 4-byte OCF |
| Frame Error Control Field | §4.1.6 | O | Y — CRC-16-CCITT |

---

## A1.3 SERVICES

| Feature | Reference | Status | Support |
|---|---|---|---|
| Multiplexing PDU (M_PDU) | §3.3.2 | O | Y — Packet spanning with FHP-based resync; partial packet zones completed with SPP idle packets, discarded by APID on extraction |
| Bitstream PDU (B_PDU) | §3.3.3 | O | Y — Bitstream Data Pointer with full/partial frames; partials larger than the pointer can express are split across frames |
| Virtual Channel Access (VCA) | §3.3.4 | O | Y — Constant-length SDU exactly fills the data field; short/long SDUs rejected |
| Virtual Channel Frame (VCF) | §3.3.5 | O | Y — Raw frame ingest/emit |
| Only Idle Data (OID) | §4.1.2.4 | M | Y — VCID 63 reserved; idle frames run their own VC 63 frame count; fill pattern configurable |

---

## A1.4 CHANNEL MANAGEMENT

| Feature | Reference | Status | Support |
|---|---|---|---|
| Physical Channel | §3.2.1 | M | Y — PhysicalChannel with MC multiplexing |
| Master Channel | §3.2.2 | M | Y — MasterChannel with VC multiplexing |
| Virtual Channel | §3.2.3 | M | Y — VirtualChannel with frame buffering |
| Frame Gap Detection | §3.3 | M | Y — Per-VC 24-bit count tracking |
| VC Multiplexing | §3.3.2 | M | Y — Weighted round-robin via SDL |
| MC Multiplexing | §3.3.3 | M | Y — Weighted round-robin via SDL |

---

**Legend**: M = Mandatory, O = Optional, C = Conditional, Y = Yes (supported), N = No (not supported)
