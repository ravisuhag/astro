---
title: astro pus
short: PUS
description: PUS packet utilisation services — encode, decode, list what is implemented.
order: 160
---

PUS packet utilisation service operations — build and read ECSS PUS-C secondary headers and message bodies ([ECSS-E-ST-70-41C](https://ecss.nl/standard/ecss-e-st-70-41c-space-engineering-telemetry-and-telecommand-packet-utilization-15-april-2016/)).

PUS rides inside a Space Packet's data field, so these commands work on what is left after the Space Packet primary header. Pipe through [`astro spp`](/cli/spp) to get there.

## Subcommands

| Command | Description |
|---------|-------------|
| `astro pus encode` | Build a secondary header with a body |
| `astro pus decode` | Decode a secondary header and, where known, its body |
| `astro pus services` | List the message types this build can decode |

## The profile flags

ECSS-E-ST-70-41C states **no defaults** for the tailorable field widths. A real mission declares them, and two missions can disagree about the width of the same field. So they are flags, and both ends of a round trip have to be given the same ones.

| Flag | Default | Description |
|------|---------|-------------|
| `--tc-spare` | `0` | Octets of spare in the TC secondary header |
| `--tm-spare` | `0` | Octets of spare in the TM secondary header |
| `--time` | `cuc` | TM time format: `cuc`, `cuc-explicit`, `raw`, or `none` |
| `--cuc-coarse` | `4` | Octets of CUC coarse time |
| `--cuc-fine` | `2` | Octets of CUC fine time |
| `--time-raw` | `0` | Octets of an opaque time field, when `--time raw` |

The defaults come from the library's `DefaultProfile`, which is a convenience for tooling, not a standard-mandated default.

Getting them wrong reads the body from the wrong offset. Decoding a report that was written with `--time none` while assuming a CUC field shifts everything by six octets, and the body that comes out is not the body that went in.

---

## astro pus encode

Construct a TM or TC secondary header from fields and append a body, ready to go in a Space Packet's data field.

```
astro pus encode [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--direction` | `tc` | `tm` (report) or `tc` (request) |
| `--service` | *(required)* | Service type (ST) |
| `--subtype` | *(required)* | Message subtype |
| `--data` | | Message body as hex |
| `--source` | `0` | Source ID, for a TC |
| `--dest` | `0` | Destination ID, for a TM |
| `--ack` | `0` | Acknowledgement flags, for a TC (4 bits) |
| `--time-tag` | *(now)* | RFC 3339 time tag for a TM report |
| `--format` | `hex` | Output format: `text` or `hex` |

Plus the profile flags above — both ends of a round trip need the same ones.

A TM report is time tagged. The default is now; `--time-tag` pins it, which is what you want in a test.

**Examples**

```bash
# A TC[3,1] request
astro pus encode --direction tc --service 3 --subtype 1 --data 01020304

# A TM[1,1] verification report with a fixed time
astro pus encode --direction tm --service 1 --subtype 1 \
  --time-tag 2026-09-01T12:00:00Z --data 0064c000
```

---

## astro pus decode

Decode a secondary header, then decode the message body against the service registry when the service is implemented.

A body whose service is **not** implemented is shown as raw octets **and labelled as undecoded**, with the reason. It is never presented as if understood.

```
astro pus decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--direction` | `tm` | `tm` (report) or `tc` (request) |
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |

Plus the profile flags above.

**Examples**

```bash
# Round trip a verification report
astro pus encode --direction tm --service 1 --subtype 1 \
  --time-tag 2026-09-01T12:00:00Z --data 0064c000 |
  astro pus decode --direction tm --input hex
```

```
PUS TM[1,1]
PUS TM Secondary Header
  Version ....... 2
  Time status ... 0
  Message type .. TM[1,1]
  Counter ....... 0
  Destination ... 0
  Time .......... 2026-09-01T12:00:00Z
Body: 4 octets
PUS TM[1,1] verification report
  Request APID .. 100
  Sequence ...... 0
```

---

## astro pus services

List the service and subtype pairs the default registry knows how to decode.

A message type that is not listed still decodes as far as its secondary header; only its body is left as raw octets.

```
astro pus services [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
astro pus services
```

## Limits

The implemented services are ST[01] request verification, ST[03] housekeeping, ST[05] event reporting, ST[08] function management, ST[11] time-based scheduling, ST[12] on-board monitoring and ST[17] test.

Seven ST[12] message types — TC[12,5], TC[12,7], TC[12,23], TM[12,9], TM[12,11], TM[12,12] and TM[12,26] — carry fields whose widths come from your mission's parameter definitions. The CLI has no way to know those, so it reports them as needing a parameter resolver rather than guessing. Decode those from Go, passing `pus.WithParameterResolver`.

---

**See also** — [the protocol page](/protocols/mission/pus) for the standard and the Go API, and the [conformance statement](/conformance/pus) for what is and is not implemented.
