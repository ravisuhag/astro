---
title: Introduction
description: CCSDS and ECSS space communication standards, implemented in Go.
order: 0
---

Astro is a Go library and command line tool for the protocols spacecraft use to talk to the ground. It implements the [CCSDS](https://public.ccsds.org) and [ECSS](https://ecss.nl) standards that NASA, ESA, JAXA and others run their missions on.

It has no dependencies outside the Go standard library.

## Start here

**New to Astro** — [Install it](/docs/start/install), then build your first packet in [the Go quickstart](/docs/start/quickstart-go) or [the CLI quickstart](/docs/start/quickstart-cli).

**New to CCSDS** — read [the stack](/docs/start/concepts). It explains how a sensor reading becomes radio symbols, and which package does which part. Every protocol page assumes you have read it.

**Looking for a specific standard** — the [protocol index](/protocols) lists all 22, grouped by layer.

**Debugging a capture** — the [CLI reference](/cli) covers encoding, decoding, inspecting, and validating from a terminal.

## Build something

Three worked chains, each backed by a runnable program in the repository:

- [Downlink](/docs/guides/downlink) — telemetry from a spacecraft to the ground, packets through frames to CADUs.
- [Uplink](/docs/guides/uplink) — commands from the ground to a spacecraft, with reliable delivery.
- [A lossy link](/docs/guides/lossy-link) — what happens when frames get dropped and corrupted, and how the protocols cope.
- [Compose a link](/docs/guides/composed-downlink) — one configuration builds both ends of a downlink or an uplink, so they cannot drift apart.

## What is covered

22 protocols across transport, data link, coding and synchronization, ground-to-ground transfer, compression, time, packet utilization, and mission databases. Every one has a scope statement saying what Astro implements and what it leaves to you, plus a conformance page.

See the [protocol index](/protocols) for the full list.

## What Astro is not

It is not a mission control system, and it is not a flight software framework. It is the wire format layer that both of those need. It moves bytes correctly and tells you when they are wrong.
