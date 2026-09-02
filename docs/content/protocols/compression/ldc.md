---
title: Lossless Data Compression
description: CCSDS 121.0-B-3 — the Rice adaptive entropy coder, every bit recoverable.
order: 50
---

> **CCSDS 121.0-B-3** · [Blue Book](https://public.ccsds.org/Pubs/121x0b3.pdf) · [`pkg/ldc`](https://github.com/ravisuhag/astro/tree/main/pkg/ldc)

## Overview

Downlink is the scarcest thing a mission has. An instrument that produces more
data than the link can carry has three choices: send less of it, throw some
away, or compress it. This standard is the third choice done without loss —
every bit comes back exactly.

It is the most widely used CCSDS compression standard, and it is small. Two
stages, both pure integer arithmetic:

```
  samples ──►  preprocessor  ──►  adaptive entropy coder  ──►  coded data sets
                   §4                     §3                        §5
              decorrelate            price every option
              and fold to            for each block and
              non-negative           write the cheapest
```

Where it sits in this library: an instrument produces samples, `pkg/ldc`
compresses them, and the caller puts the result into packets with `pkg/spp` or
into a file. This package does no packetization — §5.3 leaves that to the
packet formatter, and so does this code.

## Scope

**Implemented.** Both stages — the preprocessor of section 4 and the adaptive
entropy coder of section 3 — all five code options, reference samples, the
zero-block run counting with its ROS codeword, and the section 7 file header.
Compression and decompression both, verified against the standard's test
vectors.

**Not here.** Packetization. Clause 5.3 leaves it to the mission, so `pkg/ldc`
hands you bytes and you put them in [Space Packets](/protocols/transport/spp) or a file.

**Configuration lives in `Params`.** `BlockSize` (J, one of 8/16/32/64),
`Resolution` (n, 1–32 bits), `Signed`, `Predictor`, `ReferenceInterval`
(r, 1–4096 blocks), and `Restricted`. `DefaultParams()` gives you 8-bit
unsigned samples in blocks of 16, unit-delay prediction, and a reference every
256 blocks.

## A worked run

12-bit samples from a slowly drifting sensor, 4096 of them:

```go
p := ldc.Params{
    BlockSize:         16,
    Resolution:        12,
    Predictor:         ldc.PredictorUnitDelay,
    ReferenceInterval: 128,
}

file, err := ldc.CompressFile(samples, p, 1)
// 8192 octets in, 2617 out — a ratio of 3.13

back, err := ldc.DecompressFile(file)
// identical to samples
```

`CompressFile` writes the file format of section 7: a twelve-octet header
carrying every parameter and the sample count, then the coded data, then zero
fill. That header is what makes the output self-describing, and it exists
because a coded stream on its own says nothing about how it was made.

For a caller who already shares a configuration with the far end — a mission
putting coded data sets straight into space packets — `Compress` and
`Decompress` skip the header.

## The preprocessor

The entropy coder wants small non-negative integers. Raw telemetry is neither.

**Prediction** subtracts what the previous sample suggests this one will be
(§4.2.5). On a drifting sensor the residual is near zero; on white noise it is
no better than the original, which is why the predictor is optional.

**Mapping** folds the signed residual onto the non-negative integers (§4.4).
Small errors of either sign become small values:

| Δ | 0 | −1 | +1 | −2 | +2 |
|---|---|---|---|---|---|
| δ | 0 | 1 | 2 | 3 | 4 |

That interleaving only works while the error can go both ways. Near the ends
of the sample range it cannot, and the mapping switches to running straight on
— which is what keeps an (n+1)-bit residual inside n bits instead of spilling.
The variable that decides where the switch happens is θ, the distance from the
prediction to the nearer end of the range.

Three predictor settings:

| | |
|---|---|
| `PredictorUnitDelay` | predict from the previous sample; the standard's own |
| `PredictorBypass` | predict zero, keep the mapper — for data already decorrelated but signed |
| `PredictorNone` | no preprocessor at all |

## Reference samples

A unit-delay chain needs a starting point, so every so often an uncoded sample
travels in the clear (§4.2.6). `ReferenceInterval` sets how often, in blocks.

It also bounds damage. A bit error in a coded stream corrupts everything until
the next reference sample, so the interval is really a choice about error
containment: shorter costs more bits and loses less to a hit.

Reference samples are inserted only with the unit-delay predictor. §4.2.6 is
explicit that otherwise they "shall not be employed", and the bypass predictor
looks at nothing, so it needs none.

## The five code options

For each block the coder prices every option and writes the cheapest, prefixed
by an identifier saying which it chose (§3.7).

| Option | § | What it does | Good for |
|---|---|---|---|
| Fundamental sequence | 3.2 | a sample of value m becomes m zeros and a one | values near zero |
| Split sample, k | 3.3 | FS-code the top n−k bits, send the low k raw | moderate values |
| Second extension | 3.4 | pair samples, code the pair as one symbol | very low entropy |
| Zero block | 3.5 | one codeword for a run of all-zero blocks | constant data |
| No compression | 3.6 | send the block unaltered | noise |

The fundamental sequence is the split-sample option with k = 0, which is why
they share an identifier range.

**Zero block is not chosen, it is imposed.** §3.7.2 says a run of all-zero
blocks always takes it, whatever anything else would cost. It is also the only
option whose coded data set spans more than one block.

**Ties have a defined winner.** §3.7.4 is normative and not the order you would
guess: no compression first, then second extension, then the smallest k. An
implementation that broke ties the other way would produce output a conforming
decoder still reads, but it would not be this standard.

## Segments and the ROS codeword

The zero-block option counts runs, and the count has two boundaries it cannot
cross. A run stops at the end of its reference interval, because the next
interval opens with an uncoded sample. And within an interval, §3.5.2 divides
the blocks into segments of 64, and a run stops at a segment end too.

Table 3-2 numbers the run lengths, with one oddity: the
remainder-of-segment codeword sits *between* four and five.

| Blocks | Codeword |
|---|---|
| 1 | `1` |
| 2 | `01` |
| 3 | `001` |
| 4 | `0001` |
| **ROS** | `00001` |
| 5 | `000001` |
| … | … |
| 63 | 63 zeros and a one |

ROS means "the rest of this segment is zeros", and §3.5.3 allows it for runs of
five or more. It earns its place: a segment is 64 blocks and the table counts
only to 63, so a wholly zero segment can be written no other way.

## Choosing parameters

`Resolution` must match the data — samples that do not fit are refused rather
than truncated, because truncating would make a lossless coder lossy.

The rest are trades:

- **BlockSize** — smaller blocks adapt faster to changing statistics and pay
  more identifier bits. 16 is a reasonable default.
- **Predictor** — unit delay for correlated data. If prediction does not help,
  it does not hurt much either; the coder will simply pick no-compression more
  often.
- **ReferenceInterval** — see above; it is an error-containment choice.
- **Restricted** — at four bits or fewer, §5.2.1.1 allows a shorter identifier
  at the cost of most split-sample options. Worth it only when blocks are small
  and identifiers are a real fraction of the output.

## Inspecting a stream

`Analyze` walks a coded stream and reports what each coded data set holds,
without reconstructing the samples:

```go
infos, err := ldc.Analyze(body, p, sampleCount)
for _, info := range infos {
    fmt.Println(info.Block, info.Option, info.K, info.Bits)
}
```

Useful for checking parameter choices against real data. If every block is
coming out as no-compression, the preprocessor is not helping and the
resolution or predictor is probably wrong.

## Test vectors

The official CCSDS 121.0-B-2 vector set, as mirrored in libaec's
`data/121B2TestData`, is vendored in `pkg/ldc/testdata/` and run in full: all
72 AllOptions and LowEntropyOptions vectors, covering resolutions 1 through
32, each required to encode byte-identically and decode back to the exact
samples. The ExtendedParameters set is excluded — its streams use
per-reference-interval byte alignment, an application framing choice this
package does not implement (see the [PICS](/conformance/ldc)).

The Green Book, CCSDS 120.0-G-4, publishes a worked preprocessor table in
§3.3.3 that this package transcribes as a test — including the two rows that
fall past θ, where the mapping stops interleaving. An implementation that got
only the interleaved branch right would pass every other row of that table, so
those two are the ones worth having.

The Blue Book's own tables are pinned the same way: table 3-1 for the
fundamental sequence, table 3-2 for the zero-block codewords including the
displaced ROS, table 5-1 for every option identifier at every resolution, and
table 7-1 for the file header field by field.

Annex A of the Green Book names a fuller vector set at
`cwe.ccsds.org/sls/docs/sls-dc/BB121B3TestData`. That location needs a CCSDS
login and returned 403, so it is not used here. Anyone with access should run
it against this package.

## Reference

- [CCSDS 121.0-B-3](https://public.ccsds.org/Pubs/121x0b3.pdf) — Lossless Data
  Compression, the Blue Book
- [CCSDS 120.0-G-4](https://public.ccsds.org/Pubs/120x0g4.pdf) — the Green Book
  report, with the worked examples
- [Conformance](/conformance/ldc)
