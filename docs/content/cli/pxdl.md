---
title: astro pxdl
short: PXDL
description: Proximity-1 data link — encode, decode, SPDUs.
order: 70
---

Proximity-1 transfer frames and supervisory protocol data units ([CCSDS 211.0-B-6](https://public.ccsds.org/Pubs/211x0b6.pdf)).

Proximity-1 is the short-range link: a lander to an orbiter, rather than a spacecraft to the ground.

## Subcommands

| Command | Description |
|---------|-------------|
| `astro pxdl encode` | Build a transfer frame |
| `astro pxdl decode` | Decode a transfer frame |
| `astro pxdl spdu` | Decode supervisory protocol data units |

---

## astro pxdl encode

```
astro pxdl encode [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--scid` | `0` | Spacecraft ID (10 bits) |
| `--port` | `0` | Port ID (3 bits) |
| `--data` | *(required)* | Frame data as hex |
| `--seq` | `0` | Frame sequence number |
| `--pcid` | `0` | Physical Channel ID (1 bit) |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
astro pxdl encode --scid 42 --port 1 --data 0102030405
```

---

## astro pxdl decode

```
astro pxdl decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `json`, or `hex` |

**Examples**

```bash
# Round trip
astro pxdl encode --scid 42 --port 1 --data 0102 | astro pxdl decode --input hex

# Out of a PLTU
astro pxsc unwrap --input hex < pltu.hex | astro pxdl decode --input hex
```

---

## astro pxdl spdu

Decode the Supervisory Protocol Data Units a supervisory frame carries: the PLCW that reports link status, and the variable-length SPDUs that carry directives.

Several SPDUs can share one frame, so this decodes all of them and reports each.

```
astro pxdl spdu [file] [flags]
```

**Flags**

Same as `decode`.

---

**See also** — [the protocol page](/protocols/data-link/pxdl) for the standard and the Go API, and the [conformance statement](/conformance/pxdl) for what is and is not implemented.
