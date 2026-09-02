---
title: Quickstart — Go
short: Go
description: Build a Space Packet, frame it, and wrap it for transmission.
order: 2
---

Five minutes, from a temperature reading to something you could put on a radio. Every snippet here is real output from running the code.

## Make a packet

A [Space Packet](/protocols/transport/spp) is the unit an application sends. It needs an APID — the number naming which application produced it — and some data.

```go
package main

import (
    "fmt"
    "log"

    "github.com/ravisuhag/astro/pkg/spp"
)

func main() {
    pkt, err := spp.NewTMPacket(100, []byte("temp=22.5"))
    if err != nil {
        log.Fatal(err)
    }

    raw, err := pkt.Encode()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("% X\n", raw)
}
```

```
00 64 C0 00 00 08 74 65 6D 70 3D 32 32 2E 35
```

Fifteen bytes: six of header, nine of payload. `NewTMPacket` makes telemetry; `NewTCPacket` makes a telecommand.

## Read it back

```go
back, err := spp.Decode(raw)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("apid=%d seq=%d data=%q\n",
    back.PrimaryHeader.APID,
    back.PrimaryHeader.SequenceCount,
    back.UserData)
```

```
apid=100 seq=0 data="temp=22.5"
```

Every packet type has a `Humanize()` for when you want to see all of it:

```
SpacePacket Information:
Primary Header:
  Version: 0
  Type: 0
  Secondary Header Flag: 0
  APID: 100
  Sequence Flags: 3
  Sequence Count: 0
  Packet Length: 8
```

Note `Packet Length: 8` for nine bytes of data. That field is [length minus one](/protocols/transport/spp#gotchas) on the wire, and Astro shows you what is actually there.

## Frame it and wrap it

A packet does not go on a radio by itself. It goes inside a [TM transfer frame](/protocols/data-link/tmdl), which then gets [error coding and a sync marker](/protocols/coding/tmsc).

```go
import (
    "github.com/ravisuhag/astro/pkg/tmdl"
    "github.com/ravisuhag/astro/pkg/tmsc"
)

// Spacecraft 42, virtual channel 0.
frame, err := tmdl.NewTMTransferFrame(42, 0, raw, nil, nil)
if err != nil {
    log.Fatal(err)
}
fb, err := frame.Encode()
if err != nil {
    log.Fatal(err)
}

// Attach the sync marker and randomize.
cadu := tmsc.WrapCADU(fb, tmsc.DefaultASM(), true)
fmt.Printf("%d bytes, starts % X\n", len(cadu), cadu[:8])
```

```
27 bytes, starts 1A CF FC 1D FD E8 0E C0
```

`1A CF FC 1D` is the Attached Sync Marker — the pattern a receiver looks for to find where a frame begins. Everything after it is randomized, which is why the frame header does not look like `2A 00` any more.

## Adding a CRC

Real telemetry usually carries error control. It is an option, not a default:

```go
pkt, err := spp.NewTMPacket(100, []byte("temp=22.5"),
    spp.WithErrorControl(),
)
```

The receiver has to be told to expect it:

```go
back, err := spp.Decode(raw, spp.WithDecodeErrorControl())
```

If the two disagree, the receiver reads the last two bytes as payload. That mismatch is a common first bug.

## What to read next

This built one packet by hand. Real systems use the service layer, which handles sequence counting, packing packets into frames, and filling the leftover space correctly.

- [Downlink guide](/docs/guides/downlink) — the same chain with services, two virtual channels, and a ground-side receiver
- [SPP protocol page](/protocols/transport/spp) — the field map and the rules that bite
- [SPP Go API](/protocols/transport/spp) — the service layer, secondary headers, options
