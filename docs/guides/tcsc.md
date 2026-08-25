# TC Synchronization and Channel Coding

> CCSDS 231.0-B-4 — TC Synchronization and Channel Coding

## Overview

The TC Synchronization and Channel Coding sublayer is the bridge between the TC Data Link Protocol and the physical uplink. It provides three critical functions: **CLTU framing** (how the receiver finds command boundaries), **BCH error correction** (detecting and correcting bit errors in each codeblock), and **pseudo-randomization** (ensuring good signal properties).

This sublayer is the telecommand counterpart to the TM Synchronization and Channel Coding (`tmsc`) sublayer, but with fundamentally different design choices driven by the unique requirements of commanding a spacecraft.

### Where TCSC Fits

```
+-----------------------------------------+
|  TC Space Data Link Protocol (TCDL)     |
|  Builds variable-length TC Frames       |
+-----------------------------------------+
|  TC Sync & Channel Coding (TCSC)        |  <-- This sublayer
|  CLTU construction, BCH FEC,            |
|  pseudo-randomization                   |
+-----------------------------------------+
|  Physical Layer (RF uplink)             |
+-----------------------------------------+
```

### Key Characteristics

- **Per-codeblock FEC**: Each 7-byte block gets its own BCH parity — no single codeword spans the entire frame, unlike TM's Reed-Solomon.
- **Immediate error detection**: The spacecraft can detect and reject a corrupted codeblock as it arrives, without waiting for the entire frame.
- **CLTU-based framing**: Uses start/tail sequences instead of TM's Attached Sync Marker, reflecting the on-demand (not continuous) nature of TC transmission.
- **Conservative correction**: BCH offers two decoding modes. SEC (Single Error Correction) corrects 1 bit per codeblock. TED (Triple Error Detection) corrects nothing but is guaranteed to detect up to 3 bit errors. For commands, it is safer to reject and retransmit than to risk a miscorrection, which is why TED exists.

### TC vs. TM Sync Layer: Key Differences

| Aspect | TM (Downlink) | TC (Uplink) |
|--------|--------------|-------------|
| Framing unit | CADU (ASM + frame) | CLTU (start + codeblocks + tail) |
| Sync pattern | 4-byte ASM (0x1ACFFC1D) | 2-byte start (0xEB90) + 8-byte tail |
| FEC code | Reed-Solomon (codeword = 255 bytes) | BCH(63,56) (codeblock = 8 bytes) |
| Correction | Up to 16 symbol errors per codeword | Up to 1 bit error per codeblock |
| Transmission | Continuous (fixed-rate bitstream) | On-demand (CLTUs sent when needed) |
| Frame length | Fixed (same for all frames) | Variable (CLTU adapts to frame size) |

## Command Link Transmission Unit (CLTU)

The CLTU is the fundamental transmission unit for telecommands. Unlike TM's continuous bitstream of fixed-length frames, TC CLTUs are sent individually on demand — each CLTU carries one TC Transfer Frame.

### Structure

```
+-----------+------------+-----+------------+----------+
|  Start    | Codeblock  | ... | Codeblock  |  Tail    |
|  Sequence |     1      |     |     N      | Sequence |
|  (2B)     |   (8B)     |     |   (8B)     |  (8B)    |
+-----------+------------+-----+------------+----------+
```

### Start Sequence

The 2-byte start sequence `0xEB90` marks the beginning of a CLTU. The receiver uses it to detect when a new command is arriving.

