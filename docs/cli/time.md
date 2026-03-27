# astro time

CCSDS Time Code Format operations — encode, decode, and inspect CCSDS time codes ([CCSDS 301.0-B-4](https://public.ccsds.org/Pubs/301x0b4e1.pdf)). Supports CUC, CDS, CCS, and ASCII formats.

## Subcommands

| Command | Description |
|---------|-------------|
| `astro time decode` | Decode a binary/hex time code into a human-readable timestamp |
| `astro time encode` | Encode a timestamp into a CCSDS time code |
| `astro time inspect` | Annotated P-field and T-field breakdown with hex dump |
| `astro time now` | Encode the current UTC time in all supported formats |

## Supported Formats

| Format | Codec Flag | Description |
|--------|-----------|-------------|
| CUC | `cuc` | CCSDS Unsegmented Time Code — coarse + fine seconds since epoch |
| CDS | `cds` | CCSDS Day Segmented — day count + milliseconds of day |
| CCS | `ccs` | CCSDS Calendar Segmented — calendar fields (year, day, hour, etc.) |
| ASCII Type A | `ascii-a` | ISO 8601 calendar date (`YYYY-MM-DDThh:mm:ss.dZ`) |
| ASCII Type B | `ascii-b` | ISO 8601 ordinal date (`YYYY-DDDThh:mm:ss.dZ`) |

For binary formats (CUC, CDS, CCS), the codec is auto-detected from the P-field when `--codec` is omitted.

---

## astro time decode

Decode a CCSDS time code into a human-readable timestamp.

```
astro time decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |
| `--codec` | auto | Time code format: `cuc`, `cds`, `ccs`, `ascii-a`, `ascii-b` |

**Examples**

```bash
# Decode a CUC time code (auto-detected from P-field)
echo "1c7e67d175" | astro time decode --input hex

# Decode a CDS time code
echo "405fe102af5508" | astro time decode --codec cds --input hex

# Decode with JSON output
echo "1c7e67d175" | astro time decode --input hex --format json

# Decode an ASCII Type A time string
echo "2025-03-15T12:30:45.123Z" | astro time decode --codec ascii-a
```

---

## astro time encode

Convert a timestamp into a CCSDS time code.

```
astro time encode [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--codec` | `cuc` | Time code format: `cuc`, `cds`, `ccs`, `ascii-a`, `ascii-b` |
| `--time` | `now` | Timestamp to encode (RFC3339 or `now`) |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |

**CUC-specific flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--coarse-bytes` | `4` | Coarse time octets (1-4) |
| `--fine-bytes` | `0` | Fine time octets (0-3) |

**CDS-specific flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--day-bytes` | `2` | Day segment width (2 or 3) |
| `--subms-bytes` | `0` | Sub-millisecond width (0, 2, or 4) |

**CCS-specific flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--month-day` | `false` | Use month/day variant instead of day-of-year |
| `--sub-sec-bytes` | `0` | Sub-second octets (0-6) |

**ASCII-specific flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--precision` | `3` | Fractional second digits (0-9) |

**Examples**

```bash
# Encode current time as CUC
astro time encode --codec cuc

# Encode a specific timestamp as CDS
astro time encode --codec cds --time "2025-03-15T12:30:45Z"

# Encode as CCS with month/day variant
astro time encode --codec ccs --time "2025-03-15T12:30:45Z" --month-day

# Encode as ASCII Type A
astro time encode --codec ascii-a --time "2025-03-15T12:30:45.123Z"

# Encode CUC with fine time resolution
astro time encode --codec cuc --time "2025-03-15T12:30:45.5Z" --fine-bytes 2

# JSON output
astro time encode --codec cuc --format json
```

---

## astro time inspect

Display an annotated breakdown of a CCSDS time code, including P-field details, T-field segments, resolved timestamp, and a hex dump.

```
astro time inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--codec` | auto | Time code format: `cuc`, `cds`, `ccs` |

**Examples**

```bash
# Inspect a CUC time code
echo "1c7e67d175" | astro time inspect --input hex

# Inspect a CDS time code
echo "405fe102af5508" | astro time inspect --codec cds --input hex
```

**Sample Output**

```
Time Code Inspector
────────────────────────────────────────────────────────────
P-Field (1 byte)
  Extension ............ false
  Time Code ID ......... 1 (CUC Level 1)
  Detail Bits .......... 0xC
────────────────────────────────────────────────────────────
CUC T-Field
  Level ................ Level 1 (CCSDS epoch: 1958-01-01)
  Coarse Octets ........ 4
  Fine Octets .......... 0
  Coarse Time .......... 2120733045 s
  Resolved Time ........ 2025-03-15T12:30:45Z
────────────────────────────────────────────────────────────
Raw (5 bytes)
  0000  1c 7e 67 d1 75                                    |.~g.u|
```

---

## astro time now

Encode the current UTC time in all supported CCSDS formats at once.

```
astro time now [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--codec` | all | Specific format: `cuc`, `cds`, `ccs`, `ascii-a`, `ascii-b` |
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
# Show current time in all formats
astro time now

# Show only CUC
astro time now --codec cuc

# JSON output with all formats
astro time now --format json
```

**Sample Output**

```
Current UTC: 2025-03-15T12:30:45.123456Z
────────────────────────────────────────────────────────────
CUC .... 1e7e67d1751fca
CDS .... 40615a02af5508
CCS .... 5a202500741230451234
ASCII-A  2025-03-15T12:30:45.123Z
ASCII-B  2025-074T12:30:45.123Z
```

---

## Piping

```bash
# Encode → Decode round-trip
astro time encode --codec cuc --time "2025-03-15T12:30:45Z" | astro time decode --input hex --format json

# Encode → Inspect
astro time encode --codec cds --time "2025-03-15T12:30:45Z" | astro time inspect --input hex
```
