---
title: Performance
short: Performance
description: Measured throughput and allocations for every layer, so you can size a link before building it.
order: 3
---

Every frame protocol, both compressors, the Reed-Solomon, Viterbi, and BCH coders, the SDLS security layer, and the checksums are benchmarked. Run them:

```bash
make bench
```

The numbers below came from that suite. They are here so you can answer "will this keep up with my downlink?" without writing the benchmark yourself.

## How to read these

```
goos: darwin   goarch: arm64   cpu: Apple M2 Pro
```

One machine, one architecture, single-threaded, warm cache, no I/O. Treat them as **relative costs between layers**, not as a promise about your hardware. The shape of the answer travels; the absolute figures do not.

`MB/s` counts the octets that went through the operation. Allocations matter as much as time on a flight-adjacent system, so they are in the table.

Figures are rounded to two significant figures, because they move by 10 to 15 percent between runs on the same machine. A difference of a few percent between two rows here means nothing; a factor of two means something.

## The bottleneck is Reed-Solomon (Viterbi, on Proximity-1)

| Operation | Throughput | Allocations |
|---|---|---|
| RS(255,223) encode | 38 MB/s | 3 |
| RS(255,239) encode | 73 MB/s | 3 |
| RS decode, no errors | 34 MB/s | 3 |
| RS decode, 1 error | 7.7 MB/s | 8 |
| RS decode, 8 errors | 7.1 MB/s | 8 |
| RS decode, 16 errors | 7.1 MB/s | 9 |
| Viterbi decode, 4096-octet frame | 0.73 MB/s | 3 |

Three things to plan around.

**Decoding costs about 4.4× more once it is actually correcting.** A clean link decodes at about 34 MB/s and a link near the correction limit at about 7.1 MB/s. Size for the bad case, because the bad case is when you need the data.

**The stronger code is nearly half the speed.** RS(255,223) corrects 16 symbols at about 38 MB/s; RS(255,239) corrects 8 at about 73 MB/s. That is the trade, in numbers.

**On a Proximity-1 link, Viterbi is the actual bottleneck, not Reed-Solomon.** Decoding the rate-1/2, constraint-length-7 convolutional code of CCSDS 211.2-B-3 runs at 0.73 MB/s, about 10 times slower than RS decode even at its worst. `pkg/pxsc` was missing from `make bench` and its one benchmark reported no allocation figures, so this number was simply never in the picture before. "Reed-Solomon is the bottleneck" is true for a TM, AOS, or USLP downlink; a Proximity-1 link is bottlenecked by Viterbi instead.

The time here is inherent Galois-field arithmetic rather than anything redundant, so it is unlikely to improve much — but that is only true of the time, not of the allocations. Berlekamp-Massey used to allocate a fresh scratch buffer on every non-zero discrepancy, which is why the 1/8/16-error rows above used to show 9, 23, and 40 allocations; they now show a flat 8, 8, and 9, because that buffer is allocated once and reused. Viterbi decode got the same kind of cleanup: allocations dropped from 14 per call to 3. Its time did not measurably change, because the trellis update dominates it, not allocation.

## Everything else is far cheaper

| Operation | Throughput | Allocations |
|---|---|---|
| CADU wrap | 8000 MB/s | 1 |
| CADU wrap, randomized | 1000 MB/s | 2 |
| TM frame encode | 310 MB/s | 2 |
| TM frame decode | 300 MB/s | 2 |
| TC frame encode | 280 MB/s | 3 |
| TC frame decode | 300 MB/s | 2 |
| AOS frame encode | 290 MB/s | 7 |
| AOS frame decode | 300 MB/s | 3 |
| USLP frame encode | 280 MB/s | 11 |
| USLP frame decode | 300 MB/s | 3 |
| CRC-16, 1115-octet frame | 310 MB/s | 0 |
| CRC-32, 1115-octet frame | 310 MB/s | 0 |

Frame encoding and decoding sit around 300 MB/s across all four data link protocols, and that number is dominated by the CRC: a table-driven CRC-16 over the same 1115 octets runs at about the same rate. The framing itself is nearly free.

Header decoding alone is 3.6 ns for TM and 3.0 ns for AOS, with no allocations, so demultiplexing a stream by VCID before deciding what to keep costs almost nothing.

## Packets

| Operation | Throughput | Allocations |
|---|---|---|
| Encode, 256 octets | 4000 MB/s | 3 |
| Encode, 4096 octets | 9000 MB/s | 3 |
| Encode with CRC, 256 octets | 290 MB/s | 3 |
| Decode, 256 octets | 4000 MB/s | 3 |
| `spp.PacketSizer` | 0.59 ns | 0 |

