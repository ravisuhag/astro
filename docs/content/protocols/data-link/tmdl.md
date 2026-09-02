---
title: TM Space Data Link Protocol
short: TMDL
description: CCSDS 132.0-B-3, fixed-length telemetry frames on the downlink.
order: 20
---

> **CCSDS 132.0-B-3** | [Blue Book](https://public.ccsds.org/Pubs/132x0b3.pdf) | [`pkg/tmdl`](https://github.com/ravisuhag/astro/tree/main/pkg/tmdl) | [`astro tm`](/cli/tm)

TM carries telemetry from a spacecraft to the ground in fixed-length transfer frames. The length is chosen once per physical channel and never changes. Inside a frame you usually find [Space Packets](/protocols/transport/spp), packed end to end and spanning frame boundaries when they are too big to fit.

One spacecraft owns a master channel. That master channel is split into up to eight virtual channels, which share the downlink. See [the stack](/docs/start/concepts) for how this sits under the packet layer and over the coding layer.

## Scope

**Implemented.** The transfer frame format, all three services from the standard — VCP (clause 3.4), VCF, and VCA: master and virtual channel multiplexing, frame gap detection, and idle frame generation with the mandatory PN fill.

**Somewhere else.** The sync layer is not here. Attached Sync Markers, pseudo-randomization, and CADU wrapping live in [`pkg/tmsc`](/protocols/coding/tmsc). `tmdl.PhysicalChannel` does master channel multiplexing and nothing below it.

**Left to you.** Secondary header contents, and the CLCW you put in the Operational Control Field. [`pkg/cop`](/protocols/data-link/cop) builds CLCWs if you want one.

**Also checked.** `ChannelConfig.Validate()` enforces the ECSS-E-ST-50-03C 2048-octet frame ceiling. It is not called for you. A CCSDS-only mission may legitimately run longer frames. See the [ECSS conformance statement](/conformance/tmdl-ecss).

## Field map

The 6-byte Transfer Frame Primary Header. Go fields are on `tmdl.PrimaryHeader`.

| Field | Bits | Go | Notes |
|---|---|---|---|
| Transfer Frame Version Number | 2 | `VersionNumber` | Always `0` for TM. AOS uses `1`. |
| Spacecraft Identifier | 10 | `SpacecraftID` | 0-1023, assigned by your agency |
| Virtual Channel Identifier | 3 | `VirtualChannelID` | 0-7 |
| OCF Flag | 1 | `OCFFlag` | Fixed for the whole physical channel |
| Master Channel Frame Count | 8 | `MCFrameCount` | Wraps at 255. Counts every frame from the spacecraft. |
| Virtual Channel Frame Count | 8 | `VCFrameCount` | Wraps at 255. Counts frames on this VC only. |
| Frame Secondary Header Flag | 1 | `FSHFlag` | Fixed for the virtual channel |
| Synchronization Flag | 1 | `SyncFlag` | `false` = VCP/VCF data, `true` = VCA data |
| Packet Order Flag | 1 | `PacketOrderFlag` | Must be 0 |
| Segment Length Identifier | 2 | `SegmentLengthID` | Must be `3` (`0b11`) when `SyncFlag` is false |
| First Header Pointer | 11 | `FirstHeaderPtr` | See below |

The rest of the frame, on `tmdl.TMTransferFrame`:

| Part | Size | Go | Notes |
|---|---|---|---|
| Secondary Header | 2-64 B | `SecondaryHeader` | Optional. One prefix octet plus 1-63 data octets. |
| Data Field | variable | `DataField` | Frame length minus everything else |
| Operational Control Field | 4 B | `OperationalControl` | Optional. Usually a CLCW. |
| Frame Error Control | 2 B | `FrameErrorControl` | Optional CRC-16-CCITT over the whole frame |

Two derived identifiers are available as methods: `PrimaryHeader.MCID()` and `PrimaryHeader.GVCID()`.

Channel-wide settings live on `ChannelConfig`, `FrameLength`, `HasOCF`, `HasFEC`, and `FSHDataLength`. `ChannelConfig.DataFieldCapacity(n)` does the arithmetic for you.

### First Header Pointer

The FHP is how a receiver finds packet boundaries in a fixed-length stream, and how it recovers after losing a frame. When `SyncFlag` is false:

| Value | Meaning |
|---|---|
| 0-2045 | Byte offset in the data field where the first new packet starts |
| `0x07FE` (2046) | Only idle data. No packet starts or continues here. |
| `0x07FF` (2047) | No packet starts here. The whole data field continues a packet from an earlier frame. |

When `SyncFlag` is true the FHP is undefined. Astro sends `0x07FF` and a receiver must not reject a frame over the value.

## Gotchas

**The secondary header length field is not the data field length minus one.** Clause 4.1.3.2.2.3 defines it as the *total* header length minus one, and the total includes the identification octet. So for an N-octet data field the field holds N, not N-1. Use `SecondaryHeader.SetDataField()` and let Astro work it out.

**Fill is a real idle packet, not padding.** When VCP has spare room at the end of a data field it writes a Space Packet with APID `0x7FF`. Raw fill bytes would be read as a packet header by a conformant receiver. If the spare room is under 7 octets (the smallest a Space Packet can be) the idle packet spans into the next frame.

**Idle frames must carry a PN sequence.** Clause 4.1.4.6.2 requires fill from a 32-cell shift register, polynomial D0+D1+D2+D22+D32, started at all ones and never restarted. Note 5 of clause 4.1.4.6.3 gives the reason: a repeating pattern gives the receiver too little to lock onto. The first octets are `FF FF FF FF 6D B6 D8 61 45 1F`. Each `MasterChannel` owns one generator, so back-to-back idle frames differ.

**An idle frame is only idle in the data field.** Its secondary header and OCF can carry real data, and Astro fills them from the master channel's MC_FSH and MC_OCF suppliers. Pass the shared `FrameCounter` through `MasterChannel.SetFrameCounter` so idle frames keep the master channel count moving.

**OCF presence is all or nothing.** You cannot mix frames with and without an OCF on one physical channel. Same for the secondary header on one virtual channel. Both are channel configuration, not per-frame choices.

**MC and VC counts answer different questions.** An MC gap says the spacecraft lost a frame. A VC gap says *this* virtual channel lost one. Seeing an MC gap with no VC gap means the lost frame belonged to another VC. `FrameGapDetector` tracks both.

**VCP needs a `PacketSizer` before it can receive.** It has to know how long the packet at the FHP offset claims to be. Without one you get `ErrNoPacketSizer`.

## Quick Start

```go
import "github.com/ravisuhag/astro/pkg/tmdl"

// Create and encode a TM Transfer Frame
frame, _ := tmdl.NewTMTransferFrame(0x1A, 1, []byte("telemetry"), nil, nil)
encoded, _ := frame.Encode()

// Decode a received frame
decoded, _ := tmdl.DecodeTMTransferFrame(encoded)
fmt.Println(decoded.Header.Humanize())
```

## Architecture

The package follows a layered architecture mapping to the CCSDS data plane:

```
┌─────────────────────────────────────────────┐
│  Service Layer                              │
│  VCP (Packet) / VCF (Frame) / VCA (Access)  │
│  TMServiceManager                           │
├─────────────────────────────────────────────┤
│  Master Channel Layer                       │
│  MasterChannel / VirtualChannelMultiplexer  │
├─────────────────────────────────────────────┤
│  Virtual Channel Layer                      │
│  VirtualChannel (frame buffer per VCID)     │
├─────────────────────────────────────────────┤
│  Frame Layer                                │
│  TMTransferFrame / PrimaryHeader            │
│  SecondaryHeader / FrameCounter / CRC-16    │
├─────────────────────────────────────────────┤
│  Physical Layer                             │
│  PhysicalChannel (MC multiplexing)          │
└─────────────────────────────────────────────┘
```

> **Note:** The sync and channel coding layer (ASM, pseudo-randomization, CADU framing) is handled by the `tmsc` package, which implements CCSDS 131.0-B-5. See [tmsc](/protocols/coding/tmsc) for details.

## Transfer Frames

The `TMTransferFrame` is the fundamental data unit. Each frame has a fixed length on a given physical channel and carries telemetry data identified by Spacecraft ID and Virtual Channel ID.

### Creating Frames

```go
// Basic frame with SCID=0x1A, VCID=1
frame, err := tmdl.NewTMTransferFrame(0x1A, 1, data, nil, nil)

// Frame with a secondary header
frame, err := tmdl.NewTMTransferFrame(0x1A, 1, data, secondaryHeaderBytes, nil)

// Frame with Operational Control Field (4 bytes)
frame, err := tmdl.NewTMTransferFrame(0x1A, 1, data, nil, ocfBytes)

// Idle (OID) frame: PN-filled data field, FHP=0x07FE
idle, err := tmdl.NewIdleFrame(0x1A, 7, config)

// Long-lived senders pass the channel's frame counter and PN generator so
// the counts continue and the PN sequence is never restarted.
idle, err := tmdl.NewIdleFrameWithCounter(0x1A, 1, config, counter, oidFill)
```

### Encoding and Decoding

```go
// Encode to bytes (includes CRC-16)
encoded, err := frame.Encode()

// Encode without Frame Error Control
raw, err := frame.EncodeWithoutFEC()

// Decode bytes back to a frame (validates CRC)
frame, err := tmdl.DecodeTMTransferFrame(encoded)

// Check if a frame is idle
if tmdl.IsIdleFrame(frame) { ... }
```

### Inspecting Frames

```go
// Human-readable header dump
fmt.Println(frame.Header.Humanize())

// Access identifiers
mcid := frame.Header.MCID()   // Master Channel ID (TFVN + SCID)
gvcid := frame.Header.GVCID() // Global Virtual Channel ID (MCID + VCID)
```

## Channel Configuration

`ChannelConfig` defines the fixed parameters shared by all frames on a physical channel:

```go
config := tmdl.ChannelConfig{
    FrameLength:   1024, // Total frame length in octets
    HasOCF:        true, // Operational Control Field (4 bytes)
    HasFEC:        true, // Frame Error Control (2-byte CRC)
    FSHDataLength: 4,    // Secondary header data field, 0 for none
}

// Calculate available space for user data
capacity := config.DataFieldCapacity(0)                  // No secondary header
capacity := config.DataFieldCapacity(len(secHeaderData)) // With secondary header
```

`FSHDataLength` is the length of the Transfer Frame Secondary Header data field carried by *every* frame on the channel. CCSDS 132.0-B-3 clause 4.1.3.1.6 fixes that length for the channel, so it belongs here rather than per frame. Set it and the services emit a secondary header on every frame, filling it from a VC_FSH supplier when one is installed.

`DataFieldCapacity` accounts for the 6-byte primary header, optional secondary header (1 + N bytes), optional OCF (4 bytes), and optional FEC (2 bytes).

## Virtual Channels

A `VirtualChannel` is a buffered frame queue identified by a VCID (0-7). It provides thread-safe FIFO storage for frames within a single data stream.

```go
// Create with VCID=1 and buffer capacity of 100 frames
vc := tmdl.NewVirtualChannel(1, 100)

// Add and retrieve frames
err := vc.AddFrame(frame)          // ErrBufferFull if at capacity
frame, err := vc.GetNextFrame()    // ErrNoFramesAvailable if empty
hasFrames := vc.HasFrames()
count := vc.Len()
```

## Services

Three service types provide different data transfer models over Virtual Channels:

### Virtual Channel Packet Service (VCP)

Multiplexes CCSDS Space Packets into fixed-length frames using FirstHeaderPointer for packet boundary detection.

```go
counter := tmdl.NewFrameCounter()
vc := tmdl.NewVirtualChannel(1, 100)
vcp := tmdl.NewVirtualChannelPacketService(0x1A, 1, vc, config, counter)

// Send packets, automatically packed into frames
err := vcp.Send(packet1)
err = vcp.Send(packet2)
err = vcp.Flush() // Emit remaining partial frame with idle fill

// Receive, extracts packets using FHP and PacketSizer
pkt, err := vcp.Receive()
```

**Packet packing behavior:**
- When `ChannelConfig.FrameLength > 0`: packets are buffered and packed into fixed-length frames. Multiple small packets can share a frame; large packets span multiple frames. `FirstHeaderPtr` marks where each new packet begins.
- When `ChannelConfig.FrameLength == 0`: legacy mode, one frame per packet.

**Packet sizer:**

A packet sizer must be set before calling `Receive`. For CCSDS Space Packets, use the sizer from the `spp` package:

```go
vcp.SetPacketSizer(spp.PacketSizer)
```

For non-CCSDS packet formats, provide a custom sizer function:

```go
vcp.SetPacketSizer(func(data []byte) int {
    if len(data) < 4 { return -1 }
    length := int(binary.BigEndian.Uint32(data[0:4]))
    return 4 + length
})
```

**Receive-side resync:** After a frame gap is detected (via `FrameGapDetector`), the receiver discards its buffer and resyncs at the next `FirstHeaderPtr` offset.

### Virtual Channel Frame Service (VCF)

Pass-through service, sends and receives pre-encoded frames without modification.

```go
vc := tmdl.NewVirtualChannel(2, 100)
vcf := tmdl.NewVirtualChannelFrameService(2, vc)

// Send a pre-encoded frame
err := vcf.Send(encodedFrameBytes)

// Receive an encoded frame
data, err := vcf.Receive()
```

### Virtual Channel Access Service (VCA)

Fixed-length SDU service for housekeeping data or fixed-rate streams. Sets `SyncFlag=true` per CCSDS 132.0-B-3 clause 4.1.2.7.3.2.

```go
counter := tmdl.NewFrameCounter()
vc := tmdl.NewVirtualChannel(3, 100)
vca := tmdl.NewVirtualChannelAccessService(0x1A, 3, 256, vc, config, counter)

// Send a fixed-length SDU (padded to data field capacity)
err := vca.Send(sduData)

// Receive SDU and check status
data, err := vca.Receive()
status := vca.LastStatus() // VCAStatus{SyncFlag, PacketOrderFlag, SegmentLengthID, FirstHeaderPtr}
```

### VCA Status Fields

With the Synchronization Flag set, the Packet Order Flag, Segment Length Identifier, and First Header Pointer are undefined by CCSDS and belong to the VCA service user: they are the VCA Status Fields of clause 3.4.2.3, a mandatory parameter whose meaning the mission chooses (validity, sequence, or other status of the SDU):

```go
vca.SetSendStatus(tmdl.VCAStatus{
    PacketOrderFlag: true,
    SegmentLengthID: 0b10,
    FirstHeaderPtr:  0x123,
})
err := vca.Send(sduData) // carries those bits

status := rx.LastStatus() // the receiving user reads them back
```

Without `SetSendStatus`, the First Header Pointer defaults to all ones (`FHPNoPacketStart`), which is what a receiver ignoring the status fields expects to see.

### Secondary Header and OCF Services (VC_FSH, VC_OCF)

The VCP and VCA services carry data for two more of the standard's services: VC_FSH (clause 3.5) puts an SDU in every frame's Transfer Frame Secondary Header, VC_OCF (clause 3.6) puts four octets in every frame's Operational Control Field. Both are synchronous with frame release, so they are installed as suppliers polled as each frame is built:

```go
config := tmdl.ChannelConfig{FrameLength: 1024, FSHDataLength: 4, HasOCF: true, HasFEC: true}

svc.SetFSHSupplier(func() []byte { return timecode() })  // VC_FSH
svc.SetOCFSupplier(func() []byte { return clcw.Encode() }) // VC_OCF

// Receiving side reads what arrived
fsh := svc.LastFSH()
```

The FSH SDU must be exactly `FSHDataLength` octets, and the OCF SDU exactly 4; a wrong size is refused rather than truncated. Without a supplier the fields are zero-filled, because clause 4.1.3.1.5 requires the secondary header in every frame of a channel that has one.

### Frame Counter

Manages 8-bit MC and VC frame counters. Share a single counter across all services for the same spacecraft:

```go
counter := tmdl.NewFrameCounter()
mc, vc := counter.Next(vcid) // Returns current counts, then increments both
```

Both counters wrap at 255.

## Master Channel

Groups Virtual Channels for a single spacecraft (identified by SCID) and provides weighted round-robin multiplexing:

```go
mc := tmdl.NewMasterChannel(0x1A, config)

// Register Virtual Channels with priority weights
mc.AddVirtualChannel(vc1, 3) // Higher priority
mc.AddVirtualChannel(vc2, 1) // Lower priority

// Send path: retrieve next frame from multiplexer
frame, err := mc.GetNextFrame()
frame, err := mc.GetNextFrameOrIdle() // Returns an OID idle frame if none available

// Receive path: route inbound frame to correct VC
err := mc.AddFrame(frame)

// Frame gap detection
mcGap := mc.MCFrameGap() // MC frame gap from last AddFrame
vcGap := mc.VCFrameGap() // VC frame gap from last AddFrame

// Check pending state
hasPending := mc.HasPendingFrames()
```

### Master Channel FSH and OCF Services (MC_FSH, MC_OCF)

The master channel has its own pair of services (clause 3.8, clause 3.9). Their SDUs go into *every* frame the master channel releases, whichever virtual channel it came from, and overwrite anything the virtual channel level put there. That is the Master Channel Generation Function of clause 4.2.5:

```go
mc.SetFSHSupplier(func() []byte { return spacecraftTime() })  // MC_FSH
mc.SetOCFSupplier(func() []byte { return clcw.Encode() })     // MC_OCF

// Receiving side: the SDUs from the most recent AddFrame
fsh := mc.LastFSH()
ocf := mc.LastOCF()
```

Use the master-channel services when the data is spacecraft-wide (a time code, the CLCW for the whole link) and the virtual-channel ones when it differs per stream. A supplier on a channel whose frames have nowhere to put the SDU fails with `ErrFSHNotPresent` or `ErrOCFNotPresent` rather than dropping it.

### Idle (OID) Frames

When no virtual channel has a frame ready at release time, `GetNextFrameOrIdle` creates an Only Idle Data frame to keep the stream continuous (clause 4.2.4.4). Getting one right takes more than zero-filling:

```go
mc.SetFrameCounter(counter) // counts continue through idle frames (clause 4.1.2.5)
mc.SetIdleVCID(1)           // optional: pin the VCID, else lowest registered
idle, err := mc.GetNextFrameOrIdle()
```

- The First Header Pointer is `0x7FE` (`FHPOnlyIdleData`), which says "only idle data", not the `0x7FF` that says "no packet starts here" (clause 4.1.2.7.6.5).
- The data field carries the mandatory PN sequence from `OIDSequence`, a 32-cell LFSR with polynomial D0+D1+D2+D22+D32 (clause 4.1.4.6.2). Each `MasterChannel` keeps one generator for its lifetime, since clause 4.1.4.6.2.1 forbids restarting it, so consecutive idle frames carry different octets. Constant fill would defeat the randomization the sequence exists to provide.
- The VCID is one that carries packets (clause 4.1.4.6.3), so a receiver has a reception function for it.
- The secondary header and OCF still carry their MC service data: only the *data field* of an OID frame is idle (clause 4.1.4.6.3 note 1).

## Physical Channel

Represents the physical communication link. Handles MC-level multiplexing across Master Channels:

```go
pc := tmdl.NewPhysicalChannel("TM-68", config)

// Register Master Channels with priority weights
pc.AddMasterChannel(mc1, 2)
pc.AddMasterChannel(mc2, 1)

// Send path
frame, err := pc.GetNextFrame()        // Weighted round-robin across MCs
frame, err := pc.GetNextFrameOrIdle()  // Idle frame if no data

// Receive path: demux inbound frame to correct MC by SCID
err := pc.AddFrame(frame)
```

### Composing with tmsc for Sync and Channel Coding

The `tmsc` package (CCSDS 131.0-B-5) handles the sync layer: ASM, pseudo-randomization, and CADU framing. Use it alongside `tmdl` for a complete send/receive pipeline:

```go
import "github.com/ravisuhag/astro/pkg/tmsc"

// Send: get next frame from MC multiplexer, then wrap as CADU
frame, _ := pc.GetNextFrame()
encoded, _ := frame.Encode()
cadu := tmsc.WrapCADU(encoded, nil, true) // nil=default ASM, true=randomize

// Receive: unwrap CADU, then decode frame
unwrapped, _ := tmsc.UnwrapCADU(cadu, nil, true) // nil=default ASM, true=derandomize
frame, _ := tmdl.DecodeTMTransferFrame(unwrapped)
```

## Service Manager

`TMServiceManager` provides a high-level API that wires the full pipeline:

```go
mgr := tmdl.NewTMServiceManager()

// Register services and channels
mgr.RegisterVirtualService(1, tmdl.VCP, vcp)
mgr.RegisterVirtualService(3, tmdl.VCA, vca)
mgr.RegisterMasterChannel(0x1A, mc)

// Send data through a service
err := mgr.SendData(1, tmdl.VCP, packetBytes)

// Receive data from a service
data, err := mgr.ReceiveData(1, tmdl.VCP)

// Flush a service
err := mgr.FlushService(1, tmdl.VCP)

// Route frames through Master Channels
err := mgr.AddFrameToMasterChannel(0x1A, frame)
frame, err := mgr.GetNextFrameFromMasterChannel(0x1A)
hasPending := mgr.HasPendingFramesInMasterChannel(0x1A)
```

## Full Pipeline Example

### Send Path (Spacecraft to Ground)

```go
// 1. Configure the physical channel
config := tmdl.ChannelConfig{
    FrameLength: 1024,
    HasOCF:      true,
    HasFEC:      true,
}

// 2. Create channel hierarchy
counter := tmdl.NewFrameCounter()
vc1 := tmdl.NewVirtualChannel(1, 100)
vcp := tmdl.NewVirtualChannelPacketService(0x1A, 1, vc1, config, counter)

mc := tmdl.NewMasterChannel(0x1A, config)
mc.AddVirtualChannel(vc1, 1)

pc := tmdl.NewPhysicalChannel("TM-68", config)
pc.AddMasterChannel(mc, 1)

// 3. Send packets
vcp.Send(packet1)
vcp.Send(packet2)
vcp.Flush()

// 4. Transmit frames as CADUs (using tmsc for sync layer)
for pc.HasPendingFrames() {
    frame, _ := pc.GetNextFrame()
    encoded, _ := frame.Encode()
    cadu := tmsc.WrapCADU(encoded, nil, true) // nil=default ASM, true=randomize
    transmit(cadu)
}
```

### Receive Path (Ground Station)

```go
// 1. Create matching channel hierarchy
counter := tmdl.NewFrameCounter()
vc1 := tmdl.NewVirtualChannel(1, 100)
vcp := tmdl.NewVirtualChannelPacketService(0x1A, 1, vc1, config, counter)
vcp.SetPacketSizer(spp.PacketSizer)

mc := tmdl.NewMasterChannel(0x1A, config)
mc.AddVirtualChannel(vc1, 1)

pc := tmdl.NewPhysicalChannel("TM-68", config)
pc.AddMasterChannel(mc, 1)

// 2. Process incoming CADUs (using tmsc for sync layer)
unwrapped, err := tmsc.UnwrapCADU(cadu, nil, true) // nil=default ASM, true=derandomize
if err != nil { /* handle sync marker or data errors */ }

frame, err := tmdl.DecodeTMTransferFrame(unwrapped)
if err != nil { /* handle CRC or frame errors */ }

// 3. Route to Master Channel -> Virtual Channel
err = pc.AddFrame(frame)

// 4. Extract packets
pkt, err := vcp.Receive()
```

## Errors

All errors are exported package-level variables, suitable for use with `errors.Is`:

| Error | Meaning |
|-------|---------|
| `ErrDataTooShort` | Data too short to decode |
| `ErrInvalidVersion` | Version is not 0 |
| `ErrInvalidSpacecraftID` | SCID outside 0-1023 |
| `ErrInvalidVCID` | VCID outside 0-7 |
| `ErrInvalidPacketOrderFlag` | Packet order flag set when sync flag is 0 |
| `ErrInvalidSegmentLengthID` | Segment length ID not `11` when sync flag is 0 |
| `ErrInvalidFirstHeaderPtr` | FHP outside 0-2047 or inconsistent with sync flag |
| `ErrInvalidSecondaryHeaderVersion` | Secondary header version is not 0 |
| `ErrInvalidHeaderLength` | Secondary header length outside 0-63 |
| `ErrCRCMismatch` | CRC integrity check failed |
| `ErrDataTooLarge` | Data exceeds maximum frame length |
| `ErrEmptyData` | Empty data provided |
| `ErrNoFramesAvailable` | No frames in buffer |
| `ErrBufferFull` | Virtual channel buffer at capacity |
| `ErrSCIDMismatch` | Frame SCID doesn't match master channel |
| `ErrSizeMismatch` | VCA data size doesn't match expected fixed size |
| `ErrServiceNotFound` | No service for specified VCID and type |
| `ErrMasterChannelNotFound` | No master channel for specified SCID |
| `ErrNoVirtualChannels` | No virtual channels registered |
| `ErrVirtualChannelNotFound` | No virtual channel for specified VCID |
| `ErrDataFieldTooSmall` | Data field capacity too small for framing |
| `ErrNoMasterChannels` | No master channels on physical channel |
| `ErrInvalidOCFLength` | OCF not exactly 4 bytes |
| `ErrFSHNotPresent` | An FSH supplier is installed but the channel's frames carry no secondary header (set `ChannelConfig.FSHDataLength`) |
| `ErrOCFNotPresent` | An OCF supplier is installed but the channel's frames carry no OCF (set `ChannelConfig.HasOCF`) |
| `ErrFSHSizeMismatch` | An FSH_SDU whose length differs from the channel's fixed `FSHDataLength` |

> **Note:** Sync-layer errors such as `ErrSyncMarkerMismatch` and `ErrDataTooShort` are defined in the `tmsc` package.

## Notes

Commentary, not sourced from the standard.

**Why fixed-length frames?** The receiver knows exactly how many bytes follow each sync marker, so it needs no length field and no delimiter. That matters most on deep-space links running near the noise floor, where any extra parsing is another thing to get wrong.

**Why only 8 virtual channels?** Three bits is enough separation for most missions and keeps the header at 6 bytes. A mission that needs finer separation can multiplex by APID inside one VC.

**Why 8-bit frame counters?** 256 values wrap slowly enough to catch the common failure (a single lost frame) while staying small. Longer outages are somebody else's problem, usually the FHP resync.

**Why no retransmission?** A TM link is one-way, and the round trip is seconds to hours. Asking again is useless. The protocol detects loss with counters, recovers with the FHP, and leaves bit errors to the coding layer.

## Reference

- [CCSDS 132.0-B-3](https://public.ccsds.org/Pubs/132x0b3.pdf), TM Space Data Link Protocol (Blue Book)
- [ECSS-E-ST-50-03C](https://ecss.nl/standard/ecss-e-st-50-03c-space-data-links-telemetry-transfer-frame-protocol/), the European profile
- [CLI](/cli/tm) | [Conformance](/conformance/tmdl) | [ECSS conformance](/conformance/tmdl-ecss)
