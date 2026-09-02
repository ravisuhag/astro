---
title: Build an uplink
short: Uplink
description: Commands from the ground to a spacecraft, with reliable delivery.
order: 2
---

Sending a command is harder than sending telemetry, because it has to arrive. This walks the telecommand chain: packets into TC frames, frames tracked by COP-1's sliding window, frames wrapped as CLTUs, and a CLCW coming back to say what landed.

The complete program is [`examples/uplink`](https://github.com/ravisuhag/astro/tree/main/examples/uplink). Run it:

```bash
go run ./examples/uplink/
```

## What we are building

```
Ground                                          Spacecraft
──────                                          ──────────
critical ──► VC0 ──┐                       ┌──► VC0 ──► critical
  APID 100         │                       │
                   ├─► FOP-1 ─► CLTU ──────┤    FARM-1
routine  ──► VC1 ──┘                       │
  APID 200                                 └──► VC1 ──► routine

        FOP-1 ◄──────── CLCW ◄──────────────── FARM-1
                    rides home on TM
```

Two virtual channels, both using Type-A frames, sequence controlled, so anything lost gets sent again.

## Ground side setup

Unlike [the downlink](/docs/guides/downlink), each virtual channel gets **its own frame counter**:

```go
gsCounterCrit := tcdl.NewFrameCounter()
gsCounterRout := tcdl.NewFrameCounter()

gsVcCrit := tcdl.NewVirtualChannel(vcidCritical, 32)
gsVcRout := tcdl.NewVirtualChannel(vcidRoutine, 32)
```

That is not an inconsistency. TC has no master channel frame count, reliability is per virtual channel, handled by COP-1, so each VC sequences independently.

Then a MAP Packet service per channel:

```go
// bypass=false means Type-A: sequence controlled, reliable.
gsMapCrit := tcdl.NewMAPPacketService(
    spacecraftID, vcidCritical, mapID, false, gsVcCrit, gsCounterCrit)
gsMapCrit.SetPacketSizer(spp.PacketSizer)
```

And a FOP-1 instance per channel, which is the ground half of COP-1:

```go
fopCrit := cop.NewFOP(spacecraftID, vcidCritical, copWindow)
fopCrit.Initialize(0)
```

`copWindow` is how many frames may be unacknowledged at once. Ten is a reasonable start.

## Sending a command

Build a telecommand packet, note `NewTCPacket`, not `NewTMPacket`:

```go
payload := append([]byte{cmd.Opcode}, cmd.Payload...)

pkt, err := spp.NewTCPacket(apidCritical, payload,
    spp.WithSequenceCount(uint16(i)),
    spp.WithErrorControl(),
)
encoded, err := pkt.Encode()

if err := gsMapCrit.Send(encoded); err != nil {
    log.Fatal(err)
}
```

## Registering with FOP-1, then transmitting

This is the step the downlink has no equivalent for. Pull each frame out of the virtual channel, hand it to FOP-1 so it gets a sequence number and goes into the retransmission window, *then* wrap it:

```go
for gsVcCrit.Len() > 0 {
    frame, err := gsVcCrit.Next()
    encoded, err := frame.Encode()

    // FOP-1 assigns N(S) and remembers the frame until it is acknowledged.
    if err := fopCrit.TransmitFrame(encoded); err != nil {
        log.Fatal(err)
    }

    // CLTU: BCH(63,56) codeblocks, start and tail sequences, randomized.
    cltu, err := tcsc.WrapCLTU(encoded, nil, nil, true)
    uplink.transmit(cltu)
}
```

Passing `nil` for the start and tail sequences uses the standard ones. The `true` turns on randomization, and note that the [TC randomizer is not the TM randomizer](/protocols/coding/tcsc#gotchas).

`TransmitFrame` returns `ErrFOPWindowFull` when too many frames are outstanding. That is the window doing its job: stop sending until something is acknowledged.

## Spacecraft side

Unwrap the CLTU, decode the frame, and let FARM-1 decide:

```go
frameBytes, corrected, err := tcsc.UnwrapCLTU(cltu, nil, nil, true)
```

FARM-1 compares the frame's N(S) against the V(R) it expects. In sequence, accept and increment. Ahead, discard and ask for a resend. Behind but within the window, a duplicate, discard silently. Way off, lockout.

Whatever it decides, it reports through a CLCW.

## The CLCW coming home

```
  CLCW received:   CLCW Version: 0
  COP in Effect: 1
  VCID: 00
  No RF Available: No
  No Bit Lock: No
  Lockout: No
  Wait: No
  Retransmit: No
  FARM-B Counter: 0
  Report Value V(R): 003 (4 bytes)
```

`Report Value V(R): 003` means "I have everything below frame 3". Three frames acknowledged in one byte. The ground feeds this back into FOP-1, which drops the acknowledged frames from its window.

```
FOP-1 after CLCW processing:
  VC0: 0 frames still pending
  VC1: 0 frames still pending
```

## Running it

```
=== Simulation Summary ===
  Commands sent:      3 critical + 2 routine = 5 total
  CLTUs transmitted:  5
  Frames accepted:    5
  Commands recovered: 3 critical + 2 routine = 5 total
  All frames acknowledged by CLCW: true
```

## Things that will bite you

**The T1 timer does not run itself.** This example completes without loss, so nothing times out. In a real system you must call `Tick` on your own schedule, or a single lost CLCW stalls FOP-1 forever. That is the exact failure T1 exists to catch.

**Set V(R) will not clear a lockout.** Only Unlock does. If the spacecraft reports `Lockout: Yes`, send an Unlock first, read V(R) from the next CLCW, and set V(S) to match. Doing it in the other order looks like it should work and does not.

**A CLCW rides home on the downlink.** It goes in the Operational Control Field of a [TM frame](/protocols/data-link/tmdl), which means your downlink config needs `HasOCF: true`. The downlink example turns it off, so the two examples are not directly composable. [The full-duplex guide](/docs/guides/full-duplex) joins them.

**Type-B frames skip everything.** `bypass=true` gets a command through even when COP-1 is wedged. That is what it is for. You lose the ordering guarantee, so use it for emergencies and recovery, not routine traffic.

## Next

- [A full-duplex link](/docs/guides/full-duplex), this chain joined to a downlink, so the CLCW actually comes back
- [Handle a lossy link](/docs/guides/lossy-link), the downlink under real loss
- [Encrypt and authenticate a link](/docs/guides/secure-a-link), because a forged command is the real threat here
- [COP-1 protocol page](/protocols/data-link/cop), both state machines in full
- [TC](/protocols/data-link/tcdl) | [TCSC](/protocols/coding/tcsc)
