---
title: astro bp
short: BP
description: Bundle Protocol v6, decode bundles and administrative records.
order: 210
---

Decode Bundle Protocol bundles and administrative records ([CCSDS 734.2-B-1](https://public.ccsds.org/Pubs/734x2b1.pdf), [RFC 5050](https://www.rfc-editor.org/rfc/rfc5050)).

**This is version 6**, which is what CCSDS profiles. BPv7 ([RFC 9171](https://www.rfc-editor.org/rfc/rfc9171)) encodes bundles in CBOR and is wire-incompatible; it is not implemented, and a BPv7 bundle will not decode here.

## Subcommands

| Command | Description |
|---|---|
| `astro bp decode` | Decode a bundle: the primary block, then each canonical block |
| `astro bp admin` | Decode an administrative record |

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

Decode an administrative record. A status report or a custody signal. This is what a bundle's payload holds when the bundle is an administrative one, so feed it the payload rather than the whole bundle.

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
