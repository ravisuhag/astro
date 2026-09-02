---
title: TM Space Data Link Protocol
description: "PICS proforma: what this package implements, clause by clause."
order: 60
---

## Conformance Statement for `pkg/tmdl` — CCSDS 132.0-B-3

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 24/08/2026 |
| PICS Serial Number | ASTRO-TMDL-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/tmdl |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS TM Space Data Link Protocol. Full pipeline: PhysicalChannel (MC mux/demux) → MasterChannel (VC mux, MC_FSH/MC_OCF insertion, frame gap detection) → VirtualChannel (single frame buffer) → Services (VCP with native multi-packet packing via FHP and PacketSizer-based reassembly with loss resync, VCA with user-set status fields, VC_FSH and VC_OCF suppliers, VCF). Fixed frame length enforcement and conformant OID idle frames (mandatory PN fill, packet-carrying VCID). The sync layer (ASM, CCSDS pseudo-randomization, CADU wrapping/unwrapping) is handled by the separate `pkg/tmsc` package. |

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

NOTE — Every mandatory item is supported: the frame codec, all seven
transfer services (VCP, VCA, VC_FSH, VC_OCF, VCF, MC_FSH, MC_OCF, MCF), and
the protocol procedures at both ends. Optional items not supported: TM-9
(Packet Quality Indicator), TM-89 (SDLS Protocol).

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
| TM-8 | Packet Version Number | 3.3.2.4 | M | — | Yes | PVN validation was removed from the VCP service; it is delegated to the application layer. The first 3 bits of packet data carry the PVN per CCSDS. |
| TM-9 | Packet Quality Indicator | 3.3.2.5 | O | — | No | Not implemented. |
| TM-10 | Verification Status Code | 3.3.2.6 | C2 | (see reference [10]) | N/A | SDLS Option not implemented. |
| | **VCA SDU Service Parameters** | | | | | |
| TM-11 | VCA SDU | 3.4.2.2 | M | — | Yes | Fixed-length data passed to `VirtualChannelAccessService.Send()`. Size enforced against `vcaSize`. Frame pushed into `VirtualChannel`. |
| TM-12 | VCA Status Fields | 3.4.2.3 | M | — | Yes | Both directions. Sending: `VirtualChannelAccessService.SetSendStatus(VCAStatus{...})` sets the Packet Order Flag, Segment Length ID, and First Header Pointer carried by subsequent `Send()` calls — the mandatory VCA Status Fields parameter of the VCA.request primitive (§3.4.3.2.2), whose semantics are the user's. Receiving: `LastStatus()` returns `VCAStatus{SyncFlag, PacketOrderFlag, SegmentLengthID, FirstHeaderPtr}` from the last received frame. The Synchronization Flag is not a user field: §4.1.2.7.3.2 fixes it at '1' for a VCA_SDU and `Send()` always sets it. |
| TM-13 | GVCID | 3.4.2.4 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. VCID configured at service construction. |
| TM-14 | VCA SDU Loss Flag | 3.4.2.5 | O | — | Yes | `FrameGapDetector` tracks VC frame count gaps. `MasterChannel.VCFrameGap()` returns gap count after each `AddFrame()`. |
| TM-15 | Verification Status Code | 3.4.2.6 | C2 | (see reference [10]) | N/A | SDLS Option not implemented. |
| | **VC FSH SDU Service Parameters** | | | | | |
| TM-16 | FSH SDU | 3.5.2.2 | M | — | Yes | `SecondaryHeader.DataField` carries the FSH_SDU on the wire. The VC_FSH service user installs it via `SetFSHSupplier()` on the VCP or VCA service, which fills the header of every frame that service emits (§3.5.1 makes the transfer synchronous with frame release), and reads it back with `LastFSH()`. Width is the channel's fixed `ChannelConfig.FSHDataLength` (§4.1.3.1.6). |
| TM-17 | GVCID | 3.5.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. |
| TM-18 | FSH_SDU Loss Flag | 3.5.2.4 | O | — | Yes | `MasterChannel.VCFrameGap()` reports the Virtual Channel Frame Count gap, which §3.5.2.4 names as the derivation for this flag; read it alongside the service's `LastFSH()`. |
| | **OCF SDU Service Parameters** | | | | | |
| TM-19 | OCF SDU | 3.6.2.2 | M | — | Yes | `TMTransferFrame.OperationalControl` — 4-byte field. |
| TM-20 | GVCID | 3.6.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. |
| TM-21 | OCF SDU Frame Loss Flag | 3.6.2.4 | O | — | Yes | `FrameGapDetector` integrated into `MasterChannel.AddFrame()`. Gap detected via MC/VC frame count tracking. |
| | **VC Frame Service Parameters** | | | | | |
| TM-22 | TM Frame | 3.7.2.2 | M | — | Yes | `VirtualChannelFrameService` accepts and delivers complete `*TMTransferFrame` objects via `Send()` / `Receive()`. |
| TM-23 | GVCID | 3.7.2.3 | M | — | Yes | Derived from `PrimaryHeader.GVCID()`. |
| TM-24 | Frame Loss Flag | 3.7.2.4 | O | — | Yes | `FrameGapDetector` integrated into `MasterChannel.AddFrame()`. Gap detected via MC/VC frame count tracking. |
| | **MC FSH Service Parameters** | | | | | |
| TM-25 | FSH SDU | 3.8.2.2 | M | — | Yes | `MasterChannel.SetFSHSupplier()` installs the MC_FSH service user: its FSH_SDU is placed into the Transfer Frame Secondary Header of every frame released through the master channel (§4.2.5.2), idle frames included. `MasterChannel.LastFSH()` delivers the SDU decommutated from the most recently received frame (§4.3.5.2). The SDU width is the channel's fixed `ChannelConfig.FSHDataLength` (§4.1.3.1.6); a mismatch is refused with `ErrFSHSizeMismatch` rather than silently truncated. |
| TM-26 | MCID | 3.8.2.3 | M | — | Yes | `PrimaryHeader.MCID()` returns TFVN + SCID. It is the SAP address of the MC_FSH service: a `MasterChannel` is created for one SCID and rejects frames from another with `ErrSCIDMismatch`, so its FSH supplier and `LastFSH()` are scoped to that MCID. |
| TM-27 | FSH_SDU Loss Flag | 3.8.2.4 | O | — | Yes | `MasterChannel.MCFrameGap()` reports the Master Channel Frame Count gap after each `AddFrame`, which §3.8.2.4 names as the way to derive the flag; read it alongside `LastFSH()`. |
| | **MC OCF Service Parameters** | | | | | |
| TM-28 | OCF SDU | 3.9.2.2 | M | — | Yes | `MasterChannel.SetOCFSupplier()` installs the MC_OCF service user: its 4-octet OCF_SDU is placed into the Operational Control Field of every frame released through the master channel (§4.2.5.3), idle frames included. `MasterChannel.LastOCF()` delivers the SDU from the most recently received frame (§4.3.5.2). Per-virtual-channel OCF remains available via the services' own `SetOCFSupplier` (VC_OCF, §3.6). |
| TM-29 | MCID | 3.9.2.3 | M | — | Yes | `PrimaryHeader.MCID()`, scoped as the MC_OCF service's SAP address by the owning `MasterChannel` (see TM-26). |
| TM-30 | OCF_SDU Loss Flag | 3.9.2.4 | O | — | Yes | `MasterChannel.MCFrameGap()` reports the Master Channel Frame Count gap after each `AddFrame` (§3.9.2.4); read it alongside `LastOCF()`. |
| | **MC Frame Service Parameters** | | | | | |
| TM-31 | TM Frame | 3.10.2.2 | M | — | Yes | Realized by `MasterChannel` rather than a `Service` type: `AddFrame()` routes inbound frames to Virtual Channels by VCID, `GetNextFrame()` pulls from the integrated multiplexer, SCID matching enforced. |
| TM-32 | MCID | 3.10.2.3 | M | — | Yes | `PrimaryHeader.MCID()`. SCID validated in `MasterChannel.AddFrame()`. |
| TM-33 | Frame Loss Flag | 3.10.2.4 | O | — | Yes | `MasterChannel.MCFrameGap()` and `VCFrameGap()` detect frame count gaps via `FrameGapDetector`. |

