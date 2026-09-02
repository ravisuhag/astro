---
title: Compress before you downlink
short: Compression
description: A downlink is the scarcest thing a mission has. Two standards shrink what goes over it.
order: 11
---

Everything else in these guides is about moving bytes correctly. This one is about moving fewer of them.

CCSDS has two compression standards and they solve different problems. [LDC](/protocols/compression/ldc) is for science data: find the redundancy between neighbouring samples and code it away, losslessly. [RHC](/protocols/compression/rhc) is for housekeeping: notice that almost nothing changed since the last packet and send only what did.

The complete program is [`examples/compression`](https://github.com/ravisuhag/astro/tree/main/examples/compression). Run it:

```bash
go run ./examples/compression/
```

## LDC: decorrelate, then entropy code

The shape of LDC is two steps:

```
samples ──► preprocessor ──► adaptive entropy coder ──► coded data sets
            subtract a       price every option
            prediction       against each block
```

The preprocessor subtracts a prediction from each sample and folds the signed residual onto the non-negative integers. The entropy coder then takes those residuals in blocks, prices every code option it has against the block, and writes the cheapest one with an identifier saying which it chose.

```go
params := ldc.DefaultParams()
params.Resolution = 12
params.BlockSize = 16
params.Predictor = ldc.PredictorUnitDelay
params.ReferenceInterval = 128

compressed, err := ldc.CompressFile(samples, params, 2)
back, err := ldc.DecompressFile(compressed)
```

On a smooth detector scan:

```
  samples ......... 4096 at 12 bits
  raw ............. 8192 octets
  compressed ...... 2014 octets
  ratio ........... 4.07:1
  bits per sample . 3.93
  lossless ........ true
```

Four to one, and every sample comes back bit for bit. Nothing here is approximate: it is integer arithmetic throughout and every step inverts exactly.

## The predictor does most of the work

Turn the predictor off and keep everything else the same:

```
  without the predictor ... 6284 octets, 1.30:1
```

From 4.07:1 to 1.30:1. The entropy coder is not what compresses the data; it exploits a skew that the predictor created. An entropy coder handed raw 12-bit sample values sees numbers spread over the whole range, and there is very little to exploit.

Which predictor to use:

| Predictor | For |
|---|---|
| `PredictorUnitDelay` | Correlated data. Subtract the previous sample. This is almost always the answer. |
| `PredictorBypass` | Data already decorrelated by something else, but signed, so it still needs mapping onto non-negative integers. |
| `PredictorNone` | Neither. Rare. |

## Looking inside the coded stream

`Analyze` reports which option won each block and what it cost. This is how you check a parameter choice against real data rather than guessing:

```go
blocks, err := ldc.Analyze(compressed, params, len(samples))
```

Run over a stream with three different regimes in it, quiet then noisy then completely flat:

```
  34 coded data sets over 768 samples

  block   0  split sample           k=1   61 bits
  block   1  split sample           k=1   52 bits
  ...
  block  31  no compression         k=0  196 bits  run=1
  block  32  split sample           k=4  106 bits  run=1
  block  33  zero block             k=0   10 bits  run=15

  Option totals:
    split sample           17
    no compression         16
    zero block             1
```

Three things to read off that. The quiet section picked split-sample with a small `k`. The noisy section picked **no compression** sixteen times, which is the coder correctly deciding that random data cannot be compressed and refusing to make it bigger. And the flat section collapsed 15 blocks, 240 samples, into 10 bits with the zero-block option.

An option tally full of "no compression" means your data is not what you thought, or your resolution is wrong.

## Choosing the other parameters

**`Resolution`** is not a trade. It has to match the data.

**`BlockSize`** (J) can be 8, 16, 32 or 64. Smaller blocks adapt faster to changing statistics and pay more identifier bits. 16 is the usual choice.

**`ReferenceInterval`** (r) is how often an uncoded sample goes in. It bounds how far a bit error propagates, and it also bounds the segments the zero-block option counts within, so it matters even when you are not using reference samples.

## RHC: send only what changed

Housekeeping telemetry barely changes. A voltage wobbles in its low bits, a mode word does not move for hours, and most of the packet is byte-identical to the last one.

RHC keeps a mask, one bit per position, saying whether that position is predictable. Predictable positions are not sent at all, because the decompressor already knows them.

```go
config := rhc.Config{
    VectorLength:         512, // bits, so a 64-octet packet
    Robustness:           3,
    NewMaskInterval:      32,
    SendMaskInterval:     16,
    UncompressedInterval: 16,
}
compressor, err := rhc.NewCompressor(config)
coded, bitLen, err := compressor.Compress(packet)
```

Watch the cost per packet as the mask learns what moves:

```
  cycle  0 ...  538 bits  (nothing is predictable yet)
  cycle  1 ...  597 bits  (still learning what moves)
  cycle  2 ...  605 bits  (still learning what moves)
  cycle  3 ...  605 bits  (still learning what moves)
  cycle  4 ...   43 bits  (the mask has settled)
  cycle  5 ...   31 bits  (the mask has settled)
  cycle 16 ...  619 bits  (the uncompressed interval came round)
  cycle 17 ...   45 bits
  ...
  raw ............. 32768 bits over 64 packets
  coded ........... 5637 bits
  ratio ........... 5.81:1
```

The first few cycles cost **more** than the raw packet. That is not a bug: the mask is filling in, and until it has settled the algorithm is describing changes rather than exploiting stability. After that a 512-bit packet costs 31 bits.

Cycle 16 jumps back to 619 because the uncompressed interval came round and the whole packet went down. That is the price of being recoverable, and it is a policy decision, not the protocol.

## The three intervals are yours to choose

The standard makes each of these a per-cycle decision and says nothing about when to make it.

**`NewMaskInterval`** lets positions go back to being predictable. Without it the mask fills up with ones over a long run and the ratio decays.

**`SendMaskInterval`** ships the whole mask, so a decompressor that lost its place can recover the mask without waiting for changes to describe it.

**`UncompressedInterval`** ships the whole input vector. This is the one that matters most, because it is the only thing that restores a decompressor's previous-vector state after a gap.

## The part worth reading twice

RHC is built for a lossy link. Each output declares how many outputs may have been lost immediately before it and still leave the mask recoverable, its effective robustness level, and `Robustness` sets the floor.

But **the decompressor cannot tell that anything was lost.** Clause 2.2 says so outright and points at packet sequence counters as the mission's answer. So noticing the gap is the caller's job:

```go
decompressor.NotifyLoss(count)
```

Here is what that is worth. Robustness 1, four consecutive outputs dropped, so the stream's own recovery cannot reach back across the gap:

```
  robustness 1, 4 consecutive outputs dropped, 20 delivered

                           refused    silently wrong
  NotifyLoss called              8              0
  NotifyLoss not called          0              8

  Both lose the same packets. Only one of them says so.
```

Same eight packets lost either way. The difference is that one decompressor refuses them and the other hands back plausible wrong octets with no error.

There are no sync markers either. A foreign or corrupt vector that happens to parse is taken for a real one. Framing and gap detection are both the mission's job.

## Things that will bite you

**LDC parameters do not travel with the data.** A coded data set on its own is undecodable. That is why the standard defines a file header and a compression identification packet: both exist to carry the parameters. `CompressFile` writes the header; plain `Compress` does not, and a caller using it has to get the parameters to the far end some other way.

**Compressed data is fragile.** One bit error can destroy everything after it, which is why `ReferenceInterval` exists. Compress, then put it through [Reed-Solomon](/docs/guides/lossy-link), never the other way round.

**A `Compressor` and `Decompressor` are stateful and not concurrency safe.** One stream, one pair, one goroutine. They hold the mask and the previous vector.

**`Strict` mode trades availability for trust.** The standard's recovery gate believes a field the output vector declares about itself, and nothing in the format lets a decompressor verify it. `Strict` waits for an output that proves itself by carrying the whole input, and refuses everything between the gap and that point.

**Random data cannot be compressed.** LDC's "no compression" option exists so a coder handed noise does not make it bigger. Seeing it dominate the tally is information, not a failure.

**Compression sits above the packet layer.** Both packages produce ordinary octets. Getting them down is still [packets, frames and CADUs](/docs/guides/downlink), and a compressed block is a natural fit for [an AOS VCA channel](/docs/guides/aos-high-rate).

## Next

- [A high-rate downlink with AOS](/docs/guides/aos-high-rate), which is what carries compressed blocks
- [Handle a lossy link](/docs/guides/lossy-link), the coding that has to go outside compression
- [LDC](/protocols/compression/ldc) | [RHC](/protocols/compression/rhc) | [LDC CLI](/cli/ldc) | [RHC CLI](/cli/rhc)
