# PICS PROFORMA FOR SPACE PACKET PROTOCOL

## Conformance Statement for `pkg/spp` — CCSDS 133.0-B-2

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 26/08/2026 |
| PICS Serial Number | ASTRO-SPP-PICS-002 |
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
| SPP-2 | Octet String SDU | 3.2.3 | M | Yes | `Service.SendBytes()` accepts raw octet strings with service parameters (APID, packet type, sequence count/packet name, and the Secondary Header Indicator in either form). `Service.ReceiveBytes()` returns an `Indication` carrying the octet string, APID, Secondary Header Indicator, and Data Loss Indicator. Per 3.2.3.1 an octet string is 1 to 65536 octets, which is exactly the Packet Data Field range the packet-length checks enforce. |

### Table A-2: Service Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| SPP-3 | APID | 3.3.2.2 | M | 0–2047 | Yes | `PrimaryHeader.APID` (11-bit). Validated in `PrimaryHeader.Validate()`. |
| SPP-4 | Packet Loss Indicator | 3.3.2.3 | O | — | Yes | `Service.ReceivePacket()` runs the sequence count continuity check per APID; `Service.LastDataLoss()` reports how many packets were missing before the packet just received (0 when the count was continuous). `Service.ResetContinuity()` clears the state after a link outage. Counts are tracked per APID and never shared across APIDs (4.1.3.4.3.3), so idle packets (APID 0x7FF) have their own sequence and cannot disturb an application's. |
| SPP-5 | QoS Requirement | 3.3.2.4 | O | — | No | Not implemented. |
| SPP-6 | Octet String | 3.4.2.1 | M | — | Yes | The `data` parameter in `Service.SendBytes(apid, data, opts...)` is the octet string service parameter. |
| SPP-7 | APID (Octet String Service) | 3.4.2.2 | M | 0–2047 | Yes | The `apid` parameter in `Service.SendBytes(apid, data, opts...)`. Validated via `NewSpacePacket()`. Returned by `Service.ReceiveBytes()`. |
| SPP-8 | Secondary Header Indicator (Octet String Service) | 3.4.2.3 | M | 0 or 1 | Yes | Sending: `WithSendSecondaryHeaderIndicator(bool)` is the parameter in the form 3.4.2.3.2 describes — a signal that a Packet Secondary Header leads the octet string the user is handing over, which the Packet Assembly Function translates into the flag (3.4.2.3.3, 4.2.2.4). `WithSendSecondaryHeader()` is the convenience form for a caller that has a `SecondaryHeader` implementation rather than raw octets; it sets the same flag. Supplying both is refused with `ErrSecondaryHeaderTwice`, since counting the header twice would declare a data field longer than the packet carries. Receiving: `Indication.SecondaryHeaderIndicator`, read from the Secondary Header Flag, and the octets themselves always lead `Indication.Data` so the two agree (4.3.2.2). |
| SPP-9 | Data Loss Indicator | 3.4.2.4 | O | — | Yes | `Indication.DataLoss` and `Indication.PacketsLost` from `Service.ReceiveBytes()`, derived from the mandatory continuity check of 4.3.2.2. |