**C2:** O if SDLS Option else N/A.

### Table A-3: Service Primitives

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| | **VCP Service Primitives** | | | | |
| TM-34 | VCP.request | 3.3.3.2 | M | Yes | `VirtualChannelPacketService.Send(data)` implements VCP.request. When `ChannelConfig` is set, packs packets into fixed-length frames using native FHP (multi-packet packing: tail-of-previous + start-of-next in same frame). `Flush()` fills the remaining space with an SPP idle packet and emits the resulting frame(s). |
| TM-35 | VCP.indication | 3.3.3.3 | M | Yes | `VirtualChannelPacketService.Receive()` implements VCP.indication. When `ChannelConfig` is set, uses FHP to locate packet boundaries and `PacketSizer` (via `spp.PacketSizer`) to extract complete packets. `SetPacketSizer` must be called explicitly. Resyncs via FHP after VC frame count gaps. Skips idle frames and discards extracted idle packets (APID 0x7FF). |
| | **VCA Service Primitives** | | | | |
| TM-36 | VCA.request | 3.4.3.2 | M | Yes | `VirtualChannelAccessService.Send(data)` implements VCA.request. Enforces fixed `vcaSize`, constructs a frame, stamps counters/CRC via `stampFrame()`, and pushes it into the `VirtualChannel`. |
| TM-37 | VCA.indication | 3.4.3.3 | M | Yes | `VirtualChannelAccessService.Receive()` implements VCA.indication. Pulls the next frame from the `VirtualChannel` and returns its fixed-length Data Field. |
| | **VC FSH Service Primitives** | | | | |
| TM-38 | VC_FSH.request | 3.5.3.2 | M | Yes | `SetFSHSupplier()` on the VCP or VCA service is the VC_FSH.request path: the supplier is polled as each frame is built and its FSH_SDU written into the secondary header, giving the synchronous transfer of §3.5.1. Direct per-frame construction stays available via `NewTMTransferFrame(..., secondaryHeaderData, ...)`, which sets the FSH flag automatically. |
| TM-39 | VC_FSH.indication | 3.5.3.3 | M | Yes | `DecodeTMTransferFrame()` extracts the secondary header when the FSH flag is set; the VCP and VCA services deliver its SDU through `LastFSH()` as they decommutate each frame (§4.3.3.3), including on OID frames whose secondary header can still carry valid data (§4.1.4.6.3 note 1). |
| | **VC OCF Service Primitives** | | | | |
| TM-40 | VC_OCF.request | 3.6.3.2 | M | Yes | OCF data passed via `NewTMTransferFrame(..., ocf)`. OCFFlag auto-set when OCF is present. |
| TM-41 | VC_OCF.indication | 3.6.3.3 | M | Yes | `DecodeTMTransferFrame()` extracts 4-byte OCF when OCFFlag is set. |
| | **VC Frame Service Primitives** | | | | |
| TM-42 | VCF.request | 3.7.3.2 | M | Yes | `VirtualChannelFrameService.Send(data)` decodes frame bytes and pushes the frame into the `VirtualChannel`. |
| TM-43 | VCF.indication | 3.7.3.3 | M | Yes | `VirtualChannelFrameService.Receive()` pulls the next frame from the `VirtualChannel` and returns it as encoded bytes. |
| | **MC FSH Service Primitives** | | | | |
| TM-44 | MC_FSH.request | 3.8.3.2 | M | Yes | `MasterChannel.SetFSHSupplier()` is the MC_FSH.request path: the supplier is polled at frame release and its FSH_SDU written into every frame of the master channel, which is the synchronous transfer §3.8.1 describes. |
| TM-45 | MC_FSH.indication | 3.8.3.3 | M | Yes | `MasterChannel.AddFrame()` decommutates the secondary header per §4.3.5.2 and `MasterChannel.LastFSH()` delivers the FSH_SDU, with `MCFrameGap()` supplying the optional loss flag. |
| | **MC OCF Service Primitives** | | | | |
| TM-46 | MC_OCF.request | 3.9.3.2 | M | Yes | `MasterChannel.SetOCFSupplier()` is the MC_OCF.request path: the supplier is polled at frame release and its OCF_SDU written into every frame of the master channel. |
| TM-47 | MC_OCF.indication | 3.9.3.3 | M | Yes | `MasterChannel.AddFrame()` decommutates the Operational Control Field per §4.3.5.2 and `MasterChannel.LastOCF()` delivers the OCF_SDU, with `MCFrameGap()` supplying the optional loss flag. |
| | **MC Frame Service Primitives** | | | | |
| TM-48 | MCF.request | 3.10.3.2 | M | Yes | Realized by `MasterChannel.AddFrame(frame)` rather than a `Service` type: accepts a `*TMTransferFrame` with SCID validation and routes it to the appropriate `VirtualChannel` by VCID. |
| TM-49 | MCF.indication | 3.10.3.3 | M | Yes | Realized by `MasterChannel.GetNextFrame()`, which pulls the next frame from the integrated `VirtualChannelMultiplexer`. |

