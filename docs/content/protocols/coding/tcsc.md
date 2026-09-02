---
title: TC Sync and Channel Coding
description: TC Synchronization and Channel Coding (CCSDS 231.0-B-4) — CLTU framing, BCH coding, and randomization on the uplink.
order: 31
---

> **CCSDS 231.0-B-4** · [Blue Book](https://public.ccsds.org/Pubs/231x0b4e1.pdf) · [`pkg/tcsc`](https://github.com/ravisuhag/astro/tree/main/pkg/tcsc) · [`astro cltu`](/cli/cltu)

This is the layer between a [TC frame](/protocols/data-link/tcdl) and the uplink radio. It wraps the frame in a **CLTU** — a Command Link Transmission Unit — codes it in 7-byte chunks with BCH parity, and scrambles the bits.

It is the uplink counterpart to [`pkg/tmsc`](/protocols/coding/tmsc), and almost every design choice went the other way. TC sends on demand rather than continuously, and it codes per block rather than per frame, so a spacecraft can throw out a bad chunk the moment it arrives.

## Scope

**Implemented.** CLTU wrap and unwrap, BCH(63,56) in both decode modes, the TC pseudo-randomizer, and the PLOP sequences that put CLTUs on the channel.

**Out of scope.** Carrier on and off control — the CMM state machine — belongs to the transmitting equipment, not to this library.

## Field map

A CLTU has no header fields, just a shape:

| Piece | Size | Go | Notes |
|---|---|---|---|
| Start Sequence | 2 B | `DefaultStartSequence()` | `0xEB90` |
| Codeblocks | 8 B each | `BCHEncode` | 7 info bytes + 1 byte of parity and filler |
| Tail Sequence | 8 B | `DefaultTailSequence()` | `0xC5C5C5C5C5C5C579` |

`WrapCLTU(frame, startSeq, tailSeq, randomize)` builds one. `UnwrapCLTU` reverses it, and `UnwrapCLTUWithMode` lets you pick the decode mode.

| Constant | Value | Meaning |
|---|---|---|
| `InfoBytes` | 7 | Information bytes per codeblock |
| `CodeblockBytes` | 8 | Total bytes per codeblock |
| `ModeSEC` | — | Single Error Correction. Fixes 1 bit. |
| `ModeTED` | — | Triple Error Detection. Fixes nothing, detects up to 3. |
| `PLOP1` / `PLOP2` | 1 / 2 | How CLTUs are placed on the channel |
| `DefaultAcquisitionOctets` | 16 | Recommended minimum, clause 7.2.2 |
| `DefaultIdleOctets` | 8 | Astro's choice, not the standard's |

## Gotchas

**The TC randomizer is not the TM randomizer.** Different polynomial: `x⁸ + x⁶ + x⁴ + x³ + x² + x + 1` (clause 6.2), seeded all ones. The first octets are `FF 39 9E 5A 68`; TM's sequence opens `FF 48 0E C0 9A`.

Use the TM sequence here and every CLTU is unreadable to conformant equipment — and **no round-trip test will catch it**, because XOR is self-inverse and your own decoder will happily undo your own mistake. This is the single most dangerous confusion in the package.

**Randomization covers the fill bytes too.** It is applied after padding and before BCH encoding, so it covers the frame data and the `0x55` fill octets — everything between the start and tail sequences. The start and tail sequences themselves are never randomized.

**SEC mode can silently miscorrect.** BCH(63,56) corrects one bit error. A 3-bit error pattern can produce a syndrome that looks like a valid single-bit error somewhere else, and the decoder will "fix" it into wrong data with no complaint. Missions that would rather lose a command than execute the wrong one run `ModeTED`, which detects up to 3 errors and corrects none.

**The tail sequence is deliberately not a valid codeblock.** The pattern was picked so a decoder rejects it even after a single bit error. That rejection is what ends reception — the receiver stops at the first codeblock that fails to decode, so it never needs a bit-exact tail match.

**The filler bit is always 0.** Clause 3.3.2. It exists only to round the 63-bit BCH codeword up to 64 bits so it fits in whole octets. Decoders ignore it.

**Frame data is padded to a multiple of 7 with `0x55`.** Not zeros. The alternating bit pattern keeps the clock happy in the padding too.

**Parity is transmitted inverted.** The encoder writes the complement of the LFSR contents. A decoder has to complement it back before computing the syndrome, which is easy to miss when writing one from the standard.

**`DefaultIdleOctets` is Astro's number, not CCSDS's.** Clause 7.2.4 constrains nothing — the idle sequence is "an unconstrained number of bits" and the PLOP-2 figure shows it as optional, so zero is conformant. What your mission uses is a managed parameter.

## PLOP

A Physical Layer Operations Procedure (clause 8) decides how CLTUs land on the channel. Both helper sequences are the alternating pattern `0x55`.

- **Acquisition sequence** goes first so the receiver can lock its bit clock. At least 16 octets recommended.
- **Idle sequence** keeps the channel modulated between CLTUs.

| Procedure | Behaviour |
|---|---|
| PLOP-1 | The session ends after each CLTU. Every CLTU gets its own acquisition sequence. |
| PLOP-2 | One session carries many CLTUs. One acquisition sequence up front, idle between. CCSDS recommends this one. |

Build the stream with `AcquisitionSequence()`, `IdleSequence()`, and `UplinkSequence(plop, cltus, acqOctets, idleOctets)`.

## Quick Start

```go
import "github.com/ravisuhag/astro/pkg/tcsc"

// Wrap a TC Transfer Frame into a CLTU for transmission
cltu, err := tcsc.WrapCLTU(encodedFrame, nil, nil, true) // nil=defaults, true=randomize

// Unwrap a received CLTU back into a TC Transfer Frame
frameData, corrected, err := tcsc.UnwrapCLTU(cltu, nil, nil, true)

// BCH encode/decode individual codeblocks
cb := tcsc.BCHEncode(infoBytes)
info, corrections, err := tcsc.BCHDecode(cb)
```

## Architecture

The TCSC sublayer sits between the TC Data Link Protocol (`tcdl`) and the physical layer:

```
+-----------------------------------------+
|  TC Space Data Link Protocol (tcdl)     |
|  Packs commands into Transfer Frames    |
+-----------------------------------------+
|  TC Sync & Channel Coding (tcsc)        |  <-- This package
|  CLTU, BCH encoding, randomization      |
+-----------------------------------------+
|  Physical Layer (RF uplink)             |
+-----------------------------------------+
```

## Command Link Transmission Unit (CLTU)

A **CLTU** is the unit transmitted over the physical uplink. It consists of a start sequence, one or more BCH-encoded codeblocks, and a tail sequence.

```
+-----------------+-------------+-----+-------------+-----------------+
| Start Sequence  | Codeblock 1 | ... | Codeblock N | Tail Sequence   |
| (2 bytes: EB90) | (8 bytes)   |     | (8 bytes)   | (8 bytes)       |
+-----------------+-------------+-----+-------------+-----------------+
|<------------------------- CLTU ---------------------------------->|
```

### Start and Tail Sequences

```go
// Standard CCSDS start sequence (0xEB90)
start := tcsc.DefaultStartSequence()

// Standard CCSDS tail sequence (0xC5C5C5C5C5C5C579)
tail := tcsc.DefaultTailSequence()
```

The start sequence marks the beginning of a CLTU in the bitstream. The tail sequence marks the end and allows the decoder to detect the final codeblock boundary. Fresh copies are returned each call to prevent accidental mutation.

### Wrapping (Send Path)

```go
// Wrap with default sequences and pseudo-randomization
cltu, err := tcsc.WrapCLTU(encodedFrame, nil, nil, true)

// Wrap with default sequences, no randomization
cltu, err := tcsc.WrapCLTU(encodedFrame, nil, nil, false)

// Wrap with custom sequences
customStart := []byte{0xDE, 0xAD}
customTail := []byte{0xBE, 0xEF, 0xBE, 0xEF, 0xBE, 0xEF, 0xBE, 0xEF}
cltu, err := tcsc.WrapCLTU(encodedFrame, customStart, customTail, true)
```

**Wrapping process:**
1. Optionally pseudo-randomize the frame data
2. Pad to a multiple of 7 bytes (fill pattern: `0x55`)
3. BCH-encode each 7-byte block into an 8-byte codeblock
4. Prepend start sequence, append tail sequence

### Unwrapping (Receive Path)

```go
// Unwrap with default sequences and de-randomization
frameData, corrections, err := tcsc.UnwrapCLTU(cltu, nil, nil, true)
if errors.Is(err, tcsc.ErrStartSequenceMismatch) {
    // Start sequence not found
}
if errors.Is(err, tcsc.ErrUncorrectable) {
    // More than 1 bit error in a codeblock
}
fmt.Printf("Corrected %d bit errors\n", corrections)
```

**Note:** The caller must know the original data length to strip any padding, as the fill pattern is not self-describing.

## BCH(63,56) Error Correction

Each codeblock uses a BCH code that encodes 56 information bits (7 bytes) into 64 bits (8 bytes):

```
+-------------------+----------------+--------+
| Information Bits  | Parity Bits    | Filler |
| (56 bits, 7 B)    | (7 bits)       | (1 bit)|
+-------------------+----------------+--------+
|<------------- Codeblock (64 bits, 8 B) ---->|
```

- **Generator polynomial:** g(x) = x^7 + x^6 + x^2 + 1
- **Error correction:** 1 bit error per codeblock
- **Error detection:** Up to 3 bit errors per codeblock
- **Filler bit:** Complement of the last parity bit (prevents all-zero codeblocks)

### Direct BCH Usage

```go
// Encode 7 information bytes into an 8-byte codeblock
info := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
cb := tcsc.BCHEncode(info) // Returns [8]byte

// Decode an 8-byte codeblock, correcting up to 1 bit error
decoded, corrections, err := tcsc.BCHDecode(cb)
if errors.Is(err, tcsc.ErrUncorrectable) {
    // More than 1 bit error
}
```

## Pseudo-Randomization

CCSDS pseudo-randomization ensures good signal properties by preventing long runs of identical bits. The TC Transfer Frame bytes are XORed with a pseudo-random noise (PN) sequence before BCH encoding.

```go
// Randomize data (XOR with PN sequence)
randomized := tcsc.Randomize(data)

// De-randomize (same operation — XOR is self-inverse)
original := tcsc.Randomize(randomized)
```

The PN sequence is generated by an 8-bit LFSR with polynomial `h(x) = x^8 + x^6 + x^4 + x^3 + x^2 + x + 1`, initialized to all 1s (`0xFF`), per CCSDS 231.0-B-4 §6.2. Its first 40 digits are `FF 39 9E 5A 68`.

This is **not** the TM sequence. CCSDS 131.0-B-5 §10.4.2 uses a different polynomial and a sequence that opens `FF 48 0E C0 9A`. Do not substitute one for the other: a round trip still succeeds, because XOR is self-inverse, but a conformant peer recovers noise.

```go
// Generate PN sequence of arbitrary length
pnSeq := tcsc.GeneratePNSequence(256)
```

## Full Pipeline Example

### Send Path (Ground to Spacecraft)

```go
import (
    "github.com/ravisuhag/astro/pkg/tcdl"
    "github.com/ravisuhag/astro/pkg/tcsc"
)

// 1. Get a TC Transfer Frame from the TC Data Link layer
frame, _ := pc.GetNextFrame()
encoded, _ := frame.Encode()

// 2. Wrap as CLTU — randomize, BCH-encode, add framing
cltu, _ := tcsc.WrapCLTU(encoded, nil, nil, true)

// 3. Transmit CLTU over the physical uplink
transmit(cltu)
```

### Receive Path (Spacecraft)

```go
// 1. Receive CLTU from physical link
cltu := receive()

// 2. Unwrap CLTU — validate framing, BCH-decode, de-randomize
frameData, corrections, err := tcsc.UnwrapCLTU(cltu, nil, nil, true)
if err != nil { /* handle errors */ }

// 3. Decode the TC Transfer Frame
frame, err := tcdl.DecodeTCTransferFrame(frameData)
if err != nil { /* handle CRC errors */ }
```

## Errors

All errors are exported package-level variables, suitable for use with `errors.Is`:

| Error | Meaning |
|-------|---------|
| `ErrDataTooShort` | CLTU too short to contain start sequence, codeblock, and tail |
| `ErrStartSequenceMismatch` | CLTU does not start with the expected start sequence |
| `ErrTailSequenceMismatch` | CLTU does not end with the expected tail sequence |
| `ErrInvalidCLTULength` | CLTU body is not a multiple of the codeblock size (8 bytes) |
| `ErrUncorrectable` | Codeblock has more than 1 bit error (exceeds BCH capability) |
| `ErrEmptyData` | Empty data provided for encoding |

## Notes

Commentary, not sourced from the standard.

**Why BCH per codeblock instead of Reed–Solomon per frame?** A spacecraft can check each 8-byte block as it arrives and reject a bad one immediately, without buffering the whole frame. On a command link, catching a corrupt block early matters more than correcting a lot of errors.

**Why a 2-byte start sequence when TM uses 4?** The uplink is a much easier channel. The ground station controls the transmit power, the data rate is low, and there is no light-hours of path loss. Two bytes is enough to find the boundary, and uplink bandwidth is scarce.

**Why offer a mode that corrects nothing?** Because a miscorrected command is worse than a lost one. TED trades correction for certainty, and for a lot of missions that is the right trade.

**Why does TC randomize at all, given the good link?** Clock recovery is the same problem everywhere. A command full of zeros still gives the receiver nothing to lock onto.

## Reference

- [CCSDS 231.0-B-4](https://public.ccsds.org/Pubs/231x0b4e1.pdf) — TC Synchronization and Channel Coding (Blue Book)
- [CCSDS 230.2-G-1](https://public.ccsds.org/Pubs/230x2g1.pdf) — TC Synchronization and Channel Coding Summary (Green Book)
- [CLI](/cli/cltu) · [Conformance](/conformance/tcsc)
