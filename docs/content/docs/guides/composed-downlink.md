---
title: Compose a downlink
description: One configuration builds both ends of a telemetry link, so they cannot drift apart.
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

## Next

- [Build a downlink](/docs/guides/downlink) — the same chain with every layer visible
- [Build an uplink](/docs/guides/uplink) — the other direction, with retransmission
- [Handle a lossy link](/docs/guides/lossy-link) — what happens when frames get dropped
