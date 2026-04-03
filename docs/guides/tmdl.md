# TM Space Data Link Protocol

> CCSDS 132.0-B-3 — TM Space Data Link Protocol

## Overview

The Telemetry (TM) Space Data Link Protocol is a **Data Link Layer** protocol for transmitting telemetry data from spacecraft to ground stations. It takes application data (typically Space Packets) and packages it into fixed-length **Transfer Frames** that can be reliably transmitted over the space link.

Think of it as the postal system of spacecraft communication: Space Packets are the letters containing your data, and Transfer Frames are the standardized envelopes that the postal system (the physical link) knows how to handle. The protocol ensures every envelope is the same size, carries proper addressing, and includes integrity checks — even if it has to pad the remaining space with filler.

### Where TMDL Fits

```
┌─────────────────────────────────────────────┐
│  Space Packet Protocol / Other Upper Layer  │
│  Application data in packets                │
├─────────────────────────────────────────────┤
│  TM Space Data Link Protocol (TMDL)         │  ← Data Link Layer
│  Packs data into fixed-length frames        │
│  Virtual Channels, multiplexing, sequencing │
├─────────────────────────────────────────────┤
│  TM Sync & Channel Coding (CCSDS 131.0-B)  │
│  ASM attachment, randomization, FEC         │
├─────────────────────────────────────────────┤
│  Physical Layer (RF/Optical link)           │
└─────────────────────────────────────────────┘
```

The protocol receives data from upper layers (most commonly the Space Packet Protocol), encapsulates it into Transfer Frames, and hands those frames to the Synchronization and Channel Coding layer for physical transmission.

### Key Characteristics

- **Fixed-length frames**: Every frame on a given physical channel has exactly the same length. This simplifies synchronization and hardware design.
- **Virtual Channels**: Up to 8 independent logical channels share the same physical link, enabling bandwidth partitioning between different data streams.
- **One-way protocol**: TM is strictly spacecraft-to-ground. There is no acknowledgment or retransmission — the protocol assumes a simplex downlink.
- **In-order delivery**: Frames carry sequence counters that let the receiver detect gaps but not request retransmission.

## Channel Hierarchy

TMDL organizes data transmission through a three-level channel hierarchy. Understanding this hierarchy is essential to understanding the protocol.

```
Physical Channel
  └── Master Channel (one per spacecraft)
        ├── Virtual Channel 0 (e.g., real-time housekeeping)
        ├── Virtual Channel 1 (e.g., science data)
        ├── Virtual Channel 2 (e.g., stored playback)
        └── ...up to 8 Virtual Channels (0-7)
```

### Physical Channel

A **Physical Channel** is the actual communication link — the radio frequency or optical connection between the spacecraft and the ground station. It has physical properties like bandwidth, bit rate, and modulation scheme.

All frames transmitted on a physical channel must have the **same fixed length**. This is a fundamental constraint: you choose the frame length during mission design, and every frame — whether carrying a full payload or just idle fill — is exactly that size.

A physical channel can carry frames from multiple spacecraft (multiple Master Channels), though in practice most links serve a single spacecraft.

### Master Channel

A **Master Channel** represents all the data from a single spacecraft on a physical channel. It is identified by the combination of:
- **Transfer Frame Version Number (TFVN)**: Always `00` for TM frames
- **Spacecraft Identifier (SCID)**: A 10-bit value (0–1023) unique to each spacecraft on the link

Together, these form the **Master Channel Identifier (MCID)** = TFVN + SCID.

The Master Channel has its own 8-bit frame counter (**MC Frame Count**) that increments with every frame transmitted from this spacecraft, regardless of which Virtual Channel the frame belongs to. This counter lets the ground station detect any frame loss at the spacecraft level.

### Virtual Channel

A **Virtual Channel (VC)** is a logical subdivision of a Master Channel. Each Master Channel supports up to **8 Virtual Channels** (VCID 0–7). Virtual Channels are the primary mechanism for sharing bandwidth between different data streams.

Each Virtual Channel has:
- Its own 8-bit **VC Frame Count** that increments independently
- Its own data stream with independent buffering and flow control
- A specific **service type** (packet, frame, or access — see Services section)

The combination of MCID + VCID forms the **Global Virtual Channel Identifier (GVCID)**, which uniquely identifies a data stream across the entire space communication system.

**Typical Virtual Channel allocation:**

| VCID | Usage | Priority |
|------|-------|----------|
| 0 | Real-time housekeeping | Highest |
| 1 | Real-time science | High |
| 2 | Stored data playback | Medium |
| 3 | File transfer | Low |
| 7 | Idle frames | Lowest |

