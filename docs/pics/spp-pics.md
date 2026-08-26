# PICS PROFORMA FOR SPACE PACKET PROTOCOL

## Conformance Statement for `pkg/spp` — CCSDS 133.0-B-2

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 25/08/2026 |
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
| Have any exceptions been required? | Yes [ ] No [X] |

NOTE — A YES answer would mean a mandatory capability is missing. Every mandatory
and conditional item is supported. The two optional items that are not implemented
are listed in section A2.3.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: SPP Service Data Units

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-1 | Space Packet SDU | 3.2.2 | M | Yes | `SpacePacket` struct models the SDU. `Service` provides a formal service-layer abstraction via `SendPacket()` / `ReceivePacket()` with explicit service parameters configured through `ServiceConfig`. |
| SPP-2 | Octet String SDU | 3.2.3 | M | Yes | `Service.SendBytes()` accepts raw octet strings with service parameters (APID, packet type, sequence count/packet name, optional secondary header). `Service.ReceiveBytes()` returns an `Indication` carrying the octet string, APID, Secondary Header Indicator, and Data Loss Indicator. |

### Table A-2: Service Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| SPP-3 | APID | 3.3.2.2 | M | 0–2047 | Yes | `PrimaryHeader.APID` (11-bit). Validated in `PrimaryHeader.Validate()`. |
| SPP-4 | Packet Loss Indicator | 3.3.2.3 | O | — | Yes | `Service.ReceivePacket()` runs the sequence count continuity check per APID; `Service.LastDataLoss()` reports how many packets were missing before the packet just received (0 when the count was continuous). `Service.ResetContinuity()` clears the state after a link outage. |
| SPP-5 | QoS Requirement | 3.3.2.4 | O | — | No | Not implemented. |
| SPP-6 | Octet String | 3.4.2.1 | M | — | Yes | The `data` parameter in `Service.SendBytes(apid, data, opts...)` is the octet string service parameter. |
| SPP-7 | APID (Octet String Service) | 3.4.2.2 | M | 0–2047 | Yes | The `apid` parameter in `Service.SendBytes(apid, data, opts...)`. Validated via `NewSpacePacket()`. Returned by `Service.ReceiveBytes()`. |
| SPP-8 | Secondary Header Indicator (Octet String Service) | 3.4.2.3 | M | 0 or 1 | Yes | Sending: `WithSendSecondaryHeader()` sets the flag and includes the header in the constructed packet. Receiving: `Indication.SecondaryHeaderIndicator`, read from the Secondary Header Flag of the received packet, tells the octet-string user whether a Packet Secondary Header leads the octet string (4.3.2.2). |
| SPP-9 | Data Loss Indicator | 3.4.2.4 | O | — | Yes | `Indication.DataLoss` and `Indication.PacketsLost` from `Service.ReceiveBytes()`, derived from the mandatory continuity check of 4.3.2.2. |