### Table A-4: TM Protocol Data Unit

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TM-50 | TM Transfer Frame | 4.1.1 | M | Yes | `TMTransferFrame` struct with `Encode()` / `DecodeTMTransferFrame()` round-trip. |
| TM-51 | Transfer Frame Primary Header | 4.1.2 | M | Yes | `PrimaryHeader` — 6 octets (48 bits). All fields per CCSDS: Transfer Frame Version Number (2 bits, enforced as `00`), Spacecraft ID (10 bits), Virtual Channel ID (3 bits), OCF Flag (1 bit), MC Frame Count (8 bits), VC Frame Count (8 bits), Transfer Frame Data Field Status (16 bits). Big-endian encoding via `Encode()` / `Decode()`. Validated via `Validate()`. |
| TM-52 | Transfer Frame Secondary Header | 4.1.3 | M | Yes | `SecondaryHeader` struct: Version Number (2 bits, enforced as `00`), Header Length (6 bits, 0–63), Data Field (variable). `Encode()` / `Decode()` / `Validate()` methods. Presence controlled by FSHFlag. |
| TM-53 | Transfer Frame Data Field | 4.1.4 | M | Yes | `TMTransferFrame.DataField` — the payload, sized by `ChannelConfig.DataFieldCapacity()` as the fixed frame length minus the primary header, secondary header, and trailer (§4.1.4.2). OID frames (§4.1.4.6) carry the mandatory Pseudo Noise fill: `OIDSequence` implements the 32-cell LFSR of §4.1.4.6.2 with polynomial D0+D1+D2+D22+D32 and the 'all ones' seed, and each `MasterChannel` keeps one generator for its lifetime so the sequence is never restarted between frames (§4.1.4.6.2.1). |
| TM-54 | Operational Control Field | 4.1.5 | M | Yes | `TMTransferFrame.OperationalControl` — 4 bytes (32 bits). Included when `OCFFlag` is set. Extracted during decode. |
| TM-55 | Frame Error Control Field | 4.1.6 | M | Yes | `TMTransferFrame.FrameErrorControl` — 16-bit CRC-16-CCITT (polynomial 0x1021, init 0xFFFF). Auto-computed on encode via `crc.ComputeCRC16()`. Verified on decode; CRC mismatch returns error. |

