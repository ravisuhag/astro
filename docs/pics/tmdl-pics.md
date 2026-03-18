# PICS PROFORMA FOR TM SPACE DATA LINK PROTOCOL

## Conformance Statement for `pkg/tmdl` — CCSDS 132.0-B-3

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 18/03/2026 |
| PICS Serial Number | ASTRO-TMDL-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/tmdl |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS TM Space Data Link Protocol. Full pipeline: PhysicalChannel (ASM, randomization, MC mux/demux) → MasterChannel (VC mux, frame gap detection) → VirtualChannel (single frame buffer) → Services (VCP with segmentation/reassembly, VCA with status fields, VCF). Fixed frame length enforcement, idle frame insertion, PVN validation, and CCSDS pseudo-randomization supported. |

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
| Have any exceptions been required? | Yes [ ] No [X] |

NOTE — All mandatory capabilities defined in the Recommended Standard are supported.
Optional items not supported: TM-9 (Packet Quality Indicator), TM-89 (SDLS Protocol).

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
| TM-8 | Packet Version Number | 3.3.2.4 | M | — | Yes | `VirtualChannelPacketService.SetValidPVNs()` configures accepted PVNs. `Send()` validates the first 3 bits of packet data against the set. |
| TM-9 | Packet Quality Indicator | 3.3.2.5 | O | — | No | Not implemented. |
| TM-10 | Verification Status Code | 3.3.2.6 | C2 | (see reference [10]) | N/A | SDLS Option not implemented. |
| | **VCA SDU Service Parameters** | | | | | |
| TM-11 | VCA SDU | 3.4.2.2 | M | — | Yes | Fixed-length data passed to `VirtualChannelAccessService.Send()`. Size enforced against `vcaSize`. Frame pushed into `VirtualChannel`. |
| TM-12 | VCA Status Fields | 3.4.2.3 | M | — | Yes | `VirtualChannelAccessService.LastStatus()` returns `VCAStatus{SyncFlag, PacketOrderFlag, SegmentLengthID}` from the last received frame. |
| TM-13 | GVCID | 3.4.2.4 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. VCID configured at service construction. |
| TM-14 | VCA SDU Loss Flag | 3.4.2.5 | O | — | Yes | `FrameGapDetector` tracks VC frame count gaps. `MasterChannel.VCFrameGap()` returns gap count after each `AddFrame()`. |
| TM-15 | Verification Status Code | 3.4.2.6 | C2 | (see reference [10]) | N/A | SDLS Option not implemented. |
| | **VC FSH SDU Service Parameters** | | | | | |
| TM-16 | FSH SDU | 3.5.2.2 | M | — | Yes | `SecondaryHeader.DataField` carries the FSH SDU. Encoded/decoded via `SecondaryHeader.Encode()` / `Decode()`. |
| TM-17 | GVCID | 3.5.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. |
| TM-18 | FSH_SDU Loss Flag | 3.5.2.4 | O | — | Yes | `FrameGapDetector` integrated into `MasterChannel.AddFrame()`. Gap detected via MC/VC frame count tracking. |
| | **OCF SDU Service Parameters** | | | | | |
| TM-19 | OCF SDU | 3.6.2.2 | M | — | Yes | `TMTransferFrame.OperationalControl` — 4-byte field. |
| TM-20 | GVCID | 3.6.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. |
| TM-21 | OCF SDU Frame Loss Flag | 3.6.2.4 | O | — | Yes | `FrameGapDetector` integrated into `MasterChannel.AddFrame()`. Gap detected via MC/VC frame count tracking. |
| | **VC Frame Service Parameters** | | | | | |
| TM-22 | TM Frame | 3.7.2.2 | M | — | Yes | `VirtualChannelFrameService` accepts and delivers complete `*TMTransferFrame` objects via `Send()` / `Receive()`. |
| TM-23 | GVCID | 3.7.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. |
| TM-24 | Frame Loss Flag | 3.7.2.4 | O | — | Yes | `FrameGapDetector` integrated into `MasterChannel.AddFrame()`. Gap detected via MC/VC frame count tracking. |
| | **MC FSH Service Parameters** | | | | | |
| TM-25 | FSH SDU | 3.8.2.2 | M | — | Yes | `SecondaryHeader.DataField` at Master Channel level. |
| TM-26 | MCID | 3.8.2.3 | M | — | Yes | `PrimaryHeader.MCID()` returns TFVN + SCID. |
| TM-27 | FSH_SDU Loss Flag | 3.8.2.4 | O | — | Yes | `MasterChannel.MCFrameGap()` detects MC-level frame count gaps via `FrameGapDetector`. |
| | **MC OCF Service Parameters** | | | | | |
| TM-28 | OCF SDU | 3.9.2.2 | M | — | Yes | `TMTransferFrame.OperationalControl` at Master Channel level. |
| TM-29 | MCID | 3.9.2.3 | M | — | Yes | `PrimaryHeader.MCID()`. |
| TM-30 | OCF_SDU Loss Flag | 3.9.2.4 | O | — | Yes | `MasterChannel.MCFrameGap()` detects MC-level frame count gaps via `FrameGapDetector`. |
| | **MC Frame Service Parameters** | | | | | |
| TM-31 | TM Frame | 3.10.2.2 | M | — | Yes | `MasterChannel` manages complete `*TMTransferFrame` objects. `AddFrame()` routes inbound frames to Virtual Channels by VCID. `GetNextFrame()` pulls from the integrated multiplexer. SCID matching enforced. |
| TM-32 | MCID | 3.10.2.3 | M | — | Yes | `PrimaryHeader.MCID()`. SCID validated in `MasterChannel.AddFrame()`. |
| TM-33 | Frame Loss Flag | 3.10.2.4 | O | — | Yes | `MasterChannel.MCFrameGap()` and `VCFrameGap()` detect frame count gaps via `FrameGapDetector`. |