### Multiplexing

When multiple Virtual Channels have data to send, a **multiplexing scheme** determines which VC gets the next frame slot. The CCSDS standard does not prescribe a specific algorithm — common approaches include:

- **Priority-based**: Higher-priority VCs always go first
- **Round-robin**: VCs take turns
- **Weighted round-robin**: VCs take turns proportional to assigned weights
- **Time-division**: VCs are assigned specific time slots

The same multiplexing concept applies at the Master Channel level when a physical channel serves multiple spacecraft.

## Transfer Frame Structure

The TM Transfer Frame is the fundamental data unit of the protocol. It has a fixed length (chosen per mission) and consists of these fields:

```
┌──────────┬─────────────┬───────────┬─────┬─────┐
│ Primary  │  Secondary  │   Data    │ OCF │ FEC │
│ Header   │  Header     │   Field   │     │     │
│ (6B)     │ (opt,1-64B) │ (variable)│(4B) │(2B) │
└──────────┴─────────────┴───────────┴─────┴─────┘
│←─────────────── Fixed Frame Length ────────────→│
```

### Primary Header (6 bytes)

The primary header is always exactly 6 bytes (48 bits) and is present in every frame. It carries all the information needed to identify, route, and sequence the frame.

```
Byte 0-1: Identification
┌──────────┬─────────────────────┬──────────┬──────┐
│ TFVN(2b) │   Spacecraft ID     │ VCID(3b) │ OCF  │
│          │      (10b)          │          │ (1b) │
└──────────┴─────────────────────┴──────────┴──────┘

Byte 2: Master Channel Frame Count (8b)

Byte 3: Virtual Channel Frame Count (8b)

Byte 4-5: Data Field Status
┌─────┬──────┬────────┬─────────┬──────────────────┐
│ FSH │ Sync │ PktOrd │ SegLen  │ First Header Ptr │
│(1b) │ (1b) │  (1b)  │  (2b)   │     (11b)        │
└─────┴──────┴────────┴─────────┴──────────────────┘
```

#### Transfer Frame Version Number (2 bits)

Always `00` for TM Transfer Frames. This distinguishes TM frames from TC frames (`01`) and AOS frames (`01`) on shared links.

#### Spacecraft Identifier (10 bits)

Identifies the spacecraft that generated the frame. Range: 0–1023. Assigned by the space agency operating the mission and registered with CCSDS to ensure uniqueness on shared ground networks.

#### Virtual Channel Identifier (3 bits)

Identifies which of the 8 possible Virtual Channels this frame belongs to. Range: 0–7.

#### Operational Control Field Flag (1 bit)

Indicates whether the 4-byte OCF is present at the end of the frame (before the FEC). This flag must be the same for all frames on a given physical channel — you cannot mix OCF and non-OCF frames.

#### Master Channel Frame Count (8 bits)

A sequential counter (0–255, wrapping) that increments with every frame from this spacecraft, across all Virtual Channels. If the ground station sees counts 41, 42, 44 — it knows one frame was lost.

#### Virtual Channel Frame Count (8 bits)

A sequential counter (0–255, wrapping) that increments only for frames on this specific Virtual Channel. This provides per-VC loss detection independent of other VCs.

**Example:** If VC 0 sends frames with VC counts 10, 11, 12 and VC 1 sends frames with VC counts 5, 6 — they are independent. The MC count, however, increments across both: it might read 20, 21, 22, 23, 24 across the interleaved frames.

#### Frame Secondary Header Flag (1 bit)

Indicates whether a Secondary Header follows the Primary Header. Must be constant for all frames on a given Virtual Channel.

#### Synchronization Flag (1 bit)

Indicates the type of data in the frame:

| Value | Meaning |
|-------|---------|
| `0` | Data from the VCP or VCF service (packets or frames) |
| `1` | Data from the VCA service (fixed-length access SDUs) |

This flag determines how the **First Header Pointer** field is interpreted.

#### Packet Order Flag (1 bit)

When `SyncFlag=0`, indicates whether packets are in the order generated by the source. Must be `0` when `SyncFlag=0` in current practice. Must be `0` when `SyncFlag=1`.

#### Segment Length Identifier (2 bits)

Reserved for future use. Must be `11` when `SyncFlag=0`.

#### First Header Pointer (11 bits)

This is one of the most important fields in the frame and enables **packet boundary detection** within the fixed-length frame stream.

When `SyncFlag=0` (packet or frame data):

