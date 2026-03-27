# PICS PROFORMA FOR ENCAPSULATION PACKET PROTOCOL

## Conformance Statement for `pkg/epp` — CCSDS 133.1-B-3

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 27/03/2026 |
| PICS Serial Number | ASTRO-EPP-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/epp |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS Encapsulation Packet Protocol encoding, decoding, validation, and service-layer I/O for all five header formats |

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
| Specification | CCSDS 133.1-B-3 (Encapsulation Packet Protocol, Blue Book, Issue 3, October 2014) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE — Non-supported optional capabilities are identified in section A2.2 with explanations.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: Encapsulation Packet Structure

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| EPP-1 | Encapsulation Packet | 4.1 | M | Yes | `EncapsulationPacket` struct with `Header` and `Data` fields. `Encode()` / `Decode()` round-trip support. |
| EPP-2 | Packet Version Number | 4.1.2.2 | M | Yes | `Header.PVN` — 4 bits, enforced as 7 (0111) via `ErrInvalidPVN`. Distinguishes from Space Packets (PVN = 0). |
| EPP-3 | Protocol ID | 4.1.2.3 | M | Yes | `Header.ProtocolID` — 3 bits (0–7). Named constants: `ProtocolIDIdle` (0), `ProtocolIDIPE` (2), `ProtocolIDUserDef` (6), `ProtocolIDExtended` (7). Validated in `Header.Validate()`. |
| EPP-4 | Length of Length | 4.1.2.4 | M | Yes | `Header.LengthOfLength` — 1 bit. Determines header format together with Protocol ID. Configurable via `WithLongLength()` option. |
| EPP-5 | Variable-Length Header | 4.1.2 | M | Yes | Five header formats (1, 2, 4, 4, 8 bytes) determined by Protocol ID and Length of Length. `Header.Format()` returns 1–5, `Header.Size()` returns byte count. |
| EPP-6 | Packet Length Field | 4.1.2.5 | M | Yes | `Header.PacketLength` — total packet length in octets (including header). 8-bit (Format 2), 16-bit (Formats 3, 4), or 32-bit (Format 5). Auto-computed in `NewPacket()`. |
| EPP-7 | Data Zone | 4.1.3 | M | Yes | `Data []byte` field. Contains the encapsulated protocol data unit. Required for non-idle packets (`ErrEmptyData`). Empty for idle packets (`ErrIdleWithData`). |

### Table A-2: Header Formats

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| EPP-8 | Format 1 — Idle (1 byte) | 4.1.2 | M | Yes | PID=0, LoL=0. `NewIdlePacket()` constructor. `IsIdle()` detection. No data zone. |
| EPP-9 | Format 2 — Short (2 bytes) | 4.1.2 | M | Yes | PID=1–6, LoL=0. 8-bit packet length. Max total packet size 255 bytes. `NewIPEPacket()` and `NewUserDefinedPacket()` default to this format. |
| EPP-10 | Format 3 — Medium (4 bytes) | 4.1.2 | M | Yes | PID=1–6, LoL=1. Includes `UserDefined` field (8 bits) and 16-bit packet length. Max 65,535 bytes. Selectable via `WithLongLength()` or `WithUserDefined()`. |
| EPP-11 | Format 4 — Extended Medium (4 bytes) | 4.1.2 | M | Yes | PID=7, LoL=0. `ExtendedProtocolID` (8 bits) and 16-bit packet length. Selectable via `WithExtendedProtocolID()`. |
| EPP-12 | Format 5 — Extended Long (8 bytes) | 4.1.2 | M | Yes | PID=7, LoL=1. `ExtendedProtocolID` (8 bits), `CCSDSDefined` (16 bits), and 32-bit packet length. Max 4,294,967,295 bytes. Selectable via `WithCCSDSDefined()`. |

### Table A-3: Protocol ID Values

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| EPP-13 | Idle Packet (PID=0) | 4.1.2.3 | M | Yes | `ProtocolIDIdle` constant. `NewIdlePacket()` constructor. `IsIdle()` method for detection. |
| EPP-14 | Internet Protocol Extension (PID=2) | 4.1.2.3 | M | Yes | `ProtocolIDIPE` constant. `NewIPEPacket()` convenience constructor. Carries IPv4 or IPv6 datagrams. |
| EPP-15 | User-Defined Protocol (PID=6) | 4.1.2.3 | M | Yes | `ProtocolIDUserDef` constant. `NewUserDefinedPacket()` convenience constructor. |
| EPP-16 | Protocol ID Extension (PID=7) | 4.1.2.3 | M | Yes | `ProtocolIDExtended` constant. Extended Protocol ID in second header byte. Configurable via `WithExtendedProtocolID()` and `WithCCSDSDefined()`. |
| EPP-17 | Reserved Protocol IDs (1, 3, 4, 5) | 4.1.2.3 | M | Partial | Constants not defined for reserved values. Packets with reserved PIDs can be constructed via `NewPacket()` but are identified as reserved in `Humanize()`. |

