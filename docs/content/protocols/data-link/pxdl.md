---
title: Proximity-1 Data Link Layer
description: CCSDS 211.0-B-6 — the short-range link between an orbiter and a lander or rover.
order: 24
---

> **CCSDS 211.0-B-6** · [Blue Book](https://public.ccsds.org/Pubs/211x0b6e1.pdf) · [`pkg/pxdl`](https://github.com/ravisuhag/astro/tree/main/pkg/pxdl)

## Overview

Proximity-1 is the short-range link protocol: orbiter to lander, orbiter to
rover, spacecraft to spacecraft. It is what the Mars relay network runs on —
a rover talks to an orbiter overhead, and the orbiter relays to Earth on a
different link entirely.

The short range shapes everything about it. Compare it to the long-haul
protocols this library also ships:

| | TM / TC / AOS / USLP | Proximity-1 |
|---|---|---|
| Range | Planet to Earth | A few thousand km |
| Frame size | Up to 65535 octets | Up to 2048 |
| Header | 5 to 14 octets | 5 octets, fixed |
| Error control | Frame Error Control Field | None — the coding layer handles it |
| Frame types | Separate protocols per direction | One frame, both directions |

That last row is the interesting one. Proximity-1 uses a single frame type for
both user data and the protocol's own control traffic, told apart by one bit.

```
U-frame:  header │ user data (packets, segments, or raw)
P-frame:  header │ supervisory PDUs (link control words, directives)
```

### Where it fits

```
┌─────────────────────────────────────────────┐
│  Packets (pkg/spp, pkg/epp) or raw data     │
├─────────────────────────────────────────────┤
│  Proximity-1 Data Link (pkg/pxdl)           │  ← this package
├─────────────────────────────────────────────┤
│  Proximity-1 Coding and Sync (pkg/pxsc)     │
└─────────────────────────────────────────────┘
```

## Scope

**Implemented.** The transfer frame, both qualities of service, packet
segmentation and reassembly, and supervisory PDUs including the Proximity Link
Control Word.

**Not here yet.** COP-P, the retransmission procedure — sequence numbers are
carried, but the state machine that acts on them is a follow-up. The contents of
directives and status reports from annex B: variable-length SPDUs encode and
decode, and this package moves the payload without reading it. The MAC and PHY
sublayers, and session establishment. A CLI, once the API settles.

**Somewhere else.** Coding and synchronization are
[`pkg/pxsc`](/protocols/coding/pxsc).

## Field map: the Transfer Frame

Five octets of header, then up to 2043 octets of data. Ten fields, packed
tight (§3.2.2, figure 3-3):

```
Octet 0:  version(2) │ QoS(1) │ PDU type(1) │ DFC ID(2) │ SCID[9:8](2)
Octet 1:  SCID[7:0](8)
Octet 2:  PCID(1) │ port ID(3) │ src/dest(1) │ length[10:8](3)
Octet 3:  length[7:0](8)
Octet 4:  frame sequence number(8)
```

Two details worth pinning down.

**The version field is binary `10`**, not `11` or `3`. It identifies a
"Version-3" frame, which is confusing but is what §3.2.2.2.2 says.

**The frame length field holds one less than the total.** §3.2.2.10.2:
`C = total octets − 1`, measured from the first octet of the header to the last
octet of the data field. An 11-bit field therefore tops out at a 2048-octet
frame.

### Quality of Service

One bit, two services (§3.2.2.3):

- **Sequence controlled** (`0`) — COP-P checks the frame sequence number.
  Lost frames are retransmitted.
- **Expedited** (`1`) — the sequence check is bypassed. Supervisory PDUs travel
  only here.

### Source or destination

A single SCID field serves both directions, and one bit says which end it
names (§3.2.2.9). The polarity comes from table 3-2: `0` means the SCID is the
**source** spacecraft (the sender's own ID), `1` means it is the
**destination**. Set `WithSourceSCID()` when the SCID is yours; leave it
alone when it is the far end's.

## Sending user data

```go
import "github.com/ravisuhag/astro/pkg/pxdl"

frame, err := pxdl.NewTransferFrame(scid, portID, payload,
    pxdl.WithQoS(pxdl.SequenceControlled),
    pxdl.WithDFCID(pxdl.DFCPackets),
    pxdl.WithSequenceNumber(n))
if err != nil {
    return err
}

raw, err := frame.Encode()
```

The **Data Field Construction ID** says how the data field is arranged
(§3.2.2.5, table 3-1):

| DFC ID | Content |
|---|---|
| `00` | An integer number of unsegmented packets |
| `01` | One segment of a packet, behind a segment header |
| `10` | Reserved |
| `11` | User defined |

The reserved value `10` is rejected on encode and validate, so it cannot reach
the wire.

## Segmentation

A packet too big for one frame gets cut up. Each piece rides behind a one-octet
segment header (§3.2.3.3):

```
bits 0-1: sequence flags
bits 2-7: pseudo packet identifier
```

The sequence flag values are not what you would guess (table 3-4):

| Flags | Meaning |
|---|---|
| `01` | First segment |
| `00` | Continuing segment |
| `10` | Last segment |
| `11` | No segmentation — the whole packet |

Note `01` is *first* and `00` is *continuing*. Getting those backwards produces
a reassembler that never starts.

```go
segments, err := pxdl.Segmentize(packet, pseudoPacketID, maxSegmentData)
for _, seg := range segments {
    body, _ := seg.Encode()
    frame, _ := pxdl.NewTransferFrame(scid, portID, body,
        pxdl.WithDFCID(pxdl.DFCSegment))
    transmit(frame)
}
```

### Reassembly

Segments of one packet must travel with the same PCID and Port ID. Segments of
*different* packets may interleave, as long as they differ in one of those
(§3.2.3.3.2 c). So the reassembler keys on a **routing ID**: PCID, Port ID, and
pseudo packet ID together.

```go
r := pxdl.NewReassembler()

for frame := range incoming {
    packet, err := r.AcceptFrame(frame)
    if err != nil {
        log.Printf("pxdl: %v", err)
        continue
    }
    if packet != nil {
        deliver(packet) // a complete packet, never a partial one
    }
}
```

§3.2.3.3.4 is strict: **only complete packets are delivered**. A stream that
starts mid-packet is rejected rather than guessed at, per §3.2.3.3.5 b).

Set `MaxPacketSize` to bound an accumulating packet. The default is 64 KiB. The
standard sets no ceiling, but a run of "continuing" segments that never ends
would otherwise grow without limit.

## Supervisory PDUs

P-frames carry the protocol talking to itself. Two shapes, told apart by the
leading bit (§3.2.4.2):

```
fixed:     format '1' │ type(1) │ data(14 bits)      — 2 octets
variable:  format '0' │ type(3) │ length(4) │ data   — 1 to 16 octets
```

SPDUs are self-identifying and self-delimiting, so a decoder walks a run of
them without being told how many there are.

One quirk: **the variable-length SPDU's length field is the actual count, not
a count-less-one.** §3.2.4.2.2 calls this out explicitly, presumably because
everything else in CCSDS goes the other way.

### The Proximity Link Control Word

The one fixed-length SPDU defined so far. It is Proximity-1's acknowledgement —
the same job COP-1's CLCW does for TC links (§3.2.4.3.2):

```go
plcw := &pxdl.PLCW{
    ReportValue:           expectedNext, // V(R)
    RetransmitFlag:        missingFrames,
    PCID:                  0,
    ExpeditedFrameCounter: count,
}

body, _ := pxdl.EncodeSPDUs([]pxdl.SPDU{{PLCW: plcw}})
frame, _ := pxdl.NewSupervisoryFrame(scid, 0, body)
```

`NewSupervisoryFrame` applies three rules the protocol fixes, so you cannot
build an invalid frame by accident: SPDUs travel only on the Expedited service
(§3.2.4.1), a P-frame's DFC ID is zero (§3.2.2.5.2), and a P-frame's Port ID is
zero (§3.2.2.8.2).

The first two are set for you whatever you pass. The port is different: it is
an argument, so a non-zero value is refused with `ErrPortIDOnSupervisoryFrame`
rather than quietly zeroed. A port names the output the I/O Sublayer delivers a
U-frame's data to (§3.2.2.8.3), and a P-frame reaches no port at all, so a port
here means you wanted `NewTransferFrame`. The same check runs on `Encode`,
because `PDUType` and `PortID` are exported and can be set past the
constructor.

## Reference

- [CCSDS 211.0-B-6](https://public.ccsds.org/Pubs/211x0b6e1.pdf) — Proximity-1 Space Link Protocol, Data Link Layer
- [CCSDS 211.2-B-3](https://public.ccsds.org/Pubs/211x2b3e1.pdf) — Coding and Synchronization Layer
- [Conformance](/conformance/pxdl)