### Table A-3: Service Primitives

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-10 | Packet.request | 3.3.3.2 | M | Yes | `Service.SendPacket(packet)` implements the Packet.request primitive. Accepts a pre-built `*SpacePacket`, encodes it, enforces the configurable maximum packet length, and writes to the transport. A packet whose count the caller owns is sent unchanged — one built with `WithSequenceCount()`, and one returned by `Decode()`, which already carries the count its originating application assigned (4.1.3.4.3.3, 3.3.1). In both cases the service resynchronizes its counter for that APID to one past the value it sent, so the count stays continuous modulo 16384 per 4.1.3.4.3.4. Any other packet is stamped with the next count for its APID (4.1.3.4.3), mutating the caller's packet. A send that fails before reaching the transport returns the count to the counter rather than leaving a hole. |
| SPP-11 | Packet.indication | 3.3.3.3 | M | Yes | `Service.ReceivePacket()` implements the Packet.indication primitive. Reads the primary header, calculates total packet size, reads remaining octets, and decodes into a `*SpacePacket`. `ServiceConfig.NewSecondaryHeader` is a factory, so every delivered packet gets its own decoder instance, configured by the caller — a header whose width lives in the value (a PUS header reads it from its mission profile) is built correctly because only the caller can build it. `ServiceConfig.DiscardIdle` drops received idle packets. A packet longer than the managed Maximum Packet Length is rejected *and* its body skipped, so the reader stays on a real packet boundary instead of resynchronizing mid-packet and delivering packets that were never sent. |
| SPP-12 | Octet_String.request | 3.4.3.2 | M | Yes | `Service.SendBytes(apid, data, opts...)` implements the OctetString.request primitive. All parameters of 3.4.3.2.2 are available: octet string and APID as arguments, Secondary Header Indicator via `WithSendSecondaryHeader()`, Packet Type via `WithSendPacketType()` (defaulting to `ServiceConfig.PacketType`), and Packet Sequence Count/Packet Name via `WithSendSequenceCount()` / `WithSendPacketName()`. |
| SPP-13 | Octet_String.indication | 3.4.3.3 | M | Yes | `Service.ReceiveBytes()` implements the OctetString.indication primitive, returning an `Indication` with every parameter of 3.4.3.3.2: the octet string, the APID, the Secondary Header Indicator, and the optional Data Loss Indicator (`DataLoss` plus `PacketsLost`). `Indication.Data` is the Packet Data Field — what is left once the Packet Extraction Function removes the Packet Primary Header (4.3.2.2) — so a secondary header's octets lead it whenever the indicator is set, whether or not a decoder was configured. A configured decoder additionally fills `Indication.SecondaryHeader`. The only octets omitted are the error control field, when the service was configured to expect one: those are consumed and verified by this layer and were never part of the user's octet string. |

### Table A-4: SPP Protocol Data Unit

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SPP-14 | Space Packet | 4.1 | M | Yes | `SpacePacket` struct with `Encode()` / `Decode()` round-trip support. `Decode(data, opts...)` accepts functional options: `WithDecodeSecondaryHeader()` to decode mission-specific header bytes, and `WithDecodeErrorControl()` to extract and verify the trailing CRC-16-CCITT. If no secondary header decoder is provided, secondary header bytes are included in `UserData` and the packet re-encodes byte for byte, so a relay can forward it. `WithSecondaryHeaderIndicator()` lets a caller assemble the same shape from octets it did not parse. The Secondary Header Flag and the `SecondaryHeader` field must agree (4.1.3.3.3.2): a header set with the flag clear returns `ErrSecondaryHeaderFlagClear`, and a packet with the flag set and neither a parsed header nor header octets in its data field returns `ErrSecondaryHeaderMissing`. Both `NewSpacePacket()` and `Decode()` copy the octets they are handed, so neither a caller's buffer nor a decoded packet can be changed through the other. A failed `Encode()` leaves the packet exactly as the caller had it. |
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
| SPP-20 | Packet Transfer Function | 4.2.3 | M | Yes | `Service.SendPacket()` writes the encoded packet to the transport via `io.ReadWriter`. A packet obtained from `Decode()` is forwarded byte for byte — the sequence count is not rewritten, because 3.3.1 requires Packet Service SDUs to travel "without further formatting", 4.1.3.4.3.3 makes the count the property of the originating application, and the sequence counter belongs to the Packet Assembly Function of 4.2.2.4, which serves the Octet String Service. Packets the local user originates without a count are stamped per APID (14-bit, wraps at 16383). Multiplexing of packets from multiple APIDs is delegated to the caller, which controls the order and scheduling of `SendPacket()` calls. The multiplexing scheme itself is an optional management parameter (SPP-25). |
| SPP-21 | Packet Extraction Function | 4.3.2 | M | Yes | `Service.ReceivePacket()` reads the 6-octet Primary Header, computes total packet size from the header's Packet Length field, reads the remaining octets, and invokes `Decode()` with configured decode options (a per-packet secondary header decoder and error control validation). Both mandatory parts of 4.3.2.2 are performed: the octet string is extracted by removing the primary header and nothing else, with the Secondary Header Indicator generated for every received packet (`Indication.SecondaryHeaderIndicator`) to announce a secondary header at the start of it; and the Packet Sequence Count is checked for continuity per APID modulo 16384. Only the Data Loss Indicator *parameter* is optional (SPP-9); the check that produces it is not, and it is always run. |
| SPP-22 | Packet Reception Function | 4.3.3 | M | Yes | `Decode()` parses raw octets into a `SpacePacket` and automatically validates the result via `Validate()`. Trailing bytes beyond the declared packet length are ignored (documented), so buffers may carry multiple packets. When `WithDecodeErrorControl()` is used, the trailing 2-byte CRC is verified against the packet contents using CRC-16-CCITT; mismatches return `ErrCRCValidationFailed`. `PacketSizer()` refuses a packet that does not fit the buffer it is handed; `DeclaredPacketSize()` gives a stream reader the header's declared length before the body has been fetched. |

