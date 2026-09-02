---
title: Build a downlink
short: Downlink
description: Telemetry from a spacecraft to the ground — packets, frames, CADUs, and back.
order: 1
---

This walks the whole telemetry chain: application data becomes Space Packets, packets get multiplexed into TM frames on two virtual channels, frames become CADUs, and the ground station takes it all apart again.

The complete program is [`examples/downlink`](https://github.com/ravisuhag/astro/tree/main/examples/downlink). Run it:

```bash
go run ./examples/downlink/
```

## What we are building

```
Spacecraft                                    Ground
──────────                                    ──────
housekeeping ──┐                          ┌── housekeeping
   APID 100    │                          │      APID 100
               ├─► VC0 ─┐        ┌─► VC0 ─┤
                        │        │
science ───────┐        ├► CADU ─┤        ┌── science
   APID 200    ├─► VC1 ─┘        └─► VC1 ─┤      APID 200
               │                          │
```

Two applications, two virtual channels, one physical link. Housekeeping gets the higher priority.

## One config, both ends

The frame length and what the frame carries are fixed for the whole physical channel. Both ends must agree, and nothing on the wire tells them — so this struct is shared:

```go
config := tmdl.ChannelConfig{
    FrameLength: 256,
    HasOCF:      false,
    HasFEC:      true,
}
```

`HasFEC: true` puts a 2-byte CRC at the end of every frame. `HasOCF: false` means no [CLCW](/protocols/data-link/cop) riding home — this is a telemetry-only example.

## The spacecraft side

Build the channel hierarchy from the bottom up. A physical channel holds a master channel, which holds virtual channels.

```go
scPhysical := tmdl.NewPhysicalChannel("SC-downlink", config)

scMaster := tmdl.NewMasterChannel(spacecraftID, config)
scPhysical.AddMasterChannel(scMaster, 1)

vcHK  := tmdl.NewVirtualChannel(vcidHK, 32)   // 32-frame buffer
vcSci := tmdl.NewVirtualChannel(vcidScience, 32)

scMaster.AddVirtualChannel(vcHK, 3)   // priority 3 — housekeeping wins
scMaster.AddVirtualChannel(vcSci, 1)  // priority 1 — science yields
```

Then one **frame counter shared by everything**, and a VCP service per virtual channel:

```go
counter := tmdl.NewFrameCounter()

vpcHK := tmdl.NewVirtualChannelPacketService(
    spacecraftID, vcidHK, vcHK, config, counter)
vpcHK.SetPacketSizer(spp.PacketSizer)
```

Two things matter here.

**The counter is shared.** The Master Channel Frame Count has to increment across every virtual channel, so all services take the same `FrameCounter`. Give each its own and the ground will report gaps that never happened.

**`SetPacketSizer` is required.** The receiver has to know how long the packet at the First Header Pointer claims to be. Without it you get `ErrNoPacketSizer`. `spp.PacketSizer` reads a Space Packet's length field.

## Sending

Make a packet, encode it, hand the bytes to the service:

```go
pkt, err := spp.NewTMPacket(apidHK, sample.encode(),
    spp.WithSequenceCount(uint16(i)),
    spp.WithErrorControl(),
)
encoded, err := pkt.Encode()

if err := vpcHK.Send(encoded); err != nil {
    log.Fatal(err)
}
```

`Send` does not necessarily transmit. It packs the packet into the current frame and releases the frame when it fills. Several small packets share one frame; a large one spans several.

**So you must flush.** Whatever is left sitting in a partly full frame goes nowhere until you say so:

```go
if err := vpcHK.Flush(); err != nil {
    log.Fatal(err)
}
```

`Flush` completes the leftover space with a real [idle packet](/protocols/transport/spp) at APID `0x7FF`, not raw padding. A conformant receiver would read raw fill as a packet header.

## Off the spacecraft

Frames become CADUs — sync marker plus randomization:

```go
cadu := tmsc.WrapCADU(frameBytes, tmsc.DefaultASM(), true)
link.transmit(cadu)
```

## The ground side

The same structure in reverse, built from the same `config`. Unwrap, demultiplex by VCID, then pull packets out of each virtual channel.

## Running it

```
--- Spacecraft Side ---

Generating 3 housekeeping packets (APID 100, VC0)...
Generating science packets (APID 200, VC1)...
  Packet 0: 400 bytes payload (408 bytes on wire)
  Packet 1: 400 bytes payload (408 bytes on wire)

Transmitted 5 CADUs over RF link (260 bytes each)

--- Ground Station Side ---

Received and demultiplexed 5 frames

Extracting housekeeping packets from VC0:
  HK Packet (APID=100, Seq=0): Battery=28.1V, Temp=22.5°C, CPU=35%, Mem=60%, Mode=1
  HK Packet (APID=100, Seq=1): Battery=28.0V, Temp=22.7°C, CPU=42%, Mem=61%, Mode=1
  HK Packet (APID=100, Seq=2): Battery=27.9V, Temp=23.0°C, CPU=38%, Mem=60%, Mode=2
  Total: 3 housekeeping packets recovered

Extracting science packets from VC1:
  Science Packet (APID=200, Seq=0): 100 float32 samples, 400 bytes
  Science Packet (APID=200, Seq=1): 100 float32 samples, 400 bytes
  Total: 2 science packets recovered
```

Five packets in, five out. Note the arithmetic: two 408-byte science packets do not fit in one 256-byte frame, so they span. The First Header Pointer is what lets the ground find the boundaries again.

## Things that will bite you

**The config must be identical on both ends.** Frame length, OCF, and FEC are not signaled. A mismatch does not produce a clear error — it produces garbage that sometimes passes CRC.

**Forgetting `Flush` loses your last packets.** They sit in a partial frame forever. This is the single most common mistake with the service layer.

**One `FrameCounter` per master channel, not per service.** See above.

**A packet larger than the data field is fine.** It spans frames. A packet larger than 65,542 bytes is not — that is the Space Packet ceiling, and you need to segment above this layer.

## Next

- [Handle a lossy link](/docs/guides/lossy-link) — what happens when frames get dropped
- [Build an uplink](/docs/guides/uplink) — the other direction, with retransmission
- [TM protocol page](/protocols/data-link/tmdl) | [TMSC](/protocols/coding/tmsc) | [SPP](/protocols/transport/spp)
