# astro aos

AOS Transfer Frame operations — encode, decode, inspect, and generate AOS Transfer Frames ([CCSDS 732.0-B-4](https://public.ccsds.org/Pubs/732x0b4.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro aos encode` | Construct an AOS Transfer Frame from fields |
| `astro aos decode` | Decode an AOS Transfer Frame into header fields and data |
| `astro aos inspect` | Annotated frame breakdown with hex dump |
| `astro aos gen` | Generate synthetic AOS Transfer Frames |

## Common Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | varies | Output format: `text`, `json`, or `hex` |
| `--fecf` | `false` | Toggle the 2-byte CRC-16 Frame Error Control Field |
| `--ocf` (decode/inspect) | `false` | Frame includes a 4-byte OCF |
| `--insert-len` | `0` | Insert zone length in bytes |

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
