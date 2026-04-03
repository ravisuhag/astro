# TC Space Data Link Protocol (TCDL)

The `tcdl` package implements the CCSDS 232.0-B-4 TC Space Data Link Protocol — the Data Link Layer protocol used for transferring telecommand data from ground stations to spacecraft.

## Quick Start

```go
import "github.com/ravisuhag/astro/pkg/tcdl"

// Create and encode a TC Transfer Frame
frame, _ := tcdl.NewTCTransferFrame(0x1A, 1, []byte("SET_MODE=SAFE"))
encoded, _ := frame.Encode()

// Decode a received frame
decoded, _ := tcdl.DecodeTCTransferFrame(encoded)
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

> **Note:** The sync and channel coding layer (CLTU, BCH encoding) is handled by the [`tcsc` package](tcsc.md).

## Transfer Frames

The `TCTransferFrame` is the fundamental data unit. TC frames are variable-length (up to 1024 bytes) and carry telecommand data identified by Spacecraft ID and Virtual Channel ID.

### Creating Frames

```go
// Basic frame with SCID=0x1A, VCID=1
frame, err := tcdl.NewTCTransferFrame(0x1A, 1, data)

// Type-B (expedited/bypass) frame
frame, err := tcdl.NewTCTransferFrame(0x1A, 1, data, tcdl.WithBypass())

// Control command frame
frame, err := tcdl.NewTCTransferFrame(0x1A, 1, data, tcdl.WithControlCommand())

// Frame with segment header (MAP sublayer)
sh := tcdl.SegmentHeader{SequenceFlags: tcdl.SegUnsegmented, MAPID: 0}
frame, err := tcdl.NewTCTransferFrame(0x1A, 1, data, tcdl.WithSegmentHeader(sh))

// Frame with explicit sequence number (for COP-1)
frame, err := tcdl.NewTCTransferFrame(0x1A, 1, data, tcdl.WithSequenceNumber(42))

// Combining options
frame, err := tcdl.NewTCTransferFrame(0x1A, 1, data,
    tcdl.WithSegmentHeader(sh),
    tcdl.WithSequenceNumber(42),
)
```

### Encoding and Decoding

```go
// Encode to bytes (includes CRC-16)
encoded, err := frame.Encode()

// Encode without Frame Error Control
raw, err := frame.EncodeWithoutFEC()

// Decode bytes back to a frame (validates CRC)
frame, err := tcdl.DecodeTCTransferFrame(encoded)

// Check frame type
if tcdl.IsBypass(frame) { /* Type-B expedited frame */ }
if tcdl.IsControlFrame(frame) { /* Control command */ }
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
| Version (2b)   | Bypass (1b)    | CtrlCmd (1b)   | Rsvd (2b)      |
+----------------+----------------+----------------+----------------+
| Spacecraft ID (10b)             | VCID (6b)      | Frame Len (10b)|
+----------------+----------------+----------------+----------------+
| Frame Sequence Number (8b)                                        |
+----------------+----------------+----------------+----------------+
| Segment Header (optional, 1 byte)                                 |
+----------------+----------------+----------------+----------------+
| Frame Data Field (variable)                                       |
+----------------+----------------+----------------+----------------+
| Frame Error Control (CRC-16-CCITT, 2 bytes)                       |
+----------------+----------------+----------------+----------------+
```

### Primary Header

The 5-byte primary header identifies and routes each frame:

| Field | Bits | Range | Description |
|-------|------|-------|-------------|
| Version Number | 2 | 0 | Transfer Frame Version (`00` for TC) |
| Bypass Flag | 1 | 0-1 | 0=Type-A (sequence-controlled), 1=Type-B (expedited) |
| Control Command Flag | 1 | 0-1 | 0=data transfer, 1=control command |
| Reserved | 2 | 0 | Spare bits, must be `00` |
| Spacecraft ID | 10 | 0-1023 | Identifies the spacecraft |
| Virtual Channel ID | 6 | 0-63 | Identifies the virtual channel |
| Frame Length | 10 | 0-1023 | Total frame octets minus 1 |
| Frame Sequence Number | 8 | 0-255 | Per-VC sequence number N(S) for COP-1 |

### Segment Header

Optional 1-byte header for the MAP sublayer:

| Field | Bits | Range | Description |
|-------|------|-------|-------------|
| Sequence Flags | 2 | 0-3 | `11`=unsegmented, `01`=first, `00`=continuation, `10`=last |
| MAP ID | 6 | 0-63 | Multiplexer Access Point Identifier |

**Sequence flag constants:**

```go
tcdl.SegUnsegmented  // 11 - complete, standalone data unit
tcdl.SegFirst        // 01 - first segment of a multi-frame data unit
tcdl.SegContinuation // 00 - middle segment
tcdl.SegLast         // 10 - last segment
```

## Virtual Channels

A `VirtualChannel` is a buffered frame queue identified by a VCID (0-63). TC supports up to 64 Virtual Channels — significantly more than TM's 8.

```go
// Create with VCID=1 and buffer capacity of 100 frames
vc := tcdl.NewVirtualChannel(1, 100)
```

## Services

Three service types provide different data transfer models over Virtual Channels:

### MAP Packet Service

Supports segmentation: packets larger than one frame are automatically split across multiple frames using segment header sequence flags.

```go
counter := tcdl.NewFrameCounter()
vc := tcdl.NewVirtualChannel(1, 100)
svc := tcdl.NewMAPPacketService(0x1A, 1, 0, false, vc, counter)

// Send packets — automatically segmented if too large
err := svc.Send(packetData)

// Receive — reassembles segments into complete packets
svc.SetPacketSizer(spp.PacketSizer)
pkt, err := svc.Receive()
```

**Segmentation behavior:**
- Packets that fit in a single frame are sent as `Unsegmented`.
- Large packets are split: `First` segment, zero or more `Continuation` segments, and a `Last` segment.
- Each segment is placed in a separate TC frame with the appropriate segment header flags.

**Bypass mode:**

```go
// Create a bypass (Type-B) MAP Packet Service — frames skip COP-1 sequencing
svc := tcdl.NewMAPPacketService(0x1A, 1, 0, true, vc, counter)
```

### MAP Access Service

Sends raw data units without packet boundaries. Each data unit produces a single unsegmented frame.

```go
svc := tcdl.NewMAPAccessService(0x1A, 1, 0, false, vc, counter)

// Send a raw data unit
err := svc.Send(rawData)

// Receive the data field
data, err := svc.Receive()
```

### VC Frame Service

Pass-through service — sends and receives pre-encoded frames without modification.

```go
vc := tcdl.NewVirtualChannel(2, 100)
vcf := tcdl.NewVCFrameService(2, vc)

// Send a pre-encoded frame
err := vcf.Send(encodedFrameBytes)

// Receive an encoded frame
data, err := vcf.Receive()
```

### Frame Counter

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

## Service Manager

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

## Full Pipeline Example

### Send Path (Ground to Spacecraft)

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

### Receive Path (Spacecraft)

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
frame, err := tcdl.DecodeTCTransferFrame(receivedBytes)
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
|-------|---------|
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

## Reference

- [CCSDS 232.0-B-4](https://public.ccsds.org/Pubs/232x0b4.pdf) — TC Space Data Link Protocol Blue Book
- [CCSDS 232.1-B-2](https://public.ccsds.org/Pubs/232x1b2.pdf) — Communications Operation Procedure-1 (COP-1)
- [`cop` package](cop.md) — COP-1 (FOP-1, FARM-1, CLCW)
