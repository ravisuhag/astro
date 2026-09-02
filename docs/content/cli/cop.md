---
title: astro cop
description: COP-1 — build and read the Communications Link Control Word.
order: 58
---

Build and read the Communications Link Control Word ([CCSDS 232.1-B-2](https://public.ccsds.org/Pubs/232x1b2ec1.pdf)).

The CLCW is the part of COP-1 that travels on the wire. FARM-1 on the spacecraft generates it, and it comes back in a telemetry frame's Operational Control Field, where it tells FOP-1 on the ground what the receiver has accepted and whether it can take more.

FOP-1 and FARM-1 themselves are state machines driven by a session, not by a single invocation, so there is nothing here for them. Use the library.

## Subcommands

| Command | Description |
|---------|-------------|
| `astro cop clcw-encode` | Build a CLCW from fields |
| `astro cop clcw-decode` | Decode a CLCW |

---

## astro cop clcw-encode

Construct a four-octet CLCW, ready to go in a telemetry frame's OCF.

`COP In Effect` is set to 01 for COP-1 ([table 4-1](https://public.ccsds.org/Pubs/232x1b2ec1.pdf)) rather than offered as a flag, because it is the only procedure this package implements.

```
astro cop clcw-encode [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--vcid` | `0` | Virtual Channel ID this CLCW reports on (0-63) |
| `--report-value` | `0` | Next frame sequence number the receiver expects, N(R) |
| `--wait` | `false` | Set the Wait flag: the receiver has no buffer |
| `--retransmit` | `false` | Set the Retransmit flag |
| `--lockout` | `false` | Set the Lockout flag |
| `--no-rf` | `false` | Set the No RF Available flag |
| `--no-bit-lock` | `false` | Set the No Bit Lock flag |
| `--farm-b-counter` | `0` | FARM-B counter (2 bits) |
| `--format` | `hex` | Output format: `text` or `hex` |

**Examples**

```bash
# A nominal CLCW
astro cop clcw-encode --vcid 0 --report-value 7

# The receiver has no buffer
astro cop clcw-encode --vcid 0 --report-value 7 --wait

# Straight into a frame's OCF
astro tm encode --scid 42 --vcid 0 --data 0102 \
  --ocf "$(astro cop clcw-encode --vcid 0 --report-value 7)"
```

---

## astro cop clcw-decode

Decode a four-octet CLCW and print its fields. This is what you want when a frame's OCF is not saying what you expected: it shows every flag that decides how FOP-1 will react.

```
astro cop clcw-decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
astro cop clcw-decode --input hex < ocf.hex
```

See the [conformance statement](/conformance/cop) for the FOP-1 and FARM-1 state coverage.
