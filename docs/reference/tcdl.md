# TC Space Data Link Protocol

> CCSDS 232.0-B-4 — TC Space Data Link Protocol

## Overview

The Telecommand (TC) Space Data Link Protocol is a **Data Link Layer** protocol for transmitting telecommand data from ground stations to spacecraft. It takes command data and packages it into **TC Transfer Frames** that can be reliably transmitted over the uplink.

If the TM protocol is the postal system for receiving letters from a spacecraft, the TC protocol is the postal system for sending letters to it. But the TC protocol has a fundamentally different design philosophy: because commands must arrive correctly (a wrong command could damage or destroy a spacecraft), TC is built around **reliability** rather than throughput.

### Where TCDL Fits

```
+-----------------------------------------+
|  Space Packet Protocol / Other Upper    |
|  Application data (commands)            |
+-----------------------------------------+
|  TC Space Data Link Protocol (TCDL)     |  <-- Data Link Layer
|  Packs commands into Transfer Frames    |
|  Virtual Channels, MAP sublayer         |
+-----------------------------------------+
|  COP-1 (CCSDS 232.1-B-2)               |
|  Reliable delivery via FOP-1/FARM-1     |
+-----------------------------------------+
|  TC Sync & Channel Coding               |
|  CLTU construction, BCH encoding        |
+-----------------------------------------+
|  Physical Layer (RF uplink)             |
+-----------------------------------------+
```

The protocol receives command data from upper layers, encapsulates it into Transfer Frames, and works with COP-1 to ensure reliable delivery to the spacecraft.

### Key Characteristics

