---
title: Compose a link
description: One configuration builds both ends of a downlink or an uplink, so they cannot drift apart.
order: 4
---

[Build a downlink](/docs/guides/downlink) wires the layers by hand, which is what you want when you are learning where the boundaries are. This page is the short version: [`pkg/stack`](https://github.com/ravisuhag/astro/tree/main/pkg/stack) takes one configuration value and builds both the spacecraft side and the ground side from it.

## Why it exists

Wiring a downlink by hand is about forty lines: a channel configuration, a physical channel, a master channel, then a virtual channel and a packet service for each stream, a shared frame counter, a packet sizer on every service, and the CADU wrapping at the end.

The problem is that the ground station needs the same forty lines again. The two ends have to agree on the frame length, on whether there is a frame error control field, on whether the frames are randomized, and on which virtual channels exist. Nothing checks that they do.

A ground station configured with a frame length two octets different from the spacecraft's decodes nothing. The failure looks like a bad link, not like a typo.

## The whole thing

```go
config := stack.Downlink{
    SpacecraftID: 42,
    FrameLength:  1115,
    FECF:         true,
    Channels: []stack.VC{
        {ID: 0, Priority: 3}, // housekeeping
        {ID: 1, Priority: 1}, // science
    },
}

// Spacecraft side.
sender, err := stack.NewSender(config)
if err != nil {
    return err
}

telemetry, err := spp.NewTMPacket(100, []byte("battery ok"))
if err != nil {
    return err
}
if err := sender.SendPacket(0, telemetry); err != nil {
    return err
}
if err := sender.Flush(); err != nil {
    return err
}

// Ground side, from the same config.
receiver, err := stack.NewReceiver(config)
if err != nil {
    return err
}

for cadu, err := range sender.CADUs() {
    if err != nil {
        return err
    }
    if err := receiver.Accept(cadu); err != nil {
        return err
    }
}

for packet, err := range receiver.Packets(0) {
    if err != nil {
        return err
    }
    fmt.Printf("%d octets\n", len(packet))
}
```

`NewSender` and `NewReceiver` build the same objects wired the same way. They differ only in which direction data moves, which is what guarantees the two ends match.

## What the shared config buys

Both ends read one value, so a setting cannot be applied to one side only. Set `Randomize` and the sender randomizes while the receiver de-randomizes. Clear `FECF` and neither end looks for the field.

The receive path uses the config-aware frame decoder, which rejects a frame that is not the configured length rather than decoding whatever it is handed. So a genuinely mismatched configuration fails loudly at the first frame instead of producing plausible rubbish.

## Sending and receiving

`Send` takes encoded octets and `SendPacket` takes an `*spp.SpacePacket`. Either way the packet is buffered, not transmitted: a frame carries as many packets as fit, and a packet longer than a frame is split across several. Nothing becomes a CADU until a frame fills or you call `Flush`.

`Flush` matters at the end of a pass. Without it the last packets sit in a buffer waiting for traffic that is not coming.

On the ground, `Accept` strips the sync marker, decodes the frame and routes it to its virtual channel. `Next` and `Packets` then return whole packets. A packet split across frames does not appear until its last frame has arrived, so `Next` returning false means "not yet", not "never".

`Accept` treats a CADU that will not decode as an error. Only the caller can tell a corrupt frame from a misconfigured channel, so a station that would rather keep going should log it and move to the next CADU.

## What it does not do

Reed-Solomon is deliberately absent. CCSDS 131.0 puts the codeblock between the frame and the sync marker, and a caller who wants it can run [`pkg/tmsc`](/protocols/coding/tmsc) over the encoded frame. The interleaving depth and the shortened-codeblock choices are real decisions, and a composer guessing at them would get them wrong.

The layers underneath are still there and still separately usable. This is a convenience over them, not a replacement. Anything the composer cannot express is a reason to drop down a layer, not a reason to grow it until it can express everything.

## The uplink

`pkg/stack` composes an uplink too, and it is deliberately **not** the downlink backwards.

A downlink is a stream: the spacecraft sends, the ground receives, and nothing the ground does changes what arrives. Commanding is a conversation. COP-1 gives the ground a sequence-controlled service that guarantees delivery in order, and it pays for that with feedback — FOP-1 on the ground will not send past its sliding window until a CLCW comes back on the telemetry link saying what the spacecraft has accepted.

So the two ends are asymmetric. The ground side sends packets and accepts CLCWs; the spacecraft side accepts frames and produces them.

```go
config := stack.Uplink{
    SpacecraftID: 42,
    Channels: []stack.UplinkVC{
        {ID: 0, Window: 10}, // critical
        {ID: 1, Window: 10}, // routine
    },
}

commander, err := stack.NewCommander(config)
onboard, err := stack.NewOnboard(config)   // the same config

// Queue a command. Succeeding means queued, not sent.
telecommand, _ := spp.NewTCPacket(100, []byte("SET MODE 3"))
commander.SendPacket(0, telecommand)

for cltu, err := range commander.CLTUs() {
    accepted, err := onboard.Accept(cltu)
    _ = accepted
}

// The spacecraft reports what it took; the ground acts on it, and the
// window moves.
clcw, _ := onboard.CLCW(0)
commander.AcceptCLCW(clcw)
```

### What the window means

`Send` queues a command. `CLTUs` yields only what FOP-1 will release, which stops at the sliding window — so a commander that never receives a CLCW sends its window's worth and then nothing. That is sequence control working, not a fault, and `State` and `Pending` are what tell you which.

The composer holds the backlog itself. FOP-1 refuses a frame outright once its window is full, but a caller offering a command wants it queued: the window is a transmission constraint, not a limit on how much a pass may hold. Frames wait in the commander and are offered again whenever a CLCW might have moved the window.

### Two services, not one

`SendExpedited` bypasses the sequence check. Type BD frames are not counted, not retransmitted and not acknowledged, and they arrive whatever state FOP-1 is in — which is what you use when the sequence machinery is the thing that is broken, such as an unlock after a lockout.

They need their own packet service, because the bypass flag is stamped into the frame header when the frame is built, not chosen when it is transmitted. Sending an AD-shaped frame down FOP-1's BD path gets it rejected on arrival.

### Rejection is not failure

`Accept` returns whether FARM-1 took the frame. A retransmission of something already accepted, or a frame outside the window, is exactly what the procedure exists to filter — so that comes back as `false`, not as an error. An error means the CLTU would not decode at all.

## Next

- [Build a downlink](/docs/guides/downlink) — the same chain with every layer visible
- [Build an uplink](/docs/guides/uplink) — the same commanding chain with every layer visible
- [Handle a lossy link](/docs/guides/lossy-link) — what happens when frames get dropped
