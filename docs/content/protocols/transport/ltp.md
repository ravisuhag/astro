---
title: Licklider Transmission Protocol
short: LTP
description: CCSDS 734.1-B-1 — reliable block transfer over long, one-way-ish links.
order: 13
---

> **CCSDS 734.1-B-1** | [Blue Book](https://public.ccsds.org/Pubs/734x1b1.pdf) | [RFC 5326](https://www.rfc-editor.org/rfc/rfc5326) | [`pkg/ltp`](https://github.com/ravisuhag/astro/tree/main/pkg/ltp) | [`astro ltp`](/cli/ltp)

## Overview

LTP moves blocks of data over links where a round trip takes minutes or hours.

That delay is what makes it different. TCP opens a connection with a
handshake, then adjusts to feedback continuously. Over a light-hour, a
handshake costs two hours before the first byte moves, and feedback describes
a link state that is long gone. LTP does not try. It pushes the whole block,
then asks "what did you miss?" at planned checkpoints.

### Red and green

A block has two parts, and this is the core idea:

```
block:  [ ────── red part ────── │ ─── green part ─── ]
          delivered reliably       best effort
          gaps retransmitted       sent once, never chased
```

The **red part** is chased until the receiver confirms every octet. The
**green part** goes out once; if it is lost, it is gone.

A block can be all red, all green, or red followed by green. Mission data that
must arrive goes in the red part. A video frame that will be superseded in a
second goes in the green part, where a retransmission would waste the link.

The order matters: red always comes first. Green data below a red offset, or
red data above a green one, is a **miscolored** block and cancels the session.

### Where LTP fits

```
┌─────────────────────────────────────────────┐
│  Bundle Protocol / CFDP / application       │
├─────────────────────────────────────────────┤
│  LTP — reliable block delivery              │  <- this package
├─────────────────────────────────────────────┤
│  Link layer (frames, or UDP on the ground)  │
└─────────────────────────────────────────────┘
```

LTP is the retransmission layer of the DTN stack. Bundle Protocol rides on top
of it; CFDP solves a similar problem a different way.

## Scope

**Implemented.** Segment encode and decode, the sender and receiver state machines, reception claims and reports, cancellation, and the shared SDNV codec.

The library owns no clock. You drive every timer yourself — see below.

**Not here yet.**

- **Session multiplexing** — one `Sender` and one `Receiver` handle one
  session each. Managing many is the caller's job for now.
- **The authentication and cookie extensions** — the TLVs encode and decode,
  but nothing acts on them.
- **CLI subcommands** — a follow-up once the API settles.

## Segments

Every segment starts the same way (clause 3.1): a control octet holding a 4-bit
version and a 4-bit type code, a session ID, an extension-count octet, then
the header extensions.

The type code says everything about what follows. Clause 3.1.2 assigns sixteen:

| Code | Segment |
|---|---|
| 0 | Red data |
| 1 | Red data, checkpoint |
| 2 | Red data, checkpoint, end of red part |
| 3 | Red data, checkpoint, end of red part, end of block |
| 4 | Green data |
| 7 | Green data, end of block |
| 8 | Report |
| 9 | Report acknowledgment |
| 12, 13 | Cancel from sender, and its acknowledgment |
| 14, 15 | Cancel from receiver, and its acknowledgment |

Codes 5, 6, 10 and 11 are undefined and decode to an error.

Nearly every field is a **Self-Delimiting Numeric Value**, which is why this
package builds on `pkg/sdnv`. An SDNV packs an integer into as few octets as
it needs — small values cost one octet, and there is no fixed ceiling.

## Sending a block

```go
import "github.com/ravisuhag/astro/pkg/ltp"

sender, err := ltp.NewSender(block, ltp.SenderConfig{
    SessionID:             ltp.SessionID{EngineID: 42, SessionNumber: 7},
    ClientServiceID:       1,
    SegmentSize:           1024,
    RedPartLength:         uint64(len(block)), // all red
    FirstCheckpointSerial: randomNonZero(),
})
if err != nil {
    return err
}

for {
    seg, ok, err := sender.NextSegment()
    if err != nil {
        return err
    }
    if !ok {
        break // nothing pending right now
    }
    raw, _ := seg.Encode()
    transmit(raw)
}
```

`NextSegment` returning `false` does not mean the session is over — a red-part
sender will have more to do once a report arrives. Check `Done()`.

**You pick the first checkpoint serial.** clause 3.2.1 says it must be chosen
randomly for security and must never be zero. This package refuses a zero
rather than inventing randomness of its own, because a library has no business
deciding a mission's randomness policy.

## Receiving

```go
receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
    SessionID:         sessionID,
    FirstReportSerial: randomNonZero(),
    MaxBlockSize:      16 << 20,
})

for raw := range incoming {
    seg, err := ltp.DecodeSegment(raw)
    if err != nil {
        continue
    }
    if err := receiver.HandleSegment(seg); err != nil {
        log.Printf("ltp: %v", err)
    }

    for {
        out, ok, err := receiver.NextSegment()
        if err != nil || !ok {
            break
        }
        encoded, _ := out.Encode()
        transmit(encoded)
    }
}

if receiver.RedPartComplete() {
    data := receiver.RedPart()
}
```

### Set MaxBlockSize

A data segment's offset is an SDNV, so it can name a position near 2^64. A
corrupt or hostile segment claiming a huge offset would make a naive receiver
try to allocate that much memory.

`MaxBlockSize` caps it. The default is 64 MiB; set it to what your mission
actually sends. This cap is not in RFC 5326 — the protocol states no ceiling —
but no real implementation can go without one.

## Reports and reception claims

When a checkpoint arrives, the receiver answers with a **report segment**
listing what it has: an upper and lower bound, then a run of reception claims.

One detail causes more bugs here than anything else:

> **Claim offsets are relative to the report's lower bound, not the start of
> the block.** clause 3.2.2: "The offset within the entire block can be calculated by
> summing this offset with the lower bound of the RS."

Use `ReportSegment.ClaimedRanges()` to get absolute block offsets, and the
addition is done for you.

The sender folds the claims in, works out what is still missing, and queues
those ranges for retransmission. The last segment of each retransmission
cycle is a checkpoint — wherever it sits in the block — and carries the
serial of the report that prompted it, so the receiver answers with a fresh
report and the loop converges. The sender also acknowledges every report,
because clause 3.2.3 requires it, including the final one after the session has
closed.

The receiver holds its session open until that final acknowledgment arrives,
and answers a retransmitted checkpoint by resending the same report rather
than minting a new serial. A report always carries at least one claim, in
ascending order without overlaps; a report with nothing to claim is not sent.

## The library owns no clock

Same contract as `pkg/cop`'s FOP-1, and it matters more here than anywhere.

LTP has timers: how long to wait for a report before resending a checkpoint,
when to send an asynchronous report, when to give up. **None of them live in
this package.** No goroutines, no `time.Timer`.

On a link with a forty-minute round trip, only the mission knows what a
sensible timeout is — and it changes with range. So your scheduler drives it:

```go
// Your timer fired and no report came back.
sender.ResendCheckpoint()

// Your timer fired and the block is still incomplete.
receiver.RequestReport()
```

Tests run instantly and deterministically as a result.

## Cancelling

Either end can cancel, with a reason code from clause 3.2.4:

| Code | Meaning |
|---|---|
| 0 | Client service cancelled the session |
| 1 | Unreachable client service |
| 2 | Retransmission limit exceeded |
| 3 | Miscolored block |
| 4 | System error |
| 5 | Retransmission cycles limit exceeded |

Codes 6 to 255 are reserved and rejected. A cancel is acknowledged by the far
end, which is why there are separate types for cancels from the sender and
from the receiver — it lets a loopback mode work without ambiguity.

## Self-Delimiting Numeric Values

`pkg/sdnv` is a separate package because two protocols here need it: LTP and
Bundle Protocol version 6. It plays the same role `pkg/crc` plays for
checksums.

The encoding is simple. Seven bits of value per octet, most significant first,
with the top bit set on every octet but the last:

```
     0x7F  ->  01111111
    0x80   ->  10000001 00000000
  0x4234   ->  10000001 10000100 00110100
```

The decoder refuses a value past 64 bits (`ErrOverflow`) and refuses input
that ends while the continuation bit is still set (`ErrDataTooShort`), rather
than wrapping or guessing. It also caps an encoding at ten octets: RFC 6256
allows leading `0x80` padding octets, but a padded encoding running past ten
octets is refused with `ErrTooLong` even when the value itself is small.

## Reference

- [RFC 5326](https://www.rfc-editor.org/rfc/rfc5326.txt) — LTP specification, the wire format
- [CCSDS 734.1-B-1](https://public.ccsds.org/Pubs/734x1b1.pdf) — the CCSDS profile for space links
- [RFC 5050 clause 4.1](https://www.rfc-editor.org/rfc/rfc5050.txt) — where SDNV is defined
- [CLI](/cli/ltp) | [Conformance](/conformance/ltp) | [The stack](/docs/start/concepts)