**C2:** O if SDLS Option else N/A.

### Table A-3: Service Primitives

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| | **VCP Service Primitives** | | | | |
| TM-34 | VCP.request | 3.3.3.2 | M | Yes | `VirtualChannelPacketService.Send(data)` implements VCP.request. When `ChannelConfig` is set, segments packets across fixed-length frames with 2-byte length prefix; FirstHeaderPtr=0 for first frame, 0x07FE for continuations. Validates PVN when configured via `SetValidPVNs()`. |
| TM-35 | VCP.indication | 3.3.3.3 | M | Yes | `VirtualChannelPacketService.Receive()` implements VCP.indication. When `ChannelConfig` is set, reassembles segmented packets using length prefix, skipping idle frames. |
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
| TM-56 | Packet Processing Function | 4.2.2 | M | Yes | `VirtualChannelPacketService.Send()` accepts packet data, validates PVN if configured, segments across fixed-length frames when `ChannelConfig` is set (2-byte length prefix, idle-padded data fields, FirstHeaderPtr management), and pushes frames into the `VirtualChannel`. |
| TM-57 | VC Generation Function | 4.2.3 | M | Yes | `NewTMTransferFrame()` generates frames with SCID, VCID, data, optional secondary header, and optional OCF. CRC auto-computed. Frame counts applied by `stampFrame()` when a `FrameCounter` is provided. |
| TM-58 | VC Multiplexing Function | 4.2.4 | M | Yes | `VirtualChannelMultiplexer` schedules frames from multiple Virtual Channels using weighted round-robin via `GetNextFrame()`. Integrated into `MasterChannel`. |
| TM-59 | MC Generation Function | 4.2.5 | M | Yes | `MasterChannel.AddFrame()` routes inbound frames to Virtual Channels by VCID with SCID validation. |
| TM-60 | MC Multiplexing Function | 4.2.6 | M | Yes | `PhysicalChannel` implements weighted round-robin MC multiplexing across registered `MasterChannel`s via `GetNextFrame()`. |
| TM-61 | All Frames Generation Function | 4.2.7 | M | Yes | `PhysicalChannel.Wrap()` produces CADUs: encodes frame, applies optional CCSDS pseudo-randomization (LFSR x^8+x^7+x^5+x^3+1), and prepends ASM (default 0x1ACFFC1D). `GetNextFrameOrIdle()` inserts idle frames when no MC has data. |
| TM-62 | Packet Extraction Function | 4.3.2 | M | Yes | `VirtualChannelPacketService.Receive()` reassembles segmented packets using 2-byte length prefix when `ChannelConfig` is set, skipping idle frames. |
| TM-63 | VC Reception Function | 4.3.3 | M | Yes | `DecodeTMTransferFrame()` parses raw octets into a `TMTransferFrame`, verifying CRC and extracting all fields. `MasterChannel.AddFrame()` routes received frames to the appropriate `VirtualChannel` by VCID. |
| TM-64 | VC Demultiplexing Function | 4.3.4 | M | Yes | `MasterChannel.AddFrame()` demultiplexes inbound frames to Virtual Channels by VCID. `TMServiceManager` dispatches to the correct VC service. |
| TM-65 | MC Reception Function | 4.3.5 | M | Yes | `MasterChannel.GetNextFrame()` pulls the next frame from the integrated multiplexer. |
| TM-66 | MC Demultiplexing Function | 4.3.6 | M | Yes | `PhysicalChannel.AddFrame()` demultiplexes inbound frames to the correct `MasterChannel` by SCID. |
| TM-67 | All Frames Reception Function | 4.3.7 | M | Yes | `PhysicalChannel.Unwrap()` strips ASM, applies optional de-randomization, and decodes the frame. |

