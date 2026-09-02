---
title: Housekeeping Compression
short: RHC
description: Robust Compression of Housekeeping Data (CCSDS 124.0-B-1) — POCKET+, lossless compression that survives packet loss.
order: 51
---

> **CCSDS 124.0-B-1** · [Blue Book](https://public.ccsds.org/Pubs/124x0b1.pdf) · [`pkg/rhc`](https://github.com/ravisuhag/astro/tree/main/pkg/rhc)

## Overview

Housekeeping telemetry barely changes. A bus voltage, a mode word, a
thermistor count, a set of status flags: sample them once a second and most
bits are identical to the last sample. POCKET+ exists to send only the ones
that are not.

```
  housekeeping reports  ──►  pkg/rhc  ──►  packetization by the caller
    e.g. PUS ST[03]              │
    from pkg/pus                 │
                        send only the bits
                        that actually changed
```

The mechanism is a **mask**: one bit per position in the packet, saying whether
that position is *predictable* — unchanged since the last packet — or not.
Predictable positions are not transmitted at all, because the decompressor
already knows them. Only the unpredictable ones travel.

What makes this different from a general-purpose compressor is that it is
designed for a link that drops things. Each output vector carries enough
information about recent mask changes that a decompressor which missed the
last few can still catch up.

## Scope

**Implemented.** The POCKET+ compressor in full, plus a decompressor derived from it. Mask update, the encoder, robustness levels, and loss recovery.

**Not here yet.**

- **A CLI.** `astro rhc compress|decompress` is the natural follow-up.
- **Loss-adaptive scheduling.** Raising the robustness level or shortening the
  uncompressed interval when the link degrades is a mission heuristic on top of
  `Config`, deliberately not built in.
- **The application-specific predictor and mapper** named in the file header
  fields of related standards — not applicable here.

## A worked run

```go
config := rhc.Config{
    VectorLength:         512,  // F: every input is 512 bits
    Robustness:           3,    // survive 3 lost outputs
    NewMaskInterval:      32,
    SendMaskInterval:     16,
    UncompressedInterval: 16,
}

compressor, _ := rhc.NewCompressor(config)
decompressor, _ := rhc.NewDecompressor(config)

for _, packet := range reports {
    coded, bitLen, err := compressor.Compress(packet)
    // ... transmit coded, carrying bitLen alongside ...

    back, err := decompressor.Decompress(coded, bitLen)
    // back == packet
}
```

On 200 slowly changing 512-bit reports this gives about 2.5× — and the ratio
climbs the less the data moves.

`bitLen` matters. An output vector is a whole number of *bits*, not octets, and
`Compress` pads to the next octet only because it returns a byte slice. §2.2
leaves framing to the mission, so if you are packing several outputs together
you need the bit length to find the boundaries.

## The three components

Each output vector is `h_t || q_t || u_t` (§5.3.1).

**h_t** describes what changed in the mask lately. Not just this cycle: the
change vectors of the last few cycles are ORed together, so a decompressor that
missed some of them still learns every position that moved. It also carries the
effective robustness level, which says how far back that reach goes.

**q_t** carries the whole mask, when the send mask flag is set. Mask changes
alone are enough to track the mask, so this is redundant — until something is
lost, when it becomes the fastest way back.

**u_t** carries the values: either the unpredictable bits, or the entire input
vector when the uncompressed flag is set.

## A note on the decompressor

CCSDS 124.0-B-1 specifies the compressor and nothing else. Its normative
sections are inputs (§3), mask update (§4) and encoder (§5), and the
conformance list in annex A2.2.1 has five items, all encoder items. There is no
decoder section.

The decompressor in this package is therefore the encoder run backwards. That
is legitimate — §2.1 lists exactly what a decompressor needs and the encoding
is losslessly invertible — but it is derived rather than transcribed, and the
[PICS](/conformance/rhc) says so. The round-trip and loss tests are what
stand behind it.

One reading of the standard deserves a flag: equation 11's bit extraction
emits the selected bits in the *reverse* of transmission order — the last
selected position travels first. That falls out of §1.6.1's own conventions
and matches the independent VisionSpace PocketPlus implementation, but no
published test vector confirms it. The PICS records the full reasoning.

## Mask and build

The mask only ever grows. Once a position changes, it is unpredictable and
stays that way — otherwise the decompressor could not trust any position.

That would be a one-way ratchet, so there is a second vector, the **build**,
accumulating the same changes in parallel. When the new mask flag fires
(§4.2.2), the mask is replaced by the build and the build resets to empty. A
position that has been quiet since the last time the flag fired is not in the
build, so it goes back to being predictable.

This is a two-step business, and it surprises people:

```go
compressor.ForceNewMask()   // resets the build; the mask is unchanged,
                            // because the build held the same bits
// ... some quiet cycles ...
compressor.ForceNewMask()   // now the mask really does clear
```

The first flag mostly serves to start a fresh build. §2.1 puts it as positions
moving to predictable "only on the cycle when the new mask is requested" — and
only if they have been quiet since the previous request.

## Loss, and what the caller must do

This is the part to read twice.

Each output says how many outputs may have been lost immediately before it
without stopping it from being decoded. That is its **effective robustness
level**, and `Robustness` sets the floor. §2.1:

> the mask can be synchronized even if the number of consecutive output binary
> vectors lost immediately before this output bit vector is equal to, or less
> than, the effective robustness level

But the decompressor **cannot tell that anything was lost**. §2.2 is explicit:

> it does not provide a mechanism for identifying the number of sequential
> output binary vectors that were lost. Such mechanisms are assumed to be
> mission specific.

and suggests packet sequence counters as the answer. So detecting gaps is
yours, and you must say so:

```go
if gap := expectedCounter - receivedCounter; gap > 0 {
    decompressor.NotifyLoss(gap)
}
back, err := decompressor.Decompress(coded, bitLen)
```

With `NotifyLoss`, the decompressor refuses anything it cannot vouch for.
Without it, a stream with holes reconstructs wrong bytes and nobody notices.
That is a property of the standard, not of this package.

The same goes for framing. There are no sync markers — §2.2 again — so a
corrupt or foreign vector that happens to parse will be taken for a real one.
Carry the outputs in something with a length field, such as space packets.

### Recovering

Two things restore a decompressor, and they fix different halves of §2.1's
list:

| What arrives | What it restores |
|---|---|
| the whole mask (`SendMaskInterval`, `ForceSendMask`) | the mask |
| the whole input (`UncompressedInterval`, `ForceUncompressed`) | the last reconstructed vector, and the mask if it comes too |

A whole mask on its own is *not* enough after a gap. It fixes the mask and
leaves the previous vector as stale as it was, and the values needed to bring
that up to date went out in the outputs that were lost. An uncompressed output
is the only unconditional repair.

`Synchronized()` reports whether the next output can be reconstructed.

### Strict mode

The recovery rule trusts one thing it cannot check: after a gap, an output is
accepted when the gap fits inside the output's effective robustness level —
a field the output declares about itself. The format has no way to verify it,
so a corrupt vector arriving right after a gap can claim any reach and be
believed.

`Config.Strict` removes that trust. A strict decompressor, once told of a
loss, refuses everything until the next uncompressed output — the one kind
that carries the whole input and so proves itself. The trade is availability:
honest outputs between the gap and that repair are refused too. It is this
package's addition, not the standard's, and is off by default.

## Choosing the knobs

`VectorLength` is fixed by the data. The rest are trades, and only the first is
in the standard at all:

| Knob | § | Trade |
|---|---|---|
| `Robustness` | 3.3.2a | 0 to 7. Higher survives longer gaps; costs bits, because the change information is ORed over more cycles and so names more positions. |
| `UncompressedInterval` | policy | How often to send a whole input. The recovery lever. Short means fast recovery and a much worse ratio — an uncompressed output is bigger than the input. |
| `SendMaskInterval` | policy | How often to send the whole mask. Cheaper than an uncompressed output and fixes half the problem. |
| `NewMaskInterval` | policy | How often to let positions go back to predictable. Never setting it means the mask fills with ones over a long run and compression decays. |
| `Strict` | policy | Decompressor side. After a reported loss, accept only an uncompressed output rather than trusting an output's self-declared robustness reach. Safer against corruption; refuses more. |

Only `Robustness` is normative. §3.3.2 makes the three flags user-specified at
every cycle and says nothing about when to set them, so the intervals here are
this package's convenience and nothing more. `CompressWith` takes the flags
directly if you want to drive them from your own logic — which §2.1 explicitly
allows, since "all the information required for decompression is contained in
the output bit vectors".

## Pairing with PUS housekeeping

The intended source is a stream of fixed-length housekeeping reports. `pkg/pus`
produces ST[03] reports; feed their data fields in as fixed-length vectors, one
per cycle, and carry the compressed output in space packets with `pkg/spp`.
The two ends need only agree on `VectorLength`.

## Reference

- [CCSDS 124.0-B-1](https://public.ccsds.org/Pubs/124x0b1.pdf) — Robust
  Compression of Fixed-Length Housekeeping Data, February 2023
- [Conformance](/conformance/rhc)
