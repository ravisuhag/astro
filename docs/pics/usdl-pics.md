# PICS PROFORMA FOR UNIFIED SPACE DATA LINK PROTOCOL

## Conformance Statement for `pkg/usdl` — CCSDS 732.1-B-2

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 04/04/2026 |
| PICS Serial Number | ASTRO-USDL-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/usdl |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS Unified Space Data Link Protocol. Full pipeline: PhysicalChannel (MC mux/demux) → MasterChannel (VC mux, frame gap detection) → VirtualChannel (frame buffer) → Services (MAPP with packet spanning via FHO, MAPA with fixed SDUs, MAPO with octet stream). Supports both CRC-16 and CRC-32 FECF. Fixed-length and variable-length frames. Insert zone support. |

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
| Specification | CCSDS 732.1-B-2 (Unified Space Data Link Protocol, Blue Book, Issue 2, October 2022) |
| Have any exceptions been required? | Yes [ ] No [X] |

---

## A2.2 IDENTIFICATION OF PROTOCOL

| Feature | Reference | Status | Support |
|---|---|---|---|
| Transfer Frame Version Number | §4.1.2.2 | M | Y — TFVN = 12 (0b1100) |
| Spacecraft Identifier | §4.1.2.3 | M | Y — 16-bit SCID (0-65535) |
| Source or Destination Identifier | §4.1.2.4 | M | Y — 1-bit flag |
| Virtual Channel Identifier | §4.1.2.5 | M | Y — 6-bit VCID (0-63) |
| MAP Identifier | §4.1.2.6 | M | Y — 6-bit MAP ID (0-63) |
| End of Frame Primary Header Flag | §4.1.2.7 | M | Y — Fixed/variable length selection |
| Frame Length | §4.1.2.8 | C | Y — 16-bit, present when EOFPH=0 |
| Insert Zone | §4.1.3 | O | Y — Configurable length via ChannelConfig |
| Transfer Frame Data Field Header | §4.1.4 | M | Y — 5-byte TFDFH |
| TFDZ Construction Rule | §4.1.4.2.2 | M | Y — Rules 0 (packet), 1 (access), 2 (octet stream), 7 (idle) |
| First Header Offset | §4.1.4.2.3 | M | Y — 16-bit FHO with special values |
| Frame Sequence Number | §4.1.4.2.5 | M | Y — 16-bit per-VC counter |
| Operational Control Field | §4.1.5 | O | Y — Optional 4-byte OCF |
| Frame Error Control Field (CRC-16) | §4.1.6 | O | Y — CRC-16-CCITT |
| Frame Error Control Field (CRC-32) | §4.1.6 | O | Y — CRC-32C (Castagnoli) |

---

## A2.3 SERVICES

| Feature | Reference | Status | Support |
|---|---|---|---|
| MAP Packet Service (MAPP) | §3.4.3 | O | Y — Packet spanning with FHO-based resync |
| MAP Access Service (MAPA) | §3.4.4 | O | Y — Fixed-length SDU transfer |
| MAP Octet Stream Service (MAPO) | §3.4.5 | O | Y — Unstructured octet stream |
| Idle Data Service | §3.4.6 | M | Y — Idle frames with construction rule 7 |

---

## A2.4 CHANNEL MANAGEMENT

| Feature | Reference | Status | Support |
|---|---|---|---|
| Physical Channel | §3.2.1 | M | Y — PhysicalChannel with MC multiplexing |
| Master Channel | §3.2.2 | M | Y — MasterChannel with VC multiplexing |
| Virtual Channel | §3.2.3 | M | Y — VirtualChannel with frame buffering |
| MAP Channel | §3.2.4 | M | Y — MAP-level service multiplexing |
| Frame Gap Detection | §3.3 | M | Y — Per-VC 16-bit sequence tracking |
| VC Multiplexing | §3.3.2 | M | Y — Weighted round-robin via SDL |
| MC Multiplexing | §3.3.3 | M | Y — Weighted round-robin via SDL |

---

**Legend**: M = Mandatory, O = Optional, C = Conditional, Y = Yes (supported), N = No (not supported)