- **Variable-length frames**: TC frames can be up to 1024 bytes, unlike TM's fixed-length frames. This avoids wasting bandwidth on small commands.
- **64 Virtual Channels**: TC supports up to 64 VCs (vs. TM's 8), reflecting the greater diversity of command types and priorities.
- **Reliable delivery**: TC works with COP-1 to provide sequence-controlled delivery with acknowledgment and retransmission.
- **Two frame types**: Type-A (sequence-controlled) frames follow COP-1 rules; Type-B (bypass) frames skip sequencing for urgent commands.
- **MAP sublayer**: An optional segmentation mechanism that splits large command data units across multiple frames.

### TC vs. TM: Key Differences

| Aspect | TM (Downlink) | TC (Uplink) |
|--------|--------------|-------------|
| Direction | Spacecraft -> Ground | Ground -> Spacecraft |
| Frame length | Fixed per channel | Variable (up to 1024B) |
| Virtual Channels | 8 (3 bits) | 64 (6 bits) |
| Frame counter | MC (8b) + VC (8b) | VC only (8b, N(S)) |
| Reliability | One-way, no ACK | COP-1 with ACK/retransmit |
| Segmentation | FHP-based packet packing | Segment Header with MAP |
| Idle frames | Yes (continuous transmission) | No (demand-driven) |

## Transfer Frame Structure

The TC Transfer Frame is the fundamental data unit. Unlike TM, TC frames are **variable-length** — each frame carries exactly as much data as needed, up to 1024 bytes total.

```
+-----------+----------+---------+-----+
|  Primary  | Segment  |  Data   | FEC |
|  Header   | Header   |  Field  |     |
|  (5B)     | (opt,1B) |(variable|(2B) |
+-----------+----------+---------+-----+
|<--------- Up to 1024 bytes --------->|
```

### Primary Header (5 bytes)

The primary header is 5 bytes (40 bits) — one byte shorter than TM's 6-byte header. Every bit is carefully allocated:

```
Byte 0: Frame Identification
+----------+--------+---------+--------+----------+
| TFVN(2b) |Bypass  |CtrlCmd  |Rsvd    |SCID_hi   |
|          | (1b)   | (1b)    |(2b)    | (2b)     |
+----------+--------+---------+--------+----------+

Byte 1: Spacecraft ID (low 8 bits)

Byte 2: Channel and Length
+----------+-------------------+
| VCID(6b) | Frame Length_hi   |
|          |     (2b)          |
+----------+-------------------+

Byte 3: Frame Length (low 8 bits)

Byte 4: Frame Sequence Number (8 bits)
```

#### Transfer Frame Version Number (2 bits)

Always `00` for TC Transfer Frames. Distinguishes TC frames from TM frames on shared ground systems.

#### Bypass Flag (1 bit)

| Value | Name | Meaning |
|-------|------|---------|
| `0` | Type-A | Sequence-controlled. Subject to COP-1 acceptance rules. Frame must arrive in order. |
| `1` | Type-B | Expedited/bypass. Always accepted by FARM-1. Skips sequence checking. |

Type-B is used for urgent commands that must get through regardless of the current sequencing state — for example, an emergency safe-mode command when COP-1 is in lockout.

#### Control Command Flag (1 bit)

| Value | Meaning |
|-------|---------|
| `0` | Data transfer frame |
| `1` | Control command (COP-1 directive) |

Control commands are used to manage COP-1 itself — for example, to unlock FARM-1 from lockout state or to set the V(R) counter to a specific value.

#### Spacecraft Identifier (10 bits)

Identifies the target spacecraft. Range: 0-1023. Same field as in TM, ensuring consistent addressing across uplink and downlink.

#### Virtual Channel Identifier (6 bits)

Identifies which of up to 64 Virtual Channels this frame belongs to. Range: 0-63.

**Why 64 VCs instead of TM's 8?** Telecommanding often requires finer-grained bandwidth allocation. Different command priorities (emergency, real-time, deferred), different command sources (primary/backup ground station), and different subsystem destinations may each need their own VC.

#### Frame Length (10 bits)

Total frame length **minus one**, in bytes. Range: 0-1023, representing actual lengths of 1-1024 bytes. This is a significant difference from TM, where frame length is fixed and not carried in the header.

#### Frame Sequence Number (8 bits)

The per-VC sequence counter N(S) used by COP-1. Range: 0-255, wrapping. This counter is managed by FOP-1 on the ground side and checked by FARM-1 on the spacecraft side.

Unlike TM (which has separate MC and VC counters), TC has only this single per-VC counter. There is no Master Channel counter because TC uplink doesn't need MC-level gap detection — COP-1 provides per-VC reliability.

### Segment Header (optional, 1 byte)

When the MAP (Multiplexer Access Point) sublayer is used, a 1-byte Segment Header follows the Primary Header:

| Field | Bits | Description |
|-------|------|-------------|
| Sequence Flags | 2 | Segmentation status |
| MAP ID | 6 | Multiplexer Access Point Identifier (0-63) |

**Sequence Flags:**

| Value | Name | Meaning |
|-------|------|---------|
| `11` | Unsegmented | Complete data unit in one frame |
| `01` | First | First segment of a multi-frame data unit |
| `00` | Continuation | Middle segment |
| `10` | Last | Last segment |

The MAP sublayer allows a single Virtual Channel to carry data from multiple sources (identified by MAP ID) and supports segmentation of data units larger than one frame.

### Frame Error Control (2 bytes)

CRC-16-CCITT checksum over the entire frame (excluding the FEC itself). Uses polynomial `x^16 + x^12 + x^5 + 1` (0x1021) with initial value `0xFFFF`.

## Channel Hierarchy

TC uses a similar channel hierarchy to TM but with some key differences:

```
Physical Channel
  └── Master Channel (one per spacecraft)
        ├── Virtual Channel 0 (e.g., real-time commands)
        ├── Virtual Channel 1 (e.g., deferred commands)
        ├── Virtual Channel 2 (e.g., file uploads)
        └── ...up to 64 Virtual Channels (0-63)
```

### Physical Channel

The uplink RF connection. Unlike TM, TC physical channels do not require continuous transmission — frames are sent on demand.

### Master Channel

Identified by TFVN + SCID (Master Channel Identifier). Unlike TM, there is no MC frame counter — reliability is handled per-VC by COP-1.

### Virtual Channel

Each VC has its own independent Frame Sequence Number (N(S)) counter, its own COP-1 instance (FOP-1 on ground, FARM-1 on spacecraft), and its own data stream.

## Services

### MAP Packet Service

Provides packet-oriented data transfer with automatic segmentation. The MAP sublayer uses Segment Headers to split large packets across multiple frames and reassemble them on the spacecraft side.

### MAP Access Service

Provides raw data transfer through the MAP sublayer. Each data unit is sent in a single unsegmented frame. No packet boundary information is maintained.

### VC Frame Service

Pass-through service — the upper layer provides complete pre-built TC Transfer Frames. No segmentation, no MAP sublayer.

## Relationship with COP-1

COP-1 is the heart of TC reliability. Every Type-A frame carries a sequence number N(S) in the header that ties into the COP-1 sliding window protocol:

1. **FOP-1** (ground) assigns N(S) = V(S) and increments V(S).
2. **FARM-1** (spacecraft) checks N(S) against V(R) — the next expected sequence number.
3. If N(S) == V(R): frame accepted, V(R) incremented.
4. If N(S) is within the window but != V(R): frame rejected, retransmit requested.
5. If N(S) is outside the window: lockout — ground must send an unlock command.
6. FARM-1 reports its state via the **CLCW** in the TM return link's OCF field.

This makes TC fundamentally different from TM: lost frames are detected **and recovered** through retransmission, not just detected.

## Design Rationale

**Why variable-length frames?** Commands are typically much smaller than telemetry data. A "turn on heater" command might be 10 bytes; forcing it into a 1024-byte fixed frame would waste 99% of the uplink bandwidth. Variable-length frames match the natural size of commands.

**Why 64 Virtual Channels?** TC needs finer-grained multiplexing than TM. Emergency commands, routine housekeeping, file uploads, and COP-1 management traffic all benefit from separate VCs with independent sequencing and flow control.

**Why no MC frame counter?** COP-1 provides per-VC reliability with acknowledgment and retransmission. An MC-level counter would be redundant — if you know every VC's frames arrived correctly, you know all MC frames arrived correctly.

**Why the MAP sublayer?** Space Packets can be up to 65,542 bytes but TC frames max out at 1,024 bytes. The MAP sublayer provides a standard way to segment large packets across multiple frames without requiring upper layers to manage fragmentation.

**Why two frame types?** In an emergency, you need commands to reach the spacecraft immediately, even if the normal COP-1 sequencing is in a bad state (lockout, window full). Type-B bypass frames provide this safety valve — they always get through regardless of sequence state.

## Reference

- [CCSDS 232.0-B-4](https://public.ccsds.org/Pubs/232x0b4.pdf) — TC Space Data Link Protocol (Blue Book)
- [CCSDS 232.1-B-2](https://public.ccsds.org/Pubs/232x1b2.pdf) — Communications Operation Procedure-1 (Blue Book)
- [CCSDS 230.1-G-2](https://public.ccsds.org/Pubs/230x1g2.pdf) — TC Synchronization and Channel Coding Summary (Green Book)