### Table A-5: Protocol Procedures

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TM-56 | Packet Processing Function | 4.2.2 | M | Yes | `VirtualChannelPacketService.Send()` accepts packet data and packs it contiguously across fixed-length frames when `ChannelConfig` is set, with native FirstHeaderPtr management. `Flush()` fills spare data field space with SPP idle packets (APID 0x7FF), spanning into following frames when the spare space is under the 7-octet minimum packet size. |
| TM-57 | VC Generation Function | 4.2.3 | M | Yes | `NewTMTransferFrame()` generates frames with SCID, VCID, data, optional secondary header, and optional OCF. CRC auto-computed. Frame counts applied by `stampFrame()` when a `FrameCounter` is provided. |
| TM-58 | VC Multiplexing Function | 4.2.4 | M | Yes | `VirtualChannelMultiplexer` schedules frames from multiple Virtual Channels using weighted round-robin via `GetNextFrame()`. Integrated into `MasterChannel`. `GetNextFrameOrIdle()` creates the OID Transfer Frame §4.2.4.4 requires when no valid frame is available at release time: First Header Pointer '11111111110', a VCID that carries packets (§4.1.4.6.3, the lowest registered channel or the one pinned by `SetIdleVCID()`), a PN-filled data field, and MC/VC frame counts continuing the channel's sequence. |
| TM-59 | MC Generation Function | 4.2.5 | M | Yes | `MasterChannel.GetNextFrame()` / `GetNextFrameOrIdle()` insert the MC_FSH and MC_OCF service data units into every frame released through the master channel (§4.2.5.2, §4.2.5.3) and refresh the frame error control field over the result, then hand the frame on. The Master Channel Frame Count is generated by the shared `FrameCounter`. |
| TM-60 | MC Multiplexing Function | 4.2.6 | M | Yes | `PhysicalChannel` implements weighted round-robin MC multiplexing across registered `MasterChannel`s via `GetNextFrame()`. `GetNextFrameOrIdle()` creates the OID Transfer Frame of §4.2.6.4 when no master channel has a frame ready, choosing the lowest registered SCID so the result is deterministic. |
| TM-61 | All Frames Generation Function | 4.2.7 | M | Yes | `PhysicalChannel.GetNextFrameOrIdle()` keeps the transmitted stream continuous, delegating idle frame creation to the chosen `MasterChannel` so the OID frame carries that channel's counts, PN fill, and MC_FSH/MC_OCF data. The frame error control field is appended here by `EncodeWithConfig()` when the channel carries one. CADU wrapping (ASM prepending, CCSDS pseudo-randomization) is done by the `tmsc` package. |
| TM-62 | Packet Extraction Function | 4.3.2 | M | Yes | `VirtualChannelPacketService.Receive()` uses FHP to locate packet starts and `PacketSizer` to extract complete packets per §4.3.2. Resyncs after frame loss by aborting partial packets and finding next FHP. Skips idle frames and discards extracted idle packets (APID 0x7FF) per §4.3.2. |
| TM-63 | VC Reception Function | 4.3.3 | M | Yes | `DecodeTMTransferFrame()` parses raw octets into a `TMTransferFrame`, verifying CRC and extracting all fields. `MasterChannel.AddFrame()` routes received frames to the appropriate `VirtualChannel` by VCID. |
| TM-64 | VC Demultiplexing Function | 4.3.4 | M | Yes | `MasterChannel.AddFrame()` demultiplexes inbound frames to Virtual Channels by VCID. `TMServiceManager` dispatches to the correct VC service. |
| TM-65 | MC Reception Function | 4.3.5 | M | Yes | `MasterChannel.GetNextFrame()` pulls the next frame from the integrated multiplexer. |
| TM-66 | MC Demultiplexing Function | 4.3.6 | M | Yes | `PhysicalChannel.AddFrame()` demultiplexes inbound frames to the correct `MasterChannel` by SCID. |
| TM-67 | All Frames Reception Function | 4.3.7 | M | Yes | `tmsc.UnwrapCADU()` handles ASM stripping and de-randomization. `tmdl.DecodeTMTransferFrame()` handles frame decoding. |

