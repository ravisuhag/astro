---
title: COP-1
short: COP
description: Communications Operation Procedure-1 (CCSDS 232.1-B-2) — reliable telecommand delivery with FOP-1 and FARM-1.
order: 25
---

> **CCSDS 232.1-B-2** | [Blue Book](https://public.ccsds.org/Pubs/232x1b2e1.pdf) | [`pkg/cop`](https://github.com/ravisuhag/astro/tree/main/pkg/cop) | [`astro cop`](/cli/cop)

COP-1 is what makes telecommand reliable. It sits on top of [TC](/protocols/data-link/tcdl) and manages sequence numbers, acknowledgement, and retransmission. It is a sliding window protocol, like TCP's, built for links where the round trip is seconds to hours and a wrong command can end the mission.

It has two halves. **FOP-1** runs on the ground and decides what to send. **FARM-1** runs on the spacecraft and decides what to accept. They talk through the **CLCW**, a 4-byte status word that rides home in the OCF of a [TM frame](/protocols/data-link/tmdl).

Most of this page is behaviour, not format. The state machines are the protocol.

## Scope

**Implemented.** Both state machines in full — FOP-1's six states and FARM-1's three — every operator directive, the T1 timer, the sliding window with its positive and negative halves, and CLCW encode and decode.

**Caller-driven.** The T1 timer does not run itself. You set the initial value in whatever time units you like and call `Tick`. Astro has no opinion about your clock.

**Somewhere else.** The Type-BC frames that carry Unlock and Set V(R) are built by [`pkg/tcdl`](/protocols/data-link/tcdl) — `NewUnlockFrame`, `NewSetVRFrame`, `ParseControlCommand`.

## Field map

The CLCW is the only thing on the wire. Go fields on `cop.CLCW`.

| Field | Bits | Go | Notes |
|---|---|---|---|
| Control Word Type | 1 | `ControlWordType` | Always `0` for a CLCW |
| Version | 2 | `Version` | Always `00` |
| Status Field | 3 | `StatusField` | Mission-specific |
| COP in Effect | 2 | `COPInEffect` | `01` = COP-1 |
| Virtual Channel Identifier | 6 | `VirtualChannelID` | Which VC this reports on |
| Reserved | 2 | `Reserved` | Spare |
| No RF Available | 1 | `NoRFAvailableFlag` | |
| No Bit Lock | 1 | `NoBitLockFlag` | |
| Lockout | 1 | `LockoutFlag` | FARM is locked out |
| Wait | 1 | `WaitFlag` | FARM has no buffer |
| Retransmit | 1 | `RetransmitFlag` | FARM wants a resend |
| FARM-B Counter | 2 | `FARMBCounter` | Counts accepted Type-B frames, 0-3 |
| Report Value | 8 | `ReportValue` | V(R), the next sequence number FARM expects |

The Report Value is a cumulative acknowledgement. It means "I have everything below this". One byte confirms a whole window.

## FOP-1, on the ground

`cop.FOP`. Six states:

| State | Name | Meaning |
|---|---|---|
| S1 | Active | Normal. Sending frames. |
| S2 | Retransmit without Wait | Spacecraft asked for a resend; unacknowledged frames are going out again. |
| S3 | Retransmit with Wait | Resend needed, but the spacecraft has no buffer. Hold. |
| S4 | Initialising without BC Frame | Started with CLCW check; waiting for a clean CLCW. |
| S5 | Initialising with BC Frame | Unlock or Set V(R) is out; waiting for the CLCW to confirm. |
| S6 | Initial | Not started, terminated, or an Alert fired. |

An **Alert** — lockout seen, bad N(R), transmission limit hit, inconsistent CLCW, or an operator terminate — purges every queue and drops to S6 with a reason code. The errors carry it: `ErrFOPLockout`, `ErrFOPInvalidNR`, `ErrFOPLimit`, `ErrFOPSynch`, `ErrFOPInvalidCLCW`.

The service can also **suspend** instead of alerting. Under timeout type TT1, a T1 expiry at the transmission limit remembers which state it was in (SS 1-4) so a later Resume picks up there.

Directives, all implemented: Initiate AD Service — plain, with CLCW check, with Unlock, or with Set V(R) — plus Terminate, Resume, Set V(S), Set FOP Sliding Window, Set T1 Initial, Set Transmission Limit, and Set Timeout Type.

### The window

```
    N(R)                 V(S)
      |                    |
  ... [ack] [ack] [sent] [sent] [sent] [not sent] ...
              |<---- FW (window width) ---->|
```

V(S) is the next number to assign. N(R) comes back in the CLCW. When `V(S) - N(R) >= FW` the window is full and nothing more goes out until an acknowledgement arrives — `ErrFOPWindowFull`. Width is 1-255.

## FARM-1, on the spacecraft

`cop.FARM`. Three states: **Open**, **Wait** (no buffer), and **Lockout** (needs ground intervention).

The window W is even and splits in half around V(R): a positive half PW = W/2 ahead, a negative half NW = W/2 behind.

| N(S) is... | What happens |
|---|---|
| Exactly V(R) | **Accept.** Increment V(R), clear the retransmit flag. With no free buffer instead: discard, set Wait and Retransmit. |
| Ahead, within PW | **Discard, set retransmit.** An earlier frame was lost. |
| Behind, within NW | **Discard silently.** A duplicate. No flags change, no lockout. |
| Anything else | **Lockout.** |

Type-B frames skip the whole check and are always accepted. Every accepted Type-B — expedited data and control commands alike — bumps the 2-bit FARM-B counter in the CLCW.

### Control commands

A control command is Bypass=1 *and* Control Command=1. Bypass=0 with Control Command=1 is not a valid frame type and is discarded (`ErrInvalidFrameType`). The data field holds exactly one of two things:

| Directive | Data field | Effect |
|---|---|---|
| Unlock | `0x00` | Clears Lockout, Wait, and Retransmit. Leaves V(R) alone. |
| Set V(R) | `0x82 0x00 <vr>` | Sets V(R) and clears Retransmit. |

## Gotchas

**Set V(R) does not end a lockout.** While FARM is locked out, a Set V(R) is ignored apart from the FARM-B count. Only Unlock clears lockout. Sending Set V(R) first and expecting recovery is a common mistake.

**Unlock does not reset V(R).** It clears flags. After an unlock you still have to read V(R) from the next CLCW and set V(S) to match, or issue Set V(R) to pick a fresh value.

**The negative window is why duplicates are harmless.** Without NW, a retransmitted frame that crossed its own acknowledgement would look like a catastrophic sequence error and trigger lockout. That is the whole reason the window has a back half.

**Directives are state-sensitive.** Several only work from a particular state — `ErrFOPNotActive`, `ErrFOPNotInitial`, `ErrFOPNotSuspended` tell you which. Resume only works from a suspended service, not from S6.

**T1 will not fire unless you tick it.** No goroutine, no wall clock. If your loop forgets to call `Tick`, a lost CLCW stalls the machine forever, which is exactly what T1 exists to prevent.

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

## Notes

Commentary, not sourced from the standard.

**Why a sliding window?** Stop-and-wait would idle the uplink for a full round trip after every frame. That is seconds at GEO, minutes to the Moon, hours further out.

**Why split FOP and FARM?** The ground has computers and operators. The spacecraft side has to be simple enough to trust in a radiation-hardened box: check one number, set a few flags, emit four bytes. The asymmetry in the protocol matches the asymmetry in the hardware.

**Why lockout instead of automatic recovery?** Because the failure mode on the other side is physical. When the two ends have lost track of each other badly enough, guessing is worse than stopping. Lockout makes a person look at it.

**Why no explicit NAK?** The retransmit flag plus V(R) already says "resend from here". Per-frame negative acknowledgements would add uplink traffic and spacecraft-side complexity to say the same thing.

**Why put the CLCW on TM frames?** The downlink is already running, idle frames and all. Feedback costs nothing extra and arrives at a predictable rate.

## Reference

- [CCSDS 232.1-B-2](https://public.ccsds.org/Pubs/232x1b2e1.pdf) — Communications Operation Procedure-1 (Blue Book)
- [CLI](/cli/cop) | [Conformance](/conformance/cop) | [The stack](/docs/start/concepts)