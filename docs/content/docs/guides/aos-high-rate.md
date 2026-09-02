---
title: A high-rate downlink with AOS
short: AOS downlink
description: Three kinds of data on three virtual channels, plus header protection TM does not have.
order: 9
---

The [downlink guide](/docs/guides/downlink) uses TM frames, which carry packets and nothing else. That is fine for housekeeping. It is awkward for a payload that produces a stream of octets with no packet structure, and it is awkward for anything already framed by the instrument that made it.

[AOS](/protocols/data-link/aos) is the data link protocol for those missions. It carries three different things, adds a synchronous slot in every frame, and protects the frame header.

The complete program is [`examples/aos`](https://github.com/ravisuhag/astro/tree/main/examples/aos). Run it:

```bash
go run ./examples/aos/
```

## What we are building

```
                     one physical channel, 1115-octet frames
                     ┌────────────────────────────────────┐
housekeeping ──► VC0 │ M_PDU: Space Packets               │
                     ├────────────────────────────────────┤
instrument ────► VC1 │ B_PDU: a raw octet bitstream       │
                     ├────────────────────────────────────┤
image blocks ──► VC2 │ VCA:   opaque fixed-size SDUs      │
                     └────────────────────────────────────┘
                       │             │           │
                       Insert Zone   FHEC        FECF
                       every frame   the header  the whole frame
```

## The channel config

```go
var config = aos.ChannelConfig{
    FrameLength:   1115,
    InsertZoneLen: 8,
    HasOCF:        false,
    HasFHEC:       true,
    HasFECF:       true,
}
```

`1115` is a common AOS frame length: it fits a set of RS(255,223) codewords cleanly. Same rule as TM, though: it is fixed for the whole physical channel, nothing on the wire says what it is, and both ends need the same value.

`DataFieldCapacity()` is what is left after the header, insert zone, and trailing fields. Here that is 1097 octets.

## Three services, three shapes of data

**M_PDU** is the packet service, and it works exactly like [TM's](/docs/guides/downlink):

```go
transmit := aos.NewMultiplexingService(spacecraftID, vcidPackets, sendVC, config, counter)
transmit.SetPacketSizer(spp.PacketSizer)

transmit.Send(encoded)
transmit.Flush()
```

Packets go into the packet zone, a First Header Pointer marks where the first one in each frame starts, and `Flush` completes a partial frame with a real idle packet. The `SetPacketSizer` requirement and the flush trap are both the same as TM.

**B_PDU** is the bitstream service, and it is the one TM has no answer for:

```go
transmit := aos.NewBitstreamService(spacecraftID, vcidBitstream, sendVC, config, counter)
transmit.Send(stream) // 2500 octets, no packets in it
```

There are no boundaries in that stream that AOS knows or cares about. A Bitstream Data Pointer marks where real data stops in the last partial frame, and that is the whole protocol.

```
  sent .......... 2500 octets
  frames ........ 3
  recovered ..... 2500 octets, identical true
```

An instrument that produces a continuous digital output can be downlinked without being dressed up as packets first. On a high-rate mission that saves a real amount of work and a real amount of overhead.

**VCA** is the opaque service. One SDU per frame, and the data field has no protocol header at all:

```go
blockSize := config.DataFieldCapacity()
transmit := aos.NewVirtualChannelAccessService(
    spacecraftID, vcidBlocks, blockSize, sendVC, config, counter)
```

The SDU has to fill the data field exactly. A short one is rejected rather than padded, because the receiver has no in-band way to find where it ended. That is a constraint, not a limitation: use VCA when the thing you are sending is already a fixed-size block, such as a [compressed](/docs/guides/compression) image tile.

## The insert zone

This is the field TM has no equivalent for: a fixed slot at a known offset in **every** frame, whether or not the virtual channel had anything to say.

```go
zone := make([]byte, config.InsertZoneLen)
copy(zone, tfield) // a 7-octet CUC time code

frame, err := aos.NewTransferFrame(spacecraftID, vcidPackets, data,
    aos.WithInsertZone(zone),
    aos.WithFHEC(),
    aos.WithFECF(),
    aos.WithVCFrameCount(1))
```

```
  insert zone ... 80 74 4e 3c 40 00 00 00
  reads as ...... 2026-04-17T08:30:15.25Z
```

A [time code](/docs/guides/time-correlation) is the usual passenger, and the reason is the "synchronous" part. The insert zone is sampled at the frame rate, so the stream through it is evenly spaced by construction. Putting a time stamp in a packet gets you a time stamp whenever a packet happened to be sent.

Note that the services fill the insert zone with zeros. If you want something in it, build the frames yourself or write the zone after the service produced them.

## Header error control

TM has no header protection. A corrupted VCID sends a frame to the wrong virtual channel, and the frame CRC tells you something is wrong without telling you it was the header, by which point the frame is already in the wrong queue.

AOS puts Reed-Solomon over the primary header:

```go
header, err := frame.Header.Encode()
fhec, err := aos.ComputeFHEC(header)

aos.VerifyFHEC(header, fhec)
```

```
  header ........ 4a 81 00 0f ff 00
  FHEC .......... 0c 76
  verifies ...... true
  one bit flipped false (VCID 1 became 0)
```

One flipped bit in the VCID octet, caught. Without it that frame goes to VC0 and is decoded as a packet stream.

The FHEC is also why the [SDLS](/docs/guides/secure-a-link) mask for AOS excludes those two octets. They are computed after the security function ran.

## A 24-bit frame count

AOS has **no master channel frame count**. Each virtual channel counts for itself, in 24 bits rather than TM's 8:

```
  frame 100 ..... in sequence
  frame 101 ..... in sequence
  frame 105 ..... 3 frame(s) missing
  frame 106 ..... in sequence

  A 24-bit count wraps every 16777216 frames, not every 256.
```

That width matters at rate. A TM channel at a few thousand frames a second wraps its 8-bit count several times a second, so a gap and a wrap are hard to tell apart. AOS wraps about once every five hours at that rate.

## Things that will bite you

**The frame does not say which PDU type it carries.** M_PDU, B_PDU and VCA all just fill the data field. Which one a virtual channel uses is mission configuration, and a receiver that guesses wrong decodes garbage without erroring. Write it down next to the frame length.

**VCA SDUs must be exactly the data field size.** Not "at most". `ErrSizeMismatch` on a short block is the service refusing to invent a length the receiver cannot recover.

**AOS spacecraft IDs are 8 bits.** TM gives you 10. An ID above 255 that worked on a TM channel will not fit.

**Virtual channel 63 is reserved.** `aos.OIDVCID` is the Only Idle Data channel. Do not configure a real stream on it.

**M_PDU still needs `Flush` and `SetPacketSizer`.** Both traps from the TM guide apply unchanged. Forgetting the flush loses the last packets; forgetting the sizer gets you `ErrNoPacketSizer` on receive.

**The insert zone costs you on every frame.** Eight octets of a 1115-octet frame is nothing. Eight octets of a 128-octet frame is 6% of your downlink, spent whether you filled it or not.

## Next

- [Handle a lossy link](/docs/guides/lossy-link), the coding layer underneath, which is the same for AOS
- [Compress before you downlink](/docs/guides/compression), which is what fills a VCA channel
- [AOS protocol page](/protocols/data-link/aos) | [Conformance](/conformance/aos) | [CLI](/cli/aos)
