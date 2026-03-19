# Communications Operation Procedure-1 (COP-1)

The `cop` package implements the CCSDS 232.1-B-2 Communications Operation Procedure-1 — the reliable frame delivery protocol used to ensure TC frames arrive correctly at the spacecraft.

## Quick Start

```go
import "github.com/ravisuhag/astro/pkg/cop"

// Ground side: FOP-1 manages frame transmission
fop := cop.NewFOP(0x1A, 1, 10) // SCID=0x1A, VCID=1, window=10
fop.Initialize(0)
fop.TransmitFrame(encodedFrame)

// Spacecraft side: FARM-1 validates incoming frames
farm := cop.NewFARM(1, 10) // VCID=1, window=10
accepted, err := farm.ProcessFrame(0, 0, frameSeqNum) // Type-A data frame

// CLCW carries acknowledgment back via TM return link
clcw := farm.GenerateCLCW()
encoded, _ := clcw.Encode()
```

## How COP-1 Works

COP-1 provides reliable TC frame delivery over the inherently unreliable space link. It uses a sliding window protocol with three cooperating components:

```
Ground Station                              Spacecraft
+------------------+                        +------------------+
|   FOP-1          |    TC Uplink           |   FARM-1         |
|   (sends frames, |  ─────────────────>    |   (validates     |
|    manages window,|   TC Transfer Frames  |    sequence,     |
|    retransmits)   |                       |    accepts/       |
|                   |    TM Return Link     |    rejects)      |
|   Processes CLCW  |  <─────────────────   |                  |
|                   |   CLCW in TM OCF      |   Generates CLCW |
+------------------+                        +------------------+
```

1. **FOP-1** (ground) assigns sequence numbers to Type-A frames and transmits them.
2. **FARM-1** (spacecraft) checks the sequence number against its expected value V(R).
3. **FARM-1** generates a **CLCW** reporting its state (including V(R)) back via the TM downlink.
4. **FOP-1** processes the CLCW to acknowledge frames, detect lockout, and trigger retransmission.

## Frame Types

TC frames come in two types, determined by the Bypass Flag in the TC header:

| Type | Bypass Flag | Description |
|------|------------|-------------|
| **Type-A** | 0 | Sequence-controlled. Subject to COP-1 window-based acceptance. Frames are delivered in order with gap detection. |
| **Type-B** | 1 | Expedited/bypass. Always accepted by FARM-1. Used for urgent commands that must get through regardless of sequencing state. |

## FOP-1 (Ground Side)

The Flight Operations Procedure manages frame transmission with sliding window acknowledgment.

### Creating and Initializing

```go
// Create FOP-1 for SCID=0x1A, VCID=1 with sliding window width 10
fop := cop.NewFOP(0x1A, 1, 10)

// Initialize — sets V(S) to starting sequence number, enters Active state
fop.Initialize(0)
```

### Transmitting Frames

```go
// Queue a Type-A frame for transmission
// The frame is assigned sequence number V(S), then V(S) increments
err := fop.TransmitFrame(encodedFrame)
if errors.Is(err, cop.ErrFOPWindowFull) {
    // Window exhausted — wait for CLCW acknowledgment
}

// Get the next frame to send (from wait queue or retransmit queue)
data, seqNum, ok := fop.GetNextFrame()
if ok {
    transmit(data)
}
```

### Processing CLCW Acknowledgments

```go
// When a CLCW arrives on the TM return link
var clcw cop.CLCW
clcw.Decode(clcwBytes)

err := fop.ProcessCLCW(&clcw)
if errors.Is(err, cop.ErrFOPLockout) {
    // FARM-1 entered lockout — must send unlock command
}
```

**ProcessCLCW behavior:**
- Acknowledges all sent frames with sequence numbers before V(R).
- If the Retransmit flag is set, re-queues unacknowledged frames for retransmission.
- If the Lockout flag is set, transitions FOP to Initial state.

### Inspecting State

```go
state := fop.State()        // FOPActive or FOPInitial
vs := fop.VS()              // Current V(S) value
pending := fop.PendingCount() // Unacknowledged frames in sent queue
```

## FARM-1 (Spacecraft Side)

The Frame Acceptance and Reporting Mechanism validates incoming TC frames.

### Creating

```go
// Create FARM-1 for VCID=1 with window width 10
farm := cop.NewFARM(1, 10)
```

### Processing Incoming Frames

```go
// Process a Type-A data frame
accepted, err := farm.ProcessFrame(bypassFlag, controlCommandFlag, frameSeqNum)
```

**Acceptance rules for Type-A frames:**

| Condition | Result |
|-----------|--------|
| N(S) == V(R) | Accepted. V(R) incremented. Retransmit flag cleared. |
| N(S) within window but != V(R) | Rejected. Retransmit flag set. |
| N(S) outside window | Rejected. FARM enters Lockout state. |

**Type-B frames** are always accepted regardless of sequence state.

