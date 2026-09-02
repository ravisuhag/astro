---
title: Quickstart, CLI
short: CLI
description: Encode, inspect, and validate packets from a terminal.
order: 3
---

The `astro` command reads and writes CCSDS data on stdin and stdout, so you can pipe it. Everything below is real output.

## Make a packet

```bash
astro spp encode --apid 100 --type tm --data 68656c6c6f
```

```
0064c000000468656c6c6f
```

That is a [Space Packet](/protocols/transport/spp) carrying the five bytes `hello`, from application 100. Hex out by default; `--format bin` gives you raw bytes.

## Look inside one

```bash
astro spp encode --apid 100 --type tm --data 68656c6c6f | astro spp inspect --input hex
```

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
  Packet Data Length ... 4 (total packet: 11 bytes)
────────────────────────────────────────────────────────────
User Data (5 bytes)
  0000  68 65 6c 6c 6f                                    |hello|
────────────────────────────────────────────────────────────
Raw Packet (11 bytes)
  0000  00 64 c0 00 00 04 68 65  6c 6c 6f                 |.d....hello|
```

`inspect` is the one you want when staring at a capture. It shows the raw field value *and* what it means, note Packet Data Length reads 4 for a five-byte payload, because [that field is length minus one](/protocols/transport/spp#gotchas).

## Check a CRC

```bash
astro spp encode --apid 100 --type tm --data a1b2c3d4 --crc | astro spp validate --input hex --crc
```

```
Packet is valid.
  Type: TM, APID: 100, SeqCount: 0, Data: 4 bytes
  CRC: 0x6956 (OK)
```

Both sides need `--crc`. Without it on the decode side, the two CRC bytes come back as payload.

## Get JSON out

Every decode command takes `--format json`, so you can pipe into `jq`:

```bash
astro spp encode --apid 100 --type tm --data 68656c6c6f \
  | astro spp decode --input hex --format json
```

```json
{
  "version": 0,
  "type": 0,
  "type_name": "TM",
  "secondary_header_flag": 0,
  "apid": 100,
  "sequence_flags": 3,
  "sequence_flags_name": "unsegmented",
  "sequence_count": 0,
  "packet_length": 4,
  "user_data": "68656c6c6f",
  "is_idle": false
}
```

## Build a whole chain

Packet into a [TM frame](/protocols/data-link/tmdl), frame into a [CADU](/protocols/coding/tmsc):

```bash
PKT=$(astro spp encode --apid 100 --type tm --data 68656c6c6f)
FRAME=$(astro tm encode --scid 42 --vcid 0 --data "$PKT")
echo "$FRAME" | astro cadu wrap --input hex
```

```
1acffc1d02a0000018000064c000000468656c6c6f7629
```

The `1acffc1d` on the front is the Attached Sync Marker. Unwrap it again with `astro cadu unwrap`.

## Time codes

```bash
astro time now
```

```
Current UTC: 2026-09-01T03:47:32.798716Z
────────────────────────────────────────────────────────────
CUC .... 1e8128a979cc78
CDS .... 4061f800d0533e
CCS .... 5a202602440347327987
ASCII-A  2026-09-01T03:47:32.798Z
ASCII-B  2026-244T03:47:32.798Z
```

Four [time code formats](/protocols/mission/tcf), same instant. `astro time decode` reads any of them back.

## Generate test data

```bash
astro spp gen --apid 100 --count 10 --size 64 --format hex
```

Useful for feeding a receiver you are writing. `astro tm gen` and `astro cadu gen` do the same at their layers.

## Manual pages

Every command's full reference is built into the binary:

```bash
astro manual spp
astro manual        # list the topics
```

## What to read next

- [CLI reference](/cli), shared flags, formats, and piping conventions
- [Downlink guide](/docs/guides/downlink), the same chain in Go, with services
- [The stack](/docs/start/concepts), what each layer is for
