---
title: astro ocsc
short: OCSC
description: Optical coding, condition frames into codeblocks, randomise.
order: 110
---

Optical communications coding and synchronisation ([CCSDS 142.0-B-1](https://public.ccsds.org/Pubs/142x0b1.pdf)).

## This layer works in bits

A codeblock is a bit string whose length depends on the code rate and is **not** a whole number of octets. So these commands report a bit length alongside the octets, and the last octet is zero-padded.

That matters for `randomize`: randomising the padding would corrupt the block, so the bit length has to be given rather than inferred from the octet count.

## Subcommands

| Command | Description |
|---|---|
| `astro ocsc condition` | Split frames into SCPPM codeblocks |
| `astro ocsc randomize` | Apply the randomiser to a codeblock |

---

## astro ocsc condition

Split a stream of fixed-length frames into SCPPM codeblocks at the given code rate.

Conditioning is **not** one frame in, one codeblock out. The conditioner fills each codeblock from the frame stream and holds the remainder until enough arrives, so the codeblock count depends on the total input rather than the frame count.

```
astro ocsc condition [file] [flags]
```

**Flags**

| Flag | Default | Description |
|---|---|---|
| `--frame-len` | *(required)* | Fixed frame length in octets |
| `--rate` | `1/2` | SCPPM code rate: `1/3`, `1/2`, or `2/3` |
| `--input` | `bin` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `hex`, or `json` |

**Examples**

```bash
astro ocsc condition --input bin --frame-len 256 --rate 1/2 frames.bin
```

---

## astro ocsc randomize

Exclusive-or a codeblock with the pseudo-randomiser sequence. The randomiser is **its own inverse**, so this both applies and removes it.

```
astro ocsc randomize [file] [flags]
```

**Flags**

| Flag | Default | Description |
|---|---|---|
| `--bits` | *(all of the input)* | Codeblock length in bits |
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `hex` | Output format: `text`, `hex`, or `json` |

**Examples**

```bash
# There and back
astro ocsc randomize --input hex --bits 64 < block.hex |
  astro ocsc randomize --input hex --bits 64
```

---

**See also**: [the protocol page](/protocols/coding/ocsc) for the standard and the Go API, and the [conformance statement](/conformance/ocsc) for what is and is not implemented.