### Table A-6: Management Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| SPP-23 | Maximum Packet Length (octets) | Table 5-1 | M | Integer | Yes | Configurable via `ServiceConfig.MaxPacketLength`. Defaults to 65542 octets, the absolute maximum of 3.2.2.1. Enforced on send in `Service.SendPacket()` and on receive in `Service.ReceivePacket()`, where an oversize packet is rejected and its body skipped so the reader keeps its framing. Minimum packet size of 7 octets enforced in `NewSpacePacket()`. |
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

None. Every mandatory and conditional item is fully supported, including both
indicators the Octet String Service must deliver (SPP-8, and the mandatory
continuity check inside SPP-21).

Two requirements are stated in the standard but cannot be checked by this
package, and are the responsibility of whoever supplies the secondary header.
They are not PICS items and do not affect the counts above; they are listed so
nobody reads a "Yes" against SPP-17 as more than it is.

| Requirement | Reference | Why it is not checked here |
|---|---|---|
| A Packet Secondary Header consists of a Time Code Field, an Ancillary Data Field, or a Time Code Field followed by an Ancillary Data Field, and the choice stays static for a managed data path. | 4.1.4.2.1.5, 4.1.4.2.1.6 | The `SecondaryHeader` interface sees only octets. Which of the three shapes an implementation produces, and whether it keeps producing the same one, is invisible from here. |
| The Secondary Header Flag is static with respect to the APID and managed data path throughout a Mission Phase. | 4.1.3.3.3.3 | This is a property of a data path over time, not of a packet. The package holds no per-APID configuration to compare a packet against; a mission's own configuration layer has to enforce it. |

### Outside the Proforma

The items below are not PICS items — the CCSDS 133.0-B-2 proforma does not list them — but they come up often enough to be worth stating.

| Topic | Reference | Status |
|-------|-----------|--------|
| Segmentation / reassembly | 4.1.3.4.2 | The sequence flag values (`SeqFlagFirstSegment`, `SeqFlagContinuation`, `SeqFlagLastSegment`) can be set via `WithSequenceFlags()`, but the package provides no segmentation or reassembly procedures; applications split and rejoin large data units themselves. Note that 4.1.3.4.2.3 forbids segmentation on any managed data path that uses the Octet String Service. |
| Idle packet handling | 4.1.3.3.4.4 | `NewIdlePacket()` builds them, `IsIdleBytes()` recognizes an encoded one (reading the 11 APID bits only, so a telecommand idle packet is recognized too), and `ServiceConfig.DiscardIdle` makes the receiving service drop them instead of delivering fill to an application. Idle packets are counted under APID 0x7FF, which is a sequence of its own, so discarding them cannot disturb an application's continuity. |
| Concurrent use | — | `Service` is safe for concurrent use. Sends are serialized against each other so a count and the octets carrying it reach the transport together, which 4.1.3.4.3.4 requires; receives are serialized against each other so a packet's header and body are not spliced with another reader's. Sending and receiving proceed independently. |
| Reordered and duplicated packets | 4.1.3.4.3, note 1 | The continuity check is modulo-16384 subtraction, so a packet arriving out of order or twice reports a large phantom loss rather than a negative one. The standard's own note acknowledges that order "may be disturbed during transport" and sets no requirement to tell reordering apart from loss, so this is the arithmetic the Data Loss Indicator is defined on, not a defect. A receiver that needs to distinguish them must order packets itself, e.g. against a time code (4.1.3.4.3, note 3). |

