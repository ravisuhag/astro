# astro spp

Space Packet Protocol operations — encode, decode, inspect, validate, and stream CCSDS Space Packets ([CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2e2.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro spp decode` | Decode raw bytes into Space Packet fields |
| `astro spp encode` | Construct a Space Packet from header fields and user data |
| `astro spp inspect` | Pretty-print an annotated packet breakdown with hex dump |
| `astro spp validate` | Check field ranges, length consistency, and optional CRC |
| `astro spp stream` | Decode a stream of concatenated Space Packets |

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

## astro spp decode

Decode a Space Packet from raw bytes, printing its header fields and user data.

```
astro spp decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Decode hex from stdin
echo "0064c000000361626364" | astro spp decode --input hex

# Decode with JSON output
echo "0064c000000361626364" | astro spp decode --input hex --format json

# Decode a binary file
astro spp decode --input bin packet.bin
```

---

## astro spp encode

Construct a Space Packet from individual header fields and hex-encoded user data.

```
astro spp encode [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--apid` | `0` | Application Process Identifier (0-2047) |
| `--type` | `tm` | Packet type: `tm` or `tc` |
| `--data` | *(required)* | User data as hex string |
| `--seq-count` | `0` | Sequence count (0-16383) |
| `--seq-flags` | `3` | Sequence flags (0=continuation, 1=first, 2=last, 3=unsegmented) |
| `--crc` | `false` | Append CRC-16-CCITT error control field |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Encode a TM packet
astro spp encode --apid 100 --type tm --data 68656c6c6f

# Encode a TC packet with CRC
astro spp encode --apid 42 --type tc --data a1b2c3d4 --crc

# Encode with JSON output
astro spp encode --apid 100 --type tm --data 61626364 --format json
```

---

## astro spp inspect

Display an annotated breakdown of a Space Packet including header field table, user data hex dump, and full raw packet hex dump.

```
astro spp inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |

**Examples**

```bash
# Inspect from hex stdin
echo "0064c000000361626364" | astro spp inspect --input hex

# Inspect a binary file
astro spp inspect --input bin packet.bin
```

**Sample Output**

```
Space Packet Inspector
────────────────────────────────────────────────────────────
Primary Header (6 bytes)
  Version .............. 0
  Type ................. 0 (TM)
  Secondary Header Flag  0
  APID ................. 100 (0x064)
  Sequence Flags ....... 3 (unsegmented)
  Sequence Count ....... 0
  Packet Data Length ... 3 (total packet: 10 bytes)
────────────────────────────────────────────────────────────
User Data (4 bytes)
  0000  61 62 63 64                                       |abcd|
────────────────────────────────────────────────────────────
Raw Packet (10 bytes)
  0000  00 64 c0 00 00 03 61 62  63 64                    |.d....abcd|
```

---

## astro spp validate

Validate a Space Packet for CCSDS conformance — checks version, field ranges, packet length consistency, and optionally verifies the CRC-16-CCITT error control field.

```
astro spp validate [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--crc` | `false` | Verify CRC-16-CCITT error control field |

**Examples**

```bash
# Validate a packet
echo "0064c000000361626364" | astro spp validate --input hex

# Validate with CRC verification
astro spp encode --apid 100 --type tm --data a1b2c3d4 --crc | astro spp validate --input hex --crc
```

---

## astro spp stream

Decode a stream of concatenated Space Packets, printing each packet as it is parsed. Useful for processing capture files containing multiple back-to-back packets.

```
astro spp stream [file] [flags]
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
astro spp stream --input bin capture.bin

# Stream decode with JSON output for jq processing
astro spp stream --input bin capture.bin --format json | jq '.apid'

# Concatenate multiple encoded packets and stream decode
P1=$(astro spp encode --apid 100 --type tm --data aabb)
P2=$(astro spp encode --apid 200 --type tc --data ccdd)
echo "${P1}${P2}" | astro spp stream --input hex
```

---

## Piping

All commands support stdin/stdout piping for composability:

```bash
# Encode → Inspect
astro spp encode --apid 255 --type tc --data 0102030405 | astro spp inspect --input hex

# Encode with CRC → Validate CRC
astro spp encode --apid 100 --type tm --data a1b2c3d4 --crc | astro spp validate --input hex --crc

# Encode → Decode as JSON
astro spp encode --apid 42 --type tm --data 0102030405 | astro spp decode --input hex --format json
```
