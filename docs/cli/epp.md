# astro epp

Encapsulation Packet Protocol operations — encode, decode, inspect, validate, and stream CCSDS Encapsulation Packets ([CCSDS 133.1-B-3](https://public.ccsds.org/Pubs/133x1b3e1.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro epp decode` | Decode raw bytes into Encapsulation Packet fields |
| `astro epp encode` | Construct an Encapsulation Packet from header fields and data zone |
| `astro epp inspect` | Pretty-print an annotated packet breakdown with hex dump |
| `astro epp validate` | Check PVN, Protocol ID, header format, and packet length consistency |
| `astro epp stream` | Decode a stream of concatenated Encapsulation Packets |
| `astro epp gen` | Generate synthetic Encapsulation Packets |

## Common Flags

All subcommands that read input support:

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` (hex-encoded text) or `bin` (raw binary) |

All subcommands that produce output support:

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | varies | Output format: `text`, `json`, or `hex` |

Input can be provided as a file argument or piped via stdin. Hex input accepts optional `0x` prefix, spaces, and newlines.

---

## astro epp decode

Decode an Encapsulation Packet from raw bytes, printing its header fields and data zone.

```
astro epp decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Decode hex from stdin
echo "740661626364" | astro epp decode --input hex

# Decode with JSON output
echo "740661626364" | astro epp decode --input hex --format json

# Decode a binary file
astro epp decode --input bin packet.bin
```

---

## astro epp encode

Construct an Encapsulation Packet from header fields and hex-encoded data zone.

```
astro epp encode [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pid` | `2` | Protocol ID (0=idle, 2=IPE, 6=user-defined, 7=extended) |
| `--data` | | Data zone as hex string (omit for idle packets) |
| `--long-length` | `false` | Force longer header format (LoL=1) |
| `--user-defined` | `0` | User-defined field value (Format 3) |
| `--ext-pid` | `0` | Extended Protocol ID (Formats 4 and 5) |
| `--ccsds-defined` | `0` | CCSDS-defined field value (Format 5) |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Encode an IPE packet
astro epp encode --pid 2 --data 4500001400

# Encode a user-defined packet with user-defined field
astro epp encode --pid 6 --data a1b2c3d4 --user-defined 42

# Encode with extended protocol ID (Format 4)
astro epp encode --pid 7 --ext-pid 99 --data a1b2c3d4

# Encode with CCSDS-defined field (Format 5)
astro epp encode --pid 7 --ext-pid 99 --ccsds-defined 4660 --data a1b2c3d4

# Encode an idle packet
astro epp encode --pid 0

# Encode with JSON output
astro epp encode --pid 2 --data 61626364 --format json
```

---

## astro epp inspect

Display an annotated breakdown of an Encapsulation Packet showing header fields, data zone, and a hex dump.

```
astro epp inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |

**Examples**

```bash
# Inspect from hex stdin
echo "740661626364" | astro epp inspect --input hex

# Inspect binary file
astro epp inspect --input bin packet.bin
```

**Sample Output**

```
Encapsulation Packet Inspector
────────────────────────────────────────────────────────────
Header (Format 2, 2 bytes)
  PVN .................. 7
  Protocol ID .......... 2 (ipe)
  Length of Length ...... 0
  Packet Length ........ 6 (total packet: 6 bytes)
────────────────────────────────────────────────────────────
Data Zone (4 bytes)
  0000  61 62 63 64                                       |abcd|
────────────────────────────────────────────────────────────
Raw Packet (6 bytes)
  0000  74 06 61 62 63 64                                 |t.abcd|
```

---

## astro epp validate

Validate an Encapsulation Packet for CCSDS conformance — checks PVN, Protocol ID, header format, and packet length consistency.

```
astro epp validate [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |

**Examples**

```bash
# Validate a packet
echo "740661626364" | astro epp validate --input hex

# Validate a binary file
astro epp validate --input bin packet.bin

# Encode then validate
astro epp encode --pid 2 --data a1b2c3d4 | astro epp validate --input hex
```

---

## astro epp stream

Decode a stream of concatenated Encapsulation Packets, printing each packet as it is parsed. Useful for processing capture files containing multiple back-to-back packets.

```
astro epp stream [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `hex` |

With `--format json`, each packet is printed as a single JSON line (NDJSON), suitable for piping to `jq` or other tools.

**Examples**

```bash
# Stream decode a binary capture
astro epp stream --input bin capture.bin

# Stream decode with JSON output for jq processing
astro epp stream --input bin capture.bin --format json | jq '.protocol_id'

# Concatenate multiple encoded packets and stream decode
P1=$(astro epp encode --pid 2 --data aabb)
P2=$(astro epp encode --pid 6 --data ccdd)
echo "${P1}${P2}" | astro epp stream --input hex
```

---

## astro epp gen

Generate a stream of synthetic Encapsulation Packets with random data zones.

```
astro epp gen [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pid` | `2` | Protocol ID (2=IPE, 6=user-defined, 7=extended) |
| `--count` | `10` | Number of packets to generate |
| `--size` | `64` | Data zone size in bytes per packet |
| `--long-length` | `false` | Force longer header format (LoL=1) |
| `--format` | `bin` | Output format: `bin` or `hex` |

**Examples**

```bash
# Generate 10 IPE packets of 64 bytes each
astro epp gen --pid 2 --count 10 --size 64

# Generate and pipe to stream decoder
astro epp gen --pid 2 --count 50 --size 128 --format bin | astro epp stream --input bin

# Generate user-defined packets with long headers
astro epp gen --pid 6 --count 5 --size 32 --long-length --format hex
```

---

## Piping

All commands support stdin/stdout piping for composability:

```bash
# Encode → Inspect
astro epp encode --pid 2 --data 0102030405 | astro epp inspect --input hex

# Encode → Validate
astro epp encode --pid 6 --data a1b2c3d4 | astro epp validate --input hex

# Encode → Decode as JSON
astro epp encode --pid 2 --data 0102030405 | astro epp decode --input hex --format json

# Generate → Stream decode
astro epp gen --pid 2 --count 20 --size 32 --format bin | astro epp stream --input bin

# EPP → SPP interop: EPP carrying SPP-encoded data
SPP=$(astro spp encode --apid 100 --type tm --data 61626364)
astro epp encode --pid 6 --data $SPP
```
