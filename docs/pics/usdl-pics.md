# PICS PROFORMA FOR UNIFIED SPACE DATA LINK PROTOCOL

## Conformance Statement for `pkg/usdl` — CCSDS 732.1-B-2

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 25/08/2026 |
| PICS Serial Number | ASTRO-USDL-PICS-002 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/usdl |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS Unified Space Data Link Protocol. Full pipeline: PhysicalChannel (MC mux/demux) → MasterChannel (VC mux, gap detection via the VCF Count) → VirtualChannel (frame buffer) → MAP services (MAPP packets under rules '000'/'111', MAPA SDUs under rules '001'/'010'/'111', MAPO octet stream under rule '011'). Non-truncated and truncated (annex D) headers. Partial fixed-length packet zones completed with Encapsulation Idle Packets per §4.1.4.3.4. |

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
| Specification | CCSDS 732.1-B-2 (Unified Space Data Link Protocol, Blue Book, Issue 2) |
| Have any exceptions been required? | Yes [X] No [ ] — see notes: COP-1/COP-P frame acceptance procedures are out of scope (the Bypass/Sequence Control and Protocol Control Command flags are carried but no FARM/FOP runs on them); SDLS integration is out of scope; OID TFDZ fill uses the project-specified idle pattern rather than a continuous LFSR PN stream. |

---

## A2.2 TRANSFER FRAME FIELDS

| Feature | Reference | Status | Support |
|---|---|---|---|
| Transfer Frame Version Number | §4.1.2.2.2 | M | Y — TFVN = '1100' (12) |
| Spacecraft Identifier | §4.1.2.2.3 | M | Y — 16-bit SCID |
| Source-or-Destination Identifier | §4.1.2.3 | M | Y — 1-bit flag |
| Virtual Channel Identifier | §4.1.2.4 | M | Y — 6-bit VCID (0-63; 63 reserved for OID) |
| MAP Identifier | §4.1.2.5 | M | Y — 4-bit MAP ID (0-15) |
| End of Frame Primary Header Flag | §4.1.2.6 | M | Y — selects the truncated 4-octet header |
| Frame Length | §4.1.2.7 | M | Y — 16-bit, total octets − 1; cross-checked against the delivered buffer on decode |
| Bypass/Sequence Control Flag | §4.1.2.8.1 | M | Y — carried and round-tripped (COP procedures out of scope) |
| Protocol Control Command Flag | §4.1.2.8.2 | M | Y — carried and round-tripped |
| Reserve Spares | §4.1.2.9 | M | Y — encoded '00'; validated on decode |
| OCF Flag | §4.1.2.10 | M | Y — set from OCF presence; drives OCF extraction on decode |
| VCF Count Length | §4.1.2.11 | M | Y — 3-bit field, 0-7 octet counts |
| VCF Count | §4.1.2.12 | C | Y — 0-56 bit per-VC count, big-endian |
| Insert Zone | §4.1.3 | O | Y — configurable length via ChannelConfig |
| TFDF Header | §4.1.4.2 | M | Y — 1 octet (rules + UPID) plus 16-bit pointer only for rules '000'/'001'/'010' |
| TFDZ Construction Rules | §4.1.4.2.2 | M | Y — all eight values defined; services emit '000', '001', '010', '011', '111' |
| USLP Protocol Identifier | §4.1.4.2.3 | M | Y — SANA registry values (0-8, 31) provided as constants; set per service |
| First Header / Last Valid Octet Pointer | §4.1.4.2.4 | C | Y — FHP for rule '000', LVOP for '001'/'010'; 'all ones' specials |
| Only Idle Data frames | §4.1.4.1.5-1.9 | M | Y — VCID 63, MAP 0, rule '001', UPID 'Idle Data', LVOP = last TFDZ octet |
| OID TFDZ fill | §4.1.4.1.10 | M | N — project-specified repeating idle pattern (configurable) instead of the continuous 32-cell LFSR PN sequence |
| Idle fill of partial fixed TFDZs | §4.1.4.3.4 | M | Y — Encapsulation Idle Packet completes the TFDZ; discarded on extraction |
| Operational Control Field | §4.1.5 | O | Y — 4 octets, presence signaled by the OCF Flag |
| Frame Error Control Field (CRC-16) | §4.1.6, annex B1 | O | Y — CRC-16-CCITT with known-answer tests |
| Frame Error Control Field (CRC-32) | §4.1.6, annex B2 | O | Y — CCSDS/Proximity-1 CRC-32 (poly 0x00A00805, zero preset, no inversion) with known-answer tests. Not CRC-32C. |
| Truncated Transfer Frame | Annex D | O | Y — 4-octet header + 1-octet TFDF header (rule '111'); no insert zone, OCF, FECF, or pointer |

---

## A2.3 SERVICES

| Feature | Reference | Status | Support |
|---|---|---|---|
| MAP Packet Service (MAPP) | §3.4, §4.2.2 | O | Y — rule '000' with FHP on fixed-length channels (EPP idle fill, FHP resync after loss); rule '111' on variable-length channels |
| MAP Access Service (MAPA) | §3.4, §4.2.3 | O | Y — constant-length MAPA_SDUs under rules '001'/'010' with LVOP delimiting; rule '111' on variable-length channels |
| MAP Octet Stream Service (MAPO) | §3.4, §4.2.4 | O | Y — rule '011'; variable-length frames only (fixed-length rejected per §4.2.4.1) |
| MAP Multiplexing / Demultiplexing | §2.2.1, §3.2.4 | M | Y — services filter Receive by MAP ID; frames of other MAPs on the shared VC are not delivered to the wrong service |
| Idle frame generation | §4.1.4.1 | M | Y — GetNextFrameOrIdle on fixed-length channels; OID frames keep their own VC 63 count |
| COP-1 / COP-P procedures | refs [9], [10] | O | N — flags carried; FOP/FARM not implemented |
| SDLS | ref [15] | O | N |

---

## A2.4 CHANNEL MANAGEMENT

| Feature | Reference | Status | Support |
|---|---|---|---|
| Physical Channel | §2.1.3 | M | Y — PhysicalChannel with MC multiplexing |
| Master Channel | §2.1.3 | M | Y — MasterChannel with VC multiplexing |
| Virtual Channel | §2.1.3 | M | Y — VirtualChannel with frame buffering |
| MAP Channel | §2.1.3 | M | Y — per-MAP service instances with Receive filtering |
| Frame Gap Detection | §4.1.2.12 | M | Y — per-VC tracking of the VCF Count at the managed field width |
| VC Multiplexing | §4.2.6 | M | Y — weighted round-robin via SDL |
| MC Multiplexing | §4.2.8 | M | Y — weighted round-robin via SDL |

---

**Legend**: M = Mandatory, O = Optional, C = Conditional, Y = Yes (supported), N = No (not supported)
