# TM Space Data Link Protocol (TMDL)

The `tmdl` package implements the CCSDS 132.0-B-3 TM Space Data Link Protocol — the Data Link Layer protocol used for transferring telemetry data from spacecraft to ground stations.

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
│  VCP (Packet) · VCF (Frame) · VCA (Access)  │
│  TMServiceManager                           │
├─────────────────────────────────────────────┤
│  Master Channel Layer                       │
│  MasterChannel · VirtualChannelMultiplexer  │
├─────────────────────────────────────────────┤
│  Virtual Channel Layer                      │
│  VirtualChannel (frame buffer per VCID)     │
├─────────────────────────────────────────────┤
│  Frame Layer                                │
│  TMTransferFrame · PrimaryHeader            │
│  SecondaryHeader · FrameCounter · CRC-16    │
├─────────────────────────────────────────────┤
│  Physical Layer                             │
│  PhysicalChannel (MC multiplexing)          │
└─────────────────────────────────────────────┘
```

> **Note:** The sync and channel coding layer (ASM, pseudo-randomization, CADU framing) is handled by the `tmsc` package, which implements CCSDS 131.0-B-4. See [tmsc](tmsc.md) for details.

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

// Idle frame (all-ones data, FHP=0x07FF)
idle, err := tmdl.NewIdleFrame(0x1A, 7, config)
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

## Frame Structure

```
+----------------+----------------+----------------+----------------+
| Version (2b)   | Spacecraft ID (10b)             | VCID (3b)      |
+----------------+----------------+----------------+----------------+
| OCF Flag (1b)  | MC Frame Count (8b)             | VC Frame Count |
+----------------+----------------+----------------+----------------+
| FSH (1b) | Sync (1b) | PktOrd (1b) | SegLen (2b) | FHP (11b)     |
+----------------+----------------+----------------+----------------+
| Secondary Header (optional, 1–64 bytes)                           |
+----------------+----------------+----------------+----------------+
| Transfer Frame Data Field (variable)                              |
+----------------+----------------+----------------+----------------+
| Operational Control Field (optional, 4 bytes)                     |
+----------------+----------------+----------------+----------------+
| Frame Error Control (CRC-16-CCITT, 2 bytes)                      |
+----------------+----------------+----------------+----------------+
```

### Primary Header

The 6-byte primary header identifies and routes each frame:

| Field | Bits | Range | Description |
|-------|------|-------|-------------|
| Version Number | 2 | 0 | Transfer Frame Version (`00` for TM) |
| Spacecraft ID | 10 | 0–1023 | Identifies the spacecraft |
| Virtual Channel ID | 3 | 0–7 | Identifies the virtual channel |
| OCF Flag | 1 | 0–1 | Operational Control Field present |
| MC Frame Count | 8 | 0–255 | Master Channel sequence counter |
| VC Frame Count | 8 | 0–255 | Virtual Channel sequence counter |
| FSH Flag | 1 | 0–1 | Frame Secondary Header present |
| Sync Flag | 1 | 0–1 | Synchronization flag (VCA sets to 1) |
| Packet Order Flag | 1 | 0–1 | Must be 0 when Sync Flag is 0 |
| Segment Length ID | 2 | 0–3 | Must be `11` when Sync Flag is 0 |
| First Header Pointer | 11 | 0–2047 | Offset to first packet start in data field |

**First Header Pointer special values:**
- `0x07FE` — no packet starts in this frame (continuation only)
- `0x07FF` — idle frame (VCP) or VCA service data

### Secondary Header

Optional mission-defined header (1 prefix byte + up to 64 data bytes):

```go
// Included when secondaryHeaderData is non-nil in NewTMTransferFrame
frame, err := tmdl.NewTMTransferFrame(scid, vcid, data, myHeaderBytes, nil)
```

| Field | Bits | Description |
|-------|------|-------------|
| Version Number | 2 | Always `00` for Version 1 |
| Header Length | 6 | Length of data field minus 1 (0–63) |
| Data Field | variable | Mission-specific content (1–64 bytes) |

## Channel Configuration

`ChannelConfig` defines the fixed parameters shared by all frames on a physical channel:

```go
config := tmdl.ChannelConfig{
    FrameLength: 1024, // Total frame length in octets
    HasOCF:      true, // Operational Control Field (4 bytes)
    HasFEC:      true, // Frame Error Control (2-byte CRC)
}