| Value | Meaning |
|-------|---------|
| 0–2045 | Byte offset to the first packet header that starts in this frame's Data Field |
| `0x07FE` (2046) | No packet starts in this frame — it is entirely a continuation of a packet from a previous frame |
| `0x07FF` (2047) | The Data Field contains only idle data (no valid packets) |

**Why this matters:** Packets can be any size, but frames are fixed-length. A packet might start in one frame and end in the next (or span several frames). The First Header Pointer tells the receiver where to find the beginning of a new packet so it can resynchronize after a frame loss.

```
Frame N:                    Frame N+1:
┌─────┬──────────────────┐ ┌─────┬──────────────────────┐
│ HDR │[PktA tail][PktB  │ │ HDR │ PktB cont][PktC][pad]│
│     │          ↑       │ │     │           ↑          │
│     │  FHP = offset    │ │     │   FHP = offset       │
│     │  to PktB start   │ │     │   to PktC start      │
└─────┴──────────────────┘ └─────┴──────────────────────┘
```

When `SyncFlag=1` (VCA access data): the FHP must be `0x07FF`.

### Secondary Header (optional)

The secondary header is mission-defined and provides a place for data that applies to every frame on a Virtual Channel — most commonly a timestamp. It has a one-byte prefix:

| Field | Bits | Description |
|-------|------|-------------|
| Version | 2 | Always `00` |
| Header Length | 6 | Length of data field minus 1 (0–63) |

Followed by 1–64 bytes of mission-defined data. The Header Length field makes the secondary header self-describing, so a decoder can skip it without knowing its internal format.

### Transfer Frame Data Field

This is where the actual payload lives. Its size is determined by the frame length minus the header(s), OCF, and FEC:

```
Data Field Capacity = Frame Length - 6 (primary header)
                    - Secondary Header size (if present)
                    - 4 (if OCF present)
                    - 2 (if FEC present)
```

The Data Field content depends on the service type (see Services section).

### Operational Control Field (optional, 4 bytes)

The OCF carries a **Communications Link Control Word (CLCW)** used by the Communications Operation Procedure (COP-1) for telecommand acknowledgment. Even though TM is one-way, the OCF piggybacks TC acknowledgment data on the downlink.

The OCF presence is fixed for all frames on a physical channel — you cannot include it in some frames and omit it in others.

### Frame Error Control (2 bytes)

A CRC-16-CCITT checksum computed over the entire frame (excluding the FEC field itself). Uses polynomial `x^16 + x^12 + x^5 + 1` (0x1021).

The FEC lets the receiver detect corrupted frames. Unlike higher-layer CRCs, this one protects the frame header as well as the data, ensuring that routing information (SCID, VCID) is not corrupted.

## Services

The TM Space Data Link Protocol defines three services that determine how upper-layer data is placed into Transfer Frame Data Fields:

### Virtual Channel Packet Service (VCP)

The most common service. VCP packs **Space Packets** (or similar variable-length protocol data units) into the fixed-length Data Field, using the **First Header Pointer** to mark packet boundaries.

**Key behaviors:**
- Multiple packets can fit in a single frame
- A single packet can span multiple frames
- The FHP tells the receiver where the first new packet starts in each frame
- Idle fill (`0xFF`) pads unused space at the end of the Data Field

```
Frame with two complete packets and start of a third:
┌─────┬───────────┬──────────┬──────────┬───────┐
│ HDR │  Packet A │ Packet B │ Packet C │  pad  │
│     │ (complete)│(complete)│ (start)  │ 0xFF  │
│     │↑ FHP=0    │          │          │       │
└─────┴───────────┴──────────┴──────────┴───────┘

Frame that is entirely continuation data:
┌─────┬──────────────────────────────────────────┐
│ HDR │  Packet C (continuation)                 │
│     │  FHP = 0x07FE (no new packet starts)     │
└─────┴──────────────────────────────────────────┘
```

**Receiving with FHP-based resync:** If a frame is lost, the receiver discards its partial packet buffer and waits for the next frame with a valid FHP (not `0x07FE`). It then starts reading from the FHP offset, skipping any continuation data from the lost packet. This is how the protocol recovers from frame loss without retransmission.

### Virtual Channel Frame Service (VCF)

A pass-through service — the upper layer provides a complete, pre-built Transfer Frame, and the service inserts it directly. No multiplexing, no FHP management. This is used when the upper layer needs full control over frame content.

### Virtual Channel Access Service (VCA)

A fixed-length service where each frame carries exactly one Service Data Unit (SDU) of a predetermined size. The VCA service sets:
- `SyncFlag = 1`
- `FirstHeaderPtr = 0x07FF`

