# astro usdl

USLP Transfer Frame operations — encode, decode, inspect, and generate USLP Transfer Frames ([CCSDS 732.1-B-2](https://public.ccsds.org/Pubs/732x1b2.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro usdl decode` | Decode a USLP Transfer Frame into header fields and data |
| `astro usdl encode` | Construct a USLP Transfer Frame from fields |
| `astro usdl inspect` | Annotated frame breakdown with hex dump |
| `astro usdl gen` | Generate synthetic USLP Transfer Frames |

## Common Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | varies | Output format: `text`, `json`, or `hex` |
| `--crc32` | `false` | Use CRC-32 instead of CRC-16 for FECF |

---

## astro usdl decode

Decode a USLP Transfer Frame from raw bytes. FECF is verified automatically.

```
astro usdl decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `hex` |
| `--crc32` | `false` | Use CRC-32 instead of CRC-16 |

**Examples**

```bash
# Decode from hex stdin
astro usdl encode --scid 100 --vcid 1 --mapid 0 --data 0102030405 | astro usdl decode --input hex

# Decode with JSON output
astro usdl encode --scid 100 --vcid 1 --mapid 0 --data 0102030405 | astro usdl decode --input hex --format json
```

---

## astro usdl encode

Construct a USLP Transfer Frame from header fields and data. FECF is computed automatically.

```
astro usdl encode [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--scid` | `0` | Spacecraft ID (0-65535) |
| `--vcid` | `0` | Virtual Channel ID (0-63) |
| `--mapid` | `0` | MAP ID (0-63) |
| `--data` | required | Data field as hex string |
| `--ocf` | | Operational Control Field as hex string (4 bytes) |
| `--seq` | `0` | Frame sequence number |
| `--crc32` | `false` | Use CRC-32 instead of CRC-16 |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Encode a basic USLP frame
astro usdl encode --scid 100 --vcid 1 --mapid 0 --data 0102030405

# Encode with OCF and CRC-32
astro usdl encode --scid 100 --vcid 1 --mapid 0 --data 0102030405 --ocf 00000000 --crc32

# Encode with JSON output
astro usdl encode --scid 100 --vcid 1 --mapid 0 --data 0102030405 --format json
```

---

## astro usdl inspect

Display an annotated breakdown of a USLP Transfer Frame showing all header fields, data regions, and hex dump.

```
astro usdl inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--crc32` | `false` | Use CRC-32 instead of CRC-16 |

**Examples**

```bash
# Inspect from hex stdin
astro usdl encode --scid 100 --vcid 1 --mapid 0 --data 0102030405 | astro usdl inspect --input hex

# Inspect a binary file
astro usdl inspect --input bin frame.bin
```

---

## astro usdl gen

Generate a stream of synthetic USLP Transfer Frames with incrementing sequence numbers and random data.

```
astro usdl gen [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--scid` | `0` | Spacecraft ID (0-65535) |
| `--vcid` | `0` | Virtual Channel ID (0-63) |
| `--mapid` | `0` | MAP ID (0-63) |
| `--count` | `10` | Number of frames to generate |
| `--data-size` | `64` | Data field size in bytes per frame |
| `--crc32` | `false` | Use CRC-32 instead of CRC-16 |
| `--format` | `bin` | Output format: `bin` or `hex` |

**Examples**

```bash
# Generate 10 USLP frames
astro usdl gen --scid 100 --vcid 1 --mapid 0 --count 10 --data-size 64

# Generate with CRC-32 and hex output
astro usdl gen --scid 100 --vcid 1 --count 5 --data-size 32 --crc32 --format hex
```
