---
title: astro sle
description: Space Link Extension — decode transfer service PDUs.
order: 56
---

Decode SLE transfer service PDUs (CCSDS [911.1](https://public.ccsds.org/Pubs/911x1b4.pdf), [911.2](https://public.ccsds.org/Pubs/911x2b3.pdf), [911.5](https://public.ccsds.org/Pubs/911x5b3.pdf), [912.1](https://public.ccsds.org/Pubs/912x1b4.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro sle decode` | Decode a PDU envelope: which service and operation, and the content |

## Why --service is required

An SLE PDU's wire tag means different things in different services. The same number is one operation in RAF and another in FCLTU, and nothing in the octets says which service they came from — the association does, and that is out of band.

So `--service` is required, and it is not a default worth guessing: guessing wrong names the wrong operation and reads the content against the wrong decoder.

---

## astro sle decode

Decode the outer envelope: which service and operation the PDU is, its wire tag, and the encoded content.

The content is reported as octets. Decoding it further needs the operation's own decoder, and which one applies depends on the service.

```
astro sle decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--service` | *(required)* | Transfer service: `raf`, `rcf`, `rocf`, or `fcltu` |
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
astro sle decode --service raf --input hex < pdu.hex
astro sle decode --service fcltu --input hex < pdu.hex
```

## Limits

The per-service GET-PARAMETER parameter sets are carried as raw BER rather than typed values, and the provider role is a test double rather than a production implementation. See the [conformance statement](/conformance/sle).