This service is used for fixed-rate data streams like housekeeping telemetry sampled at regular intervals. Each frame always contains one complete SDU — no spanning, no multiplexing.

## Physical Layer Interface

> **Implementation note:** In astro, the sync layer (ASM attachment/stripping, pseudo-randomization, and CADU wrapping/unwrapping) is handled by the `pkg/tmsc` package, not by `pkg/tmdl`. The `tmdl.PhysicalChannel` handles MC multiplexing and demultiplexing only.

### Attached Sync Marker (ASM)

Before transmission, each Transfer Frame is prepended with an **Attached Sync Marker** — a known bit pattern that the receiver uses to find frame boundaries in the continuous bitstream. The standard ASM for TM is:

```
0x1ACFFC1D  (hex)
0001 1010 1100 1111 1111 1100 0001 1101  (binary)
```

This 32-bit pattern was carefully chosen for its autocorrelation properties — it is easy to detect reliably even in the presence of noise.

The combination of ASM + Transfer Frame is called a **Channel Access Data Unit (CADU)**:

```
┌──────┬────────────────────────┐
│ ASM  │    Transfer Frame      │
│ (4B) │                        │
└──────┴────────────────────────┘
│←──────── CADU ────────────────→│
```

### Pseudo-Randomization

To ensure good signal properties (preventing long runs of identical bits that can confuse clock recovery), the CCSDS standard defines an optional pseudo-randomization step. The Transfer Frame bytes are XORed with a pseudo-random sequence generated by an **8-bit LFSR** (Linear Feedback Shift Register) with polynomial:

```
h(x) = x^8 + x^7 + x^5 + x^3 + 1
```

The LFSR is initialized to all 1s (`0xFF`) and generates one bit per clock. The randomizer is applied after encoding and removed before decoding — it is transparent to all protocol layers above.

**Important:** The ASM is never randomized. Only the Transfer Frame content is XORed.

## Idle Frames

When no Virtual Channel has data to send but the link must keep transmitting (continuous-mode links), the protocol generates **idle frames**:

- Data Field filled entirely with `0xFF` (idle fill)
- `FirstHeaderPtr = 0x07FF` (indicating no valid data)
- `SyncFlag = 0`
- Typically sent on VCID 7 (convention, not required)

Idle frames still carry valid headers with incrementing MC/VC frame counts, maintaining synchronization.

## Frame Gap Detection

Both the MC Frame Count and VC Frame Count are 8-bit counters that wrap from 255 to 0. The receiver tracks expected counts and reports gaps:

- **MC gap**: Indicates any frame was lost from this spacecraft, regardless of VC
- **VC gap**: Indicates a frame was lost on a specific Virtual Channel

A gap triggers recovery actions in upper layers — for VCP, the receiver discards its partial packet buffer and waits for FHP-based resync.

```
Expected MC: 42, Received MC: 44  →  MC gap = 1 frame lost
Expected VC: 10, Received VC: 10  →  VC gap = 0 (lost frame was on a different VC)
```

## Design Rationale

**Why fixed-length frames?** Fixed-length frames greatly simplify receiver hardware. The receiver knows exactly how many bytes to expect after each ASM, eliminating the need for length fields or delimiters. This is critical for deep-space links operating at extremely low signal-to-noise ratios.

**Why only 8 Virtual Channels?** 8 channels (3 bits) is sufficient for most missions while keeping the header compact. The VC mechanism provides bandwidth sharing without the overhead of a full multiplexing protocol. Missions needing more separation can use APID multiplexing within a single VC.

**Why 8-bit frame counters?** With 256 values, even at high frame rates, the counter wraps infrequently enough to detect single-frame losses (the most common failure mode) while keeping the header small. For links where multi-frame losses are common, the protocol relies on upper-layer mechanisms.

**Why no retransmission?** TM links are typically one-way (simplex) with light-speed delays of seconds to hours. Retransmission would be too slow to be useful. Instead, the protocol focuses on detection (via counters) and recovery (via FHP resync), while forward error correction at the coding layer handles bit errors.

## Reference

- [CCSDS 132.0-B-3](https://public.ccsds.org/Pubs/132x0b3.pdf) — TM Space Data Link Protocol (Blue Book)
- [CCSDS 130.1-G-3](https://public.ccsds.org/Pubs/130x1g3.pdf) — TM Synchronization and Channel Coding Summary (Green Book)
- [CCSDS 131.0-B-5](https://public.ccsds.org/Pubs/131x0b5.pdf) — TM Synchronization and Channel Coding (Blue Book)
