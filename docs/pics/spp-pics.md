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
| Other Information | Go library implementing CCSDS Space Packet Protocol encoding, decoding, validation, and service-layer I/O for both Packet Service and Octet String Service |

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

NOTE — Non-supported optional capabilities and remaining partial conformances are
identified in section A2.2 with explanations.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: SPP Service Data Units

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-1 | Space Packet SDU | 3.2.2 | M | Yes | `SpacePacket` struct models the SDU. `Service` provides a formal service-layer abstraction via `SendPacket()` / `ReceivePacket()` with explicit service parameters configured through `ServiceConfig`. |
| SPP-2 | Octet String SDU | 3.2.3 | M | Yes | `Service.SendBytes()` accepts raw octet strings with service parameters (APID, optional secondary header). `Service.ReceiveBytes()` delivers the octet string and APID, stripping packet structure. |

### Table A-2: Service Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| SPP-3 | APID | 3.3.2.2 | M | 0–2047 | Yes | `PrimaryHeader.APID` (11-bit). Validated in `PrimaryHeader.Validate()`. Thread-safe allocation via `APIDManager`. |
| SPP-4 | Packet Loss Indicator | 3.3.2.3 | O | — | No | Not implemented. No mechanism to detect or report packet loss via sequence count gaps. |
| SPP-5 | QoS Requirement | 3.3.2.4 | O | — | No | Not implemented. |
| SPP-6 | Octet String | 3.4.2.1 | M | — | Yes | The `data` parameter in `Service.SendBytes(apid, data, opts...)` is the octet string service parameter. |
| SPP-7 | APID (Octet String Service) | 3.4.2.2 | M | 0–2047 | Yes | The `apid` parameter in `Service.SendBytes(apid, data, opts...)`. Validated via `NewSpacePacket()`. Returned by `Service.ReceiveBytes()`. |
| SPP-8 | Secondary Header Indicator (Octet String Service) | 3.4.2.3 | M | 0 or 1 | Yes | Exposed via `WithSendSecondaryHeader()` send option. When provided, the secondary header flag is set and the header is included in the constructed packet. |
| SPP-9 | Data Loss Indicator | 3.4.2.4 | O | — | No | Not implemented. |

### Table A-3: Service Primitives

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-10 | Packet.request | 3.3.3.2 | M | Yes | `Service.SendPacket(packet)` implements the Packet.request primitive. Accepts a pre-built `*SpacePacket`, encodes it, enforces the configurable maximum packet length, and writes to the transport. |
| SPP-11 | Packet.indication | 3.3.3.3 | M | Yes | `Service.ReceivePacket()` implements the Packet.indication primitive. Reads the primary header, calculates total packet size, reads remaining octets, and decodes into a `*SpacePacket`. Uses the optional `SecondaryHeader` decoder from `ServiceConfig` if the flag is set. |
| SPP-12 | Octet_String.request | 3.4.3.2 | M | Yes | `Service.SendBytes(apid, data, opts...)` implements the OctetString.request primitive. Accepts raw octet string and service parameters, constructs a space packet internally via `NewSpacePacket()`, and sends it via `SendPacket()`. |
| SPP-13 | Octet_String.indication | 3.4.3.3 | M | Yes | `Service.ReceiveBytes()` implements the OctetString.indication primitive. Reads a space packet via `ReceivePacket()` and returns the APID and user data, stripping away the packet structure. |

