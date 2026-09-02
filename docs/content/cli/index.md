---
title: CLI
description: Shared flags, input and output formats, and how to pipe commands together.
order: 0
---

`astro` encodes, decodes, inspects, and validates CCSDS data from a terminal. It reads stdin and writes stdout, so commands pipe into each other and into `jq`.

If you have not used it yet, start with the [CLI quickstart](/docs/start/quickstart-cli).

## Commands

| Command | Protocol | |
|---|---|---|
| [`astro spp`](/cli/spp) | Space Packets | encode, decode, inspect, validate, stream, gen |
| [`astro epp`](/cli/epp) | Encapsulation Packets | encode, decode, inspect, validate, stream, gen |
| [`astro tm`](/cli/tm) | TM transfer frames | encode, decode, inspect, gaps, demux, gen |
| [`astro tc`](/cli/tc) | TC transfer frames | encode, decode, inspect, gen |
| [`astro aos`](/cli/aos) | AOS transfer frames | encode, decode, inspect, gen |
| [`astro usdl`](/cli/usdl) | USLP transfer frames | encode, decode, inspect, gen |
| [`astro cadu`](/cli/cadu) | Channel Access Data Units | wrap, unwrap, inspect, sync, gen |
| [`astro cltu`](/cli/cltu) | Command Link Transmission Units | wrap, unwrap, inspect, gen |
| [`astro time`](/cli/time) | Time codes | encode, decode, inspect, now |

## Shared flags

Every command that reads input takes:

| Flag | Default | Meaning |
|---|---|---|
| `--input` | `hex` | `hex` for hex-encoded text, `bin` for raw binary |

Every command that writes output takes:

| Flag | Default | Meaning |
|---|---|---|
| `--format` | varies | `text`, `json`, or `hex` |

Input comes from a file argument or from stdin. Hex input tolerates a `0x` prefix, spaces, and newlines, so you can paste from a log without cleaning it up.

## The verbs

They mean the same thing everywhere.

**`encode`** builds a unit from flags. **`decode`** takes one apart into fields. **`inspect`** does the same but prints an annotated hex dump alongside — this is the one for staring at a capture. **`validate`** checks a unit is well formed and reports what is wrong. **`stream`** reads a concatenated run of units from a file or pipe. **`gen`** produces synthetic test data. **`wrap`** and **`unwrap`** add and remove a coding layer.

## Piping

Layers compose the way the protocol stack does:

```bash
# Packet → frame → CADU
PKT=$(astro spp encode --apid 100 --type tm --data 68656c6c6f)
FRAME=$(astro tm encode --scid 42 --vcid 0 --data "$PKT")
echo "$FRAME" | astro cadu wrap --input hex
```

```bash
# Encode, then look at what you built
astro spp encode --apid 100 --type tm --data 68656c6c6f \
  | astro spp inspect --input hex
```

```bash
# Anything decodable can go to jq
astro spp gen --apid 100 --count 100 --format hex \
  | astro spp stream --input hex --format json \
  | jq 'select(.apid == 100) | .sequence_count'
```

## Binary files

Use `--format bin` on the way out and `--input bin` on the way in:

```bash
astro spp gen --apid 100 --count 1000 --format bin > capture.bin
astro spp stream --input bin capture.bin
```

## CRC handling

Where a protocol makes error control optional, both ends must agree. Pass `--crc` when encoding *and* when decoding:

```bash
astro spp encode --apid 100 --type tm --data a1b2c3d4 --crc \
  | astro spp validate --input hex --crc
```

Leave it off the decode side and the CRC bytes come back as payload. That mismatch is the most common first bug.

## Built-in manual

Every command's full reference is embedded in the binary:

```bash
astro manual        # list the topics
astro manual spp    # the full spp reference
```

These are the same pages as this section of the site.
