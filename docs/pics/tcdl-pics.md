# PICS PROFORMA FOR TC SPACE DATA LINK PROTOCOL

## Conformance Statement for `pkg/tcdl` — CCSDS 232.0-B-4

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 19/03/2026 |
| PICS Serial Number | ASTRO-TCDL-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/tcdl |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS TC Space Data Link Protocol. Full pipeline: PhysicalChannel (MC mux/demux) → MasterChannel (VC mux, frame gap detection) → VirtualChannel (frame buffer) → Services (MAP Packet with segmentation/reassembly, MAP Access, VC Frame). Variable frame length (up to 1024 bytes). Segment Header (MAP sublayer) support for large packet segmentation. Frame Sequence Number N(S) for COP-1 integration. |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/tcdl (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 232.0-B-4 (TC Space Data Link Protocol, Blue Book, Issue 4, October 2019) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE — Non-supported optional capabilities are identified in section A2.2 with explanations.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: TC Service Data Units

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TC-1 | MAP Packet SDU | 3.2.2 | M | Yes | `MAPPacketService` accepts packet data via `Send()`, segments across multiple frames when data exceeds capacity, pushes frames into `VirtualChannel`, and reassembles via `Receive()`. |
| TC-2 | MAP Access SDU | 3.2.3 | M | Yes | `MAPAccessService` accepts raw data via `Send()`, wraps in unsegmented frame with segment header, delivers via `Receive()`. |
| TC-3 | VC Frame SDU | 3.2.4 | M | Yes | `VCFrameService` accepts pre-encoded frame bytes via `Send()`, decodes and buffers, delivers encoded bytes via `Receive()`. |
| TC-4 | TC Transfer Frame | 3.2.5 | M | Yes | `TCTransferFrame` struct with `Encode()` / `DecodeTCTransferFrame()` round-trip support. Composed of Primary Header, optional Segment Header, Data Field, and Frame Error Control. |

### Table A-2: Service Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| | **MAP Packet Service Parameters** | | | | | |
| TC-5 | MAP Packet | 3.3.2.2 | M | — | Yes | Packet data passed as `[]byte` to `MAPPacketService.Send()`. Segmented if larger than frame capacity. |
| TC-6 | GVCID | 3.3.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()` (TFVN + SCID + VCID). VCID/SCID configured at service construction. |
| TC-7 | MAP ID | 3.3.2.4 | M | 0-63 | Yes | `SegmentHeader.MAPID` — 6 bits. Configured per service instance. |
| | **MAP Access Service Parameters** | | | | | |
| TC-8 | MAP Access SDU | 3.4.2.2 | M | — | Yes | Raw data passed to `MAPAccessService.Send()`. Wrapped in single unsegmented frame. |
| TC-9 | GVCID | 3.4.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. |
| TC-10 | MAP ID | 3.4.2.4 | M | 0-63 | Yes | `SegmentHeader.MAPID`. |
| | **VC Frame Service Parameters** | | | | | |
| TC-11 | TC Frame | 3.5.2.2 | M | — | Yes | `VCFrameService` accepts complete `[]byte` frame data via `Send()` / `Receive()`. |
| TC-12 | GVCID | 3.5.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. |

### Table A-3: Service Primitives

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| | **MAP Packet Service Primitives** | | | | |
| TC-13 | MAP_PACKET.request | 3.3.3.2 | M | Yes | `MAPPacketService.Send(data)` implements MAP_PACKET.request. Segments data across multiple frames using Segment Header sequence flags (First/Continuation/Last/Unsegmented). Each segment assigned sequence number via `FrameCounter.Next()`. |
| TC-14 | MAP_PACKET.indication | 3.3.3.3 | M | Yes | `MAPPacketService.Receive()` implements MAP_PACKET.indication. Reassembles segments by buffering First/Continuation data and delivering on Last segment. Skips orphaned Continuation/Last segments without a First. Requires `SetPacketSizer()`. |
| | **MAP Access Service Primitives** | | | | |
| TC-15 | MAP_ACCESS.request | 3.4.3.2 | M | Yes | `MAPAccessService.Send(data)` implements MAP_ACCESS.request. Wraps data in frame with unsegmented segment header. |
| TC-16 | MAP_ACCESS.indication | 3.4.3.3 | M | Yes | `MAPAccessService.Receive()` implements MAP_ACCESS.indication. Returns data field of next frame. |
| | **VC Frame Service Primitives** | | | | |
| TC-17 | VCF.request | 3.5.3.2 | M | Yes | `VCFrameService.Send(data)` decodes frame bytes and pushes into `VirtualChannel`. |
| TC-18 | VCF.indication | 3.5.3.3 | M | Yes | `VCFrameService.Receive()` pulls next frame and returns as encoded bytes. |

### Table A-4: TC Protocol Data Unit

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TC-19 | TC Transfer Frame | 4.1.1 | M | Yes | `TCTransferFrame` struct with `Encode()` / `DecodeTCTransferFrame()` round-trip. Variable length up to 1024 bytes. |
| TC-20 | Transfer Frame Primary Header | 4.1.2 | M | Yes | `PrimaryHeader` — 5 octets (40 bits). All fields per CCSDS: Transfer Frame Version Number (2 bits, enforced as `00`), Bypass Flag (1 bit), Control Command Flag (1 bit), Reserved (2 bits, enforced as `00`), Spacecraft ID (10 bits), Virtual Channel ID (6 bits), Frame Length (10 bits, total-1), Frame Sequence Number (8 bits). Big-endian encoding. |
| TC-21 | Segment Header | 4.1.4.1 | M | Yes | `SegmentHeader` — 1 octet: Sequence Flags (2 bits), MAP ID (6 bits). Present when MAP sublayer is used. `Encode()` / `Decode()` / `Validate()` methods. Constants: `SegUnsegmented`, `SegFirst`, `SegContinuation`, `SegLast`. |
| TC-22 | Transfer Frame Data Field | 4.1.4.2 | M | Yes | `TCTransferFrame.DataField` — variable-length telecommand payload. |
| TC-23 | Frame Error Control Field | 4.1.5 | M | Yes | `TCTransferFrame.FrameErrorControl` — 16-bit CRC-16-CCITT (polynomial 0x1021, init 0xFFFF). Auto-computed on construction via `NewTCTransferFrame()`. Verified on decode; CRC mismatch returns `ErrCRCMismatch`. |

### Table A-5: Protocol Procedures

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TC-24 | MAP Packet Processing Function | 4.2.2 | M | Yes | `MAPPacketService.Send()` segments packets into frames using Segment Header flags. Small packets produce unsegmented frames. Large packets use First/Continuation/Last segmentation. Frame Sequence Number assigned via `FrameCounter`. |
| TC-25 | MAP Access Processing Function | 4.2.3 | M | Yes | `MAPAccessService.Send()` wraps raw data in unsegmented frame with MAP sublayer. |
| TC-26 | VC Generation Function | 4.2.4 | M | Yes | `NewTCTransferFrame()` generates frames with SCID, VCID, data, optional Segment Header. CRC auto-computed. Frame Sequence Number configurable via `WithSequenceNumber()`. |
| TC-27 | VC Multiplexing Function | 4.2.5 | M | Yes | `VirtualChannelMultiplexer` schedules frames from multiple Virtual Channels using weighted round-robin. Integrated into `MasterChannel`. |
| TC-28 | MC Generation Function | 4.2.6 | M | Yes | `MasterChannel.AddFrame()` routes inbound frames to Virtual Channels by VCID with SCID validation. |
| TC-29 | MC Multiplexing Function | 4.2.7 | M | Yes | `PhysicalChannel` implements weighted round-robin MC multiplexing across registered `MasterChannel`s via `GetNextFrame()`. |
| TC-30 | MAP Packet Extraction Function | 4.3.2 | M | Yes | `MAPPacketService.Receive()` reassembles segmented packets from frames. Uses `PacketSizer` for packet boundary detection. Discards orphaned segments. |
| TC-31 | MAP Access Extraction Function | 4.3.3 | M | Yes | `MAPAccessService.Receive()` returns data field of next frame. |
| TC-32 | VC Reception Function | 4.3.4 | M | Yes | `DecodeTCTransferFrame()` parses raw octets into `TCTransferFrame`, verifying CRC and extracting all fields. `MasterChannel.AddFrame()` routes received frames to the appropriate `VirtualChannel` by VCID. |
| TC-33 | VC Demultiplexing Function | 4.3.5 | M | Yes | `MasterChannel.AddFrame()` demultiplexes inbound frames to Virtual Channels by VCID. |
| TC-34 | MC Reception Function | 4.3.6 | M | Yes | `MasterChannel.GetNextFrame()` pulls next frame from the multiplexer. |
| TC-35 | MC Demultiplexing Function | 4.3.7 | M | Yes | `PhysicalChannel.AddFrame()` demultiplexes inbound frames to the correct `MasterChannel` by SCID. |

### Table A-6: Management Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| | **Managed Parameters for a Physical Channel** | | | | | |
| TC-36 | Physical Channel Name | Table 5-1 | M | Character String | Yes | `PhysicalChannel.Name` — configured at construction via `NewPhysicalChannel(name)`. |
| TC-37 | Maximum Frame Length (octets) | Table 5-1 | M | Integer (up to 1024) | Yes | `MaxFrameLength` constant = 1024. Enforced in `NewTCTransferFrame()` via `ErrDataTooLarge`. |
| TC-38 | Transfer Frame Version Number (TFVN) | Table 5-1 | M | '00' binary | Yes | `PrimaryHeader.VersionNumber` — enforced as `0` in `Validate()`. |
| TC-39 | Valid Spacecraft IDs | Table 5-1 | M | Integers | Yes | `PrimaryHeader.SpacecraftID` — 10 bits (0-1023). Validated in `Validate()`. |
| TC-40 | MC Multiplexing Scheme | Table 5-1 | M | Mission Specific | Yes | `PhysicalChannel` implements weighted round-robin MC multiplexing. |
| TC-41 | Presence of Frame Error Control | Table 5-1 | M | Present ('1') | Yes | Always present. CRC-16-CCITT auto-computed and verified. |
| | **Managed Parameters for a Master Channel** | | | | | |
| TC-42 | SCID | Table 5-2 | M | Integer | Yes | `MasterChannel.scid` — configured at construction. Enforced in `AddFrame()`. |
| TC-43 | Valid VCIDs | Table 5-2 | M | Selectable set of integers (0-63) | Yes | `PrimaryHeader.VirtualChannelID` — 6 bits (0-63). `MasterChannel.channels` maps registered VCIDs. |
| TC-44 | VC Multiplexing Scheme | Table 5-2 | M | Mission Specific | Yes | `VirtualChannelMultiplexer` implements weighted round-robin scheduling. |
| | **Managed Parameters for a Virtual Channel** | | | | | |
| TC-45 | SCID | Table 5-3 | M | Integer | Yes | `PrimaryHeader.SpacecraftID`. Set via `NewTCTransferFrame()`. |
| TC-46 | VCID | Table 5-3 | M | 0 to 63 | Yes | `PrimaryHeader.VirtualChannelID`. `VirtualChannel.ID` configured at construction. |
| TC-47 | Presence of Segment Header | Table 5-3 | M | Present ('1') / Absent ('0') | Yes | Controlled via `WithSegmentHeader()` option. `SegmentHeader` struct with encode/decode. |
| TC-48 | Service Type | Table 5-3 | M | MAP Packet, MAP Access, or VC Frame | Yes | Three service implementations: `MAPPacketService`, `MAPAccessService`, `VCFrameService`. Registered via `TCServiceManager.RegisterVirtualService()`. |

### Table A-7: Protocol Specification with SDLS Option

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TC-49 | SDLS Protocol | (see ref. [8]) | O | No | SDLS Option not implemented. |

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Not Supported |
|----------|-------------|-----------|---------------|
| Mandatory (M) | 48 | 48 | 0 |
| Optional (O) | 1 | 0 | 1 |
| **Total** | **49** | **48** | **1** |

### Non-Conformances (Mandatory Items Not Supported)

None. All 48 mandatory items are fully supported.

### Non-Supported Optional Items

| Item | Description | Reason |
|------|-------------|--------|
| TC-49 | SDLS Protocol | SDLS Option not implemented. |

### Fully Supported Mandatory Items

All 48 mandatory items (TC-1 through TC-48) are supported. Key implementations:

| Area | Items | Implementation |
|------|-------|----------------|
| Service Data Units | TC-1–4 | `TCTransferFrame` encode/decode, `MAPPacketService`, `MAPAccessService`, `VCFrameService`. |
| MAP Packet Service | TC-5–7, TC-13–14 | `MAPPacketService` with segmentation (First/Continuation/Last/Unsegmented), `PacketSizer`-based reassembly. |
| MAP Access Service | TC-8–10, TC-15–16 | `MAPAccessService` with unsegmented frame wrapping. |
| VC Frame Service | TC-11–12, TC-17–18 | `VCFrameService` pass-through via `VirtualChannel`. |
| Protocol Data Unit | TC-19–23 | `PrimaryHeader` (40-bit), `SegmentHeader` (8-bit), CRC-16-CCITT. |
| Packet Processing | TC-24–25, TC-30–31 | MAP Packet segmentation/reassembly, MAP Access raw delivery. |
| VC Functions | TC-26–27, TC-32–33 | `NewTCTransferFrame()`, `VirtualChannelMultiplexer` (weighted round-robin), `MasterChannel` demux by VCID. |
| MC Functions | TC-28–29, TC-34–35 | `MasterChannel.AddFrame()` routes by VCID. `PhysicalChannel` MC mux/demux by SCID. |
| Management Params | TC-36–48 | Physical channel name, max frame length 1024, TFVN enforced, SCID/VCID validated, segment header support, all service types. |