// Calculate available space for user data
capacity := config.DataFieldCapacity(0)                  // No secondary header
capacity := config.DataFieldCapacity(len(secHeaderData)) // With secondary header
```

`DataFieldCapacity` accounts for the 6-byte primary header, optional secondary header (1 + N bytes), optional OCF (4 bytes), and optional FEC (2 bytes).

## Virtual Channels

A `VirtualChannel` is a buffered frame queue identified by a VCID (0–7). It provides thread-safe FIFO storage for frames within a single data stream.

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

// Send packets — automatically packed into frames
err := vcp.Send(packet1)
err = vcp.Send(packet2)
err = vcp.Flush() // Emit remaining partial frame with idle fill

// Receive — extracts packets using FHP and PacketSizer
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

Pass-through service — sends and receives pre-encoded frames without modification.

```go
vc := tmdl.NewVirtualChannel(2, 100)
vcf := tmdl.NewVirtualChannelFrameService(2, vc)

// Send a pre-encoded frame
err := vcf.Send(encodedFrameBytes)

// Receive an encoded frame
data, err := vcf.Receive()
```

### Virtual Channel Access Service (VCA)

Fixed-length SDU service for housekeeping data or fixed-rate streams. Sets `SyncFlag=true` and `FirstHeaderPtr=0x07FF` per CCSDS spec.

```go
counter := tmdl.NewFrameCounter()
vc := tmdl.NewVirtualChannel(3, 100)
vca := tmdl.NewVirtualChannelAccessService(0x1A, 3, 256, vc, config, counter)

// Send a fixed-length SDU (padded to data field capacity)
err := vca.Send(sduData)

// Receive SDU and check status
data, err := vca.Receive()
status := vca.LastStatus() // VCAStatus{SyncFlag, PacketOrderFlag, SegmentLengthID}
```

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
frame, err := mc.GetNextFrameOrIdle() // Returns idle frame if none available

// Receive path: route inbound frame to correct VC
err := mc.AddFrame(frame)

// Frame gap detection
mcGap := mc.MCFrameGap() // MC frame gap from last AddFrame
vcGap := mc.VCFrameGap() // VC frame gap from last AddFrame

// Check pending state
hasPending := mc.HasPendingFrames()
```

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

The `tmsc` package (CCSDS 131.0-B-4) handles the sync layer — ASM, pseudo-randomization, and CADU framing. Use it alongside `tmdl` for a complete send/receive pipeline:

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

// 3. Route to Master Channel → Virtual Channel
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
| `ErrInvalidSpacecraftID` | SCID outside 0–1023 |
| `ErrInvalidVCID` | VCID outside 0–7 |
| `ErrInvalidPacketOrderFlag` | Packet order flag set when sync flag is 0 |
| `ErrInvalidSegmentLengthID` | Segment length ID not `11` when sync flag is 0 |
| `ErrInvalidFirstHeaderPtr` | FHP outside 0–2047 or inconsistent with sync flag |
| `ErrInvalidSecondaryHeaderVersion` | Secondary header version is not 0 |
| `ErrInvalidHeaderLength` | Secondary header length outside 0–63 |
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

> **Note:** Sync-layer errors such as `ErrSyncMarkerMismatch` and `ErrDataTooShort` are defined in the `tmsc` package.

## Reference

- [CCSDS 132.0-B-3](https://public.ccsds.org/Pubs/132x0b3.pdf) — TM Space Data Link Protocol Blue Book
- [CCSDS 131.0-B-5](https://public.ccsds.org/Pubs/131x0b5.pdf) — TM Synchronization and Channel Coding
- [`tmsc` package](tmsc.md) — Sync and Channel Coding (ASM, randomization, CADU framing)