**Control commands** (Type-A with ControlCommandFlag=1) clear lockout and reset V(R):

```go
// Unlock directive — clears lockout, resets FARM to Open state
accepted, err := farm.ProcessFrame(0, 1, newVR)
```

### Generating CLCW

```go
// Generate a CLCW reflecting current FARM-1 state
clcw := farm.GenerateCLCW()
encoded, _ := clcw.Encode()

// The CLCW is typically placed in the TM Transfer Frame's OCF field
```

### Inspecting State

```go
state := farm.State() // FARMOpen, FARMWait, or FARMLockout
vr := farm.VR()       // Current V(R) — next expected sequence number
```

## CLCW (Communications Link Control Word)

The CLCW is a 4-byte status word generated by FARM-1 and transported to the ground via the TM Operational Control Field (OCF).

### Structure

```
Byte 0: [CWType:1][Version:2][Status:3][COP:2]
Byte 1: [VCID:6][Reserved:2]
Byte 2: [NoRF:1][NoBitLock:1][Lockout:1][Wait:1][Retransmit:1][FARMB:2][spare:1]
Byte 3: [ReportValue:8]
```

| Field | Bits | Description |
|-------|------|-------------|
| Control Word Type | 1 | Always 0 for CLCW |
| Version | 2 | Always 00 |
| Status Field | 3 | Mission-specific status |
| COP in Effect | 2 | 01 = COP-1 |
| Virtual Channel ID | 6 | VC this CLCW reports on |
| No RF Available | 1 | Spacecraft RF status |
| No Bit Lock | 1 | Spacecraft bit lock status |
| Lockout Flag | 1 | FARM-1 is in lockout state |
| Wait Flag | 1 | FARM-1 is in wait state |
| Retransmit Flag | 1 | FARM-1 requests retransmission |
| FARM-B Counter | 2 | Type-B frame acceptance counter (0-3) |
| Report Value | 8 | V(R) — next expected frame sequence number |

### Encoding and Decoding

```go
// Encode CLCW to bytes
clcw := &cop.CLCW{
    COPInEffect:    1,
    VirtualChannelID: 1,
    ReportValue:    42,
}
encoded, err := clcw.Encode()

// Decode CLCW from bytes
var clcw cop.CLCW
err := clcw.Decode(data)

// Human-readable dump
fmt.Println(clcw.Humanize())
```

## Full Integration Example

### Ground-to-Spacecraft Round Trip

```go
import (
    "github.com/ravisuhag/astro/pkg/cop"
    "github.com/ravisuhag/astro/pkg/tcdl"
)

// === Ground Side ===

// 1. Create FOP-1
fop := cop.NewFOP(0x1A, 1, 10)
fop.Initialize(0)

// 2. Build and queue TC frames
frame, _ := tcdl.NewTCTransferFrame(0x1A, 1, commandData,
    tcdl.WithSequenceNumber(fop.VS()),
)
encoded, _ := frame.Encode()
fop.TransmitFrame(encoded)

// 3. Transmit frame over uplink
data, _, ok := fop.GetNextFrame()
if ok {
    transmitUplink(data)
}

// === Spacecraft Side ===

// 4. FARM-1 validates the received frame
farm := cop.NewFARM(1, 10)
accepted, err := farm.ProcessFrame(
    frame.Header.BypassFlag,
    frame.Header.ControlCommandFlag,
    frame.Header.FrameSequenceNum,
)

// 5. Generate CLCW and send on TM return link
clcw := farm.GenerateCLCW()
clcwBytes, _ := clcw.Encode()
// Place clcwBytes in TM Transfer Frame OCF field

// === Ground Side (continued) ===

// 6. Process returned CLCW
var returnedCLCW cop.CLCW
returnedCLCW.Decode(clcwBytes)
fop.ProcessCLCW(&returnedCLCW)

// Frames before V(R) are now acknowledged
```

## Errors

All errors are exported package-level variables, suitable for use with `errors.Is`:

| Error | Meaning |
|-------|---------|
| `ErrDataTooShort` | Data too short to decode CLCW |
| `ErrInvalidCLCWType` | Control word type is not 0 |
| `ErrInvalidCLCWVersion` | CLCW version is not 00 |
| `ErrFOPLockout` | FOP-1 detected lockout from CLCW |
| `ErrFOPWindowFull` | FOP-1 sliding window is full |
| `ErrFARMReject` | FARM-1 rejected frame (out of sequence but within window) |
| `ErrFARMLockout` | FARM-1 is in lockout state |

## Reference

- [CCSDS 232.1-B-2](https://public.ccsds.org/Pubs/232x1b2.pdf) — Communications Operation Procedure-1 Blue Book
- [CCSDS 232.0-B-4](https://public.ccsds.org/Pubs/232x0b4.pdf) — TC Space Data Link Protocol Blue Book
- [`tcdl` package](tcdl.md) — TC Space Data Link Protocol (TC Transfer Frames)
