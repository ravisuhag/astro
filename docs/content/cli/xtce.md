---
title: astro xtce
description: XTCE mission databases — validate, list, layout, decode, match.
order: 50
---

XTCE mission database operations — validate a database, see what it defines, and decode packets against it ([XTCE 1.2](https://www.omg.org/spec/XTCE/), [CCSDS 660.1-G-2](https://public.ccsds.org/Pubs/660x1g2.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro xtce validate` | Check a database for the errors the schema cannot catch |
| `astro xtce list` | List the space systems, parameters and containers it defines |
| `astro xtce layout` | Show the fields a container lays out, with bit offsets |
| `astro xtce decode` | Decode a packet against a named container |
| `astro xtce match` | Work out which container a packet is, then decode it |

Names are qualified paths — `/Sat/Telemetry`, not `Telemetry`. `list` prints them in that form so its output can be pasted straight into the other commands.

---

## astro xtce validate

Load a database and run the semantic checks: unresolved references, duplicate names, container inheritance cycles, and enumeration members outside the schema's sets.

This is **not** schema validation. The Go standard library has no XSD validator and this package takes no dependencies, so a file that breaks the schema some other way will load and pass here. For real conformance run `xmllint --schema SpaceSystem.xsd` over the file first.

```
astro xtce validate <file>
```

**Examples**

```bash
astro xtce validate mission.xml
```

---

## astro xtce list

Walk the database and list what it defines, by qualified name. Abstract containers are marked, because you cannot decode against one.

```
astro xtce list <file> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--kind` | `all` | What to list: `all`, `systems`, `parameters`, or `containers` |
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
# Everything
astro xtce list mission.xml

# Just the containers, which is what decode and match need
astro xtce list mission.xml --kind containers
```

---

## astro xtce layout

Flatten a container into the fields a packet of that shape carries, in packet order, with each field's bit offset and width. Inherited fields come first, then the container's own.

This is what `decode` reads a packet against, so it is the thing to look at when a decode does not come out the way the database led you to expect.

```
astro xtce layout <file> <container> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
astro xtce layout mission.xml /Sat/Housekeeping
```

```
Container: Housekeeping
Size: 24 bits (3 octets minimum)
────────────────────────────────────────────────────────────────────────
OFFSET   WIDTH    TYPE         NAME
────────────────────────────────────────────────────────────────────────
0        8        integer      /Sat/Type
8        8        enumerated   /Sat/Mode
16       8        float        /Sat/Battery
```

---

## astro xtce decode

Read a packet against a named container and print each field's value.

Values are the engineering ones by default: calibrated numbers and enumeration labels. `--raw` gives the counts as the packet carried them.

A field that cannot be decoded is reported on stderr and the rest of the packet still comes out, so one unsupported encoding in the middle does not hide everything after it.

```
astro xtce decode <file> [packet-file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--container` | *(required)* | Qualified container name to decode against |
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |
| `--raw` | `false` | Show raw values rather than calibrated ones |

**Examples**

```bash
# Decode a Space Packet header against a database that models it
astro spp encode --apid 100 --type tm --data 01020304 |
  astro xtce decode ccsds-header.xml --container /CCSDS/PrimaryHeader

# The raw counts instead of the labels
astro xtce decode mission.xml packet.bin --input bin \
  --container /Sat/Housekeeping --raw
```

---

## astro xtce match

Search down from a root container for the one whose restriction criteria the packet satisfies, and decode it against that. This is what a ground station does with an unlabelled packet.

The deepest match wins, so a packet that is both a telemetry packet and a housekeeping telemetry packet is reported as the latter. A packet that satisfies nothing is an error — a normal thing for a ground station to see, but you have to be told.

```
astro xtce match <file> [packet-file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--root` | *(required)* | Qualified name of the container to search down from |
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `name` |
| `--raw` | `false` | Show raw values rather than calibrated ones |

`--format name` prints just the container it chose, which is what you want in a pipeline.

**Examples**

```bash
# Identify and decode
astro xtce match mission.xml --root /Sat/Packet < packet.hex

# Just the name
astro xtce match mission.xml --root /Sat/Packet --format name < packet.hex
```

## Limits

`decode` and `match` refuse what the database does not settle ahead of the packet: delimited or dynamically sized fields, dynamic repeat counts, and `referenceLocation="containerEnd"`. A `CustomAlgorithm` in the restriction criteria is refused too, being by definition not in the file. See the [conformance statement](/conformance/xtce) for the row-by-row picture.
