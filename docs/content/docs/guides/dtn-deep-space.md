---
title: Store and forward for deep space
short: DTN
description: When the link is not up, and a handshake would finish tomorrow.
order: 12
---

Every other guide here assumes the link is up. On a deep space mission it usually is not. A relay is behind the planet, a station is scheduled for someone else, or the round trip is forty minutes so a three-way handshake finishes tomorrow.

Delay-Tolerant Networking is built for that, and it is two layers. [Bundle Protocol](/protocols/transport/bp) is the network layer: it addresses data, gives it a lifetime, and stores it at each hop instead of holding a session open end to end. [LTP](/protocols/transport/ltp) is the convergence layer for one hop: it pushes a whole block and then asks what was missed.

The complete program is [`examples/dtn`](https://github.com/ravisuhag/astro/tree/main/examples/dtn). Run it:

```bash
go run ./examples/dtn/
```

## What we are building

```
  rover                     relay                    Earth
  ipn:1.1                                           ipn:2.1
     │                        │                        │
     │  ┌─────────────────────┴──┐  window closed  ┌───┴───┐
     └─►│ bundle stored here     │─ ─ ─ ─ ─ ─ ─ ─ ►│       │
        └────────────────────────┘  until it opens └───────┘
             one LTP session            one LTP session
             per hop                    per hop
```

Bundle Protocol addresses the data. LTP gets it across one hop and knows nothing about what is inside it.

## Version 7, not version 6

Worth saying early. `pkg/bp` implements Bundle Protocol **version 7**, RFC 9171 — the version ION, µD3TN and HDTN all speak. Version 6 (RFC 5050, profiled by CCSDS 734.2-B-1) is a different wire format, not an earlier revision: it encodes with SDNV where version 7 encodes with CBOR, and the two cannot talk to each other.

## A bundle

```go
primary := &bp.PrimaryBlock{
    Flags:   bp.FlagReportDelivery | bp.FlagStatusTimeRequested,
    CRCType: bp.CRC32C,

    Destination: bp.IPN(earthNode, scienceSvc),
    Source:      bp.IPN(roverNode, scienceSvc),
    ReportTo:    bp.IPN(roverNode, scienceSvc),

    Timestamp: bp.CreationTimestamp{Time: 828_000_000_000, Sequence: 1},
    Lifetime:  86_400_000,
}

hops, _ := bp.NewHopCountBlock(2, 32, 0)
bundle, err := bp.NewBundle(primary, payload, hops)
```

Four things in there matter more than they look.

**`CRCType`** is not optional in the way it looks. Version 7 requires a checksum on the primary block unless a security block covers it instead, and astro cannot see a security block it does not implement — so choosing `CRCNone` here is a decision, not a default.

**`Lifetime`** is how long the bundle stays worth carrying, in milliseconds from the creation timestamp. A node holding an expired bundle deletes it rather than forwarding it into a window that has already closed. Get this wrong in the small direction and your data quietly evaporates in a queue somewhere.

**The creation timestamp** is not just information. Source plus timestamp is what *identifies* a bundle: duplicates are recognised by it, fragments are reassembled by it, and status reports name their subject by it. The time is milliseconds since 2000 — version 6 counted seconds — and the sequence number distinguishes bundles made in the same millisecond.

**The hop count block** is cheap insurance against a routing loop. A bundle ping-ponging between two nodes is deleted once it passes the limit, rather than circulating until its lifetime runs out. Version 7 dropped the priority and custody machinery version 6 carried; what is left is plainer.

The endpoints travel as numbers, not strings. An `ipn` endpoint encodes as a pair of integers, which is why the scheme exists: on a link measured in kilobits, a text URI in every bundle is real money.

```
  from ............ ipn:1.1
  to .............. ipn:2.1
  priority ........ expedited, ordinal 100
  lifetime ........ 86400 s
  payload ......... 46 octets
  encoded bundle .. 90 octets
```

## One hop, over LTP

LTP calls the encoded bundle a block. It has no idea what is in it.

```go
sender, err := ltp.NewSender(block, ltp.SenderConfig{
    SessionID:             ltp.SessionID{EngineID: roverNode, SessionNumber: 42},
    ClientServiceID:       scienceSvc,
    SegmentSize:           24,
    RedPartLength:         uint64(len(block)),
    FirstCheckpointSerial: 0x5A5A,
})
```

## Red and green

`RedPartLength` is the interesting field. A block has two parts:

```
block:  [ ────── red part ────── │ ─── green part ─── ]
          retransmitted on loss     sent once
```

The red part is delivered reliably: gaps are retransmitted until the receiver confirms it has everything. The green part is best effort, sent once and never chased.

Set it to the block length and the whole thing is reliable, which is what a bundle wants. Set it to zero and the whole thing is green, which is what a live telemetry stream wants, because a sample that arrives late is worth nothing. A mixed block puts the header in the red part and the samples in the green.

The checkpoint serials must be random and must never be zero. Clause 3.2.1 says so for security reasons, and this package has no randomness policy of its own, so the caller picks.

## A lost data segment recovers on its own

```
  down  data @ 0    red data
  down  data @ 24   red data                                 LOST IN TRANSIT
  down  data @ 48   red data
  down  data @ 72   red data, checkpoint, end of red part, end of block

  red part complete ... false
  missing ............. offset 24, 24 octets

  Recovery:
    up    report     2 claim(s)
    down  report acknowledgment
    down  data @ 24   red data, checkpoint
    up    report     1 claim(s)
    down  report acknowledgment
```

No handshake anywhere. The sender pushed the whole block, the checkpoint at the end prompted a report, the report claimed two ranges, and the sender worked out the hole between them and filled it.

That is the shape of a protocol designed for hour-long round trips: one push, one question, one answer.

## A lost checkpoint does not

Here is the failure the timers exist for. Send everything **except** the checkpoint:

```
  checkpoint lost
  sender has something to send ..... false
  receiver has something to send ... false
  red part complete ................ false

  Neither end will move. The receiver is waiting for a
  checkpoint that will not arrive, and the sender is waiting
  for a report that nothing will prompt.
```

Both machines are idle and the transfer is not finished. Nothing in the protocol breaks that deadlock, because LTP has no clock:

```go
sender.ResendCheckpoint()
```

```
  After the caller's timer fires and calls ResendCheckpoint:
    down  data @ 0    red data
    ...
    up    report     1 claim(s)

  red part complete ... true
```

LTP has three timers, for checkpoint retransmission, report retransmission and cancel retransmission, and **all three are yours to run**. That is the same contract as [FOP-1's T1](/docs/guides/uplink) and [CFDP's](/docs/guides/file-downlink), and the reason is the same: on a light-minutes link only the mission knows what a sensible timeout is.

Note that `ResendCheckpoint` resends the red part, not just the one segment. On a long link that is the cheaper mistake to make.

## Fragmenting for a short window

A contact window can be too short for the bundle. Bundle Protocol splits it, each fragment travels on its own, and the destination reassembles:

```go
fragments, err := bundle.Fragment(16)
rejoined, err := bp.Reassemble(fragments)
```

```
  fragments ....... 3
    offset  0 of 46, 62 octets on the wire
    offset 16 of 46, 62 octets on the wire
    offset 32 of 46, 60 octets on the wire
  reassembled ..... 46 octets, identical true
```

Look at the overhead: 46 octets of payload became 184 octets in three fragments, because each carries a full primary block. Fragmenting small is expensive. Fragment to fill the window you have, not smaller.

Fragments can arrive by different routes, in any order, hours apart. Reassembly needs all of them, and a bundle marked `FlagNoFragment` cannot be split at all, so it waits for a window big enough or expires trying.

## Things that will bite you

**`Lifetime` deletes data.** A bundle that outlives its lifetime is dropped by whichever node is holding it, silently as far as the source is concerned unless you asked for deletion reports. Set it from the mission's actual contact schedule, with margin.

**No timers anywhere.** Neither `pkg/bp` nor `pkg/ltp` starts a goroutine or reads a clock. A stalled LTP session stays stalled until your scheduler prods it, and there is no error to tell you that has happened. Watching for it is part of the job.

**`NextSegment` returning false is not `Done`.** It means "nothing pending right now", which during a transfer usually means waiting on the far end. Check `Done` or `RedPartComplete`.

**`MaxBlockSize` is not in the RFC.** A data segment's offset is an SDNV reaching 2^64, so without a cap one corrupt segment claiming a huge offset makes the receiver try to allocate that much memory. Set it to what the mission actually sends.

**Fragment overhead is real.** Each fragment carries a whole primary block. Splitting a small bundle can more than triple it.

**Green data is really gone.** A loss in the green part is never chased and never reported. That is the point, and it means anything you cannot afford to lose belongs in the red part.

**Bundles are payload.** BP and LTP produce ordinary octets. Getting a segment across a hop is still [packets, frames and CADUs](/docs/guides/downlink) underneath.

## Next

- [Downlink a file](/docs/guides/file-downlink), the other way to move a large object, for a link that is up
- [Build a downlink](/docs/guides/downlink), what carries an LTP segment across a hop
- [BP](/protocols/transport/bp) | [LTP](/protocols/transport/ltp) | [BP CLI](/cli/bp) | [LTP CLI](/cli/ltp)
