---
title: astro ldc
short: LDC
description: Lossless data compression — compress, decompress, inspect.
order: 52
---

Lossless data compression with Rice coding ([CCSDS 121.0-B-3](https://public.ccsds.org/Pubs/121x0b3.pdf)).

Samples are whole numbers of a fixed width, not octets: a stream of 12-bit readings from an instrument, say. The commands read and write them big-endian at the width `--resolution` rounds up to, so a 12-bit sample travels in two octets with the top four unused.

## Subcommands

| Command | Description |
|---------|-------------|
| `astro ldc compress` | Compress a sample stream |
| `astro ldc decompress` | Recover the samples |
| `astro ldc inspect` | Show a compressed file's header without decompressing |

## The file carries its parameters

Section 7 defines a self-describing format: a twelve-octet header with every parameter, the coded data, then padding to the output word size.

So `decompress` takes **no** parameter flags, and that is deliberate. Guessing at parameters the header already states is how a decompression comes out plausible and wrong.

---

## astro ldc compress

```
astro ldc compress [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--resolution` | `8` | Bits per input sample, *n*: 1 to 32 |
| `--block-size` | `16` | Samples per block, *J*: 8, 16, 32 or 64 |
| `--predictor` | `unit-delay` | Preprocessor: `unit-delay`, `none`, or `bypass` |
| `--reference-interval` | `256` | Blocks between reference samples, *r*: 1 to 4096 |
| `--signed` | `false` | Samples are two's complement |
| `--restricted` | `false` | Restricted code option set (resolution 4 or fewer) |
| `--word-size` | `1` | Output word size in octets, *B*: 1 to 8 |
| `--input` | `bin` | Input format: `hex` or `bin` |
| `--format` | `bin` | Output format: `bin`, `hex`, or `text` |

A sample that does not fit the stated resolution is refused rather than truncated, and input that is not a whole number of samples is refused rather than having a trailing octet dropped. Both mean the flag and the data disagree, which is worth being told.

**Examples**

```bash
# 8-bit samples
astro ldc compress --input bin --resolution 8 readings.bin > coded.ldc

# 12-bit samples in two octets each, larger blocks
astro ldc compress --input bin --resolution 12 --block-size 32 readings.bin > coded.ldc

# See the ratio
astro ldc compress --input bin --resolution 8 --format text readings.bin
```

---

## astro ldc decompress

```
astro ldc decompress [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `bin` | Input format: `hex` or `bin` |
| `--format` | `bin` | Output format: `bin`, `hex`, or `text` |

**Examples**

```bash
# Round trip, byte for byte
astro ldc compress --input bin --resolution 8 readings.bin |
  astro ldc decompress --input bin > recovered.bin

cmp readings.bin recovered.bin
```

---

## astro ldc inspect

Read the header and print the parameters the body was coded with, without decompressing it. Useful when you have been handed a file and do not know how it was made.

```
astro ldc inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `bin` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
astro ldc inspect --input bin coded.ldc
```

```
LDC File Header
  Word size .......... 1 octets
  Samples ............ 512
LDC Parameters
  Block size ......... 16 samples
  Resolution ......... 8 bits, unsigned
  Predictor .......... unit delay
  Reference every .... 256 blocks
  Code option set .... basic (3-bit identifiers, k up to 5)
  File size .......... 300 octets
```

## Limits

The compression identification packet of section 6 is not built here, insertion into Space Packets is the caller's job, and the application-specific predictor and mapper the standard names but does not define are absent. See the [conformance statement](/conformance/ldc).
