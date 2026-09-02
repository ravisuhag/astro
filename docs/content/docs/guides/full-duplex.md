---
title: A full-duplex link
short: Full duplex
description: The downlink and the uplink are one system, joined by four octets.
order: 4
---

The [downlink guide](/docs/guides/downlink) sends telemetry. The [uplink guide](/docs/guides/uplink) sends commands. Neither is a whole link, and the uplink guide says so: the two examples are not composable, because the downlink turns the Operational Control Field off.

That field is where the CLCW travels, and the CLCW is what lets the ground send more than a window's worth of commands. So this guide closes the loop.

The complete program is [`examples/duplex`](https://github.com/ravisuhag/astro/tree/main/examples/duplex). Run it:

```bash
go run ./examples/duplex/
```

## What we are building

```
Ground                                            Spacecraft
──────                                            ──────────
commands ──► FOP-1 ──► CLTU ─────────────────────► FARM-1 ──► commands
                                                      │
                                                      ▼
   FOP-1 ◄──── CLCW ◄──── OCF of a TM frame ◄──── CLCW generated
     │
     └─ the window moves, so more commands can go
```

The third arrow is the whole point. Without it the ground sends its window's worth of commands and then stops, and nothing about the link looks wrong.

## One setting turns it on

```go
var downlinkConfig = tmdl.ChannelConfig{
    FrameLength: 256,
    HasOCF:      true,
    HasFEC:      true,
}
```

`HasOCF: true` reserves four octets in every frame. The [downlink example](/docs/guides/downlink) sets it false, which is why that example and the uplink example cannot be joined.

Note that it costs you those four octets on **every** frame, whether a CLCW needs reporting or not. That is the trade: a fixed 1.6% of a 256-octet downlink, in exchange for reliable commanding.

## The join is an OCF supplier

Here is the line that connects the two directions:

```go
l.downlinkSvc.SetOCFSupplier(func() []byte {
    clcw := l.farm.GenerateCLCW()
    encoded, err := clcw.Encode()
    if err != nil {
        log.Fatalf("encoding the CLCW: %v", err)
    }
    return encoded
})
```

Every frame the downlink service emits calls that function and puts the result in the OCF. So the uplink's receive state machine writes into the downlink's frames, which is exactly the coupling the standards intend and the reason it is easy to leave out.

Without a supplier the field is all zeros, and a ground station will read that as a CLCW reporting V(R)=0 forever. That is worse than an empty field, because it looks like data.

### The same thing through the composer

[`pkg/stack`](/docs/guides/composed-downlink) takes the supplier as a construction option, so a mission that does not need the layers visible gets the same loop in a dozen lines:

```go
sender, err := stack.NewSender(downlink, stack.WithOCF(func() []byte {
    field, err := onboard.CLCW(0)
    if err != nil {
        return nil // a nil field fails the frame rather than faking one
    }
    return field
}))
```

and on the ground:

```go
for field := range receiver.OCFs() {
    if err := commander.AcceptCLCW(field); err != nil {
        return err
    }
}
```

`Downlink{OCF: true}` **requires** a supplier: `NewSender` returns `ErrMissingOCF` without one. That is deliberate, and the reason is the paragraph above. A composer that quietly filled the field with zeros would produce a link where the ground believes a spacecraft is acknowledging nothing, and no error anywhere says so.

The example runs this version too, at the end:

```
  without a supplier: downlink carries an operational control field but no supplier: pass stack.WithOCF to say what goes in it

  round 0: "SET MODE 3" uplinked, 1 CLCW(s) home, 0 outstanding
  round 1: "POINT 12.5 -3.1" uplinked, 1 CLCW(s) home, 0 outstanding
  round 2: "START SCAN" uplinked, 1 CLCW(s) home, 0 outstanding

  commands accepted ... 3 of 3
  recovered ........... ["SET MODE 3" "POINT 12.5 -3.1" "START SCAN"]
```

Three commands through a window of two, because the CLCW came home each round. Take the supplier away and it stops at two.

## Round one: fill the window

The window here is 2, so it fills in three commands:

```
  queued in FOP-1, 1 outstanding
  queued in FOP-1, 2 outstanding
  held back: FOP-1: send window full, waiting for acknowledgment
  2 frame(s) outstanding, the window is 2
  transmitted N(S)=0 as a 42 octet CLTU
    FARM-1 accepted N(S)=0: "SET MODE 3"
  transmitted N(S)=1 as a 50 octet CLTU
    FARM-1 accepted N(S)=1: "POINT 12.5 -3.1"
```

`ErrFOPWindowFull` is not a failure. It is the sliding window doing its job, and the right response is to hold the frame:

```go
if err := l.fop.TransmitFrame(encoded); err != nil {
    if err := l.uplinkVC.Add(frame); err != nil {
        log.Fatalf("re-queueing the frame: %v", err)
    }
    break
}
```

Put it back rather than dropping it. Someone offering a command wants it queued: the window is a constraint on transmission, not a limit on how much a pass may hold.

## The spacecraft's view

```
  FARM-1 state ........ open
  commands accepted ... 2 of 3 queued
    "SET MODE 3"
    "POINT 12.5 -3.1"
  CLCW it will report .. V(R)=2, retransmit=false, lockout=false
```

`V(R)=2` means "I have everything below frame 2". Two frames acknowledged in one octet, and that octet is now waiting for a downlink to carry it.

## The downlink carries it home

```
  transmitted a 260 octet CADU
  telemetry ..... "BATT 28.1V MODE 3"
  CLCW .......... V(R)=2, retransmit=false, lockout=false
  FOP-1 ......... 0 outstanding, was 2
```

One frame carried both. The telemetry packet is what the pass was for; the CLCW is a passenger in the OCF.

On the ground, getting it out is two steps:

```go
frame, err := tmdl.DecodeTMTransferFrameWithConfig(frameBytes, downlinkConfig)

var clcw cop.CLCW
if err := clcw.Decode(frame.OperationalControl); err != nil {
    log.Fatalf("decoding the CLCW: %v", err)
}
l.fop.ProcessCLCW(&clcw)
```

`DecodeTMTransferFrameWithConfig` rather than plain `DecodeTMTransferFrame`. The plain one does not know the channel has an OCF, so it reads those four octets as data. Nothing errors; you just get a frame whose payload has four extra octets on the end and an empty `OperationalControl`.

## Round two

```
  queued in FOP-1, 1 outstanding
  transmitted N(S)=2 as a 42 octet CLTU
    FARM-1 accepted N(S)=2: "START SCAN"
```

The third command, held back a moment ago, goes up. That is the loop closed: telemetry moved the window, and the window let a command through.

## What it looks like when the downlink is missing

The example ends by running FOP-1 with no return link at all:

```
  6 commands offered, no CLCW ever arrives
  accepted by FOP-1 ... 2 (the window is 2)
  refused ............. 4
  FOP-1 state ......... active
```

Read that state again. FOP-1 is **active**. It is not in an error state, it has raised no alert, and the four refused commands are just sitting there. The link is fine, the spacecraft is fine, and commanding has stopped.

The only thing that catches this is the T1 timer, and the T1 timer does not run itself:

```go
fop.SetT1Initial(ticks)
fop.Tick() // on your own schedule
```

A FOP with `T1Initial` at its default of zero has the timer disabled. Combine that with a lost CLCW and the uplink stalls for good, with every status field reading normal.

## Things that will bite you

**`HasOCF` must match on both ends.** It is not signalled. A ground station that thinks there is no OCF reads the CLCW as four octets of payload; one that thinks there is when there is not eats the last four octets of real data. Neither errors. Building both ends from one `stack.Downlink` value is what stops this.

**A missing OCF supplier is worse than a missing field.** Zeros decode as a valid CLCW reporting V(R)=0. FOP-1 will believe it.

**The COP-1 windows must match.** FOP-1's `windowWidth` and FARM-1's are the same number in the standard, and they are set separately here. A mismatch produces frames the ground thinks are in the window and the spacecraft puts into lockout.

**Use `DecodeTMTransferFrameWithConfig` on an OCF channel.** See above.

**Set V(R) will not clear a lockout.** Only Unlock does. If a CLCW reports `Lockout: true`, send an Unlock BC frame first, read V(R) from the next CLCW, then set V(S) to match. Doing it in the other order looks like it should work.

**The T1 timer is yours.** This is the third guide to say it, about the third protocol: [FOP-1](/docs/guides/uplink), [CFDP](/docs/guides/file-downlink) and [LTP](/docs/guides/dtn-deep-space) all leave the clock to the caller, deliberately, and all three deadlock silently if you do not run it.

## Next

- [Build an uplink](/docs/guides/uplink), the commanding half in full detail
- [Build a downlink](/docs/guides/downlink), the telemetry half
- [COP-1 protocol page](/protocols/data-link/cop), both state machines in full
