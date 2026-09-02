---
title: astro epp
short: EPP
description: Encapsulation Packets, encode, decode, inspect, validate, stream.
order: 20
---

Encapsulation Packet Protocol operations: encode, decode, inspect, validate, and stream CCSDS Encapsulation Packets ([CCSDS 133.1-B-3](https://public.ccsds.org/Pubs/133x1b3e1.pdf)).

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
echo "e90661626364" | astro epp decode --input hex

# Decode with JSON output
echo "e90661626364" | astro epp decode --input hex --format json

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
| `--pid` | `2` | Protocol ID (0=idle, 1=LTP, 2=IPE, 6=extended, 7=mission) |
| `--data` | | Data zone as hex string (omit for the 1-octet idle packet) |
| `--long-length` | `false` | Force at least a 4-octet header (2-octet length field) |
| `--user-defined` | `0` | User Defined Field value, 4 bits (4- and 8-octet headers) |
| `--ext-pid` | `0` | Protocol ID Extension, 4 bits (4- and 8-octet headers) |
| `--ccsds-defined` | `0` | CCSDS Defined Field value (8-octet header) |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Encode an IPE packet
astro epp encode --pid 2 --data 4500001400

# Encode a mission-specific packet with a user defined field value
astro epp encode --pid 7 --data a1b2c3d4 --user-defined 5

# Encode with an extended protocol ID (4-octet header)
astro epp encode --pid 6 --ext-pid 9 --data a1b2c3d4

# Encode with a CCSDS-defined field (8-octet header)
astro epp encode --pid 6 --ext-pid 9 --ccsds-defined 4660 --data a1b2c3d4

# Encode the 1-octet idle packet (0xE0)
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
echo "e90661626364" | astro epp inspect --input hex

# Inspect binary file
astro epp inspect --input bin packet.bin
```

**Sample Output**

```
Encapsulation Packet Inspector
────────────────────────────────────────────────────────────
Header (2 bytes, LoL 1)
  PVN .................. 7
  Protocol ID .......... 2 (ipe)
  Length of Length ...... 1
  Packet Length ........ 6 (total packet: 6 bytes)
────────────────────────────────────────────────────────────
Data Zone (4 bytes)
  0000  61 62 63 64                                       |abcd|
────────────────────────────────────────────────────────────
Raw Packet (6 bytes)
  0000  e9 06 61 62 63 64                                 |..abcd|
```

---

## astro epp validate

Validate an Encapsulation Packet for CCSDS conformance: checks PVN, Protocol ID, header format, and packet length consistency.

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
echo "e90661626364" | astro epp validate --input hex

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
P2=$(astro epp encode --pid 7 --data ccdd)
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
| `--pid` | `2` | Protocol ID (1=LTP, 2=IPE, 7=mission) |
| `--count` | `10` | Number of packets to generate |
| `--size` | `64` | Data zone size in bytes per packet |
| `--long-length` | `false` | Force at least a 4-octet header (2-octet length field) |
| `--format` | `bin` | Output format: `bin` or `hex` |

**Examples**

```bash
# Generate 10 IPE packets of 64 bytes each
astro epp gen --pid 2 --count 10 --size 64

# Generate and pipe to stream decoder
astro epp gen --pid 2 --count 50 --size 128 --format bin | astro epp stream --input bin

# Generate mission-specific packets with long headers
astro epp gen --pid 7 --count 5 --size 32 --long-length --format hex
```

---

## Piping

All commands support stdin/stdout piping for composability:

```bash
# Encode -> Inspect
astro epp encode --pid 2 --data 0102030405 | astro epp inspect --input hex

# Encode -> Validate
astro epp encode --pid 7 --data a1b2c3d4 | astro epp validate --input hex

# Encode -> Decode as JSON
astro epp encode --pid 2 --data 0102030405 | astro epp decode --input hex --format json

# Generate -> Stream decode
astro epp gen --pid 2 --count 20 --size 32 --format bin | astro epp stream --input bin

# EPP -> SPP interop: EPP carrying SPP-encoded data
SPP=$(astro spp encode --apid 100 --type tm --data 61626364)
astro epp encode --pid 7 --data $SPP
```

---

**See also**: [the protocol page](/protocols/transport/epp) for the standard and the Go API, and the [conformance statement](/conformance/epp) for what is and is not implemented.
