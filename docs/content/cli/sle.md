---
title: astro sle
short: SLE
description: Space Link Extension — decode transfer service PDUs.
order: 220
---

Decode SLE transfer service PDUs (CCSDS [911.1-B-5](https://public.ccsds.org/Pubs/911x1b5e1.pdf), [911.2-B-4](https://public.ccsds.org/Pubs/911x2b4e1.pdf), [911.5-B-4](https://public.ccsds.org/Pubs/911x5b4e1.pdf), [912.1-B-5](https://public.ccsds.org/Pubs/912x1b5e1.pdf)).

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

A **GET-PARAMETER return** is decoded further: the parameter it carries is named and, where the schema makes its value a single integer, read. That is the one operation whose content this package models, because the per-service parameter sets are the part of SLE most often needed and least often the same between services.

Everything else is reported as octets. Decoding it further needs the operation's own decoder, and which one applies depends on the service.

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

## Why the parameter sets need the service

The same context tag names a different parameter in each service. Tag `[4]` is `requestedFrameQuality` in RAF, `reportingCycle` in RCF, `permittedControlWordTypeSet` in ROCF and `deliveryMode` in FCLTU. And `minReportingCycle`, added in a later issue, took the next free tag in each: `[7]` in RAF and RCF, `[13]` in ROCF, `[19]` in FCLTU.

So decoding against the wrong service would report the wrong parameter with a plausible value. The schema constrains each alternative's `parameterName` to match its tag, and this checks that it does — which is what catches the mistake instead of passing it on.

A value the schema makes structured — a set of GVCIDs, the online/offline choice of a latency limit — comes back as raw BER rather than a guessed-at Go type.

## Limits

The provider runs production and the transfer buffer, and serves several service instances; what it does not hold is a service agreement.

---

**See also** — [the protocol page](/protocols/ground/sle) for the standard and the Go API, and the [conformance statement](/conformance/sle) for what is and is not implemented.
