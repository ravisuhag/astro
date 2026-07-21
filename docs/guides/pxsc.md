# Proximity-1 Coding and Synchronization

> CCSDS 211.2-B-3 — Proximity-1 Space Link Protocol, Coding and Synchronization Sublayer

## Overview

This is the layer beneath `pkg/pxdl`. It wraps each transfer frame in a
**Proximity Link Transmission Unit** and fills the gaps between them so the
receiver keeps bit lock.

```
PLTU:  ASM (FAF320) │ transfer frame │ CRC-32
        3 octets       variable         4 octets
```

It does for Proximity-1 what `pkg/tmsc` does for TM and `pkg/tcsc` for TC. The
differences follow from the link being short and bursty:

| | TM (`pkg/tmsc`) | TC (`pkg/tcsc`) | Proximity-1 (`pkg/pxsc`) |
|---|---|---|---|
| Marker | 4-octet ASM | 2-octet start sequence | 3-octet ASM |
| Error control | Reed-Solomon | BCH(63,56) | CRC-32, detect only |
| Unit length | Fixed | Fixed blocks | Variable |
| Between units | Continuous | — | Idle PN pattern |

A Proximity-1 stream is not continuous. PLTUs of different lengths arrive in
bursts with gaps between them, and the receiver re-acquires for each one.

## The CRC-32 is not the one you expect

This catches people out, so it is worth being blunt.

Annex C, C1.3 gives the generator as:

```
G(X) = X^32 + X^23 + X^21 + X^11 + X^2 + 1
```

which is **0x00A00805**. That is neither of the CRC-32s you have met before:

| | Polynomial | Where |
|---|---|---|
| IEEE CRC-32 | 0x04C11DB7 | zip, Ethernet |
| CRC-32C | 0x1EDC6F41 | `pkg/crc.ComputeCRC32`, USLP FECF |
| **Proximity-1** | **0x00A00805** | here |

Two more details. The shift register **starts at zero**, not all-ones — the
spec flags this itself, noting it "differs from that performed for the 16-bit
CRC described in other CCSDS books". And there is no final inversion.

Get any of the three wrong and you produce a checksum that looks entirely
plausible and rejects every frame you receive. That is why this CRC lives in
`pkg/pxsc` with its own tests rather than borrowing from `pkg/crc`.

**The ASM is not covered by the CRC** (annex C, C1.2 note 2). The check value
is computed over the transfer frame alone.

## Sending

```go
import "github.com/ravisuhag/astro/pkg/pxsc"

frame, _ := transferFrame.Encode()   // from pkg/pxdl

pltu, err := pxsc.WrapPLTU(frame)
if err != nil {
    return err
}
transmit(pltu)
```

Between PLTUs, send idle data:

```go
transmit(pxsc.IdleSequence(n))
```

## Idle data

A repeating pseudo-noise pattern, **352EF853** (§3.3.2.2), tiled to whatever
length you need. When the end is reached it starts again from the first bit.

The same pattern serves three roles, distinguished only by when it is sent
(§3.3.1):

| Sequence | When | Duration from |
|---|---|---|
| Acquisition | Transmission starts | `Acquisition_Idle_Duration` |
| Idle | No PLTU is ready | As needed |
| Tail | Before going quiet | `Tail_Idle_Duration` |

The tail sequence matters more than it looks. Without it, the receiver loses
bit lock while still decoding the last PLTU it got.

Durations come from mission parameters, which is why these functions take a
length rather than choosing one:

```go
pxsc.AcquisitionSequence(n)
pxsc.IdleSequence(n)
pxsc.TailSequence(n)
```

## Receiving: the synchronizer

Finding PLTUs in a stream is not parsing, it is hunting. Units are
variable-length, separated by idle runs, and the marker is only 24 bits — a
random match turns up roughly every 16 million octets.

So the CRC does the real work of telling a PLTU from a coincidence:

```go
s := pxsc.NewSynchronizer()

for _, pltu := range s.Scan(stream) {
    frame, err := pxdl.DecodeTransferFrame(pltu.Frame)
    if err != nil {
        continue
    }
    handle(frame)
}
```

At each marker the synchronizer tries frame lengths from the minimum upward
and takes the first whose CRC verifies. A marker with no verifying length is a
false match: it steps one octet past and keeps hunting, so a marker pattern
sitting inside frame data does not derail it.

A PLTU whose CRC fails is skipped, per §3.6, and a good one after it is still
found. There is a test for exactly that.

Set `MinFrameLength` and `MaxFrameLength` to what your mission sends. The
defaults are the Version-3 bounds, 5 to 2048 octets.

If you already know a PLTU starts at offset zero, `UnwrapPLTU` is the direct
path.

## Convolutional encoding

Proximity-1 offers the rate 1/2, constraint-length 7 convolutional code from
CCSDS 131.0 (§3.4.3.1). Each input bit becomes two output symbols.

```go
e := pxsc.NewConvolutionalEncoder()
symbols := e.Encode(bitstream)
```

Two things to know.

**The G2 output is inverted** (§3.4.3.1 note 1). Connection vectors are
G1 = 171 octal and G2 = 133 octal, with the second path complemented.

**The encoder state carries across calls.** §3.4.3.2 encodes everything
transmitted as one continuous stream — PLTUs and idle data alike — so the shift
register must not reset at unit boundaries. Reuse one `ConvolutionalEncoder`
for the whole stream; `Reset()` is there if you genuinely need to start over.

**Only the encoder is here.** Decoding a convolutional code means a Viterbi
trellis search over soft-decision symbols. This library ships no soft-decision
decoder anywhere — `pkg/tmsc` stops at Reed-Solomon, `pkg/tcsc` at BCH — and
adding one here alone would be out of step. §3.4.3.3 recommends at least
three-bit soft decisions, which in practice means the receiver hardware.

## What is not here yet

- **The LDPC code** of §3.4.4, its Codeword Sync Marker, and the
  pseudo-randomizer of §3.4.5, which applies only when LDPC is used.
- **Viterbi decoding**, as above.
- **Reed-Solomon**, which some transceivers add but §3.4.1 notes is not part of
  the CCSDS Proximity-1 standards and is not intended for cross support.
- **CLI subcommands** — a follow-up once the API settles.

## Reference

- [CCSDS 211.2-B-3](https://public.ccsds.org/Pubs/211x2b3.pdf) — Coding and Synchronization Sublayer
- [CCSDS 211.0-B-6](https://public.ccsds.org/Pubs/211x0b6e1.pdf) — Data Link Layer, for the transfer frame
- [PICS proforma](../pics/pxsc-pics.md) — conformance statement for this package
