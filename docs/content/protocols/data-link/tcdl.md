---
title: TC Space Data Link Protocol
short: TCDL
description: CCSDS 232.0-B-4, variable-length command frames on the uplink.
identifiers:
  - "CCSDS 232.0-B-4 * TC Space Data Link Protocol"
  - "pkg/tcdl * astro tc"
order: 21
---

> **CCSDS 232.0-B-4** | [Blue Book](https://public.ccsds.org/Pubs/232x0b4e1c1.pdf) | [`pkg/tcdl`](https://github.com/ravisuhag/astro/tree/main/pkg/tcdl) | [`astro tc`](/cli/tc)

## Overview

TC carries commands from the ground to a spacecraft. Frames are variable length, up to 1024 bytes, because a "turn on the heater" command is ten bytes and padding it to a fixed size would waste most of an uplink.

The design goal is different from [TM](/protocols/data-link/tmdl). A wrong or missing command can end a mission, so TC is built for reliability, not throughput. Lost frames are detected *and* recovered, by [COP-1](/protocols/data-link/cop) sitting directly on top of this layer.

## Scope

**Implemented.** The transfer frame format, the MAP sublayer with segmentation, all three services (MAP Packet, MAP Access, and VC Frame) plus the Type-BC control command frames (Unlock and Set V(R)) that COP-1 needs.

**Somewhere else.** Retransmission logic is [`pkg/cop`](/protocols/data-link/cop). CLTU wrapping and BCH coding are [`pkg/tcsc`](/protocols/coding/tcsc). This package builds frames.

## Field map

The 5-byte Transfer Frame Primary Header. Go fields are on `tcdl.PrimaryHeader`.

| Field | Bits | Go | Notes |
|---|---|---|---|
| Transfer Frame Version Number | 2 | `VersionNumber` | Always `0` |
| Bypass Flag | 1 | `BypassFlag` | `0` = Type-A (sequence controlled), `1` = Type-B (expedited) |
| Control Command Flag | 1 | `ControlCommandFlag` | `0` = data, `1` = COP-1 control command |
| Reserved | 2 | `Reserved` | Must be `00` |
| Spacecraft Identifier | 10 | `SpacecraftID` | 0-1023 |
| Virtual Channel Identifier | 6 | `VirtualChannelID` | 0-63. Six bits, not TM's three. |
| Frame Length | 10 | `FrameLength` | Total octets minus 1 |
| Frame Sequence Number | 8 | `FrameSequenceNum` | N(S) for COP-1, counted per VC |

The rest of the frame, on `tcdl.TCTransferFrame`:

| Part | Size | Go | Notes |
|---|---|---|---|
| Segment Header | 1 B | `SegmentHeader` | Optional. Present when the MAP sublayer is in use. |
| Data Field | variable | `DataField` | |
| Frame Error Control | 2 B | `FrameErrorControl` | CRC-16-CCITT over the whole frame |

The segment header splits into 2 bits of sequence flags and a 6-bit MAP ID:

| Flags | Constant | Meaning |
|---|---|---|
| `11` | `SegUnsegmented` | Whole data unit in this frame |
| `01` | `SegFirst` | First segment |
| `00` | `SegContinuation` | Middle segment |
| `10` | `SegLast` | Last segment |

`PrimaryHeader.MCID()` and `PrimaryHeader.GVCID()` give the derived identifiers.

## Gotchas

**Frame Length is total octets minus one.** Not the data field length, and not the total. Astro handles it during encode and decode.

**A control command frame must not carry a segment header.** Clause 4.1.3.2.2.1.3. The data field of a Type-BC frame *is* the COP-1 directive, and a FARM reads it as one. An extra octet in front shifts everything, so Unlock and Set V(R) stop being recognised, and nothing errors at either end, the directive just vanishes. Astro returns `ErrSegmentHeaderOnControlCommand`. The frame options refuse the combination too, but `SegmentHeader` is exported and can be assigned past them.

**Bypass=0 with Control Command=1 is not a valid frame type.** A control command must bypass sequence control, because its whole job is fixing sequence control. You get `ErrInvalidFrameType`.

**There is no master channel frame count.** TM has one, TC does not. COP-1 gives per-VC reliability with acknowledgement, which makes an MC counter redundant.

**MAP Packet service needs a `PacketSizer` before it can receive.** Same reason as TM: it has to know how long the packet claims to be. Without one, `ErrNoPacketSizer`.

**Reassembly can fail silently if you drop segments.** A run of segments missing its `SegLast` gives `ErrIncompleteSegment` at reassembly, not at receive time.

### How this ties to COP-1

Every Type-A frame carries N(S) in its header, and that number drives the sliding window:

1. FOP-1 on the ground sets N(S) = V(S), then increments V(S).
2. FARM-1 on the spacecraft compares N(S) with V(R), the next it expects.
3. Equal, accept the frame and increment V(R).
4. Inside the window but not equal, reject, and ask for retransmission.
5. Outside the window, lockout. The ground has to send an Unlock.
6. FARM-1 reports its state in a CLCW, which rides home in the OCF of a [TM frame](/protocols/data-link/tmdl).

Type-B frames skip all of it. That is the point of them, see the note below.

## Quick start

```go
import "github.com/ravisuhag/astro/pkg/tcdl"

// Create and encode a TC Transfer Frame
frame, _ := tcdl.NewTransferFrame(0x1A, 1, []byte("SET_MODE=SAFE"))
encoded, _ := frame.Encode()

// Decode a received frame
decoded, _ := tcdl.DecodeTransferFrame(encoded)
fmt.Println(decoded.Header.Humanize())
```

## Architecture

The package follows a layered architecture mapping to the CCSDS data plane:

```
+-----------------------------------------+
|  Service Layer                          |
|  MAP Packet . MAP Access . VC Frame     |
|  TCServiceManager                       |
+-----------------------------------------+
|  Master Channel Layer                   |
|  MasterChannel . VirtualChannelMux      |
+-----------------------------------------+
|  Virtual Channel Layer                  |
|  VirtualChannel (frame buffer per VCID) |
+-----------------------------------------+
|  Frame Layer                            |
|  TCTransferFrame . PrimaryHeader        |
|  SegmentHeader . FrameCounter . CRC-16  |
+-----------------------------------------+
|  Physical Layer                         |
|  PhysicalChannel (MC multiplexing)      |
+-----------------------------------------+
```

> **Note:** The sync and channel coding layer (CLTU, BCH encoding) is handled by the [`tcsc` package](/protocols/coding/tcsc).

## Transfer frames

The `TCTransferFrame` is the fundamental data unit. TC frames are variable-length (up to 1024 bytes) and carry telecommand data identified by Spacecraft ID and Virtual Channel ID.

### Creating frames

```go
// Basic frame with SCID=0x1A, VCID=1
frame, err := tcdl.NewTransferFrame(0x1A, 1, data)

// Type-B (expedited/bypass) frame
frame, err := tcdl.NewTransferFrame(0x1A, 1, data, tcdl.WithBypass())

// Control command frame
frame, err := tcdl.NewTransferFrame(0x1A, 1, data, tcdl.WithControlCommand())

// Frame with segment header (MAP sublayer)
sh := tcdl.SegmentHeader{SequenceFlags: tcdl.SegUnsegmented, MAPID: 0}
frame, err := tcdl.NewTransferFrame(0x1A, 1, data, tcdl.WithSegmentHeader(sh))

// Frame with explicit sequence number (for COP-1)
frame, err := tcdl.NewTransferFrame(0x1A, 1, data, tcdl.WithSequenceNumber(42))

// Combining options
frame, err := tcdl.NewTransferFrame(0x1A, 1, data,
    tcdl.WithSegmentHeader(sh),
    tcdl.WithSequenceNumber(42),
)
```

### Encoding and decoding

```go
// Encode to bytes (includes CRC-16)
encoded, err := frame.Encode()

// Encode without Frame Error Control
raw, err := frame.EncodeWithoutFEC()

// Decode bytes back to a frame (validates CRC)
frame, err := tcdl.DecodeTransferFrame(encoded)

// Check frame type
if tcdl.IsBypass(frame) { /* Type-B expedited frame */ }
if tcdl.IsControlFrame(frame) { /* Control command */ }
```

### Inspecting frames

```go
// Human-readable header dump
fmt.Println(frame.Header.Humanize())

// Access identifiers
mcid := frame.Header.MCID()   // Master Channel ID (TFVN + SCID)
gvcid := frame.Header.GVCID() // Global Virtual Channel ID (MCID + VCID)
```

## Virtual channels

A `VirtualChannel` is a buffered frame queue identified by a VCID (0-63). TC supports up to 64 Virtual Channels, significantly more than TM's 8.

```go
// Create with VCID=1 and buffer capacity of 100 frames
vc := tcdl.NewVirtualChannel(1, 100)
```

## Services

Three service types provide different data transfer models over Virtual Channels:

### MAP Packet service

Supports segmentation: packets larger than one frame are automatically split across multiple frames using segment header sequence flags.

```go
counter := tcdl.NewFrameCounter()
vc := tcdl.NewVirtualChannel(1, 100)
svc := tcdl.NewMAPPacketService(0x1A, 1, 0, false, vc, counter)

// Send packets, automatically segmented if too large
err := svc.Send(packetData)

// Receive, reassembles segments into complete packets
svc.SetPacketSizer(spp.PacketSizer)
pkt, err := svc.Receive()
```

**Segmentation behavior:**
- Packets that fit in a single frame are sent as `Unsegmented`.
- Large packets are split: `First` segment, zero or more `Continuation` segments, and a `Last` segment.
- Each segment is placed in a separate TC frame with the appropriate segment header flags.

**Bypass mode:**

```go
// Create a bypass (Type-B) MAP Packet Service, frames skip COP-1 sequencing
svc := tcdl.NewMAPPacketService(0x1A, 1, 0, true, vc, counter)
```

### MAP access service

Sends raw data units without packet boundaries. Each data unit produces a single unsegmented frame.

```go
svc := tcdl.NewMAPAccessService(0x1A, 1, 0, false, vc, counter)

// Send a raw data unit
err := svc.Send(rawData)

// Receive the data field
data, err := svc.Receive()
```

### VC Frame service

Pass-through service, sends and receives pre-encoded frames without modification.

```go
vc := tcdl.NewVirtualChannel(2, 100)
vcf := tcdl.NewVCFrameService(2, vc)

// Send a pre-encoded frame
err := vcf.Send(encodedFrameBytes)

// Receive an encoded frame
data, err := vcf.Receive()
```

### Frame counter

Manages per-VC 8-bit frame sequence numbers N(S) used by COP-1:

```go
counter := tcdl.NewFrameCounter()
seqNum := counter.Next(vcid) // Returns current count, then increments
```

The counter wraps at 255.

## Master Channel

Groups Virtual Channels for a single spacecraft (identified by SCID) and provides weighted round-robin multiplexing:

```go
mc := tcdl.NewMasterChannel(0x1A)

// Register Virtual Channels with priority weights
mc.AddVirtualChannel(vc1, 3) // Higher priority
mc.AddVirtualChannel(vc2, 1) // Lower priority

// Send path: retrieve next frame from multiplexer
frame, err := mc.GetNextFrame()

// Receive path: route inbound frame to correct VC
err := mc.AddFrame(frame)

// Frame gap detection (per-VC sequence number tracking)
vcGap := mc.VCFrameGap() // VC frame gap from last AddFrame

// Check pending state
hasPending := mc.HasPendingFrames()
```

## Physical Channel

Represents the physical communication link. Handles MC-level multiplexing across Master Channels:

```go
pc := tcdl.NewPhysicalChannel("TC-Uplink")

// Register Master Channels with priority weights
pc.AddMasterChannel(mc1, 2)
pc.AddMasterChannel(mc2, 1)

// Send path
frame, err := pc.GetNextFrame() // Weighted round-robin across MCs

// Receive path: demux inbound frame to correct MC by SCID
err := pc.AddFrame(frame)

// Check state
hasPending := pc.HasPendingFrames()
numMCs := pc.Len()
```

## Service manager

`TCServiceManager` provides a high-level API that wires the full pipeline:

```go
mgr := tcdl.NewTCServiceManager()

// Register services and channels
mgr.RegisterVirtualService(1, tcdl.MAPPacket, mapSvc)
mgr.RegisterVirtualService(2, tcdl.VCFrame, vcfSvc)
mgr.RegisterMasterChannel(0x1A, mc)

// Send data through a service
err := mgr.SendData(1, tcdl.MAPPacket, packetBytes)

// Receive data from a service
data, err := mgr.ReceiveData(1, tcdl.MAPPacket)

// Route frames through Master Channels
err := mgr.AddFrameToMasterChannel(0x1A, frame)
frame, err := mgr.GetNextFrameFromMasterChannel(0x1A)
hasPending := mgr.HasPendingFramesInMasterChannel(0x1A)
```

## Full pipeline example

### Send path (ground to spacecraft)

```go
// 1. Create channel hierarchy
counter := tcdl.NewFrameCounter()
vc1 := tcdl.NewVirtualChannel(1, 100)
mapSvc := tcdl.NewMAPPacketService(0x1A, 1, 0, false, vc1, counter)

mc := tcdl.NewMasterChannel(0x1A)
mc.AddVirtualChannel(vc1, 1)

pc := tcdl.NewPhysicalChannel("TC-Uplink")
pc.AddMasterChannel(mc, 1)

// 2. Send packets (automatically segmented if needed)
mapSvc.Send(commandData1)
mapSvc.Send(commandData2)

// 3. Transmit frames
for pc.HasPendingFrames() {
    frame, _ := pc.GetNextFrame()
    encoded, _ := frame.Encode()
    transmit(encoded)
}
```

### Receive path (spacecraft)

```go
// 1. Create matching channel hierarchy
vc1 := tcdl.NewVirtualChannel(1, 100)
mapSvc := tcdl.NewMAPPacketService(0x1A, 1, 0, false, vc1, nil)
mapSvc.SetPacketSizer(spp.PacketSizer)

mc := tcdl.NewMasterChannel(0x1A)
mc.AddVirtualChannel(vc1, 1)

pc := tcdl.NewPhysicalChannel("TC-Uplink")
pc.AddMasterChannel(mc, 1)

// 2. Process incoming frames
frame, err := tcdl.DecodeTransferFrame(receivedBytes)
if err != nil { /* handle CRC or frame errors */ }

// 3. Route to Master Channel -> Virtual Channel
err = pc.AddFrame(frame)

// 4. Extract packets (reassembles segments)
pkt, err := mapSvc.Receive()
```

## Integration with COP-1

The `tcdl` package works with the `cop` package for reliable frame delivery. The Frame Sequence Number in the TC header is the N(S) value used by COP-1:

```go
import "github.com/ravisuhag/astro/pkg/cop"

// Ground side: FOP-1 manages sequence numbers and retransmission
fop := cop.NewFOP(0x1A, 1, 10)
fop.Initialize(0)

// Spacecraft side: FARM-1 validates sequence numbers
farm := cop.NewFARM(1, 10)

// See the cop package documentation for full COP-1 integration
```

## Errors

All errors are exported package-level variables, suitable for use with `errors.Is`:

| Error | Meaning |
|---|---|
| `ErrDataTooShort` | Data too short to decode |
| `ErrInvalidVersion` | Version is not 0 |
| `ErrInvalidSpacecraftID` | SCID outside 0-1023 |
| `ErrInvalidVCID` | VCID outside 0-63 |
| `ErrInvalidFrameLength` | Frame length exceeds 1024 bytes |
| `ErrInvalidReservedBits` | Reserved bits are not zero |
| `ErrInvalidMAPID` | MAP ID outside 0-63 |
| `ErrInvalidSequenceFlags` | Sequence flags outside 0-3 |
| `ErrCRCMismatch` | CRC integrity check failed |
| `ErrDataTooLarge` | Data exceeds maximum TC frame capacity |
| `ErrEmptyData` | Empty data provided |
| `ErrNoFramesAvailable` | No frames in buffer |
| `ErrBufferFull` | Virtual channel buffer at capacity |
| `ErrSCIDMismatch` | Frame SCID doesn't match master channel |
| `ErrServiceNotFound` | No service for specified VCID and type |
| `ErrMasterChannelNotFound` | No master channel for specified SCID |
| `ErrNoVirtualChannels` | No virtual channels registered |
| `ErrVirtualChannelNotFound` | No virtual channel for specified VCID |
| `ErrNoMasterChannels` | No master channels on physical channel |
| `ErrNoPacketSizer` | No PacketSizer configured for Receive |
| `ErrIncompleteSegment` | Segment reassembly is incomplete |

## Notes

Commentary, not sourced from the standard.

**Why variable length?** Commands are small and irregular. Fixed frames would burn uplink bandwidth on padding, and uplink is the scarcer resource.

**Why 64 virtual channels when TM gets 8?** Uplink traffic separates into more kinds: emergency commands, routine housekeeping, file uploads, COP-1 management. Each wants its own sequencing and flow control.

**Why the MAP sublayer?** A Space Packet can be 65,542 bytes and a TC frame stops at 1,024. Something has to cut it up. The MAP sublayer does it in a standard way so upper layers do not have to.

**Why two frame types?** In an emergency you need a command to land even when COP-1 is wedged: locked out, or window full. Type-B is the safety valve. It always gets through, and it gives up the guarantee that it arrived in order.

## Reference

- [CCSDS 232.0-B-4](https://public.ccsds.org/Pubs/232x0b4e1c1.pdf), TC Space Data Link Protocol (Blue Book)
- [CLI](/cli/tc) | [Conformance](/conformance/tcdl) | [The stack](/docs/start/concepts)
