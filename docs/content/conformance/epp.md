---
title: Encapsulation Packet Protocol
description: "PICS proforma: what this package implements, clause by clause."
order: 20
---

## Conformance Statement for `pkg/epp` — CCSDS 133.1-B-3

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 25/08/2026 |
| PICS Serial Number | ASTRO-EPP-PICS-002 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/epp |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS Encapsulation Packet Protocol encoding, decoding, validation, and service-layer I/O for all four header sizes. Wire layout pinned by spec-derived golden vectors (e.g. 1-octet idle packet = 0xE0). |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/epp (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 133.1-B-3 (Encapsulation Packet Protocol, Blue Book, Issue 3, May 2020) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE — Non-supported optional capabilities are identified in section A2.2 with explanations.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: Encapsulation Packet Structure

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| EPP-1 | Encapsulation Packet | 4.1.1 | M | Yes | `EncapsulationPacket` struct with `Header` and `Data` fields. `Encode()` / `Decode()` with golden wire-vector tests in addition to round-trips. |
| EPP-2 | Packet Version Number | 4.1.2.2 | M | Yes | Bits 0–2 of octet 0, enforced as '111' (7) via `ErrInvalidPVN`. Distinguishes from Space Packets (PVN '000'). |
| EPP-3 | Encapsulation Protocol ID | 4.1.2.3 | M | Yes | Bits 3–5 of octet 0. Named constants per the SANA registry: `ProtocolIDIdle` (0), `ProtocolIDLTP` (1), `ProtocolIDIPE` (2), `ProtocolIDExtended` (6), `ProtocolIDMission` (7). Validated in `Header.Validate()`. |
| EPP-4 | Length of Length | 4.1.2.4 | M | Yes | Bits 6–7 of octet 0 (2 bits). Header size derived from this field alone per table 4-1: '00'→1, '01'→2, '10'→4, '11'→8 octets (`Header.Size()`). 4.1.2.4.4 enforced: LoL '00' requires Protocol ID '000' (`ErrNonIdleOneOctetHeader`). |
| EPP-5 | User Defined Field | 4.1.2.5 | M | Yes | 4-bit field in octet 1 of 4- and 8-octet headers (`Header.UserDefined`, set via `WithUserDefined()`). Rejected when it cannot be encoded (`ErrFieldNeedsLongerHeader`). |
| EPP-6 | Encapsulation Protocol ID Extension | 4.1.2.6 | M | Yes | 4-bit field sharing octet 1 with the User Defined Field. Used for protocol identification when the Protocol ID is '110' (4.1.2.6.2); enforced as 'all zeros' otherwise (4.1.2.6.3, `ErrExtensionMustBeZero`). Protocol ID '110' requires a 4- or 8-octet header (`ErrExtendedNeedsLongHeader`). |
| EPP-7 | CCSDS Defined Field | 4.1.2.7 | M | Yes | 2-octet field, 8-octet header only (`Header.CCSDSDefined`, set via `WithCCSDSDefined()`). Reserved; 'all zeros' by convention (not enforced on receive, per 4.1.2.7.2 being a convention). |
| EPP-8 | Packet Length Field | 4.1.2.8 | M | Yes | Total packet length in octets, header included (4.1.2.8.2). 1, 2, or 4 octets per the Length of Length field; absent in the 1-octet header. Auto-computed in `NewPacket()`; bounds checked against table 4-2 in `Header.Validate()`. |
| EPP-9 | Encapsulated Data Field | 4.1.3 | M | Yes | `Data []byte`. Absent-data conditions enforced per 4.1.3.1.4/4.1.3.1.5: a packet without a data field must carry Protocol ID '000' (`ErrEmptyData` otherwise). |

### Table A-2: Header Sizes (figure 4-2 / table 4-1)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| EPP-10 | 1-octet header (LoL '00') | 4.1.2 | M | Yes | Idle packets only; encodes as 0xE0. `NewIdlePacket()` constructor; golden vector test. |
| EPP-11 | 2-octet header (LoL '01') | 4.1.2 | M | Yes | 1-octet Packet Length, max total 255. Default for small payloads. |
| EPP-12 | 4-octet header (LoL '10') | 4.1.2 | M | Yes | UDF + PIE octet, 2-octet Packet Length, max total 65,535. Selected automatically for larger payloads or via `WithLongLength()` / `WithUserDefined()` / `WithExtendedProtocolID()`. |
| EPP-13 | 8-octet header (LoL '11') | 4.1.2 | M | Yes | UDF + PIE octet, CCSDS Defined Field, 4-octet Packet Length, max total 4,294,967,295. Selected automatically or via `WithCCSDSDefined()`. |
| EPP-14 | Receive fixed- or variable-length headers | 4.1.2.1.2 | M | Yes | `Decode()` and `Service.ReceivePacket()` accept any of the four header sizes; the sender adapts the header size to the payload per the 4.1.2.1.2 NOTE. |

### Table A-3: Protocol ID Values (SANA registry)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| EPP-15 | Idle Packet (PID 0) | 4.1.2.3 NOTE 1 | M | Yes | `NewIdlePacket()` (1-octet form) and `NewIdleFillPacket(totalLength, fill)` for multi-octet idle fill packets used to fill fixed-length transfer frames. `IsIdle()` detection. |
| EPP-16 | LTP (PID 1) | SANA registry | O | Yes | `ProtocolIDLTP` constant; `NewLTPPacket()` constructor. |
| EPP-17 | Internet Protocol Extension (PID 2) | SANA registry | O | Yes | `ProtocolIDIPE` constant; `NewIPEPacket()` constructor. |
| EPP-18 | Protocol ID Extension (PID 6, '110') | 4.1.2.3 NOTE 2 | M | Yes | `ProtocolIDExtended` constant; `WithExtendedProtocolID()` sets the 4-bit extension. Extension values are carried opaquely; the SANA extended-protocol registry is not modeled. |
| EPP-19 | Mission-specific data (PID 7, '111') | 4.1.2.3 NOTE 3 | O | Yes | `ProtocolIDMission` constant; `NewMissionPacket()` constructor. |
| EPP-20 | Reserved Protocol IDs (3, 4, 5) | SANA registry | — | Partial | Packets with reserved PIDs can be constructed and decoded (identified as "Reserved" in `Humanize()`); no named constants. |

### Table A-4: Service Interface

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| EPP-21 | ENCAPSULATION.request | 3.3 | M | Yes | `Service.SendBytes(protocolID, data, opts...)` and `Service.SendPacket(packet)`. |
| EPP-22 | ENCAPSULATION.indication | 3.3 | M | Yes | `Service.ReceiveBytes()` and `Service.ReceivePacket()` with header-size-aware streaming reads. |
| EPP-23 | Packet Sizing | — | — | Yes | `PacketSizer(data)` implements `sdl.PacketSizer`: total length from the first octet's LoL field (1 for the idle byte). Compatible with `tmdl` and `tcdl` VCP services. |

### Table A-5: Management Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| EPP-24 | Maximum Packet Length | 5 | M | Integer | Yes | `ServiceConfig.MaxPacketLength`. Defaults to 4,294,967,295 (the protocol maximum) so no spec-valid packet is rejected unless a mission sets a lower limit. |
| EPP-25 | Packet Multiplexing | — | O | Mission specific | No | No multiplexing or scheduling logic. Caller controls ordering of `SendPacket()` calls. |

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Partial | Not Supported |
|----------|-------------|-----------|---------|---------------|
| Mandatory (M) | 17 | 17 | 0 | 0 |
| Optional (O) | 4 | 3 | 0 | 1 |
| Unclassified (—) | 2 | 1 | 1 | 0 |
| **Total** | **23** | **21** | **1** | **1** |

### Non-Conformances (Optional Items Not Supported)

| Item | Description | Reason |
|------|-------------|--------|
| EPP-25 | Packet Multiplexing | No multiplexing, scheduling, or interleaving logic. |

### Partial Conformances (Items Requiring Attention)

| Item | Description | Reason |
|------|-------------|--------|
| EPP-20 | Reserved Protocol IDs | Reserved PIDs (3, 4, 5) can be constructed and decoded, but no named constants are provided. |

### Verification Notes

- Octet 0 layout (PVN '111' | Protocol ID | 2-bit Length of Length), the
  pure-LoL header sizing, the octet-1 UDF/PIE split, and the
  total-length-including-header Packet Length semantics were verified against
  the fetched CCSDS 133.1-B-3 (May 2020) text and are pinned by golden wire
  vectors in `header_test.go` and `packet_test.go` (e.g. the 1-octet idle
  packet 0xE0).
- Protocol ID names follow the SANA Encapsulation Protocol ID registry
  (1 = LTP, 2 = IPE, 6 = extension, 7 = mission-specific).
