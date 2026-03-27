# astro tc

TC Transfer Frame operations — encode, decode, and inspect CCSDS TC Transfer Frames ([CCSDS 232.0-B-4](https://public.ccsds.org/Pubs/232x0b4e1c1.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro tc decode` | Decode a TC Transfer Frame with CRC verification |
| `astro tc encode` | Construct a TC Transfer Frame from fields |
| `astro tc inspect` | Annotated frame breakdown with hex dump |

---

## astro tc decode

Decode a TC Transfer Frame from raw bytes. CRC-16-CCITT is verified automatically.

```
astro tc decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Decode from hex stdin
astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro tc decode --input hex

# Decode with JSON output
astro tc decode --input hex --format json frame.hex
```

---

## astro tc encode

Construct a TC Transfer Frame from header fields and hex-encoded data. CRC-16-CCITT is computed automatically.

```
astro tc encode [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--scid` | `0` | Spacecraft ID (0-1023) |
| `--vcid` | `0` | Virtual Channel ID (0-63) |
| `--data` | *(required)* | Data field as hex string |
| `--bypass` | `false` | Set Type-B (expedited) bypass flag |
| `--control` | `false` | Set control command flag |
| `--seq-num` | `0` | Frame sequence number (0-255) |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Encode a basic TC frame
astro tc encode --scid 26 --vcid 1 --data 0102030405

# Encode a Type-B (bypass/expedited) frame
astro tc encode --scid 26 --vcid 1 --data 0102030405 --bypass

# Encode with sequence number
astro tc encode --scid 26 --vcid 1 --data 0102030405 --seq-num 42

# Encode with JSON output
astro tc encode --scid 26 --vcid 1 --data 0102030405 --format json
```

---

## astro tc inspect

Display an annotated breakdown of a TC Transfer Frame showing primary header fields, optional segment header, data field hex dump, and CRC.

```
astro tc inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |

**Examples**

```bash
# Inspect from pipe
astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro tc inspect --input hex

# Inspect a bypass frame
astro tc encode --scid 26 --vcid 1 --data 0102030405 --bypass | astro tc inspect --input hex
```

**Sample Output**

```
TC Transfer Frame Inspector
────────────────────────────────────────────────────────────
Primary Header (5 bytes)
  Version .............. 0
  Bypass Flag .......... 0 (Type-A (sequence-controlled))
  Control Command ...... 0 (Data)
  Spacecraft ID ........ 26 (0x01A)
  Virtual Channel ID ... 1
  Frame Length ......... 12 bytes
  Frame Sequence Num ... 0
  MCID ................. 26
  GVCID ................ 1665
────────────────────────────────────────────────────────────
Data Field (5 bytes)
  0000  01 02 03 04 05                                    |.....|
────────────────────────────────────────────────────────────
Frame Error Control: 0x9B15 (CRC-16-CCITT)
────────────────────────────────────────────────────────────
Raw Frame (12 bytes)
  0000  00 1a 04 0b 00 01 02 03  04 05 9b 15              |............|
```

---

## Piping

```bash
# Encode → Inspect
astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro tc inspect --input hex

# Encode → Decode as JSON
astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro tc decode --input hex --format json

# Encode bypass → Decode
astro tc encode --scid 26 --vcid 1 --data 0102030405 --bypass --seq-num 42 | astro tc decode --input hex --format json
```