### Table A-6: Management Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| | **Managed Parameters for a Physical Channel** | | | | | |
| TM-68 | Physical Channel Name | Table 5-1 | M | Character String | Yes | `PhysicalChannel.Name` — configured at construction via `NewPhysicalChannel(name, config)`. |
| TM-69 | Transfer Frame Length (octets) | Table 5-1 | M | Integer | Yes | `ChannelConfig.FrameLength` defines the fixed frame length. Enforced by VCP (segmentation + padding) and VCA (padding) during frame construction. `DataFieldCapacity()` computes available data space. |
| TM-70 | Transfer Frame Version Number (TFVN) | Table 5-1 | M | '00' binary | Yes | `PrimaryHeader.VersionNumber` — enforced as `0` in `Validate()`. |
| TM-71 | Valid Spacecraft IDs | Table 5-1 | M | Integers | Yes | `PrimaryHeader.SpacecraftID` — 10 bits (0–1023). Validated in `Validate()`. Configurable per frame via `NewTMTransferFrame()`. |
| TM-72 | MC Multiplexing Scheme | Table 5-1 | M | Mission Specific | Yes | `PhysicalChannel` implements weighted round-robin MC multiplexing. Priority weights configured per `MasterChannel` via `AddMasterChannel()`. |
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
| TM-86 | Valid PVNs | Table 5-4 | M | Set of Integers | Yes | `VirtualChannelPacketService.SetValidPVNs()` configures the accepted PVN set. `Send()` validates the first 3 bits of packet data. Empty set disables validation. |
| TM-87 | Maximum Packet Length (octets) | Table 5-4 | M | Integer | Yes | Maximum packet length is 65535 bytes, enforced by the 2-byte length prefix in VCP segmentation. Per-frame capacity derived from `ChannelConfig.DataFieldCapacity()`. |
| TM-88 | Whether incomplete Packets are required to be delivered to the user at the receiving end | Table 5-4 | M | Required, not required | Yes | Policy: not required. `VirtualChannelPacketService.Receive()` always delivers complete reassembled packets. Incomplete packets (insufficient continuation frames) return `ErrIncompletePacket`. |

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