### Table A-6: Management Parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| | **Managed Parameters for a Physical Channel** | | | | | |
| TM-68 | Physical Channel Name | Table 5-1 | M | Character String | Yes | `PhysicalChannel.Name` — configured at construction via `NewPhysicalChannel(name, config)`. |
| TM-69 | Transfer Frame Length (octets) | Table 5-1 | M | Integer | Yes | `ChannelConfig.FrameLength` defines the fixed frame length. Enforced by VCP (packing + idle-packet fill) and VCA (padding) during frame construction, and by the codec itself: `EncodeWithConfig()` and `DecodeTMTransferFrameWithConfig()` reject any other length with `ErrFrameLengthMismatch`. `DataFieldCapacity()` computes available data space. |
| TM-70 | Transfer Frame Version Number (TFVN) | Table 5-1 | M | '00' binary | Yes | `PrimaryHeader.VersionNumber` — enforced as `0` in `Validate()`. |
| TM-71 | Valid Spacecraft IDs | Table 5-1 | M | Integers | Yes | `PrimaryHeader.SpacecraftID` — 10 bits (0–1023). Validated in `Validate()`. Configurable per frame via `NewTMTransferFrame()`. |
| TM-72 | MC Multiplexing Scheme | Table 5-1 | M | Mission Specific | Yes | `PhysicalChannel` implements weighted round-robin MC multiplexing. Priority weights configured per `MasterChannel` via `AddMasterChannel()`. |
| TM-73 | Presence of Frame Error Control | Table 5-1 | M | Present ('1') / Absent ('0') | Yes | Configurable via `ChannelConfig.HasFEC`. The default entry points (`Encode()`, `DecodeTMTransferFrame()`) keep the field; `EncodeWithConfig()` / `DecodeTMTransferFrameWithConfig()` omit or verify it per the channel configuration, which is the Reed-Solomon case CCSDS permits. |
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
| TM-83 | Presence of VC_FSH | Table 5-3 | M | Present ('1') / Absent ('0') | Yes | `ChannelConfig.FSHDataLength` > 0 makes the services emit a secondary header in every frame of the channel, which §4.1.2.7.2.3 requires to be static; `PrimaryHeader.FSHFlag` signals it on the wire. |
| TM-84 | VC_FSH Length (if present) (octets) | Table 5-3 | M | Integer | Yes | `ChannelConfig.FSHDataLength` is the managed value, fixed for the channel per §4.1.3.1.6 and subtracted from the data field capacity by `DataFieldCapacity()`; `SecondaryHeader.HeaderLength` carries it on the wire as total-length-minus-one (§4.1.3.2.3.2). |
| TM-85 | Presence of VC_OCF | Table 5-3 | M | Present ('1') / Absent ('0') | Yes | `PrimaryHeader.OCFFlag`. |
| | **Managed Parameters for Packet Transfer** | | | | | |
| TM-86 | Valid PVNs | Table 5-4 | M | Set of Integers | Yes | PVN validation was removed from tmdl; the application layer is responsible for validating PVNs before passing packet data to the VCP service. |
| TM-87 | Maximum Packet Length (octets) | Table 5-4 | M | Integer | Yes | Maximum packet length bounded by `PacketSizer` (Space Packets: 65542 bytes max). Per-frame capacity derived from `ChannelConfig.DataFieldCapacity()`. Packets larger than one frame are automatically packed across multiple frames. |
| TM-88 | Whether incomplete Packets are required to be delivered to the user at the receiving end | Table 5-4 | M | Required, not required | Yes | Policy: not required. `VirtualChannelPacketService.Receive()` delivers only complete reassembled packets. A packet left incomplete by frame loss is silently discarded when the receiver resynchronizes from the next First Header Pointer; it is never delivered and no dedicated error is raised. |

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
| Mandatory (M) | 78 | 78 | 0 | 0 |
| Optional (O) | 9 | 7 | 0 | 2 |
| Conditional (C2) | 2 | 0 | 0 | 0 (N/A) |
| Conditional (C3) | 11 | 0 | 0 | 0 (N/A) |
| Conditional (C4) | 2 | 0 | 0 | 0 (N/A) |
| Conditional (C5) | 4 | 0 | 0 | 0 (N/A) |
| **Total** | **106** | **85** | **0** | **2 + 19 N/A** |

