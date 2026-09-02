---
title: astro sdls
description: Space Data Link Security — read a Security Header.
order: 62
---

Read the Security Header of a protected frame ([CCSDS 355.0-B-2](https://public.ccsds.org/Pubs/355x0b2.pdf)).

## Only the header, deliberately

Applying or removing security needs the Security Association's keys, and a command line is the wrong place for key material: it lands in shell history and in the process table, where anyone on the machine can read it.

So there is no `apply` or `process` here. Use the library, which takes keys from wherever your mission keeps them.

Reading the header needs no keys at all, and it is usually what you want when a protected frame is not behaving.

## Subcommands

| Command | Description |
|---------|-------------|
| `astro sdls inspect` | Decode a Security Header |

---

## astro sdls inspect

Decode the Security Header at the front of a protected frame's data field: the Security Parameter Index, and whichever of the initialisation vector, sequence number and pad length the Security Association carries.

The field widths are **per Security Association**, not per frame, and nothing in the header states them — so they are flags. Getting them wrong shifts everything after the SPI, which is why the SPI is reported separately: it is the one field whose position is fixed.

The protected data is reported as a length, not decrypted, and a MAC is shown but not verified. Both need keys.

```
astro sdls inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--iv` | `0` | Initialisation vector length in octets |
| `--seq` | `0` | Anti-replay sequence number length in octets |
| `--pad` | `0` | Pad length field width in octets |
| `--mac` | `0` | Message authentication code length in octets |
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
# A confidentiality SA: a 12-octet IV and a 16-octet MAC
astro sdls inspect --input hex --iv 12 --mac 16 < frame-data.hex

# An authentication-only SA: a sequence number, no IV
astro sdls inspect --input hex --seq 4 --mac 16 < frame-data.hex
```

See the [conformance statement](/conformance/sdls).
