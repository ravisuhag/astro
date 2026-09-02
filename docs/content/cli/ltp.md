---
title: astro ltp
short: LTP
description: Licklider Transmission Protocol, decode segments.
order: 200
---

Decode LTP segments ([CCSDS 734.1-B-1](https://public.ccsds.org/Pubs/734x1b1.pdf), [RFC 5326](https://www.rfc-editor.org/rfc/rfc5326)).

## Subcommands

| Command | Description |
|---|---|
| `astro ltp decode` | Decode one segment: the header, then its content |

---

## astro ltp decode

Decode one segment. The header says which of the content types follows (data, report, report acknowledgement, or cancel) and the matching content is read. Cancel acknowledgements have no content at all.

```
astro ltp decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|---|---|---|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
astro ltp decode --input hex < segment.hex
```

---

**See also**: [the protocol page](/protocols/transport/ltp) for the standard and the Go API, and the [conformance statement](/conformance/ltp) for what is and is not implemented.