### Partially Supported Mandatory Items

None. Every mandatory item is fully supported.

The MC_FSH and MC_OCF services were the last gap. They are now master-channel
level services: `MasterChannel.SetFSHSupplier()` and `SetOCFSupplier()` insert
their service data units into every frame the master channel releases (the
Master Channel Generation Function of §4.2.5), and `LastFSH()` / `LastOCF()`
deliver what arrives (§4.3.5.2). The virtual-channel counterparts VC_FSH and
VC_OCF have per-service suppliers of the same shape.

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
| TM-27 | FSH_SDU Loss Flag (MC) | `MasterChannel.MCFrameGap()`, read alongside `LastFSH()`. |
| TM-30 | OCF_SDU Loss Flag (MC) | `MasterChannel.MCFrameGap()`, read alongside `LastOCF()`. |
| TM-33 | Frame Loss Flag (MC Frame) | `FrameGapDetector` via `MasterChannel.MCFrameGap()` / `VCFrameGap()`. |

### Fully Supported Mandatory Items

All 78 mandatory items are fully supported. Key implementations:

| Area | Items | Implementation |
|------|-------|----------------|
| Service Data Units | TM-1–5 | `TMTransferFrame` encode/decode, `SecondaryHeader`, `OperationalControl`. |
| VCP Service | TM-6–8, TM-34–35 | `VirtualChannelPacketService` with native multi-packet packing via FHP, SPP idle-packet fill, `PacketSizer`-based reassembly with FHP resync on loss and idle-packet discard. |
| VCA Service | TM-11–13, TM-36–37 | `VirtualChannelAccessService` with fixed SDU size enforcement, `LastStatus()` for status fields. |
| VCF Service | TM-22–23, TM-42–43 | `VirtualChannelFrameService` with encode/decode via `VirtualChannel`. |
| VC FSH/OCF Services | TM-16–21, TM-38–41 | Secondary header and OCF via `NewTMTransferFrame()` / `DecodeTMTransferFrame()`, per-channel OCF via `SetOCFSupplier`. |
| MC Frame Service | TM-31–33, TM-48–49 | `MasterChannel` with SCID validation, `AddFrame()` / `GetNextFrame()`. |
| Protocol Data Unit | TM-50–55 | `PrimaryHeader` (48-bit), `SecondaryHeader`, CRC-16-CCITT. |
| Packet Processing | TM-56, TM-62 | VCP native multi-packet packing with FHP, SPP idle-packet fill on `Flush()`, `PacketSizer`-based extraction with FHP resync after loss and idle-packet discard. |
| VC Functions | TM-57–58, TM-63–64 | `NewTMTransferFrame()`, `VirtualChannelMultiplexer` (weighted round-robin), `MasterChannel` demux by VCID. |
| MC Functions | TM-59–60, TM-65–66 | `MasterChannel.AddFrame()` routes by VCID. `PhysicalChannel` MC mux/demux by SCID. |
| Physical Channel | TM-61, TM-67–69, TM-72 | `PhysicalChannel` with MC mux/demux, `Name`, `ChannelConfig.FrameLength` (codec-enforced), MC multiplexing scheme, and conformant OID frames (PN-filled, packet-carrying VCID, counter-stamped, deterministic SCID). CADU wrapping (ASM + randomization) handled by `tmsc` package. |
| Management Params | TM-70–88 | TFVN enforced, SCID/VCID validated, SyncFlag, FSHFlag, OCFFlag, configurable FECF presence, complete packet delivery. |
