# astro cadu

Channel Access Data Unit operations — wrap, unwrap, inspect, and sync CADUs ([CCSDS 131.0-B-4](https://public.ccsds.org/Pubs/131x0b5.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro cadu wrap` | Wrap a TM frame into a CADU (prepend ASM, optionally randomize) |
| `astro cadu unwrap` | Strip ASM and optionally de-randomize to extract TM frame |
| `astro cadu inspect` | Annotated CADU breakdown with ASM validation and hex dump |
| `astro cadu sync` | Scan a byte stream for ASM markers and extract aligned CADUs |

---

## astro cadu wrap

Prepend the Attached Sync Marker (0x1ACFFC1D) and optionally apply CCSDS pseudo-randomization to produce a CADU.

```
astro cadu wrap [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |
| `--randomize` | `false` | Apply CCSDS pseudo-randomization |

**Examples**

```bash
# Wrap a TM frame
astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro cadu wrap --input hex

# Wrap with randomization
astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro cadu wrap --input hex --randomize
```

---

## astro cadu unwrap

Strip the ASM and optionally de-randomize to extract TM Transfer Frame data.

```
astro cadu unwrap [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |
| `--derandomize` | `false` | Apply CCSDS de-randomization |

**Examples**

```bash
# Unwrap a CADU
astro cadu unwrap --input hex cadu.hex

# Unwrap with de-randomization
cat cadu.hex | astro cadu unwrap --input hex --derandomize
```

---

## astro cadu inspect

Display an annotated breakdown of a CADU showing ASM validation, frame data hex dump, and full raw dump.

```
astro cadu inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |

**Examples**

```bash
astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro cadu wrap --input hex | astro cadu inspect --input hex
```

---

## astro cadu sync

Scan a raw byte stream for CCSDS Attached Sync Markers (0x1ACFFC1D) and extract aligned CADUs of a given length.

```
astro cadu sync [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `hex` |
| `--frame-len` | *(required)* | Total CADU length in bytes including ASM |

**Examples**

```bash
# Sync and extract CADUs from a binary capture
astro cadu sync --input bin --frame-len 1115 capture.bin

# Sync from hex with JSON output
astro cadu sync --input hex --frame-len 17 --format json stream.hex
```

---

## Piping

```bash
# Full TM chain: encode frame → wrap CADU → inspect
astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro cadu wrap --input hex | astro cadu inspect --input hex

# Wrap → Unwrap round-trip
astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro cadu wrap --input hex | astro cadu unwrap --input hex

# Wrap with randomize → Unwrap with derandomize
astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro cadu wrap --input hex --randomize | astro cadu unwrap --input hex --derandomize
```
