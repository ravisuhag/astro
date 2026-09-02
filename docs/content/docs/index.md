---
title: Introduction
description: Conformance-tested CCSDS and ECSS protocols for building spacecraft and ground software.
order: 0
---

Astro is a Go library and command line tool for the protocols spacecraft use to talk to the ground. It implements the [CCSDS](https://public.ccsds.org) and [ECSS](https://ecss.nl) standards that NASA, ESA, JAXA and others run their missions on.

Every protocol package takes nothing outside the Go standard library, so a
mission integrating one does not inherit a dependency tree. The command line
tool is the exception: it uses a few libraries of its own.

## Start here

**New to Astro**: [Install it](/docs/start/install), then build your first packet in [the Go quickstart](/docs/start/quickstart-go) or [the CLI quickstart](/docs/start/quickstart-cli).

**New to CCSDS**: read [the stack](/docs/start/concepts). It explains how a sensor reading becomes radio symbols, and which package does which part. Every protocol page assumes you have read it.

**Looking for a specific standard**: the [protocol index](/protocols) lists all 22, grouped by layer.

**Lost in the acronyms**: the [glossary](/docs/reference/glossary) expands every one, grouped by layer.

**Debugging a capture**: read [debug a real capture](/docs/guides/debug-a-capture), which works one out end to end from the terminal. The [CLI reference](/cli) has every command.

## Build something

Fifteen worked chains, each backed by a runnable program in the repository. The [guide index](/docs/guides) lists them all; these are the ones to read first:

- [Downlink](/docs/guides/downlink), telemetry from a spacecraft to the ground, packets through frames to CADUs.
- [Uplink](/docs/guides/uplink), commands from the ground to a spacecraft, with reliable delivery.
- [A lossy link](/docs/guides/lossy-link), what happens when frames get dropped and corrupted, and how the protocols cope.
- [A full-duplex link](/docs/guides/full-duplex), both directions joined, with the CLCW riding home.

Then, depending on what you are building:

- [PUS services](/docs/guides/pus-services) and [a mission database](/docs/guides/xtce-database), for what the bytes mean.
- [File downlink](/docs/guides/file-downlink) and [DTN](/docs/guides/dtn-deep-space), for moving things bigger than a packet.
- [AOS](/docs/guides/aos-high-rate), [security](/docs/guides/secure-a-link) and [compression](/docs/guides/compression), for a high-rate or protected link.
- [Ground station](/docs/guides/sle-ground-station) and [time correlation](/docs/guides/time-correlation), for the ground segment.
- [Debug a capture](/docs/guides/debug-a-capture), when someone hands you a binary file and no documentation.

## What is covered

22 protocols across transport, data link, coding and synchronization, ground-to-ground transfer, compression, time, packet utilization, and mission databases. Every one has a scope statement saying what Astro implements and what it leaves to you, plus a conformance page.

Before you trust any of it, read [how this is verified](/docs/reference/verification): it says which claims rest on a published test vector and which rest on a reading of the standard. [Performance](/docs/reference/performance) has the measured throughput of every layer, and [security](/docs/reference/security) covers what happens when the octets are hostile.

See the [protocol index](/protocols) for the full list.

## What Astro is not

It is not a mission control system, and it is not a flight software framework. It is the wire format layer that both of those need. It moves bytes correctly and tells you when they are wrong.
