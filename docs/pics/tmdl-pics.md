# PICS PROFORMA FOR TM SPACE DATA LINK PROTOCOL

## Conformance Statement for `pkg/tmdl` — CCSDS 132.0-B-3

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 17/03/2026 |
| PICS Serial Number | ASTRO-TMDL-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/tmdl |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS TM Space Data Link Protocol transfer frame encoding, decoding, validation, virtual channel management, multiplexing, and service-layer abstractions for VCP, VCF, VCA, and Master Channel. Services use VirtualChannel as a single buffer; MasterChannel integrates a multiplexer for the full send/receive pipeline. Physical channel configuration defined via ChannelConfig. |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/tmdl (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 132.0-B-3 (TM Space Data Link Protocol, Blue Book, Issue 3, October 2021) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE — A YES answer means that the implementation does not conform to the Recommended
Standard. Non-supported mandatory capabilities are identified in the PICS, with an
explanation of why the implementation is non-conforming.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: TM Service Data Units

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TM-1 | Packet SDU | 3.2.2 | M | Yes | Space Packets are carried in `TMTransferFrame.DataField`. The `VirtualChannelPacketService` accepts packet data via `Send()`, pushes frames into a `VirtualChannel`, and delivers data via `Receive()`. |
| TM-2 | VCA_SDU | 3.2.3 | M | Yes | `VirtualChannelAccessService` accepts fixed-length VCA SDUs via `Send()` with `VCASize` enforcement, pushes frames into a `VirtualChannel`, and delivers them via `Receive()`. |
| TM-3 | FSH_SDU | 3.2.4 | M | Yes | `SecondaryHeader.DataField` carries the FSH SDU. Presence indicated by `PrimaryHeader.FSHFlag`. Encoded/decoded via `SecondaryHeader.Encode()` / `SecondaryHeader.Decode()`. |
| TM-4 | OCF_SDU | 3.2.5 | M | Yes | `TMTransferFrame.OperationalControl` — 4-byte OCF field. Presence indicated by `PrimaryHeader.OCFFlag`. Extracted during decode when present. |
| TM-5 | TM Transfer Frame | 3.2.6 | M | Yes | `TMTransferFrame` struct with `Encode()` / `DecodeTMTransferFrame()` round-trip support. Composed of Primary Header, optional Secondary Header, Data Field, optional OCF, and Frame Error Control. |

### Table A-2: Service Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| | **VCP Packet Service Parameters** | | | | | |
| TM-6 | Packet | 3.3.2.2 | M | — | Yes | Packet data passed as `[]byte` to `VirtualChannelPacketService.Send()`. Frame pushed into `VirtualChannel`. Delivered via `Receive()`. |
| TM-7 | GVCID | 3.3.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()` (TFVN + SCID + VCID). VCID configured at service construction. |
| TM-8 | Packet Version Number | 3.3.2.4 | M | — | No | Not implemented. No Packet Version Number extraction or validation at the data link layer. |
| TM-9 | Packet Quality Indicator | 3.3.2.5 | O | — | No | Not implemented. |
| TM-10 | Verification Status Code | 3.3.2.6 | C2 | (see reference [10]) | N/A | SDLS Option not implemented. |
| | **VCA SDU Service Parameters** | | | | | |
| TM-11 | VCA SDU | 3.4.2.2 | M | — | Yes | Fixed-length data passed to `VirtualChannelAccessService.Send()`. Size enforced against `vcaSize`. Frame pushed into `VirtualChannel`. |
| TM-12 | VCA Status Fields | 3.4.2.3 | M | — | No | Not implemented. No VCA status field extraction or delivery. |
| TM-13 | GVCID | 3.4.2.4 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. VCID configured at service construction. |
| TM-14 | VCA SDU Loss Flag | 3.4.2.5 | O | — | No | Not implemented. No frame loss detection mechanism. |
| TM-15 | Verification Status Code | 3.4.2.6 | C2 | (see reference [10]) | N/A | SDLS Option not implemented. |
| | **VC FSH SDU Service Parameters** | | | | | |
| TM-16 | FSH SDU | 3.5.2.2 | M | — | Yes | `SecondaryHeader.DataField` carries the FSH SDU. Encoded/decoded via `SecondaryHeader.Encode()` / `Decode()`. |
| TM-17 | GVCID | 3.5.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. |
| TM-18 | FSH_SDU Loss Flag | 3.5.2.4 | O | — | No | Not implemented. No FSH SDU loss detection. |
| | **OCF SDU Service Parameters** | | | | | |
| TM-19 | OCF SDU | 3.6.2.2 | M | — | Yes | `TMTransferFrame.OperationalControl` — 4-byte field. |
| TM-20 | GVCID | 3.6.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. |
| TM-21 | OCF SDU Frame Loss Flag | 3.6.2.4 | O | — | No | Not implemented. No OCF frame loss detection. |
| | **VC Frame Service Parameters** | | | | | |
| TM-22 | TM Frame | 3.7.2.2 | M | — | Yes | `VirtualChannelFrameService` accepts and delivers complete `*TMTransferFrame` objects via `Send()` / `Receive()`. |
| TM-23 | GVCID | 3.7.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. |
| TM-24 | Frame Loss Flag | 3.7.2.4 | O | — | No | Not implemented. No frame loss detection. |
| | **MC FSH Service Parameters** | | | | | |
| TM-25 | FSH SDU | 3.8.2.2 | M | — | Yes | `SecondaryHeader.DataField` at Master Channel level. |
| TM-26 | MCID | 3.8.2.3 | M | — | Yes | `PrimaryHeader.MCID()` returns TFVN + SCID. |
| TM-27 | FSH_SDU Loss Flag | 3.8.2.4 | O | — | No | Not implemented. |
| | **MC OCF Service Parameters** | | | | | |
| TM-28 | OCF SDU | 3.9.2.2 | M | — | Yes | `TMTransferFrame.OperationalControl` at Master Channel level. |
| TM-29 | MCID | 3.9.2.3 | M | — | Yes | `PrimaryHeader.MCID()`. |
| TM-30 | OCF_SDU Loss Flag | 3.9.2.4 | O | — | No | Not implemented. |
| | **MC Frame Service Parameters** | | | | | |
| TM-31 | TM Frame | 3.10.2.2 | M | — | Yes | `MasterChannel` manages complete `*TMTransferFrame` objects. `AddFrame()` routes inbound frames to Virtual Channels by VCID. `GetNextFrame()` pulls from the integrated multiplexer. SCID matching enforced. |
| TM-32 | MCID | 3.10.2.3 | M | — | Yes | `PrimaryHeader.MCID()`. SCID validated in `MasterChannel.AddFrame()`. |
| TM-33 | Frame Loss Flag | 3.10.2.4 | O | — | No | Not implemented. |

**C2:** O if SDLS Option else N/A.

### Table A-3: Service Primitives

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| | **VCP Service Primitives** | | | | |
| TM-34 | VCP.request | 3.3.3.2 | M | Yes | `VirtualChannelPacketService.Send(data)` implements VCP.request. Constructs a `TMTransferFrame`, stamps counters/CRC via `stampFrame()`, and pushes it into the `VirtualChannel`. |
| TM-35 | VCP.indication | 3.3.3.3 | M | Yes | `VirtualChannelPacketService.Receive()` implements VCP.indication. Pulls the next frame from the `VirtualChannel` and returns its Data Field. |
| | **VCA Service Primitives** | | | | |
| TM-36 | VCA.request | 3.4.3.2 | M | Yes | `VirtualChannelAccessService.Send(data)` implements VCA.request. Enforces fixed `vcaSize`, constructs a frame, stamps counters/CRC via `stampFrame()`, and pushes it into the `VirtualChannel`. |
| TM-37 | VCA.indication | 3.4.3.3 | M | Yes | `VirtualChannelAccessService.Receive()` implements VCA.indication. Pulls the next frame from the `VirtualChannel` and returns its fixed-length Data Field. |
| | **VC FSH Service Primitives** | | | | |
| TM-38 | VC_FSH.request | 3.5.3.2 | M | Yes | Secondary header data passed via `NewTMTransferFrame(..., secondaryHeaderData, ...)`. FSHFlag auto-set. |
| TM-39 | VC_FSH.indication | 3.5.3.3 | M | Yes | `DecodeTMTransferFrame()` extracts secondary header when FSHFlag is set and decodes via `SecondaryHeader.Decode()`. |
| | **VC OCF Service Primitives** | | | | |
| TM-40 | VC_OCF.request | 3.6.3.2 | M | Yes | OCF data passed via `NewTMTransferFrame(..., ocf)`. OCFFlag auto-set when OCF is present. |
| TM-41 | VC_OCF.indication | 3.6.3.3 | M | Yes | `DecodeTMTransferFrame()` extracts 4-byte OCF when OCFFlag is set. |
| | **VC Frame Service Primitives** | | | | |
| TM-42 | VCF.request | 3.7.3.2 | M | Yes | `VirtualChannelFrameService.Send(data)` decodes frame bytes and pushes the frame into the `VirtualChannel`. |
| TM-43 | VCF.indication | 3.7.3.3 | M | Yes | `VirtualChannelFrameService.Receive()` pulls the next frame from the `VirtualChannel` and returns it as encoded bytes. |
| | **MC FSH Service Primitives** | | | | |
| TM-44 | MC_FSH.request | 3.8.3.2 | M | Yes | Secondary header included at frame construction via `NewTMTransferFrame()`. |
| TM-45 | MC_FSH.indication | 3.8.3.3 | M | Yes | Secondary header extracted during `DecodeTMTransferFrame()`. |
| | **MC OCF Service Primitives** | | | | |
| TM-46 | MC_OCF.request | 3.9.3.2 | M | Yes | OCF included at frame construction via `NewTMTransferFrame()`. |
| TM-47 | MC_OCF.indication | 3.9.3.3 | M | Yes | OCF extracted during `DecodeTMTransferFrame()`. |
| | **MC Frame Service Primitives** | | | | |
| TM-48 | MCF.request | 3.10.3.2 | M | Yes | `MasterChannel.AddFrame(frame)` accepts a `*TMTransferFrame` with SCID validation and routes it to the appropriate `VirtualChannel` by VCID. |
| TM-49 | MCF.indication | 3.10.3.3 | M | Yes | `MasterChannel.GetNextFrame()` pulls the next frame from the integrated `VirtualChannelMultiplexer`. |

### Table A-4: TM Protocol Data Unit

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TM-50 | TM Transfer Frame | 4.1.1 | M | Yes | `TMTransferFrame` struct with `Encode()` / `DecodeTMTransferFrame()` round-trip. |
| TM-51 | Transfer Frame Primary Header | 4.1.2 | M | Yes | `PrimaryHeader` — 6 octets (48 bits). All fields per CCSDS: Transfer Frame Version Number (2 bits, enforced as `00`), Spacecraft ID (10 bits), Virtual Channel ID (3 bits), OCF Flag (1 bit), MC Frame Count (8 bits), VC Frame Count (8 bits), Transfer Frame Data Field Status (16 bits). Big-endian encoding via `Encode()` / `Decode()`. Validated via `Validate()`. |
| TM-52 | Transfer Frame Secondary Header | 4.1.3 | M | Yes | `SecondaryHeader` struct: Version Number (2 bits, enforced as `00`), Header Length (6 bits, 0–63), Data Field (variable). `Encode()` / `Decode()` / `Validate()` methods. Presence controlled by FSHFlag. |
| TM-53 | Transfer Frame Data Field | 4.1.4 | M | Yes | `TMTransferFrame.DataField` — variable-length telemetry payload. |
| TM-54 | Operational Control Field | 4.1.5 | M | Yes | `TMTransferFrame.OperationalControl` — 4 bytes (32 bits). Included when `OCFFlag` is set. Extracted during decode. |
| TM-55 | Frame Error Control Field | 4.1.6 | M | Yes | `TMTransferFrame.FrameErrorControl` — 16-bit CRC-16-CCITT (polynomial 0x1021, init 0xFFFF). Auto-computed on encode via `ComputeCRC()`. Verified on decode; CRC mismatch returns error. |

### Table A-5: Protocol Procedures

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TM-56 | Packet Processing Function | 4.2.2 | M | Yes | `VirtualChannelPacketService.Send()` accepts packet data, constructs a TM Transfer Frame via `NewTMTransferFrame()`, stamps counters/CRC via `stampFrame()`, and pushes it into the `VirtualChannel`. |
| TM-57 | VC Generation Function | 4.2.3 | M | Yes | `NewTMTransferFrame()` generates frames with SCID, VCID, data, optional secondary header, and optional OCF. CRC auto-computed. Frame counts applied by `stampFrame()` when a `FrameCounter` is provided. |
| TM-58 | VC Multiplexing Function | 4.2.4 | M | Yes | `VirtualChannelMultiplexer` schedules frames from multiple Virtual Channels using weighted round-robin via `GetNextFrame()`. Integrated into `MasterChannel`. |
| TM-59 | MC Generation Function | 4.2.5 | M | Yes | `MasterChannel.AddFrame()` routes inbound frames to Virtual Channels by VCID with SCID validation. |
| TM-60 | MC Multiplexing Function | 4.2.6 | M | No | Not implemented. No Master Channel multiplexing across physical channels. |
| TM-61 | All Frames Generation Function | 4.2.7 | M | No | Not implemented. No physical-channel-level frame generation (e.g., idle frame insertion, frame randomization). |
| TM-62 | Packet Extraction Function | 4.3.2 | M | Yes | `VirtualChannelPacketService.Receive()` pulls the next frame from the `VirtualChannel` and extracts packet data. |
| TM-63 | VC Reception Function | 4.3.3 | M | Yes | `DecodeTMTransferFrame()` parses raw octets into a `TMTransferFrame`, verifying CRC and extracting all fields. `MasterChannel.AddFrame()` routes received frames to the appropriate `VirtualChannel` by VCID. |
| TM-64 | VC Demultiplexing Function | 4.3.4 | M | Yes | `MasterChannel.AddFrame()` demultiplexes inbound frames to Virtual Channels by VCID. `TMServiceManager` dispatches to the correct VC service. |
| TM-65 | MC Reception Function | 4.3.5 | M | Yes | `MasterChannel.GetNextFrame()` pulls the next frame from the integrated multiplexer. |
| TM-66 | MC Demultiplexing Function | 4.3.6 | M | No | Not implemented. No Master Channel demultiplexing across physical channels. |
| TM-67 | All Frames Reception Function | 4.3.7 | M | No | Not implemented. No physical-channel-level frame reception (e.g., frame de-randomization, sync). |

### Table A-6: Management Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| | **Managed Parameters for a Physical Channel** | | | | | |
| TM-68 | Physical Channel Name | Table 5-1 | M | Character String | No | `ChannelConfig` provides a physical channel abstraction but does not include a name field. |
| TM-69 | Transfer Frame Length (octets) | Table 5-1 | M | Integer | No | `ChannelConfig.FrameLength` defines the fixed frame length with `DataFieldCapacity()` helper, but it is not yet enforced during frame construction. Frame length remains implicit from data size. |
| TM-70 | Transfer Frame Version Number (TFVN) | Table 5-1 | M | '00' binary | Yes | `PrimaryHeader.VersionNumber` — enforced as `0` in `Validate()`. |
| TM-71 | Valid Spacecraft IDs | Table 5-1 | M | Integers | Yes | `PrimaryHeader.SpacecraftID` — 10 bits (0–1023). Validated in `Validate()`. Configurable per frame via `NewTMTransferFrame()`. |
| TM-72 | MC Multiplexing Scheme | Table 5-1 | M | Mission Specific | No | Not implemented. |
| TM-73 | Presence of Frame Error Control | Table 5-1 | M | Present ('1') / Absent ('0') | Yes | Always present. CRC-16-CCITT auto-computed on encode and verified on decode. |
| | **Managed Parameters for a Master Channel** | | | | | |
| TM-74 | SCID | Table 5-2 | M | Integer | Yes | `MasterChannel.scid` — configured at construction. Enforced in `AddFrame()`. |
| TM-75 | Valid VCIDs | Table 5-2 | M | Selectable set of integers (0–7) | Yes | `PrimaryHeader.VirtualChannelID` — 3 bits (0–7). `MasterChannel.channels` maps registered VCIDs. |
| TM-76 | VC Multiplexing Scheme | Table 5-2 | M | Mission Specific | Yes | `VirtualChannelMultiplexer` implements weighted round-robin scheduling. Priority weights determine how many consecutive frames each VC can transmit before yielding. Integrated into `MasterChannel`. |
| TM-77 | Presence of MC_FSH | Table 5-2 | M | Present ('1') / Absent ('0') | Yes | `PrimaryHeader.FSHFlag` indicates presence. Secondary header included/excluded at frame construction. |
| TM-78 | MC_FSH Length (if present) (octets) | Table 5-2 | M | Integer (2–64) | Yes | `SecondaryHeader.HeaderLength` — 6 bits (0–63). Data field length is variable. |
| TM-79 | Presence of MC_OCF | Table 5-2 | M | Present ('1') / Absent ('0') | Yes | `PrimaryHeader.OCFFlag` indicates presence. OCF included/excluded at frame construction. |
| | **Managed Parameters for a Virtual Channel** | | | | | |
| TM-80 | SCID | Table 5-3 | M | Integer | Yes | `PrimaryHeader.SpacecraftID`. Set via `NewTMTransferFrame()`. |
| TM-81 | VCID | Table 5-3 | M | 0 to 7 | Yes | `PrimaryHeader.VirtualChannelID`. `VirtualChannel.VCID` configured at construction. |
| TM-82 | Data Field Content | Table 5-3 | M | Packets, VCA_SDU | Yes | `PrimaryHeader.SyncFlag` distinguishes: `0` = packets (VCP), `1` = VCA SDUs. |
| TM-83 | Presence of VC_FSH | Table 5-3 | M | Present ('1') / Absent ('0') | Yes | `PrimaryHeader.FSHFlag`. |
| TM-84 | VC_FSH Length (if present) (octets) | Table 5-3 | M | Integer | Yes | `SecondaryHeader.HeaderLength`. |
| TM-85 | Presence of VC_OCF | Table 5-3 | M | Present ('1') / Absent ('0') | Yes | `PrimaryHeader.OCFFlag`. |
| | **Managed Parameters for Packet Transfer** | | | | | |
| TM-86 | Valid PVNs | Table 5-4 | M | Set of Integers | No | Not implemented. No Packet Version Number validation at the data link layer. |
| TM-87 | Maximum Packet Length (octets) | Table 5-4 | M | Integer | No | Not implemented. No configurable maximum packet length at the data link layer. Packet length bounded by frame data field size. |
| TM-88 | Whether incomplete Packets are required to be delivered to the user at the receiving end | Table 5-4 | M | Required, not required | No | Not implemented. No incomplete packet delivery policy. |

### Table A-7: Protocol Specification with SDLS Option

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TM-89 | SDLS Protocol | (see ref. [10]) | O | No | SDLS Option not implemented. |
| TM-90 | Security Header | 6.3.4 | C3 | N/A | SDLS Option not implemented. |
| TM-91 | Transfer Frame Data Field in a TM Frame with SDLS | 6.3.5 | C3 | N/A | SDLS Option not implemented. |
| TM-92 | Security Trailer | 6.3.6 | C4 | N/A | SDLS Option not implemented. |
| TM-93 | Operational Control Field in a TM Frame with SDLS | 6.3.7.2 | C3 | N/A | SDLS Option not implemented. |
| TM-94 | Frame Error Control Field in a TM Frame with SDLS | 6.3.8.2 | C3 | N/A | SDLS Option not implemented. |
| TM-95 | Packet Processing Function with SDLS | 6.4.2.2 | C3 | N/A | SDLS Option not implemented. |
| TM-96 | Virtual Channel Generation Function with SDLS | 6.4.3.2, 6.4.3.3 | C3 | N/A | SDLS Option not implemented. |
| TM-97 | Virtual Channel Multiplexing Function with SDLS | 6.4.4.2 | C3 | N/A | SDLS Option not implemented. |
| TM-98 | Master Channel Multiplexing Function with SDLS | 6.4.6.2 | C3 | N/A | SDLS Option not implemented. |
| TM-99 | Error reporting | 6.5.2.2 | C4 | N/A | SDLS Option not implemented. |
| TM-100 | Packet Extraction Function with SDLS | 6.5.3.2 | C3 | N/A | SDLS Option not implemented. |
| TM-101 | Virtual Channel Reception Function with SDLS | 6.5.4.2, 6.5.4.3 | C3 | N/A | SDLS Option not implemented. |
| TM-102 | Virtual Channel Demultiplexing Function with SDLS | 6.5.5.2 | C3 | N/A | SDLS Option not implemented. |

**C3:** M if SDLS Option else N/A.
**C4:** O if SDLS Option else N/A.

### Table A-8: Additional Managed Parameters with SDLS Option

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| TM-103 | Presence of Space Data Link Security Header | Table 6-1 | C5 | Present ('1') / Absent ('0') | N/A | SDLS Option not implemented. |
| TM-104 | Presence of Space Data Link Security Trailer | Table 6-1 | C5 | Present ('1') / Absent ('0') | N/A | SDLS Option not implemented. |
| TM-105 | Length of Space Data Link Security Header (octets) | Table 6-1 | C5 | Integer (see ref. [10]) | N/A | SDLS Option not implemented. |
| TM-106 | Length of Space Data Link Security Trailer (octets) | Table 6-1 | C5 | Integer (see ref. [10]) | N/A | SDLS Option not implemented. |

**C5:** M if SDLS Option else N/A.

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Partial | Not Supported |
|----------|-------------|-----------|---------|---------------|
| Mandatory (M) | 72 | 58 | 0 | 14 |
| Optional (O) | 16 | 0 | 0 | 16 |
| Conditional (C2) | 2 | 0 | 0 | 0 (N/A) |
| Conditional (C3) | 11 | 0 | 0 | 0 (N/A) |
| Conditional (C4) | 2 | 0 | 0 | 0 (N/A) |
| Conditional (C5) | 4 | 0 | 0 | 0 (N/A) |
| **Total** | **107** | **58** | **0** | **14 + 16 optional + 19 N/A** |

### Non-Conformances (Mandatory Items Not Supported)

| Item | Description | Reason |
|------|-------------|--------|
| TM-8 | Packet Version Number | No PVN extraction or validation at the data link layer. |
| TM-12 | VCA Status Fields | No VCA status field extraction or delivery. |
| TM-60 | MC Multiplexing Function | No Master Channel multiplexing across physical channels. |
| TM-61 | All Frames Generation Function | No physical-channel-level frame generation (idle frame insertion, randomization). |
| TM-66 | MC Demultiplexing Function | No Master Channel demultiplexing across physical channels. |
| TM-67 | All Frames Reception Function | No physical-channel-level frame reception (de-randomization, sync). |
| TM-68 | Physical Channel Name | `ChannelConfig` provides a physical channel abstraction but does not include a name field. |
| TM-69 | Transfer Frame Length (octets) | `ChannelConfig.FrameLength` defined but not yet enforced during frame construction. |
| TM-72 | MC Multiplexing Scheme | No MC multiplexing scheme. |
| TM-86 | Valid PVNs | No Packet Version Number validation at the data link layer. |
| TM-87 | Maximum Packet Length (octets) | No configurable maximum packet length at the data link layer. |
| TM-88 | Incomplete Packet Delivery | No incomplete packet delivery policy. |

### Non-Supported Optional Items

| Item | Description | Reason |
|------|-------------|--------|
| TM-9 | Packet Quality Indicator | Not implemented. |
| TM-14 | VCA SDU Loss Flag | No frame loss detection mechanism. |
| TM-18 | FSH_SDU Loss Flag | No FSH SDU loss detection. |
| TM-21 | OCF SDU Frame Loss Flag | No OCF frame loss detection. |
| TM-24 | Frame Loss Flag (VC Frame) | No frame loss detection. |
| TM-27 | FSH_SDU Loss Flag (MC) | Not implemented. |
| TM-30 | OCF_SDU Loss Flag (MC) | Not implemented. |
| TM-33 | Frame Loss Flag (MC Frame) | Not implemented. |
| TM-89 | SDLS Protocol | SDLS Option not implemented. |

### Fully Supported Items

| Item | Description | Implementation |
|------|-------------|----------------|
| TM-1 | Packet SDU | `VirtualChannelPacketService` with `Send()` / `Receive()` via `VirtualChannel`. |
| TM-2 | VCA_SDU | `VirtualChannelAccessService` with fixed-size enforcement via `VirtualChannel`. |
| TM-3 | FSH_SDU | `SecondaryHeader.DataField` with encode/decode. |
| TM-4 | OCF_SDU | `TMTransferFrame.OperationalControl` — 4-byte field. |
| TM-5 | TM Transfer Frame | `TMTransferFrame` struct with encode/decode round-trip. |
| TM-6 | Packet | Packet data via `VirtualChannelPacketService.Send()` into `VirtualChannel`. |
| TM-7 | GVCID (VCP) | `PrimaryHeader.GVCID()`. |
| TM-11 | VCA SDU | `VirtualChannelAccessService.Send()` with `VCASize`. |
| TM-13 | GVCID (VCA) | `PrimaryHeader.GVCID()`. |
| TM-16 | FSH SDU | `SecondaryHeader.DataField`. |
| TM-17 | GVCID (VC FSH) | `PrimaryHeader.GVCID()`. |
| TM-19 | OCF SDU | `TMTransferFrame.OperationalControl`. |
| TM-20 | GVCID (VC OCF) | `PrimaryHeader.GVCID()`. |
| TM-22 | TM Frame (VCF) | `VirtualChannelFrameService` with `Send()` / `Receive()` via `VirtualChannel`. |
| TM-23 | GVCID (VCF) | `PrimaryHeader.GVCID()`. |
| TM-25 | FSH SDU (MC) | `SecondaryHeader.DataField`. |
| TM-26 | MCID (MC FSH) | `PrimaryHeader.MCID()`. |
| TM-28 | OCF SDU (MC) | `TMTransferFrame.OperationalControl`. |
| TM-29 | MCID (MC OCF) | `PrimaryHeader.MCID()`. |
| TM-31 | TM Frame (MCF) | `MasterChannel` with `AddFrame()` (routes to VCs by VCID) / `GetNextFrame()` (via Mux). |
| TM-32 | MCID (MCF) | `PrimaryHeader.MCID()` with SCID validation in `MasterChannel`. |
| TM-34 | VCP.request | `VirtualChannelPacketService.Send()` → `stampFrame()` → `VirtualChannel`. |
| TM-35 | VCP.indication | `VirtualChannelPacketService.Receive()` pulls from `VirtualChannel`. |
| TM-36 | VCA.request | `VirtualChannelAccessService.Send()` → `stampFrame()` → `VirtualChannel`. |
| TM-37 | VCA.indication | `VirtualChannelAccessService.Receive()` pulls from `VirtualChannel`. |
| TM-38 | VC_FSH.request | Secondary header via `NewTMTransferFrame()`. |
| TM-39 | VC_FSH.indication | Secondary header extracted in `DecodeTMTransferFrame()`. |
| TM-40 | VC_OCF.request | OCF via `NewTMTransferFrame()`. |
| TM-41 | VC_OCF.indication | OCF extracted in `DecodeTMTransferFrame()`. |
| TM-42 | VCF.request | `VirtualChannelFrameService.Send()` → `VirtualChannel`. |
| TM-43 | VCF.indication | `VirtualChannelFrameService.Receive()` pulls from `VirtualChannel`. |
| TM-44 | MC_FSH.request | Secondary header via `NewTMTransferFrame()`. |
| TM-45 | MC_FSH.indication | Secondary header in `DecodeTMTransferFrame()`. |
| TM-46 | MC_OCF.request | OCF via `NewTMTransferFrame()`. |
| TM-47 | MC_OCF.indication | OCF in `DecodeTMTransferFrame()`. |
| TM-48 | MCF.request | `MasterChannel.AddFrame()` routes to VCs by VCID. |
| TM-49 | MCF.indication | `MasterChannel.GetNextFrame()` via integrated Mux. |
| TM-50 | TM Transfer Frame | `TMTransferFrame` with encode/decode. |
| TM-51 | Transfer Frame Primary Header | `PrimaryHeader` — 6 octets, all CCSDS fields, validated. |
| TM-52 | Transfer Frame Secondary Header | `SecondaryHeader` with version/length validation. |
| TM-53 | Transfer Frame Data Field | `TMTransferFrame.DataField`. |
| TM-54 | Operational Control Field | 4-byte optional field, encode/decode support. |
| TM-55 | Frame Error Control Field | CRC-16-CCITT auto-compute and verify. |
| TM-56 | Packet Processing Function | `VirtualChannelPacketService.Send()` → `stampFrame()` → `VirtualChannel`. |
| TM-57 | VC Generation Function | `NewTMTransferFrame()` + `stampFrame()` with auto CRC. |
| TM-58 | VC Multiplexing Function | Weighted round-robin via `VirtualChannelMultiplexer`, integrated into `MasterChannel`. |
| TM-59 | MC Generation Function | `MasterChannel.AddFrame()` routes to VCs by VCID. |
| TM-62 | Packet Extraction Function | `VirtualChannelPacketService.Receive()` pulls from `VirtualChannel`. |
| TM-63 | VC Reception Function | `DecodeTMTransferFrame()` + `MasterChannel.AddFrame()` routes to `VirtualChannel`. |
| TM-64 | VC Demultiplexing Function | `MasterChannel.AddFrame()` demuxes by VCID + `TMServiceManager`. |
| TM-65 | MC Reception Function | `MasterChannel.GetNextFrame()` via integrated Mux. |
| TM-70 | TFVN | Enforced as `00` binary. |
| TM-71 | Valid Spacecraft IDs | 10 bits (0–1023), validated. |
| TM-73 | Presence of Frame Error Control | Always present (CRC-16-CCITT). |
| TM-74 | SCID (MC) | `MasterChannel.scid`, enforced in `AddFrame()`. |
| TM-75 | Valid VCIDs | 3 bits (0–7), `MasterChannel.channels` maps registered VCIDs. |
| TM-76 | VC Multiplexing Scheme | Weighted round-robin via `VirtualChannelMultiplexer` in `MasterChannel`. |
| TM-77 | Presence of MC_FSH | `PrimaryHeader.FSHFlag`. |
| TM-78 | MC_FSH Length | `SecondaryHeader.HeaderLength`. |
| TM-79 | Presence of MC_OCF | `PrimaryHeader.OCFFlag`. |
| TM-80 | SCID (VC) | `PrimaryHeader.SpacecraftID`. |
| TM-81 | VCID | `PrimaryHeader.VirtualChannelID`. |
| TM-82 | Data Field Content | `PrimaryHeader.SyncFlag` distinguishes Packets vs. VCA_SDU. |
| TM-83 | Presence of VC_FSH | `PrimaryHeader.FSHFlag`. |
| TM-84 | VC_FSH Length | `SecondaryHeader.HeaderLength`. |
| TM-85 | Presence of VC_OCF | `PrimaryHeader.OCFFlag`. |
