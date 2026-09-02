---
title: The stack
short: The stack
description: How a sensor reading becomes radio symbols, and which package does each step.
order: 4
---

A temperature reading on a spacecraft has to become radio symbols, cross millions of kilometres, and turn back into a number on a screen. Several protocols do that job in sequence, each solving one problem and handing the result to the next.

This page is the map. Every protocol page links back here rather than re-explaining it.

## The downlink, top to bottom

```
   22.5 °C                          the reading
       │
       ▼
┌──────────────────┐
│  Space Packet    │   pkg/spp     "this is from application 100"
└──────────────────┘
       │  1 to 65536 bytes, variable
       ▼
┌──────────────────┐
│  TM Frame        │   pkg/tmdl    "this is spacecraft 42, channel 0,
└──────────────────┘                frame number 137"
       │  fixed length, chosen once per mission
       ▼
┌──────────────────┐
│  Reed-Solomon    │   pkg/tmsc    "here is enough parity to fix
│  + randomize     │                16 bad bytes"     (radio; a
│                  │                laser link uses pkg/ocsc)
└──────────────────┘
       │
       ▼
┌──────────────────┐
│  ASM  ->  CADU   │   pkg/tmsc    "a frame starts HERE"
└──────────────────┘
       │
       ▼
    radio symbols                   the physical layer, not Astro's job
```

The ground runs the same thing backwards.

## What each layer is for

**The packet layer** answers *whose data is this?* A Space Packet carries an APID naming the application that produced it, and a counter so you can tell when one goes missing. Packets are whatever size the data is.

**The data link layer** answers *how do I find one thing in a continuous stream?* Frames are a fixed size, so a receiver always knows where the next one starts. Because packets and frames are different sizes, a packet can span several frames. The First Header Pointer in the frame header is how the receiver finds packet boundaries again after a loss.

**The coding layer** answers *what about bit errors?* There is no asking for a resend when the round trip is an hour. Reed-Solomon parity lets the receiver fix errors where they land. Randomization keeps the receiver's clock locked. The Attached Sync Marker is a known pattern that says a frame begins here.

## Which protocol for which job

| | Downlink | Uplink |
|---|---|---|
| Packets | [SPP](/protocols/transport/spp), [EPP](/protocols/transport/epp) | [SPP](/protocols/transport/spp), [EPP](/protocols/transport/epp) |
| Frames | [TM](/protocols/data-link/tmdl), [AOS](/protocols/data-link/aos) | [TC](/protocols/data-link/tcdl) |
| Both directions | [USLP](/protocols/data-link/usdl) | [USLP](/protocols/data-link/usdl) |
| Coding | [TMSC](/protocols/coding/tmsc), [OCSC](/protocols/coding/ocsc) | [TCSC](/protocols/coding/tcsc) |
| Reliability | - | [COP-1](/protocols/data-link/cop) |
| Security | [SDLS](/protocols/data-link/sdls) | [SDLS](/protocols/data-link/sdls) |

**TM or AOS?** TM for a normal downlink. AOS when the data rate is high enough that TM's 8-bit frame counter wraps too fast to be useful, Earth observation, deep space. AOS also carries a raw bitstream or opaque blocks, which TM cannot. Guide: [a high-rate downlink with AOS](/docs/guides/aos-high-rate).

**Why is the uplink different?** Commands must arrive correctly and in order, because a wrong one can end the mission. TC frames are variable length so a ten-byte command costs ten bytes, and [COP-1](/protocols/data-link/cop) sits on top to retransmit anything that goes missing. The downlink has no equivalent: it detects loss but cannot ask again.

That makes the two directions one system rather than two, because COP-1's acknowledgement travels back in the Operational Control Field of a telemetry frame. Guide: [a full-duplex link](/docs/guides/full-duplex).

**Security is not optional in practice.** A downlink is readable by anyone with a dish and an uplink is forgeable by anyone with a transmitter, so [SDLS](/protocols/data-link/sdls) encrypts or authenticates the frame data field. Guide: [encrypt and authenticate a link](/docs/guides/secure-a-link).

**USLP** does both directions with one frame format. It is the newer answer to running three stacks at once.

## Above and beyond the link

Some protocols do not sit in that stack at all.

**Between ground systems.** [SLE](/protocols/ground/sle) moves frames between a ground station and a control centre over the internet. It is the only protocol here that never touches a spacecraft. Guide: [pull frames from a ground station](/docs/guides/sle-ground-station).

**Between spacecraft.** [Proximity-1](/protocols/data-link/pxdl) is the short-range link, a rover to an orbiter overhead, which then relays to Earth on a completely separate downlink. It has its own coding layer, [PXSC](/protocols/coding/pxsc), which wraps frames in PLTUs the way TMSC wraps them in CADUs.

**Over a laser.** [OCSC](/protocols/coding/ocsc) is the coding layer for optical links. It replaces Reed-Solomon with SCPPM and works in bits rather than octets, because a codeblock's length is not a whole number of them.

**Files and networking.** [CFDP](/protocols/transport/cfdp) moves files. [LTP](/protocols/transport/ltp) and [BP](/protocols/transport/bp) do delay-tolerant networking for multi-hop paths. Guides: [downlink a file](/docs/guides/file-downlink) and [store and forward](/docs/guides/dtn-deep-space).

**Inside the packet.** [PUS](/protocols/mission/pus) defines what a telemetry or telecommand packet's payload actually means, housekeeping reports, event notifications, command acknowledgements. [Time codes](/protocols/mission/tcf) are how timestamps are written. [XTCE](/protocols/mission/xtce) is the database that says which bytes are which parameter. Guides: [PUS services](/docs/guides/pus-services), [time correlation](/docs/guides/time-correlation) and [a mission database](/docs/guides/xtce-database).

**Before transmission.** [LDC](/protocols/compression/ldc) and [RHC](/protocols/compression/rhc) compress data losslessly, because downlink is the scarcest thing a mission has. Guide: [compress before you downlink](/docs/guides/compression).

## A worked path

Follow one housekeeping reading end to end:

1. A thermistor reads 22.5 °C. [PUS ST[03]](/protocols/mission/pus) says how that goes into a housekeeping report, and [a time code](/protocols/mission/tcf) stamps it.
2. The report becomes the payload of a [Space Packet](/protocols/transport/spp) on APID 100.
3. The [VCP service](/protocols/data-link/tmdl) packs that packet into a TM frame on virtual channel 0, along with whatever else fits.
4. [TMSC](/protocols/coding/tmsc) adds Reed-Solomon parity, randomizes the bits, and prepends the sync marker. That is a CADU.
5. The radio sends it. Some bits arrive wrong.
6. The ground finds the sync marker, de-randomizes, and lets Reed-Solomon fix the errors.
7. The frame's counters show whether anything was lost. The First Header Pointer finds the packet boundary.
8. The packet's APID says it is housekeeping. [XTCE](/protocols/mission/xtce) says byte 4 is the temperature.
9. Someone sees 22.5 °C.

The [downlink guide](/docs/guides/downlink) builds steps 2 through 7 in real code. Step 1 is [the PUS guide](/docs/guides/pus-services), step 8 is [the mission database guide](/docs/guides/xtce-database), and [debug a real capture](/docs/guides/debug-a-capture) runs steps 6 to 8 backwards from a binary file.
