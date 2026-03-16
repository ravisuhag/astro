# PICS PROFORMA FOR SPACE PACKET PROTOCOL

## Conformance Statement for `pkg/spp` — CCSDS 133.0-B-2

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 16/03/2026 |
| PICS Serial Number | ASTRO-SPP-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/spp |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS Space Packet Protocol encoding, decoding, validation, and I/O |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/spp (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 133.0-B-2 (Space Packet Protocol, Blue Book, Issue 2, June 2020) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE — The implementation does not support the Octet String Service defined in
section 3.4 of the Recommended Standard. Additionally, conditional requirements
C1 and C2 for the Packet Data Field are not fully enforced (see SPP-17, SPP-18
below). Non-supported mandatory capabilities are identified in section A2.2 with
explanations.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: SPP Service Data Units

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-1 | Space Packet SDU | 3.2.2 | M | Partial | `SpacePacket` struct models the SDU. No formal service-layer abstraction that accepts/delivers SDUs with explicit service parameters. Packets are created directly via `NewSpacePacket()`. |
| SPP-2 | Octet String SDU | 3.2.3 | M | No | Octet String Service is not implemented. No segmentation/reassembly of arbitrarily-sized octet strings into space packets. |

### Table A-2: Service Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| SPP-3 | APID | 3.3.2.2 | M | 0–2047 | Yes | `PrimaryHeader.APID` (11-bit). Validated in `PrimaryHeader.Validate()`. Thread-safe allocation via `APIDManager`. |
| SPP-4 | Packet Loss Indicator | 3.3.2.3 | O | — | No | Not implemented. No mechanism to detect or report packet loss via sequence count gaps. |
| SPP-5 | QoS Requirement | 3.3.2.4 | O | — | No | Not implemented. |
| SPP-6 | Octet String | 3.4.2.1 | M | — | No | Octet String Service not implemented. |
| SPP-7 | APID (Octet String Service) | 3.4.2.2 | M | 0–2047 | No | Octet String Service not implemented. |
| SPP-8 | Secondary Header Indicator (Octet String Service) | 3.4.2.3 | M | 0 or 1 | No | The flag exists in `PrimaryHeader.SecondaryHeaderFlag` but is not exposed as an Octet String Service parameter. |
| SPP-9 | Data Loss Indicator | 3.4.2.4 | O | — | No | Not implemented. |

### Table A-3: Service Primitives

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-10 | Packet.request | 3.3.3.2 | M | Partial | `WritePacket(packet, writer)` in `service.go` serves as the send primitive. Does not model the formal `Packet.request` interface with explicit service parameters (APID, QoS, packet loss indicator). |
| SPP-11 | Packet.indication | 3.3.3.3 | M | Partial | `ReadPacket(reader, sh...)` in `service.go` serves as the receive primitive. Accepts an optional `SecondaryHeader` implementation for decoding mission-specific headers. Does not model the formal `Packet.indication` interface with loss indicators. |
| SPP-12 | Octet_String.request | 3.4.3.2 | M | No | Octet String Service not implemented. |
| SPP-13 | Octet_String.indication | 3.4.3.3 | M | No | Octet String Service not implemented. |

### Table A-4: SPP Protocol Data Unit

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-14 | Space Packet | 4.1 | M | Yes | `SpacePacket` struct with `Encode()` / `Decode()` round-trip support. `Decode(data, sh...)` accepts an optional `SecondaryHeader` interface implementation; if provided and the secondary header flag is set, it is used to decode mission-specific header bytes. Otherwise, secondary header bytes are included in `UserData`. |
| SPP-15 | Packet Primary Header | 4.1.3 | M | Yes | `PrimaryHeader` — 6 octets. All fields implemented per CCSDS: Version Number (3 bits, enforced as 0 for CCSDS v1 via `ErrInvalidVersion`), Packet Type (1 bit, `PacketTypeTM`=0 / `PacketTypeTC`=1), Secondary Header Flag (1 bit), APID (11 bits), Sequence Flags (2 bits, named constants `SeqFlagContinuation`/`SeqFlagFirstSegment`/`SeqFlagLastSegment`/`SeqFlagUnsegmented`), Sequence Count (14 bits), Packet Data Length (16 bits). Big-endian encoding. |
| SPP-16 | Packet Data Field | 4.1.4 | M | Yes | Composed of optional Secondary Header + User Data + optional Error Control. Length calculation follows CCSDS formula: `Packet Data Length = (data field octets) − 1`. |
| SPP-17 | Packet Secondary Header | 4.1.4.2 | C1 | Partial | `SecondaryHeader` is an interface (`Encode()`, `Decode()`, `Size()`) allowing mission-specific implementations. Configurable via `WithSecondaryHeader()` option. CCSDS size constraint (1–63 octets) is enforced by `validateSecondaryHeader()` with `ErrSecondaryHeaderTooSmall` and `ErrSecondaryHeaderTooLarge`. **C1 violation:** The specification requires that a Packet Secondary Header be present if no User Data Field exists. The implementation enforces `len(data) >= 1` in `NewSpacePacket()`, making it impossible to create a valid packet with only a secondary header and no user data. |
| SPP-18 | User Data Field | 4.1.4.3 | C2 | Partial | `UserData []byte` field. **C2 violation:** The specification states that a User Data Field is mandatory only when no Packet Secondary Header is present; otherwise it is optional. The implementation always requires non-empty user data (`len(data) < 1` guard in `NewSpacePacket()`), preventing creation of packets with a secondary header but no user data. |

**C1:** It is mandatory for a Space Packet to contain a Packet Secondary Header if
no User Data Field is present; otherwise, it is optional.

**C2:** It is mandatory for a Space Packet to contain a User Data Field if the Packet
Secondary Header is not present; otherwise, it is optional.

### Table A-5: Protocol Procedures

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-19 | Packet Assembly Function | 4.2.2 | M | Yes | `NewSpacePacket()` constructs the packet. `Encode()` serializes Primary Header + Secondary Header + User Data + Error Control into an octet stream. Packet Data Length is computed automatically. |
| SPP-20 | Packet Transfer Function | 4.2.3 | M | Partial | `WritePacket()` writes a single encoded packet to an `io.Writer`. No multiplexing of packets from multiple APIDs is supported. No packet scheduling or prioritization. |
| SPP-21 | Packet Extraction Function | 4.3.2 | M | Yes | `ReadPacket()` reads the 6-octet Primary Header, computes total packet size via `CalculatePacketSize()`, reads the remaining octets, and invokes `Decode()`. |
| SPP-22 | Packet Reception Function | 4.3.3 | M | Partial | `Decode()` parses raw octets into a `SpacePacket`. Note: `Decode()` does not call `Validate()` on the resulting packet — callers must validate explicitly if needed. No sequence count continuity checking or gap detection for packet loss reporting. |

### Table A-6: Management Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| SPP-23 | Maximum Packet Length (octets) | Table 5-1 | M | Integer | Partial | Enforced as 65542 octets maximum and 7 octets minimum (hardcoded in `packet.go:94`). Not exposed as a configurable management parameter — missions cannot set a lower maximum. |
| SPP-24 | Packet Type of Outgoing Packets | Table 5-1 | M | 0 or 1 | Yes | Selectable via named constants `PacketTypeTM` (0) and `PacketTypeTC` (1). Validated by `ErrInvalidType`. Convenience constructors `NewTMPacket()` and `NewTCPacket()` provided. |
| SPP-25 | Packet Multiplexing Scheme | Table 5-1 | O | Mission specific | No | Not implemented. No multiplexing, scheduling, or interleaving logic. |
| SPP-26 | Service Type | Table 5-1 | M | Packet Service or Octet String Service | No | No per-APID service type configuration. All APIDs are implicitly treated as Packet Service. Octet String Service is not available. |

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Partial | Not Supported |
|----------|-------------|-----------|---------|---------------|
| Mandatory (M) | 20 | 5 | 6 | 9 |
| Optional (O) | 4 | 0 | 0 | 4 |
| Conditional (C) | 2 | 0 | 2 | 0 |
| **Total** | **26** | **5** | **8** | **13** |

### Non-Conformances (Mandatory Items Not Supported)

| Item | Description | Reason |
|------|-------------|--------|
| SPP-2 | Octet String SDU | Octet String Service not in scope for current implementation. Only Packet Service is provided. |
| SPP-6 | Octet String parameter | Depends on Octet String Service. |
| SPP-7 | APID (Octet String) | Depends on Octet String Service. |
| SPP-8 | Secondary Header Indicator (Octet String) | Depends on Octet String Service. |
| SPP-12 | Octet_String.request | Depends on Octet String Service. |
| SPP-13 | Octet_String.indication | Depends on Octet String Service. |
| SPP-26 | Service Type | No per-APID service type selection; Octet String Service not available. |

### Partial Conformances (Items Requiring Attention)

| Item | Description | Gap |
|------|-------------|-----|
| SPP-1 | Space Packet SDU | Missing formal service-layer abstraction with explicit service parameters. |
| SPP-10 | Packet.request | `WritePacket()` lacks formal service parameter interface. |
| SPP-11 | Packet.indication | `ReadPacket()` lacks formal loss indicator reporting. Accepts optional `SecondaryHeader` decoder. |
| SPP-17 | Packet Secondary Header (C1) | Cannot create packet with secondary header only and no user data. SecondaryHeader is now an interface with 1–63 octet size validation. |
| SPP-18 | User Data Field (C2) | User data is always required, even when secondary header is present. |
| SPP-20 | Packet Transfer Function | No multiplexing support. |
| SPP-22 | Packet Reception Function | `Decode()` does not auto-validate; no sequence count gap detection. |
| SPP-23 | Maximum Packet Length | Hardcoded; not configurable as a management parameter. |

### Fully Supported Items

| Item | Description | Implementation |
|------|-------------|----------------|
| SPP-3 | APID | `PrimaryHeader.APID` with validation and `APIDManager` for allocation. |
| SPP-14 | Space Packet | `SpacePacket` struct with encode/decode round-trip. `Decode()` accepts optional `SecondaryHeader` interface for mission-specific decoding. |
| SPP-15 | Packet Primary Header | Complete 6-octet header with all CCSDS fields. Version enforced as 0 (CCSDS v1). Named constants for packet types and sequence flags. |
| SPP-16 | Packet Data Field | Correct composition and length calculation. |
| SPP-19 | Packet Assembly Function | Full assembly via `NewSpacePacket()` + `Encode()`. |
| SPP-21 | Packet Extraction Function | Full extraction via `ReadPacket()` + `Decode()`. |
| SPP-24 | Packet Type | TM (0) / TC (1) via named constants `PacketTypeTM`/`PacketTypeTC` with convenience constructors. |