### Table A-4: Service Interface

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| EPP-18 | Packet Send | — | M | Yes | `Service.SendPacket(packet)` encodes and writes to transport. Enforces configurable maximum packet length. |
| EPP-19 | Packet Receive | — | M | Yes | `Service.ReceivePacket()` reads header to determine format, reads remaining bytes, decodes into `*EncapsulationPacket`. |
| EPP-20 | Byte Send | — | M | Yes | `Service.SendBytes(protocolID, data, opts...)` constructs packet from raw bytes and sends via `SendPacket()`. |
| EPP-21 | Byte Receive | — | M | Yes | `Service.ReceiveBytes()` reads packet and returns Protocol ID and data zone. |
| EPP-22 | Packet Sizing | — | M | Yes | `PacketSizer(data)` implements `sdl.PacketSizer` signature. Returns total packet length from header bytes, or -1 if too short. Compatible with `tmdl` and `tcdl` VCP services. |

### Table A-5: Management Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| EPP-23 | Maximum Packet Length | — | M | Integer | Yes | Configurable via `ServiceConfig.MaxPacketLength`. Defaults to 65,535 octets. Enforced in `Service.SendPacket()` and `Service.ReceivePacket()`. |
| EPP-24 | Packet Multiplexing | — | O | Mission specific | No | No multiplexing or scheduling logic. Caller controls ordering of `SendPacket()` calls. |

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Partial | Not Supported |
|----------|-------------|-----------|---------|---------------|
| Mandatory (M) | 23 | 22 | 1 | 0 |
| Optional (O) | 1 | 0 | 0 | 1 |
| **Total** | **24** | **22** | **1** | **1** |

### Non-Conformances (Optional Items Not Supported)

| Item | Description | Reason |
|------|-------------|--------|
| EPP-24 | Packet Multiplexing | No multiplexing, scheduling, or interleaving logic. |

### Partial Conformances (Items Requiring Attention)

| Item | Description | Reason |
|------|-------------|--------|
| EPP-17 | Reserved Protocol IDs | Packets with reserved PIDs (1, 3, 4, 5) can be constructed and encoded but no named constants or dedicated constructors are provided. |

### Fully Supported Items

| Item | Description | Implementation |
|------|-------------|----------------|
| EPP-1 | Encapsulation Packet | `EncapsulationPacket` struct with encode/decode round-trip. |
| EPP-2 | Packet Version Number | `Header.PVN` enforced as 7 via `ErrInvalidPVN`. |
| EPP-3 | Protocol ID | `Header.ProtocolID` with named constants and validation. |
| EPP-4 | Length of Length | `Header.LengthOfLength` with `WithLongLength()` option. |
| EPP-5 | Variable-Length Header | All five formats (1, 2, 4, 4, 8 bytes) with `Header.Format()` and `Header.Size()`. |
| EPP-6 | Packet Length Field | Auto-computed in `NewPacket()`. 8/16/32-bit depending on format. |
| EPP-7 | Data Zone | `Data []byte` with idle/non-idle validation. |
| EPP-8 | Format 1 — Idle | `NewIdlePacket()` and `IsIdle()`. |
| EPP-9 | Format 2 — Short | Default format for standard Protocol IDs. |
| EPP-10 | Format 3 — Medium | `WithLongLength()` and `WithUserDefined()`. |
| EPP-11 | Format 4 — Extended Medium | `WithExtendedProtocolID()`. |
| EPP-12 | Format 5 — Extended Long | `WithCCSDSDefined()`. |
| EPP-13 | Idle Packet | `ProtocolIDIdle`, `NewIdlePacket()`, `IsIdle()`. |
| EPP-14 | Internet Protocol Extension | `ProtocolIDIPE`, `NewIPEPacket()`. |
| EPP-15 | User-Defined Protocol | `ProtocolIDUserDef`, `NewUserDefinedPacket()`. |
| EPP-16 | Protocol ID Extension | `ProtocolIDExtended`, `WithExtendedProtocolID()`, `WithCCSDSDefined()`. |
| EPP-18 | Packet Send | `Service.SendPacket()` with max length enforcement. |
| EPP-19 | Packet Receive | `Service.ReceivePacket()` with format-aware streaming read. |
| EPP-20 | Byte Send | `Service.SendBytes()` with packet options. |
| EPP-21 | Byte Receive | `Service.ReceiveBytes()` returns Protocol ID and data. |
| EPP-22 | Packet Sizing | `PacketSizer()` compatible with `sdl.PacketSizer`. |
| EPP-23 | Maximum Packet Length | Configurable via `ServiceConfig.MaxPacketLength`, default 65,535. |
