---
title: astro aos
short: AOS
description: AOS transfer frames, encode, decode, inspect, gaps, demux.
order: 50
---

AOS Transfer Frame operations: encode, decode, inspect, and generate AOS Transfer Frames ([CCSDS 732.0-B-4](https://public.ccsds.org/Pubs/732x0b4.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro aos encode` | Construct an AOS Transfer Frame from fields |
| `astro aos decode` | Decode an AOS Transfer Frame into header fields and data |
| `astro aos inspect` | Annotated frame breakdown with hex dump |
| `astro aos gaps` | Detect VC counter gaps in a frame stream |
| `astro aos demux` | Filter frames by Virtual Channel ID |
| `astro aos gen` | Generate synthetic AOS Transfer Frames |

## Common Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | varies | Output format: `text`, `json`, or `hex` |
| `--fecf` | `false` | Toggle the 2-byte CRC-16 Frame Error Control Field |
| `--ocf` (decode/inspect) | `false` | Frame includes a 4-byte OCF |
| `--insert-len` | `0` | Insert zone length in bytes |

Stream commands (`gaps`, `demux`) require `--frame-len`, because an AOS frame carries no length field. The length is a managed parameter agreed before the pass. They read as they go, so they work on a live pipe and on a capture larger than memory.

---

## astro aos encode

Construct an AOS Transfer Frame from header fields and data. FECF is computed automatically when enabled.

```
astro aos encode [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--scid` | `0` | Spacecraft ID (0-255) |
| `--vcid` | `0` | Virtual Channel ID (0-63) |
| `--data` | required | Data field as hex string |
| `--ocf` | | Operational Control Field as hex string (4 bytes) |
| `--insert` | | Insert Zone as hex string |
| `--fecf` | `false` | Append CRC-16 Frame Error Control Field |
| `--vc-count` | `0` | VC Frame Count (24-bit) |
| `--replay` | `false` | Set the Replay Flag |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Encode a basic AOS frame
astro aos encode --scid 50 --vcid 1 --data 0102030405

# Encode with FECF and OCF
astro aos encode --scid 50 --vcid 1 --data 0102030405 --ocf 00000000 --fecf

# Encode with JSON output
astro aos encode --scid 50 --vcid 1 --data 0102030405 --format json
```

---

## astro aos decode

Decode an AOS Transfer Frame from raw bytes. FECF is verified automatically when present.

```
astro aos decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `hex` |
| `--fecf` | `false` | Frame includes a 2-byte FECF |
| `--ocf` | `false` | Frame includes a 4-byte OCF |
| `--insert-len` | `0` | Insert zone length in bytes |

**Examples**

```bash
# Decode from hex stdin
astro aos encode --scid 50 --vcid 1 --data 0102030405 --fecf | astro aos decode --input hex --fecf

# Decode with JSON output
astro aos encode --scid 50 --vcid 1 --data 0102030405 | astro aos decode --input hex --format json
```

---

## astro aos inspect

Display an annotated breakdown of an AOS Transfer Frame showing all header fields, data regions, and hex dump.

```
astro aos inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--fecf` | `false` | Frame includes a 2-byte FECF |
| `--ocf` | `false` | Frame includes a 4-byte OCF |
| `--insert-len` | `0` | Insert zone length in bytes |

**Examples**

```bash
# Inspect from hex stdin
astro aos encode --scid 50 --vcid 1 --data 0102030405 --fecf | astro aos inspect --input hex --fecf

# Inspect a binary file
astro aos inspect --input bin frame.bin
```

---

## astro aos gaps

Scan a stream of concatenated AOS Transfer Frames and report gaps in the Virtual Channel frame counter. Each gap line gives the number of frames that went missing, not just that the counter jumped.

AOS has no Master Channel frame count, so only virtual channel gaps are reported. Where a frame sets the VC Frame Count Usage Flag, the 4-bit cycle is folded in above the 24-bit count and the pair is treated as one 28-bit counter ([clause 4.1.2.5.5.3](https://public.ccsds.org/Pubs/732x0b4.pdf)), so a wrap of the count reads as contiguous rather than as sixteen million missing frames.

Counters are tracked per spacecraft. A capture holding two SCIDs is compared within each, never across them.

```
astro aos gaps [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--frame-len` | *(required)* | Fixed frame length in bytes |
| `--fecf` | `false` | Frames include a 2-byte FECF |
| `--ocf` | `false` | Frames include a 4-byte OCF |
| `--insert-len` | `0` | Insert zone length in bytes |

**Examples**

```bash
# Detect gaps in a binary capture
astro aos gaps --input bin --frame-len 1024 capture.bin

# Frames carrying an insert zone and FECF
astro aos gaps --input bin --frame-len 1115 --insert-len 8 --fecf capture.bin
```

---

## astro aos demux

Demultiplex a stream of concatenated AOS Transfer Frames, passing on only the frames matching the given Virtual Channel ID.

```
astro aos demux [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `hex` |
| `--frame-len` | *(required)* | Fixed frame length in bytes |
| `--vcid` | *(required)* | Virtual Channel ID to filter (0-63) |
| `--fecf` | `false` | Frames include a 2-byte FECF |
| `--ocf` | `false` | Frames include a 4-byte OCF |
| `--insert-len` | `0` | Insert zone length in bytes |

**Examples**

```bash
# Extract VCID 2 frames from a binary capture
astro aos demux --input bin --frame-len 1024 --vcid 2 capture.bin

# Demux and pipe back into decode
astro aos demux --input bin --frame-len 1024 --vcid 0 --format hex capture.bin | astro aos decode --input hex --format json
```

---

## astro aos gen

Generate a stream of synthetic AOS Transfer Frames with incrementing VC frame counts and random data.

```
astro aos gen [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--scid` | `0` | Spacecraft ID (0-255) |
| `--vcid` | `0` | Virtual Channel ID (0-63) |
| `--count` | `10` | Number of frames to generate |
| `--data-size` | `64` | Data field size in bytes per frame |
| `--fecf` | `false` | Append CRC-16 Frame Error Control Field |
| `--format` | `bin` | Output format: `bin` or `hex` |

**Examples**

```bash
# Generate 10 AOS frames
astro aos gen --scid 50 --vcid 1 --count 10 --data-size 64

# Generate with FECF and hex output
astro aos gen --scid 50 --vcid 1 --count 5 --data-size 32 --fecf --format hex
```

---

**See also**: [the protocol page](/protocols/data-link/aos) for the standard and the Go API, and the [conformance statement](/conformance/aos) for what is and is not implemented.
