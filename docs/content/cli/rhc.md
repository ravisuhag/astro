---
title: astro rhc
short: RHC
description: Robust housekeeping compression (POCKET+) — compress, decompress.
order: 180
---

Compress and decompress housekeeping vectors with POCKET+ ([CCSDS 124.0-B-1](https://public.ccsds.org/Pubs/124x0b1.pdf)).

POCKET+ compresses a stream of equal-length vectors by tracking which bit positions change. Housekeeping is what it is for: a few hundred bits of status where almost nothing moves from one cycle to the next.

It is **stateful**. Each output depends on the ones before it, so the cycles have to arrive in order and none may be missing.

## Subcommands

| Command | Description |
|---------|-------------|
| `astro rhc compress` | Compress a stream of vectors |
| `astro rhc decompress` | Recover the vectors |

## The listing format

CCSDS 124.0-B-1 defines **no** file or container format. It specifies the coding of one cycle and leaves delivery to whatever carries it, which on a real mission is a Space Packet or a frame.

But a compressed output is a bit string, not an octet string, and its length in bits cannot be recovered from the octets. Nothing can be decompressed without being told how long it is.

So these commands write a listing of their own — one line per cycle, the bit length then the octets:

```
84 81bfea500ff18003300010
18 f14280
8 82
```

**This is not an interchange format.** Nothing else will read it. It exists so that `compress` and `decompress` here are inverses and the coder can be exercised end to end. The vector length is not in it either, which is why both commands need `--vector-bits`.

---

## astro rhc compress

```
astro rhc compress [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--vector-bits` | *(required)* | Length of every input vector in bits, *F*: 1 to 65535 |
| `--robustness` | `0` | Minimum required effective robustness, *R_t*: 0 to 7 |
| `--new-mask` | `0` | Set the new mask flag every N cycles (0 never) |
| `--send-mask` | `0` | Set the send mask flag every N cycles (0 never) |
| `--uncompressed` | `0` | Set the uncompressed flag every N cycles (0 never) |
| `--input` | `bin` | Input format: `hex` or `bin` |

The three interval flags are policy, not protocol. Clause 3.3.2 makes each flag user-specified at every cycle and says nothing about when to set them. Higher robustness costs bits; sending the whole mask periodically lets a receiver that has lost its place recover; an uncompressed output is the only thing that restores a decompressor's state after a gap.

The compression ratio is reported on stderr, so it does not get mixed into the listing on stdout.

**Examples**

```bash
# 64-bit vectors
astro rhc compress --input bin --vector-bits 64 housekeeping.bin > coded.rhc

# Send the whole mask every 10 cycles so a receiver can resynchronise
astro rhc compress --input bin --vector-bits 64 --send-mask 10 housekeeping.bin
```

---

## astro rhc decompress

```
astro rhc decompress [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--vector-bits` | *(required)* | Length of every vector in bits — must match `compress` |
| `--strict` | `false` | After a reported loss, accept nothing but an uncompressed output |
| `--format` | `bin` | Output format: `bin` or `hex` |

**Examples**

```bash
# Round trip, byte for byte
astro rhc compress --input bin --vector-bits 64 housekeeping.bin |
  astro rhc decompress --vector-bits 64 > recovered.bin

cmp housekeeping.bin recovered.bin
```

## Limits

The standard specifies no decoder section — its annex A conformance list holds only encoder items — so the decompressor is the encoder run backwards, which clause 2.1 lays out the requirements for.

---

**See also** — [the protocol page](/protocols/compression/rhc) for the standard and the Go API, and the [conformance statement](/conformance/rhc) for what is and is not implemented.