### Fully Supported Items

| Item | Description | Implementation |
|------|-------------|----------------|
| SPP-1 | Space Packet SDU | `SpacePacket` struct with `Service.SendPacket()` / `Service.ReceivePacket()` service-layer abstraction. |
| SPP-2 | Octet String SDU | `Service.SendBytes()` / `Service.ReceiveBytes()` for raw octet string I/O. |
| SPP-3 | APID | `PrimaryHeader.APID` with validation. |
| SPP-6 | Octet String | `data` parameter in `Service.SendBytes()`. |
| SPP-7 | APID (Octet String) | `apid` parameter in `Service.SendBytes()` / `Indication.APID` from `Service.ReceiveBytes()`. |
| SPP-8 | Secondary Header Indicator (Octet String) | `WithSendSecondaryHeaderIndicator()` on send (or `WithSendSecondaryHeader()` when the caller has a header implementation); `Indication.SecondaryHeaderIndicator` on receive, with the octets at the front of `Indication.Data`. |
| SPP-10 | Packet.request | `Service.SendPacket()` with per-APID sequence counting (continuous modulo 16384, resynchronized on a caller-owned count) and max packet length enforcement. A decoded packet is forwarded with its count intact. |
| SPP-11 | Packet.indication | `Service.ReceivePacket()` with a per-packet secondary header decoder built by the caller's `ServiceConfig.NewSecondaryHeader` factory, and framing that survives an oversize packet. |
| SPP-12 | Octet_String.request | `Service.SendBytes()` constructs a packet from raw bytes and all five request parameters of 3.4.3.2.2. |
| SPP-13 | Octet_String.indication | `Service.ReceiveBytes()` delivers an `Indication`: the Packet Data Field as the octet string, APID, Secondary Header Indicator, Data Loss Indicator. |
| SPP-14 | Space Packet | `SpacePacket` struct with encode/decode round-trip. `Decode()` accepts `WithDecodeSecondaryHeader()` and `WithDecodeErrorControl()` options. |
| SPP-17 | Packet Secondary Header (C1) | `SecondaryHeader` interface with `WithSecondaryHeader()` option. Packets with secondary header only (no user data) are valid. |
| SPP-18 | User Data Field (C2) | User data required only when no secondary header is present; optional otherwise. |
| SPP-15 | Packet Primary Header | Complete 6-octet header with all CCSDS fields. Version enforced as 0 (CCSDS v1). Named constants for packet types and sequence flags. Per-APID sequence counting in `Service`; `WithSequenceCount()` / `WithSequenceFlags()` for manual control. |
| SPP-16 | Packet Data Field | Correct composition and length calculation. CRC-16-CCITT error control (a mission/PUS extension, not part of CCSDS 133.0-B-2) on encode (`WithErrorControl()`) and decode (`WithDecodeErrorControl()`). |
| SPP-19 | Packet Assembly Function | Full assembly via `NewSpacePacket()` + `Encode()`. |
| SPP-20 | Packet Transfer Function | `Service.SendPacket()` writes to the transport, stamping a per-APID count only on packets the local user originated. Multiplexing delegated to caller. |
| SPP-21 | Packet Extraction Function | Full extraction via `Service.ReceivePacket()` + `Decode()`, with the Secondary Header Indicator and the per-APID sequence count continuity check of 4.3.2.2. |
| SPP-22 | Packet Reception Function | `Decode()` parses, validates, and optionally verifies CRC. |
| SPP-23 | Maximum Packet Length | Configurable via `ServiceConfig.MaxPacketLength`, default 65542. |
| SPP-24 | Packet Type | Configurable via `ServiceConfig.PacketType`. TM (0) / TC (1) via named constants with convenience constructors. |
| SPP-26 | Service Type | Both Packet Service and Octet String Service available via `Service`. |