| Category | Total Items | Supported | Not Supported |
|----------|-------------|-----------|---------------|
| Mandatory (M) | 78 | 78 | 0 |
| Optional (O) | 9 | 7 | 2 |
| Conditional (C2) | 2 | 0 | 0 (N/A) |
| Conditional (C3) | 11 | 0 | 0 (N/A) |
| Conditional (C4) | 2 | 0 | 0 (N/A) |
| Conditional (C5) | 4 | 0 | 0 (N/A) |
| **Total** | **106** | **85** | **2 + 19 N/A** |

### Non-Conformances (Mandatory Items Not Supported)

None. All 78 mandatory items are fully supported.

### Non-Supported Optional Items

| Item | Description | Reason |
|------|-------------|--------|
| TM-9 | Packet Quality Indicator | Not implemented. No packet quality/confidence reporting. |
| TM-89 | SDLS Protocol | SDLS Option not implemented. Planned for future phase. |

### Supported Optional Items

| Item | Description | Implementation |
|------|-------------|----------------|
| TM-14 | VCA SDU Loss Flag | `FrameGapDetector` via `MasterChannel.VCFrameGap()`. |
| TM-18 | FSH_SDU Loss Flag | `FrameGapDetector` via `MasterChannel.MCFrameGap()` / `VCFrameGap()`. |
| TM-21 | OCF SDU Frame Loss Flag | `FrameGapDetector` via `MasterChannel.MCFrameGap()` / `VCFrameGap()`. |
| TM-24 | Frame Loss Flag (VC Frame) | `FrameGapDetector` via `MasterChannel.VCFrameGap()`. |
| TM-27 | FSH_SDU Loss Flag (MC) | `FrameGapDetector` via `MasterChannel.MCFrameGap()`. |
| TM-30 | OCF_SDU Loss Flag (MC) | `FrameGapDetector` via `MasterChannel.MCFrameGap()`. |
| TM-33 | Frame Loss Flag (MC Frame) | `FrameGapDetector` via `MasterChannel.MCFrameGap()` / `VCFrameGap()`. |

### Fully Supported Mandatory Items

All 78 mandatory items (TM-1 through TM-88, excluding optional/conditional) are supported. Key implementations:

| Area | Items | Implementation |
|------|-------|----------------|
| Service Data Units | TM-1–5 | `TMTransferFrame` encode/decode, `SecondaryHeader`, `OperationalControl`. |
| VCP Service | TM-6–8, TM-34–35 | `VirtualChannelPacketService` with segmentation/reassembly, PVN validation via `SetValidPVNs()`. |
| VCA Service | TM-11–13, TM-36–37 | `VirtualChannelAccessService` with padding, `LastStatus()` for status fields. |
| VCF Service | TM-22–23, TM-42–43 | `VirtualChannelFrameService` with encode/decode via `VirtualChannel`. |
| FSH/OCF Services | TM-16–20, TM-25–32, TM-38–49 | Secondary header and OCF via `NewTMTransferFrame()` / `DecodeTMTransferFrame()`. `MasterChannel` with SCID validation. |
| Protocol Data Unit | TM-50–55 | `PrimaryHeader` (48-bit), `SecondaryHeader`, CRC-16-CCITT. |
| Packet Processing | TM-56, TM-62 | VCP segmentation with 2-byte length prefix, reassembly with idle frame skipping. |
| VC Functions | TM-57–58, TM-63–64 | `NewTMTransferFrame()`, `VirtualChannelMultiplexer` (weighted round-robin), `MasterChannel` demux by VCID. |
| MC Functions | TM-59–60, TM-65–66 | `MasterChannel.AddFrame()` routes by VCID. `PhysicalChannel` MC mux/demux by SCID. |
| Physical Channel | TM-61, TM-67–69, TM-72 | `PhysicalChannel` with `Wrap()`/`Unwrap()` (ASM + randomization), `Name`, `ChannelConfig.FrameLength`, MC multiplexing scheme. |
| Management Params | TM-70–88 | TFVN enforced, SCID/VCID validated, SyncFlag, FSHFlag, OCFFlag, PVN validation, max packet 65535, complete packet delivery. |