The packet layer is not where your time goes, with one exception: **`WithErrorControl` costs an order of magnitude.** Encoding a 256-octet packet runs at thousands of MB/s without a CRC and a few hundred with one. The CRC is the whole cost, and it is the same CRC the frame layer already computes over everything. On a link where frames carry a FECF, a per-packet CRC buys you very little for 12× the packet encoding cost.

## Compression

| Operation | Throughput | Allocations |
|---|---|---|
| LDC compress, smooth ramp | 48 MB/s | 14 |
| LDC compress, noise | 62 MB/s | 16 |
| LDC decompress, smooth ramp | 48 MB/s | 269 |
| LDC decompress, noise | 73 MB/s | 269 |
| RHC compress, one cycle | 9.7 MB/s | 13 |
| RHC decompress, one cycle | 33 MB/s | 7 |

LDC runs at roughly 50 to 70 MB/s, comfortably faster than the Reed-Solomon that will follow it, so compression is not the constraint in a downlink chain. Decompression allocates 269 times, which is worth knowing if you are processing an archive rather than a live pass.

RHC is measured per cycle over a small housekeeping vector, so its MB/s figure is dominated by per-cycle overhead rather than throughput. 13 allocations per compressed cycle is the number to watch on a spacecraft that runs it every second.

## Coding on the uplink

| Operation | Throughput | Allocations |
|---|---|---|
| BCH encode | 69 MB/s | 1 |
| BCH decode, clean | 68 MB/s | 1 |
| BCH decode, with an error | 2.3 MB/s | 1 |
| CLTU wrap | 70 MB/s | 2 |

BCH correction is 29× slower than the clean path, but an uplink is kilobits per second, so this never matters. It is listed for completeness.

## SDLS security

Measured over a 1115-octet TM frame data field, the same frame size the tables above use, for the clause E1/E3/E4 AES-256-GCM baseline (authenticated encryption), GMAC (GCM's authentication-only companion), and the clause E2 AES-CMAC telecommand baseline (authentication-only):

| Operation | Throughput | Allocations |
|---|---|---|
| `ApplySecurity`, AES-256-GCM | 1400 MB/s | 6 |
| `ApplySecurity`, GMAC | 1200 MB/s | 7 |
| `ApplySecurity`, AES-CMAC | 540 MB/s | 9 |
| `ProcessSecurity`, AES-256-GCM | 1100 MB/s | 7 |
| `ProcessSecurity`, GMAC | 1300 MB/s | 7 |
| `ProcessSecurity`, AES-CMAC | 530 MB/s | 10 |

`pkg/sdls` had no benchmarks at all until now, and was absent from `make bench`, even though it runs on the same per-frame path as the Reed-Solomon and Viterbi coders above. All six numbers land comfortably above either coder, so SDLS is not what limits a protected downlink or Proximity-1 link; it was just never checked. AES-CMAC runs at a bit under half the speed of the AES-GCM paths here, which is worth knowing if you are choosing between the clause E2 and clause E1/E3/E4 baselines on throughput alone.

## What is not measured

No end-to-end pipeline benchmark, no concurrent throughput, and nothing for `pkg/sle`, `pkg/cfdp`, `pkg/bp`, `pkg/ltp`, `pkg/pus` or `pkg/xtce`. Those are caller-pumped state machines and parsers whose cost depends far more on how you drive them than on the library, so a single number would mislead.

## Things that will bite you

**A 300 MB/s frame layer does not mean a 300 MB/s link.** The chain is packet, frame, then Reed-Solomon (or, on Proximity-1, Viterbi), and the slowest stage sets the rate. On a downlink with RS(255,223) that is about 34 MB/s clean and about 7.1 MB/s while correcting; on a Proximity-1 link, Viterbi decode at 0.73 MB/s sets the rate instead.

**These numbers are single-threaded.** A `Sender` is not safe for concurrent use, and neither are the frame services, because a downlink is one ordered stream. Scaling means one pipeline per physical channel, not one per core.

**Allocations are per operation, not per octet.** Three allocations to encode a 4096-octet packet is cheap. Three allocations to encode a 16-octet packet, two orders of magnitude slower per octet, is the same three allocations doing much less work. Small packets are dominated by fixed cost.

**Do not read `MB/s` on RHC as throughput.** It compresses one fixed-length vector per cycle, so the figure reflects per-cycle overhead on a small input.

## Reference

- [How this is verified](/docs/reference/verification), what the test suite does and does not prove
- [Handle a lossy link](/docs/guides/lossy-link), where the Reed-Solomon cost is spent
- [Compress before you downlink](/docs/guides/compression), the compressors in use
