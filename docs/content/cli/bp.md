---
title: astro bp
short: BP
description: Bundle Protocol v7, decode bundles and status reports.
order: 210
---

Decode Bundle Protocol bundles and administrative records ([RFC 9171](https://www.rfc-editor.org/rfc/rfc9171)).

**This is version 7**, the one live implementations speak. Version 6 ([RFC 5050](https://www.rfc-editor.org/rfc/rfc5050), profiled by CCSDS 734.2-B-1) is a different wire format, not an earlier revision: it encodes with SDNV where version 7 encodes with CBOR. A version 6 bundle will not decode here.

## Subcommands

| Command | Description |
|---|---|
| `astro bp decode` | Decode a bundle: the primary block, then each canonical block |
| `astro bp admin` | Decode an administrative record, which for version 7 means a bundle status report |

---

## astro bp decode

Decode one bundle: the primary block, then each canonical block it carries.

```
astro bp decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|---|---|---|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
astro bp decode --input hex < bundle.hex
```

---

## astro bp admin

Decode an administrative record. Version 7 defines one kind, the bundle status report. This is what a bundle's payload holds when its administrative record flag is set, so feed it the payload rather than the whole bundle.

```
astro bp admin [file] [flags]
```

**Flags**

Same as `decode`.

**Examples**

```bash
astro bp admin --input hex < record.hex
```

---

**See also**: [the protocol page](/protocols/transport/bp) for the standard and the Go API, and the [conformance statement](/conformance/bp) for what is and is not implemented.