### Table A-4: SPP Protocol Data Unit

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-14 | Space Packet | 4.1 | M | Yes | `SpacePacket` struct with `Encode()` / `Decode()` round-trip support. `Decode(data, sh...)` accepts an optional `SecondaryHeader` interface implementation; if provided and the secondary header flag is set, it is used to decode mission-specific header bytes. Otherwise, secondary header bytes are included in `UserData`. |
| SPP-15 | Packet Primary Header | 4.1.3 | M | Yes | `PrimaryHeader` — 6 octets. All fields implemented per CCSDS: Version Number (3 bits, enforced as 0 for CCSDS v1 via `ErrInvalidVersion`), Packet Type (1 bit, `PacketTypeTM`=0 / `PacketTypeTC`=1), Secondary Header Flag (1 bit), APID (11 bits), Sequence Flags (2 bits, named constants `SeqFlagContinuation`/`SeqFlagFirstSegment`/`SeqFlagLastSegment`/`SeqFlagUnsegmented`), Sequence Count (14 bits), Packet Data Length (16 bits). Big-endian encoding. |
| SPP-16 | Packet Data Field | 4.1.4 | M | Yes | Composed of optional Secondary Header + User Data + optional Error Control. Length calculation follows CCSDS formula: `Packet Data Length = (data field octets) − 1`. |
| SPP-17 | Packet Secondary Header | 4.1.4.2 | C1 | Yes | `SecondaryHeader` is an interface (`Encode()`, `Decode()`, `Size()`) allowing mission-specific implementations. Configurable via `WithSecondaryHeader()` option. CCSDS size constraint (1–63 octets) is enforced by `validateSecondaryHeader()` with `ErrSecondaryHeaderTooSmall` and `ErrSecondaryHeaderTooLarge`. C1 enforced: `NewSpacePacket()` allows nil/empty user data when a secondary header is provided. |
| SPP-18 | User Data Field | 4.1.4.3 | C2 | Yes | `UserData []byte` field. C2 enforced: `NewSpacePacket()` requires user data only when no secondary header is present. When a secondary header is provided, user data may be nil or empty. |

**C1:** It is mandatory for a Space Packet to contain a Packet Secondary Header if
no User Data Field is present; otherwise, it is optional.

**C2:** It is mandatory for a Space Packet to contain a User Data Field if the Packet
Secondary Header is not present; otherwise, it is optional.

### Table A-5: Protocol Procedures

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-19 | Packet Assembly Function | 4.2.2 | M | Yes | `NewSpacePacket()` constructs the packet. `Encode()` serializes Primary Header + Secondary Header + User Data + Error Control into an octet stream. Packet Data Length is computed automatically. |
| SPP-20 | Packet Transfer Function | 4.2.3 | M | Yes | `Service.SendPacket()` writes a single encoded packet to the transport via `io.ReadWriter`. Multiplexing of packets from multiple APIDs is delegated to the caller, which controls the order and scheduling of `SendPacket()` calls. The multiplexing scheme itself is an optional management parameter (SPP-25). |
| SPP-21 | Packet Extraction Function | 4.3.2 | M | Yes | `Service.ReceivePacket()` reads the 6-octet Primary Header, computes total packet size via `CalculatePacketSize()`, reads the remaining octets, and invokes `Decode()`. |
| SPP-22 | Packet Reception Function | 4.3.3 | M | Yes | `Decode()` parses raw octets into a `SpacePacket` and automatically validates the result via `Validate()`. Sequence count continuity checking for packet loss reporting is an optional capability (SPP-4). |

### Table A-6: Management Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| SPP-23 | Maximum Packet Length (octets) | Table 5-1 | M | Integer | Yes | Configurable via `ServiceConfig.MaxPacketLength`. Defaults to 65542 octets. Enforced in `Service.SendPacket()`. Minimum packet size of 7 octets enforced in `NewSpacePacket()`. |
| SPP-24 | Packet Type of Outgoing Packets | Table 5-1 | M | 0 or 1 | Yes | Configurable via `ServiceConfig.PacketType`. Selectable via named constants `PacketTypeTM` (0) and `PacketTypeTC` (1). Validated by `ErrInvalidType`. Convenience constructors `NewTMPacket()` and `NewTCPacket()` also available. |
| SPP-25 | Packet Multiplexing Scheme | Table 5-1 | O | Mission specific | No | Not implemented. No multiplexing, scheduling, or interleaving logic. |
| SPP-26 | Service Type | Table 5-1 | M | Packet Service or Octet String Service | Yes | Both service types are available via `Service`. Packet Service: `SendPacket()` / `ReceivePacket()`. Octet String Service: `SendBytes()` / `ReceiveBytes()`. |

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Partial | Not Supported |
|----------|-------------|-----------|---------|---------------|
| Mandatory (M) | 20 | 20 | 0 | 0 |
| Optional (O) | 4 | 0 | 0 | 4 |
| Conditional (C) | 2 | 2 | 0 | 0 |
| **Total** | **26** | **22** | **0** | **4** |