**Why only 2 bytes (vs. TM's 4-byte ASM)?** TC uplinks typically have much better signal conditions than deep-space TM downlinks. The ground station controls transmission power and timing precisely, and the uplink operates at a much lower data rate. A shorter sync pattern is sufficient and reduces overhead.

### Codeblocks

The frame data is divided into 7-byte blocks, each encoded into an 8-byte codeblock using BCH(63,56). If the frame data is not a multiple of 7 bytes, the last block is padded with fill bytes (`0x55`).

### Tail Sequence

The 8-byte tail sequence `0xC5C5C5C5C5C5C579` signals the end of the CLTU. It is not a valid BCH codeblock — the pattern is chosen so that the decoder rejects it, even after any single bit error. That rejection is what terminates reception: the receiver stops at the first codeblock that fails to decode, so it does not need a bit-exact tail.

## BCH(63,56) Error Correction

### Why BCH Instead of Reed-Solomon?

TM uses Reed-Solomon codes that span 255 bytes — powerful but requiring the entire codeword to decode. For telecommanding, this is problematic:

1. **Latency**: The spacecraft would need to buffer 255 bytes before it could decode anything. For a safety-critical command, every millisecond matters.
2. **Granularity**: A single uncorrectable error in a 255-byte RS codeword discards the entire block. With BCH, only the affected 8-byte codeblock is rejected.
3. **Safety**: Commands must be correct with extremely high confidence. BCH's conservative approach — correct at most 1 bit in SEC mode, or correct nothing and detect up to 3 in TED mode — keeps the miscorrection risk small. Reed-Solomon's more aggressive correction increases the (tiny but nonzero) risk of a miscorrection.

### How It Works

Each codeblock uses a shortened BCH code:

```
Information: 56 bits (7 bytes)
        ↓
    BCH Encode (generator: g(x) = x^7 + x^6 + x^2 + 1)
        ↓
Codeblock: 56 info bits + 7 parity bits + 1 filler bit = 64 bits (8 bytes)
```

**Generator polynomial**: g(x) = x^7 + x^6 + x^2 + 1

The encoding is systematic: the 7 information bytes appear unchanged in the codeblock, followed by 1 byte containing 7 parity bits and a filler bit. Two details of that last byte matter for interoperability:

- The parity bits placed on the wire are the **complement** of the LFSR remainder (CCSDS 231.0-B-4 3.3).
- The filler bit is **always '0'** (3.3.2).

Known-answer check: all-zeros information octets encode to a codeblock whose last octet is `0xFE` (remainder 0, complemented to `1111111`, filler `0`).

### Encoding Process

The encoder uses a 7-bit Linear Feedback Shift Register (LFSR) driven by the generator polynomial. Each of the 56 information bits is clocked through the LFSR:

```
info bits → [LFSR with g(x) = x^7 + x^6 + x^2 + 1] → 7 parity bits
```

After all 56 bits are processed, the complement of the LFSR contents gives the 7 parity bits. These are packed into the 8th byte along with the '0' filler bit.

### Decoding Process

1. **Syndrome computation**: Complement the received parity bits (undoing the on-the-wire inversion), then feed all 63 code bits (56 info + 7 parity) through the same LFSR.
2. **Zero syndrome**: No errors detected — return the information bytes.
3. **Non-zero syndrome, TED mode**: Report the error. Nothing is corrected; up to 3 bit errors are guaranteed to be detected.
4. **Non-zero syndrome, SEC mode**: The syndrome value identifies the error position. Search all 63 possible single-bit error positions to find a match.
5. **Match found (SEC)**: Correct the bit at that position (1 bit corrected).
6. **No match (SEC)**: The error pattern cannot be a single-bit error — report uncorrectable.

Note the trade-off: SEC mode fixes single-bit errors but a 3-bit error pattern can silently miscorrect. Missions that prefer certainty over correction run the decoder in TED mode.

### The Filler Bit

The filler bit is fixed at '0' (CCSDS 231.0-B-4 3.3.2). It exists only to round the 63-bit BCH codeword up to a whole number of octets (64 bits); the decoder ignores it.

## Pseudo-Randomization

TC uses the same pseudo-randomization scheme as TM: XOR with a PN sequence generated by an 8-bit LFSR.

- **Polynomial**: h(x) = x^8 + x^7 + x^5 + x^3 + 1
- **Initial state**: All 1s (0xFF)
- **Application**: Applied after padding and before BCH encoding, so it covers the frame data AND the 0x55 fill octets — everything between the start and tail sequences. The start and tail sequences themselves are never randomized.

The purpose is identical to TM: prevent long runs of identical bits that could cause clock recovery issues at the receiver.

## Physical Layer Operations Procedures (PLOP)

A PLOP (CCSDS 231.0-B-4 section 8) decides how CLTUs are placed onto the physical channel, using two helper sequences (section 7). Both are the alternating bit pattern `0101...` (0x55 octets):

- **Acquisition sequence**: sent before data so the receiver can lock its bit clock. At least 16 octets (128 bits) are recommended.
- **Idle sequence**: keeps the channel modulated between CLTUs.

Two procedures exist:

| Procedure | Behavior |
|-----------|----------|
| **PLOP-1** | The session ends after each CLTU. Every CLTU gets its own acquisition sequence. |
| **PLOP-2** | One session carries many CLTUs. One acquisition sequence starts the session; idle sequence fills the gaps between CLTUs. This is the CCSDS-recommended procedure. |

The package builds the resulting symbol stream with `AcquisitionSequence()`, `IdleSequence()`, and `UplinkSequence(plop, cltus, ...)`. Carrier on/off control (the CMM state machine) belongs to the transmitting equipment and is out of scope.

## Processing Order

### Transmit Path (Ground Station)

```
TC Transfer Frame
      |
      v
[Pad to 7-byte boundary] ──> Fill bytes (0x55)
      |
      v
[Pseudo-Randomize] ──> XOR with PN sequence (optional, fill included)
      |
      v
[BCH Encode] ──> 7-byte blocks → 8-byte codeblocks
      |
      v
[CLTU Assembly] ──> Start sequence + codeblocks + tail sequence
      |
      v
[PLOP] ──> Acquisition/idle sequences around the CLTUs
      |
      v
  Symbol stream ──> To physical layer (uplink modulator)
```

### Receive Path (Spacecraft)

```
    CLTU ──> From physical layer (uplink demodulator)
      |
      v
[Find start sequence] ──> Locate CLTU boundary
      |
      v
[BCH Decode codeblocks] ──> SEC: correct up to 1 bit error per block
      |                      TED: detect up to 3, correct nothing
      v
[Terminate] ──> On the tail sequence, or on the FIRST codeblock
      |         that fails to decode (tail bit errors tolerated)
      v
[De-Randomize] ──> XOR with PN sequence (optional, fill included)
      |
      v
[Strip padding] ──> Caller must know original frame length
      |
      v
TC Transfer Frame ──> To TC Data Link Protocol
```

## Design Rationale

**Why per-codeblock FEC instead of per-frame FEC?** Spacecraft TC decoders are typically simple, radiation-hardened hardware. Processing 8 bytes at a time is far simpler than buffering and decoding an entire variable-length frame. Per-codeblock processing also means a corrupted codeblock can be detected immediately without waiting for the full CLTU.

**Why correct only 1 bit?** For commands, the consequence of a miscorrection (applying the wrong command) is far worse than the consequence of a rejection (ground retransmits via COP-1). BCH(63,56) has a very large Hamming distance for its rate, making miscorrection probability negligible while still handling the most common single-bit errors.

**Why variable-length CLTUs?** TC Transfer Frames are variable-length (up to 1024 bytes). Making CLTUs variable-length too means no wasted bandwidth — a 10-byte command doesn't waste 1014 bytes of padding. This is important because uplink bandwidth is often much more constrained than downlink.

**Why a dedicated tail sequence?** Unlike TM (where fixed-length frames mean the next ASM is at a predictable offset), TC CLTUs are variable-length. The receiver needs an explicit end marker to know when to stop decoding codeblocks and hand the frame to the Data Link layer.

**Why `0x55` fill bytes?** The alternating bit pattern `01010101` provides good bit transition density even before randomization, and is easily distinguishable from real command data by the upper layer if padding needs to be identified.

## Reference

- [CCSDS 231.0-B-4](https://public.ccsds.org/Pubs/231x0b4e1.pdf) — TC Synchronization and Channel Coding (Blue Book)
- [CCSDS 230.1-G-2](https://public.ccsds.org/Pubs/230x1g2.pdf) — TC Synchronization and Channel Coding Summary (Green Book)
- [CCSDS 232.0-B-4](https://public.ccsds.org/Pubs/232x0b4e1c1.pdf) — TC Space Data Link Protocol (Blue Book)
