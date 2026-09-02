---
title: Optical Coding and Sync
short: OCSC
description: Optical Communications Coding and Synchronization (CCSDS 142.0-B-1) — the coding layer for a laser downlink.
order: 33
---

> **CCSDS 142.0-B-1** · [Blue Book](https://public.ccsds.org/Pubs/142x0b1.pdf) · [`pkg/ocsc`](https://github.com/ravisuhag/astro/tree/main/pkg/ocsc)

## Overview

This is deep-space laser communication. A spacecraft points a laser at Earth
and pulses it; a ground telescope counts photons. In the **High Photon
Efficiency** regime, so few photons arrive that the coding has to be
extraordinary — this is the standard behind NASA's Deep Space Optical
Communications demonstration.

The full standard specifies **SCPPM**: serially concatenated convolutional
coding with pulse-position modulation, a large channel interleaver, and an
iterative decoder. This package implements the deterministic front half of
that chain — the part that is pure bit manipulation:

```
transfer frames
  → attach sync marker      §3.3   ASM 1ACFFC1D
  → slice into blocks       §3.4   k bits, zero-filled
  → pseudo-randomize        §3.5   g(D) = D⁸+D⁷+D⁵+D³+1
  → attach CRC-32           §3.6   h(X) = X³²+X²⁹+X¹⁸+X¹⁴+X³+1
  → attach termination      §3.7   two zeros
  → SCPPM encoder input block
```

Everything after that — the SCPPM encoder proper, the channel interleaver, the
codeword sync marker, the slot mapper — is coupled to the modulation and is not
here. Neither is iterative SCPPM decoding: that is a research-grade job, and it
does not belong in a wire-format library. What **is** here on the receive side
is everything after the decoder: `Recover` finds the frames again (§3.14.1) and
delivers each with its quality indicator (§3.14.2) and sequence indicator
(§3.15).

## Scope

**Implemented.** The transmit chain from frame to bits — CRC-32, the pseudo-randomizer, and streaming. On receive: frame synchronization, the quality indicator, and the sequence indicator.

**Not here yet.**

- **The SCPPM encoder** (§3.8) — the convolutional and accumulator stages
  coupled to PPM mapping.
- **The channel interleaver** (§3.9) and **codeword sync marker** (§3.10) —
  both operate on PPM symbols, not bits.
- **The repeater and slot mapper** (§3.11, §3.12).
- **The receive side up to and including the decoder** — iterative SCPPM
  decoding, slot and symbol timing, soft decisions, channel estimation. (The
  steps after the decoder — frame synchronization, the quality indicator, the
  sequence indicator — are here, in `Recover`.)
- **HPE beacon and optional accompanying data transmission signaling** of §4 —
  the uplink beacon carrying LDPC-coded AOS or USLP transfer frames.
- **CLI subcommands** — a follow-up once the API settles.

## Everything is bits

This is the thing that shapes the whole API.

Table 3-1 gives the information block sizes:

| Code rate | k (information block) | k̂ (with CRC and termination) |
|---|---|---|
| 1/3 | 5006 | 5040 |
| 1/2 | 7526 | 7560 |
| 2/3 | 10046 | 10080 |

**None of those is a multiple of eight.** 5006 bits is 625 octets and six bits.
So there is no octet-oriented way to do this, and the package works in
`BitString` throughout:

```go
b := ocsc.BitStringFromBytes(frame)
b.Len()          // in bits
b.Bit(i)         // bit i, MSB-first within each octet
b.Slice(a, z)    // bits [a, z)
```

Converting to octets is something you do at the end, if at all.

## The CRC-32 is a fourth polynomial

By my count this library now contains four different CRC-32s, and none is
interchangeable with another:

| | Polynomial | Where |
|---|---|---|
| IEEE CRC-32 | 0x04C11DB7 | zip, Ethernet |
| CRC-32C | 0x1EDC6F41 | `pkg/crc`, USLP FECF |
| Proximity-1 | 0x00A00805 | `pkg/pxsc` |
| **Optical** | **0x20044009** | here |

The optical one is `h(X) = X³² + X²⁹ + X¹⁸ + X¹⁴ + X³ + 1` from §3.6.2.2, with
the register starting at all ones. That last part is written in the spec as a
`Σ X^(k+j)` term added before the modulo, which is the formal way of saying
"preset to ones".

## The pseudo-randomizer

A long run of identical data would become a long run of identical optical
pulses, and the receiver needs transitions to keep symbol timing. So every
information block is XORed with a pseudo-random sequence (§3.5.1.1).

The generator is `g(D) = D⁸ + D⁷ + D⁵ + D³ + 1`, the register starts at all
ones, and the sequence repeats every 255 digits. It restarts at the first digit
of each block, so two identical blocks randomize identically.

**A note on getting this right.** A polynomial this short has several plausible
register layouts, and they produce entirely different sequences. §3.5.2.1
publishes the first 40 digits precisely so an implementer can check:

```
1111 1111 0100 1000 0000 1110 1100 0000 1001 1010
```

`TestPNSequenceMatchesTheSpecVector` asserts exactly that. If you are
implementing this elsewhere, check against those digits before trusting your
taps — the failure mode is silent and total.

## Running the chain

```go
import "github.com/ravisuhag/astro/pkg/ocsc"

blocks, err := ocsc.Condition(frames, ocsc.RateOneThird)
if err != nil {
    return err
}
// Each block is exactly k̂ bits: hand it to your SCPPM encoder.
```

`Condition` is a **batch** call: it treats its input as one complete
transmission, so the call itself is the transmission closure of §3.4.2.1.1 and
the final block gets zero-filled. Call it twice and you have two transmissions,
not one. Frames may be at most 65536 octets — the frame-length managed
parameter's bound from §5.2, exposed as `ocsc.MaxFrameLength`.

And back:

```go
recovered, badBlocks, err := ocsc.Recover(blocks, ocsc.RateOneThird, frameLength)
for _, f := range recovered {
    f.Data  // the transfer frame
    f.Valid // Quality Indicator (§3.14.2): false if any carrying block failed its CRC
    f.Gap   // Sequence Indicator (§3.15): true when a gap precedes this frame
}
```

Or run the stages individually — `AttachASM`, `Slice`, `Randomize`,
`AttachCRC`, `AttachTermination` — if you need to inspect between them.

## Streaming a transmission

The NOTE under §3.2 says encoding may be performed in a streaming fashion: you
do not need every frame of a session in hand before you start, and you do not
need to know how many there will be. `Conditioner` is that form of the send
side:

```go
c, err := ocsc.NewConditioner(ocsc.RateOneThird)
if err != nil {
    return err
}
for frame := range downlink {
    blocks, err := c.Push(frame) // only the blocks completed so far
    if err != nil {
        return err
    }
    emit(blocks)
}
tail, err := c.Close() // transmission closure: the remainder, zero-filled
if err != nil {
    return err
}
emit(tail)
```

Bits short of a full block carry between `Push` calls — no fill is inserted
mid-stream, because §3.4.2.1.1 permits zero fill only at transmission closure.
`Close` is that closure, and afterwards the conditioner refuses further use.
Pushing the same frames through one conditioner produces bit-for-bit the same
blocks as one batch `Condition` call.

## Frame validation and sequence indication

`Recover` implements the receive side downstream of the SCPPM decoder.

**Quality Indicator (§3.14.2).** A frame is marked valid only if every block
carrying any of its bits — sync marker included — verified its CRC. A frame
straddling a corrupt block comes back with `Valid` false rather than being
dropped: the standard delivers invalid frames marked, it does not discard
them.

**Sequence Indicator (§3.15).** `Gap` is 'zero' (false) when a frame is the
direct successor of the previous one and 'one' (true) when a gap was detected
— the next frame did not start where it should have, and synchronization had
to hunt for it.

**Locked synchronization (§3.14.1).** With a frame length given, `Recover`
does not re-hunt the marker at every bit offset. After locking a frame, the
next marker is expected immediately after it and checked at that one position.
This is what keeps frame data that happens to contain `1ACFFC1D` from
producing spurious frames. Only when the expected marker is missing does it
fall back to a bit-by-bit hunt, and the frame found that way carries a raised
sequence indicator.

### Why Recover needs a frame length

The slicer zero-fills its output to a whole number of blocks (§3.4.2.1.1).
Once that fill is in the stream, **nothing distinguishes it from real frame
data** — the conditioning chain records nowhere that the data stopped.

Frame length is a managed parameter, fixed for a mission phase, so a real
receiver always knows it. Pass it and the fill is trimmed. Pass zero and each
frame runs to the next sync marker, leaving the fill attached to the last one.

## Reference

- [CCSDS 142.0-B-1](https://public.ccsds.org/Pubs/142x0b1.pdf) — Optical Communications Coding and Synchronization
- [Conformance](/conformance/ocsc)