### Table A-3: Service Primitives

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-10 | Packet.request | 3.3.3.2 | M | Yes | `Service.SendPacket(packet)` implements the Packet.request primitive. Accepts a pre-built `*SpacePacket`, stamps the per-APID sequence count (4.1.3.4.3, mutating the caller's packet), encodes it, enforces the configurable maximum packet length, and writes to the transport. A count pinned with `WithSequenceCount()` is honored: the packet keeps that count and the service resynchronizes its counter to one past it, so the APID's count stays continuous modulo 16384 per 4.1.3.4.3.4. |
| SPP-11 | Packet.indication | 3.3.3.3 | M | Yes | `Service.ReceivePacket()` implements the Packet.indication primitive. Reads the primary header, calculates total packet size, reads remaining octets, and decodes into a `*SpacePacket`. `ServiceConfig.NewSecondaryHeader` supplies a fresh decoder per packet when the flag is set, so decoded headers are never shared between delivered packets. `ServiceConfig.DiscardIdle` drops received idle packets. |
| SPP-12 | Octet_String.request | 3.4.3.2 | M | Yes | `Service.SendBytes(apid, data, opts...)` implements the OctetString.request primitive. All parameters of 3.4.3.2.2 are available: octet string and APID as arguments, Secondary Header Indicator via `WithSendSecondaryHeader()`, Packet Type via `WithSendPacketType()` (defaulting to `ServiceConfig.PacketType`), and Packet Sequence Count/Packet Name via `WithSendSequenceCount()` / `WithSendPacketName()`. |
| SPP-13 | Octet_String.indication | 3.4.3.3 | M | Yes | `Service.ReceiveBytes()` implements the OctetString.indication primitive, returning an `Indication` with every parameter of 3.4.3.3.2: the octet string, the APID, the Secondary Header Indicator, and the optional Data Loss Indicator (`DataLoss` plus `PacketsLost`). |

### Table A-4: SPP Protocol Data Unit

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-14 | Space Packet | 4.1 | M | Yes | `SpacePacket` struct with `Encode()` / `Decode()` round-trip support. `Decode(data, opts...)` accepts functional options: `WithDecodeSecondaryHeader()` to decode mission-specific header bytes, and `WithDecodeErrorControl()` to extract and verify the trailing CRC-16-CCITT. If no secondary header decoder is provided, secondary header bytes are included in `UserData` and the packet re-encodes byte for byte, so a relay can forward it. The Secondary Header Flag and the `SecondaryHeader` field must agree (4.1.3.3.3.2): a header set with the flag clear returns `ErrSecondaryHeaderFlagClear`, and a hand-built packet with the flag set and no header returns `ErrSecondaryHeaderMissing`. |
| SPP-15 | Packet Primary Header | 4.1.3 | M | Yes | `PrimaryHeader` — 6 octets. All fields implemented per CCSDS: Version Number (3 bits, enforced as 0 for CCSDS v1 via `ErrInvalidVersion`), Packet Type (1 bit, `PacketTypeTM`=0 / `PacketTypeTC`=1), Secondary Header Flag (1 bit), APID (11 bits), Sequence Flags (2 bits, named constants `SeqFlagContinuation`/`SeqFlagFirstSegment`/`SeqFlagLastSegment`/`SeqFlagUnsegmented`, configurable via `WithSequenceFlags()`), Sequence Count (14 bits, auto-incremented per APID in `Service`, manually configurable via `WithSequenceCount()`), Packet Data Length (16 bits). Big-endian encoding. |
| SPP-16 | Packet Data Field | 4.1.4 | M | Yes | Composed of optional Secondary Header + User Data + optional Error Control. Length calculation follows CCSDS formula: `Packet Data Length = (data field octets) − 1`. The Error Control field is **not defined by CCSDS 133.0-B-2**; it is a mission/PUS-style extension carried inside the packet data field (wire-compatible, since the standard leaves data field content to the mission). Set on encode via `WithErrorControl()` (CRC-16-CCITT), verified on decode via `WithDecodeErrorControl()`. Configurable at the service level via `ServiceConfig.ErrorControl`. |
| SPP-17 | Packet Secondary Header | 4.1.4.2 | C1 | Yes | `SecondaryHeader` is an interface (`Encode()`, `Decode()`, `Size()`) allowing mission-specific implementations. Configurable via `WithSecondaryHeader()` option. The Blue Book sets no fixed upper size limit; the implementation enforces at least 1 octet (`ErrSecondaryHeaderTooSmall`) and the overall packet-length maximum. 4.1.4.2.1.5 restricts the layout to a Time Code Field, an Ancillary Data Field, or a Time Code Field followed by an Ancillary Data Field, and 4.1.4.2.1.6 requires the choice to stay static per managed data path; neither is machine-checkable through an interface that exposes only octets, so both are the implementation's responsibility. `Encode()` verifies the header emits exactly `Size()` bytes (`ErrSecondaryHeaderSizeMismatch`). Idle packets (APID 0x7FF) must not carry a secondary header — 4.1.3.3.3.4 sets their Secondary Header Flag to '0' (`ErrIdleWithSecondaryHeader`); `NewIdlePacket()` builds conformant idle packets. C1 enforced: `NewSpacePacket()` allows nil/empty user data when a secondary header is provided. |
| SPP-18 | User Data Field | 4.1.4.3 | C2 | Yes | `UserData []byte` field. C2 enforced: `NewSpacePacket()` requires user data only when no secondary header is present. When a secondary header is provided, user data may be nil or empty. |

**C1:** It is mandatory for a Space Packet to contain a Packet Secondary Header if
no User Data Field is present; otherwise, it is optional.

**C2:** It is mandatory for a Space Packet to contain a User Data Field if the Packet
Secondary Header is not present; otherwise, it is optional.

### Table A-5: Protocol Procedures

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-19 | Packet Assembly Function | 4.2.2 | M | Yes | `NewSpacePacket()` constructs the packet with functional options (`WithSecondaryHeader()`, `WithErrorControl()`, `WithSequenceCount()`, `WithSequenceFlags()`). `Encode()` serializes Primary Header + Secondary Header + User Data + Error Control into an octet stream. Packet Data Length is computed automatically. When error control is enabled, `Encode()` auto-computes the CRC-16-CCITT over the serialized header and data, then appends it. |
| SPP-20 | Packet Transfer Function | 4.2.3 | M | Yes | `Service.SendPacket()` stamps the per-APID sequence count (14-bit, wraps at 16383) and writes the encoded packet to the transport via `io.ReadWriter`. A packet received without a secondary header decoder re-encodes byte for byte, so a relay can forward what it received. Multiplexing of packets from multiple APIDs is delegated to the caller, which controls the order and scheduling of `SendPacket()` calls. The multiplexing scheme itself is an optional management parameter (SPP-25). |
| SPP-21 | Packet Extraction Function | 4.3.2 | M | Yes | `Service.ReceivePacket()` reads the 6-octet Primary Header, computes total packet size from the header's Packet Length field, reads the remaining octets, and invokes `Decode()` with configured decode options (a per-packet secondary header decoder and error control validation). Both mandatory parts of 4.3.2.2 are performed: the Secondary Header Indicator is generated for every received packet (`Indication.SecondaryHeaderIndicator`), and the Packet Sequence Count is checked for continuity per APID modulo 16384. Only the Data Loss Indicator *parameter* is optional (SPP-9); the check that produces it is not, and it is always run. |
| SPP-22 | Packet Reception Function | 4.3.3 | M | Yes | `Decode()` parses raw octets into a `SpacePacket` and automatically validates the result via `Validate()`. Trailing bytes beyond the declared packet length are ignored (documented), so buffers may carry multiple packets. When `WithDecodeErrorControl()` is used, the trailing 2-byte CRC is verified against the packet contents using CRC-16-CCITT; mismatches return `ErrCRCValidationFailed`. `PacketSizer()` refuses a packet that does not fit the buffer it is handed; `DeclaredPacketSize()` gives a stream reader the header's declared length before the body has been fetched. |

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

The proforma in Annex A of CCSDS 133.0-B-2 runs from SPP-1 to SPP-26; there are no items beyond that.

| Category | Total Items | Supported | Partial | Not Supported |
|----------|-------------|-----------|---------|---------------|
| Mandatory (M) | 20 | 20 | 0 | 0 |
| Optional (O) | 4 | 2 | 0 | 2 |
| Conditional (C) | 2 | 2 | 0 | 0 |
| **Total** | **26** | **24** | **0** | **2** |

### Non-Conformances (Optional Items Not Supported)

| Item | Description | Reason |
|------|-------------|--------|
| SPP-5 | QoS Requirement | Quality of Service parameter not implemented. |
| SPP-25 | Packet Multiplexing Scheme | No multiplexing, scheduling, or interleaving logic. |

### Partial Conformances (Items Requiring Attention)

None. Every mandatory and conditional item is fully supported, including both indicators the Octet String Service must deliver (SPP-8, and the mandatory continuity check inside SPP-21).

### Outside the Proforma

The items below are not PICS items — the CCSDS 133.0-B-2 proforma does not list them — but they come up often enough to be worth stating.

| Topic | Reference | Status |
|-------|-----------|--------|
| Segmentation / reassembly | 4.1.3.4.2 | The sequence flag values (`SeqFlagFirstSegment`, `SeqFlagContinuation`, `SeqFlagLastSegment`) can be set via `WithSequenceFlags()`, but the package provides no segmentation or reassembly procedures; applications split and rejoin large data units themselves. Note that 4.1.3.4.2.3 forbids segmentation on any managed data path that uses the Octet String Service. |
| Idle packet handling | 4.1.3.3.4.4 | `NewIdlePacket()` builds them, `IsIdleBytes()` recognizes an encoded one, and `ServiceConfig.DiscardIdle` makes the receiving service drop them instead of delivering fill to an application. |

### Fully Supported Items

| Item | Description | Implementation |
|------|-------------|----------------|
| SPP-1 | Space Packet SDU | `SpacePacket` struct with `Service.SendPacket()` / `Service.ReceivePacket()` service-layer abstraction. |
| SPP-2 | Octet String SDU | `Service.SendBytes()` / `Service.ReceiveBytes()` for raw octet string I/O. |
| SPP-3 | APID | `PrimaryHeader.APID` with validation. |
| SPP-6 | Octet String | `data` parameter in `Service.SendBytes()`. |
| SPP-7 | APID (Octet String) | `apid` parameter in `Service.SendBytes()` / `Indication.APID` from `Service.ReceiveBytes()`. |
| SPP-8 | Secondary Header Indicator (Octet String) | `WithSendSecondaryHeader()` on send; `Indication.SecondaryHeaderIndicator` on receive. |
| SPP-10 | Packet.request | `Service.SendPacket()` with per-APID sequence counting (continuous modulo 16384, resynchronized on a pinned count) and max packet length enforcement. |
| SPP-11 | Packet.indication | `Service.ReceivePacket()` with a per-packet secondary header decoder from `ServiceConfig.NewSecondaryHeader`. |
| SPP-12 | Octet_String.request | `Service.SendBytes()` constructs a packet from raw bytes and all five request parameters of 3.4.3.2.2. |
| SPP-13 | Octet_String.indication | `Service.ReceiveBytes()` delivers an `Indication`: octet string, APID, Secondary Header Indicator, Data Loss Indicator. |
| SPP-14 | Space Packet | `SpacePacket` struct with encode/decode round-trip. `Decode()` accepts `WithDecodeSecondaryHeader()` and `WithDecodeErrorControl()` options. |
| SPP-17 | Packet Secondary Header (C1) | `SecondaryHeader` interface with `WithSecondaryHeader()` option. Packets with secondary header only (no user data) are valid. |
| SPP-18 | User Data Field (C2) | User data required only when no secondary header is present; optional otherwise. |
| SPP-15 | Packet Primary Header | Complete 6-octet header with all CCSDS fields. Version enforced as 0 (CCSDS v1). Named constants for packet types and sequence flags. Per-APID sequence counting in `Service`; `WithSequenceCount()` / `WithSequenceFlags()` for manual control. |
| SPP-16 | Packet Data Field | Correct composition and length calculation. CRC-16-CCITT error control (a mission/PUS extension, not part of CCSDS 133.0-B-2) on encode (`WithErrorControl()`) and decode (`WithDecodeErrorControl()`). |
| SPP-19 | Packet Assembly Function | Full assembly via `NewSpacePacket()` + `Encode()`. |
| SPP-20 | Packet Transfer Function | `Service.SendPacket()` stamps per-APID sequence count and writes to transport. Multiplexing delegated to caller. |
| SPP-21 | Packet Extraction Function | Full extraction via `Service.ReceivePacket()` + `Decode()`, with the Secondary Header Indicator and the per-APID sequence count continuity check of 4.3.2.2. |
| SPP-22 | Packet Reception Function | `Decode()` parses, validates, and optionally verifies CRC. |
| SPP-23 | Maximum Packet Length | Configurable via `ServiceConfig.MaxPacketLength`, default 65542. |
| SPP-24 | Packet Type | Configurable via `ServiceConfig.PacketType`. TM (0) / TC (1) via named constants with convenience constructors. |
| SPP-26 | Service Type | Both Packet Service and Octet String Service available via `Service`. |
