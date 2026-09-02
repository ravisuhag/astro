---
title: Guides
description: Worked chains, each backed by a runnable program in the repository.
order: 0
---

Every page here builds something end to end and shows the real output. The complete program for each one lives in [`examples/`](https://github.com/ravisuhag/astro/tree/main/examples), so you can run it, break it, and see what changes.

Read [the stack](/docs/start/concepts) first if you have not. Every guide assumes it.

## The core chain

Start here. These four cover the layers every mission uses, and the rest build on them.

| Guide | What it builds |
|---|---|
| [Downlink](/docs/guides/downlink) | Telemetry from a spacecraft to the ground: packets, frames, CADUs, and back |
| [Uplink](/docs/guides/uplink) | Commands to a spacecraft, with COP-1 making sure they arrive |
| [A lossy link](/docs/guides/lossy-link) | What happens when frames are dropped and corrupted, and how four separate mechanisms cope |
| [A full-duplex link](/docs/guides/full-duplex) | The two directions joined, with the CLCW riding home in the OCF |

## Above the frames

What the bytes mean, and moving things bigger than a packet.

| Guide | What it builds |
|---|---|
| [PUS services](/docs/guides/pus-services) | Five ECSS services working together: verification, housekeeping, events, scheduling, monitoring |
| [A mission database](/docs/guides/xtce-database) | Decoding from an XTCE file instead of hand-written structs, with calibration |
| [File downlink](/docs/guides/file-downlink) | Moving a whole file with CFDP, noticing the hole in it, and filling it |
| [Time correlation](/docs/guides/time-correlation) | Turning a drifting on-board counter into a real timestamp |

## Beside the frames

Alternatives and additions at the data link layer.

| Guide | What it builds |
|---|---|
| [AOS high-rate downlink](/docs/guides/aos-high-rate) | Packets, a raw bitstream and opaque blocks on three virtual channels |
| [Secure a link](/docs/guides/secure-a-link) | AES-GCM on the downlink, AES-CMAC on the uplink, and three attacks failing |
| [Compression](/docs/guides/compression) | Rice coding for science data, POCKET+ for housekeeping |
| [DTN](/docs/guides/dtn-deep-space) | Store and forward, for when the link is not up |

## Getting data around, and getting it wrong

| Guide | What it builds |
|---|---|
| [Ground station](/docs/guides/sle-ground-station) | An SLE session pulling frames from an antenna to a control centre |
| [Compose a link](/docs/guides/composed-downlink) | One configuration value building both ends, so they cannot drift apart |
| [Debug a capture](/docs/guides/debug-a-capture) | A binary file, no documentation, and a question about what is in it |

## One thing every guide repeats

Several protocols here hand you a state machine and no clock: [COP-1](/docs/guides/uplink), [CFDP](/docs/guides/file-downlink), [LTP](/docs/guides/dtn-deep-space) and [SLE](/docs/guides/sle-ground-station) all leave the timers to you, deliberately, because on a link with an hour-long round trip only the mission knows what a sensible timeout is.

All four of them also deadlock silently if you never run those timers. If you read only one warning from these pages, read that one.
