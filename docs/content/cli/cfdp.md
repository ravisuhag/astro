---
title: astro cfdp
description: CCSDS File Delivery Protocol — decode PDUs.
order: 53
---

Decode CFDP Protocol Data Units ([CCSDS 727.0-B-5](https://public.ccsds.org/Pubs/727x0b5.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro cfdp decode` | Decode a PDU: the fixed header, then its directive or file data |

---

## astro cfdp decode

Decode the fixed header, then the file directive or file data the PDU carries.

A file directive names itself in the first octet of its data field ([table 5-4](https://public.ccsds.org/Pubs/727x0b5.pdf)), so the body is decoded properly rather than shown as octets. EOF, Finished, ACK, Metadata, NAK and Prompt are all read down to their fields.

File data is **not** interpreted — it is your file, and the PDU says nothing about its contents — so it comes back as octets with a note saying so.

The CRC is verified when the header says one is present, and a mismatch is an error. §4.1.2 requires the receiver to discard such a PDU, so decoding it into something plausible would be wrong.

```
astro cfdp decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
astro cfdp decode --input hex < pdu.hex
astro cfdp decode --input bin pdu.bin --format json
```

## Limits

Part 2 — proxy and remote operations — is not implemented, so a PDU carrying one decodes as far as its header and reports the directive as not decoded. See the [conformance statement](/conformance/cfdp).
