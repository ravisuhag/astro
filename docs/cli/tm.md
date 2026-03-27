# astro tm

TM Transfer Frame operations — encode, decode, inspect, and analyze CCSDS TM Transfer Frames ([CCSDS 132.0-B-3](https://public.ccsds.org/Pubs/132x0b3.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro tm decode` | Decode a TM Transfer Frame into header fields and data |
| `astro tm encode` | Construct a TM Transfer Frame from fields |
| `astro tm inspect` | Annotated frame breakdown with hex dump |
| `astro tm gaps` | Detect MC/VC counter gaps in a frame stream |
| `astro tm demux` | Filter frames by Virtual Channel ID |

## Common Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | varies | Output format: `text`, `json`, or `hex` |

Stream commands (`gaps`, `demux`) require `--frame-len` to split the input into fixed-length frames.

---

## astro tm decode

Decode a TM Transfer Frame from raw bytes. CRC is verified automatically.

```
astro tm decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Decode from hex stdin
astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro tm decode --input hex

# Decode with JSON output
astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro tm decode --input hex --format json

# Decode a binary file
astro tm decode --input bin frame.bin
```

---

## astro tm encode

Construct a TM Transfer Frame from header fields and hex-encoded data. CRC-16-CCITT is computed automatically.

```
astro tm encode [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--scid` | `0` | Spacecraft ID (0-1023) |
| `--vcid` | `0` | Virtual Channel ID (0-7) |
| `--data` | *(required)* | Data field as hex string |
| `--ocf` | | Operational Control Field as hex string (4 bytes) |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Encode a basic TM frame
astro tm encode --scid 26 --vcid 1 --data 0102030405

# Encode with Operational Control Field
astro tm encode --scid 26 --vcid 1 --data 0102030405 --ocf 00000000

# Encode with JSON output
astro tm encode --scid 26 --vcid 1 --data 0102030405 --format json
```

---

## astro tm inspect

Display an annotated breakdown of a TM Transfer Frame showing primary header fields, secondary header (if present), data field hex dump, OCF, FEC, and full raw frame dump.

```
astro tm inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |

**Examples**

```bash
# Inspect from pipe
astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro tm inspect --input hex

# Inspect binary file
astro tm inspect --input bin frame.bin
```

**Sample Output**

```
TM Transfer Frame Inspector
────────────────────────────────────────────────────────────
Primary Header (6 bytes)
  Version .............. 0
  Spacecraft ID ........ 26 (0x01A)
  Virtual Channel ID ... 1
  OCF Flag ............. false
  MC Frame Count ....... 0
  VC Frame Count ....... 0
  FSH Flag ............. false
  Sync Flag ............ false
  Packet Order Flag .... false
  Segment Length ID .... 3
  First Header Ptr ..... 0 (0x000)
  MCID ................. 26
  GVCID ................ 209
────────────────────────────────────────────────────────────
Data Field (5 bytes)
  0000  01 02 03 04 05                                    |.....|
────────────────────────────────────────────────────────────
Frame Error Control: 0x292B (CRC-16-CCITT)
────────────────────────────────────────────────────────────
Raw Frame (13 bytes)
  0000  01 a2 00 00 18 00 01 02  03 04 05 29 2b           |...........)+|
```

---

## astro tm gaps

Scan a stream of concatenated TM Transfer Frames and report any gaps or discontinuities in the Master Channel (MC) and Virtual Channel (VC) frame counters. Useful for detecting lost frames in a capture.

```
astro tm gaps [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--frame-len` | *(required)* | Fixed frame length in bytes |

**Examples**

```bash
# Detect gaps in a binary capture
astro tm gaps --input bin --frame-len 1115 capture.bin

# Detect gaps in hex input
astro tm gaps --input hex --frame-len 13 frames.hex
```

---

## astro tm demux

Demultiplex a stream of concatenated TM Transfer Frames, outputting only frames matching the specified Virtual Channel ID.

```
astro tm demux [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `hex` |
| `--frame-len` | *(required)* | Fixed frame length in bytes |
| `--vcid` | *(required)* | Virtual Channel ID to filter (0-7) |

**Examples**

```bash
# Extract VCID 2 frames from a binary capture
astro tm demux --input bin --frame-len 1115 --vcid 2 capture.bin

# Demux with JSON output
astro tm demux --input hex --frame-len 13 --vcid 0 --format json frames.hex

# Demux and pipe to inspect
astro tm demux --input bin --frame-len 1115 --vcid 0 --format hex capture.bin | astro tm decode --input hex --format json
```

---

## Piping

```bash
# Encode → Inspect
astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro tm inspect --input hex

# Encode → Decode as JSON
astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro tm decode --input hex --format json

# Encode with OCF → Inspect
astro tm encode --scid 26 --vcid 1 --data 0102030405 --ocf 00000000 | astro tm inspect --input hex
```
