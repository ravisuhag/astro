---
title: Protocols
description: Every standard Astro implements, grouped by where it sits in the stack.
order: 0
---

27 standards. If you are not sure which one you need, read
[the stack](/docs/start/concepts) first. It explains how a sensor reading
becomes radio symbols and which package does each step.

Every protocol page opens the same way: what Astro implements and what it
leaves to you. From there the shape follows the standard. A wire format gets
a field map from wire fields to Go fields and a list of the rules that bite,
while a coding or compression standard gets a walkthrough of the chain
instead. The Go API is on the same page, and each has a
[conformance statement](/conformance) in its own section.

## By layer

| Layer | What sits there | |
|---|---|---|
| [Packets and transport](/protocols/transport) | The unit an application sends, and moving files and bundles across a network of them | SPP, EPP, CFDP, LTP, BP, BPSec |
| [Space data link](/protocols/data-link) | Transfer frames in both directions, plus reliable delivery and security | TM, TC, AOS, USLP, Proximity-1, COP-1, SDLS |
| [Coding and synchronization](/protocols/coding) | Finding frame boundaries, and surviving bit errors without a resend | TMSC, TCSC, PXSC, OCSC |
| [Ground to ground](/protocols/ground) | Moving frames between a ground station and a control centre | SLE |
| [Compression](/protocols/compression) | Shrinking data losslessly before it goes down the link | LDC, RHC |
| [Time, payload, database](/protocols/mission) | How timestamps are written, what goes inside a packet, what the octets mean | Time codes, PUS, XTCE, ODM, TDM, ADM, CDM |

## Shared internals

Not protocols, but packages you may end up importing.

| Package | What it is |
|---|---|
| `pkg/crc` | CRC checksums used across the standards. CRC-16-CCITT and friends. |
| `pkg/sdnv` | Self-Delimiting Numeric Values, shared by LTP and BP. |
| `pkg/sdl` | Shared data link primitives — channels, multiplexers, gap detection: used by TM, TC, AOS, and USLP. |
| `pkg/stack` | Builds both ends of a downlink or uplink from one configuration, so they cannot drift apart. See [compose a link](/docs/guides/composed-downlink). |

## Out of scope

Astro serves mission management: commanding, telemetry, spacecraft health and
pass operations. Payload data processing is a separate system, so the image and
cube compressors are not on the list — CCSDS 122.0-B-2, 122.1-B-1 and 123.0-B-2.
Astro carries a payload file down and proves it arrived intact. What the pixels
mean afterwards belongs to the science pipeline.

Housekeeping compression is a different matter and is already here. `pkg/rhc`
and `pkg/ldc` both compress spacecraft health data, which is an operations
concern.

For what astro would still take, see [adding a protocol](/docs/contribute/adding-a-protocol).