### Non-Conformances (Optional Items Not Supported)

| Item | Description | Reason |
|------|-------------|--------|
| SPP-4 | Packet Loss Indicator | No sequence count gap detection or loss reporting mechanism. |
| SPP-5 | QoS Requirement | Quality of Service parameter not implemented. |
| SPP-9 | Data Loss Indicator | No data loss detection for Octet String Service. |
| SPP-25 | Packet Multiplexing Scheme | No multiplexing, scheduling, or interleaving logic. |

### Partial Conformances (Items Requiring Attention)

None. All mandatory and conditional items are fully supported.

### Fully Supported Items

| Item | Description | Implementation |
|------|-------------|----------------|
| SPP-1 | Space Packet SDU | `SpacePacket` struct with `Service.SendPacket()` / `Service.ReceivePacket()` service-layer abstraction. |
| SPP-2 | Octet String SDU | `Service.SendBytes()` / `Service.ReceiveBytes()` for raw octet string I/O. |
| SPP-3 | APID | `PrimaryHeader.APID` with validation and `APIDManager` for allocation. |
| SPP-6 | Octet String | `data` parameter in `Service.SendBytes()`. |
| SPP-7 | APID (Octet String) | `apid` parameter in `Service.SendBytes()` / return value of `Service.ReceiveBytes()`. |
| SPP-8 | Secondary Header Indicator (Octet String) | `WithSendSecondaryHeader()` send option. |
| SPP-10 | Packet.request | `Service.SendPacket()` with max packet length enforcement. |
| SPP-11 | Packet.indication | `Service.ReceivePacket()` with optional `SecondaryHeader` decoding. |
| SPP-12 | Octet_String.request | `Service.SendBytes()` constructs packet from raw bytes and service parameters. |
| SPP-13 | Octet_String.indication | `Service.ReceiveBytes()` delivers APID and user data. |
| SPP-14 | Space Packet | `SpacePacket` struct with encode/decode round-trip. `Decode()` accepts optional `SecondaryHeader` interface for mission-specific decoding. |
| SPP-17 | Packet Secondary Header (C1) | `SecondaryHeader` interface with `WithSecondaryHeader()` option. Packets with secondary header only (no user data) are valid. |
| SPP-18 | User Data Field (C2) | User data required only when no secondary header is present; optional otherwise. |
| SPP-15 | Packet Primary Header | Complete 6-octet header with all CCSDS fields. Version enforced as 0 (CCSDS v1). Named constants for packet types and sequence flags. |
| SPP-16 | Packet Data Field | Correct composition and length calculation. |
| SPP-19 | Packet Assembly Function | Full assembly via `NewSpacePacket()` + `Encode()`. |
| SPP-20 | Packet Transfer Function | `Service.SendPacket()` writes to transport. Multiplexing delegated to caller. |
| SPP-21 | Packet Extraction Function | Full extraction via `Service.ReceivePacket()` + `Decode()`. |
| SPP-22 | Packet Reception Function | `Decode()` parses and auto-validates. Sequence gap detection is optional (SPP-4). |
| SPP-23 | Maximum Packet Length | Configurable via `ServiceConfig.MaxPacketLength`, default 65542. |
| SPP-24 | Packet Type | Configurable via `ServiceConfig.PacketType`. TM (0) / TC (1) via named constants with convenience constructors. |
| SPP-26 | Service Type | Both Packet Service and Octet String Service available via `Service`. |
