---
title: Downlink a file
short: File downlink
description: Move a whole file to the ground, notice the hole in it, and fill it.
order: 7
---

Packets carry readings. Files carry everything else: an image, a log, a software patch, a recorder dump. A file is a different problem from a packet, because it has a name and a length, and it is not delivered until every byte has arrived.

[CFDP](/protocols/transport/cfdp) is the protocol for that, and this guide sends one file over a link that loses part of it.

The complete program is [`examples/cfdp`](https://github.com/ravisuhag/astro/tree/main/examples/cfdp). Run it:

```bash
go run ./examples/cfdp/
```

## What we are building

```
Spacecraft                                 Ground
──────────                                 ──────
science/img_0042.raw
     │                                          │
     │  Metadata ──────────────────────────────►│
     │  File Data @ 0 ─────────────────────────►│
     │  File Data @ 512 ───────────────────────►│
     │  File Data @ 1024 ───────────╳────────── │
     │  File Data @ 1536 ──────────────────────►│
     │  EOF ───────────────────────────────────►│
     │                                          │
     │◄────────────── NAK, octets 1024 to 1536  │
     │  File Data @ 1024 ──────────────────────►│
     │◄────────────────────────────── Finished  │
     │  ACK ───────────────────────────────────►│
     │                                          │
                            downlink/img_0042.raw
```

## A transaction, not a stream

One file transfer is one transaction, and it is named by three numbers that both ends have to agree on:

```go
sender, err := cfdp.NewSender(onboard, cfdp.SenderConfig{
    Source:              cfdp.NewEntityID(spacecraftEntity),
    Destination:         cfdp.NewEntityID(groundEntity),
    TransactionSeq:      cfdp.NewEntityID(transactionSeq),
    Acknowledged:        true,
    SegmentSize:         512,
    SourceFileName:      sourceFile,
    DestinationFileName: destinationFile,
    CRCFlag:             true,
})
```

The receiver's config repeats those three. PDUs from any other source entity or sequence number are ignored rather than misapplied, so one receiver never absorbs a foreign transaction's data.

`Acknowledged: true` is Class 2. Class 1 is fire and forget: it will happily deliver a file with a hole in it and tell nobody. Class 2 costs a return link and gets you the file.

## The filestore is an interface

CFDP needs to read a source file and write a destination file at arbitrary offsets. That is all:

```go
onboard := cfdp.NewMemoryFilestore()
_ = onboard.WriteAt(sourceFile, 0, image)
```

`NewMemoryFilestore` is for tests. `NewOSFilestore(dir)` writes to a real directory and refuses paths that escape it. The interface is seven methods, so wrapping your own storage is not much work.

## Nothing owns a clock

`Sender` and `Receiver` have no goroutines and no timers. You pump them, exactly like [FOP-1](/protocols/data-link/cop):

```go
for {
    pdu, ok, err := sender.NextPDU()
    if err != nil {
        log.Fatalf("building a PDU: %v", err)
    }
    if !ok {
        break // nothing pending; waiting on the receiver
    }
    transmit(pdu)
}
```

`ok` going false means "nothing to say right now", not "finished". The transaction is finished when `Done` says so.

That design is deliberate. Retransmission timing on a space link is a policy decision that depends on the round trip, the pass schedule and what else is queued, and a library that picked for you would be wrong for most missions.

## A PDU is ordinary payload

CFDP does not know about frames. Its PDUs are bytes, so they need a packet to travel in:

```go
raw, err := pdu.Encode()
packet, err := spp.NewTMPacket(apidFileTransfer, raw,
    spp.WithSequenceCount(uint16(index)))
```

From there it is [frames and CADUs](/docs/guides/downlink) like anything else. CFDP composes with `pkg/spp` and `pkg/epp` from the outside and changes neither.

## The first pass

```
--- Spacecraft: sending science/img_0042.raw (3000 octets) ---

  Metadata                  64 octets in a Space Packet
  File Data @ 0            531 octets in a Space Packet
  File Data @ 512          531 octets in a Space Packet
  File Data @ 1024         LOST IN TRANSIT
  File Data @ 1536         531 octets in a Space Packet
  File Data @ 2048         531 octets in a Space Packet
  File Data @ 2560         459 octets in a Space Packet
  EOF                       25 octets in a Space Packet
```

Metadata first, describing the file. Then the contents in 512-octet segments. Then EOF, carrying the length and the checksum.

Every File Data PDU says its own offset, which is the reason a lost one is recoverable at all. The receiver is not assembling a stream in order; it is filling in a file it knows the length of.

## Noticing the hole

```
--- Ground: what is missing ---

  Complete .... false
  Gap ......... octets 1024 to 1536
```

The EOF gave the receiver the file length, so it can compare what it has against what there should be. `MissingSegments` returns the gaps by offset.

## Filling it

```
--- Recovery ---

  up   ACK
  up   NAK
  down File Data @ 1024
  up   Finished
  down ACK
```

Five PDUs. The NAK names the gap by offset, the sender reads that range out of the source file again, and the receiver checksums the finished file and says so with a Finished PDU. The sender acknowledges that, and both sides are done.

The whole exchange is the same pump loop in both directions:

```go
for {
    pdu, ok, err := receiver.NextPDU()
    if !ok {
        break
    }
    sender.HandlePDU(pdu)
}
for {
    pdu, ok, err := sender.NextPDU()
    if !ok {
        break
    }
    receiver.HandlePDU(pdu)
}
```

The ACK of the Finished PDU is asked for separately, because the sender owes it only once the receiver has declared itself done:

```go
if ack, ok, err := sender.AckFinished(); err == nil && ok {
    receiver.HandlePDU(ack)
}
```

## The result

```
  Delivered as ..... downlink/img_0042.raw
  Size ............. 3000 octets
  Identical ........ true
  Sender state ..... finished
  Receiver state ... finished
  PDUs sent ........ 9 down, 3 up
  PDUs lost ........ 1
  Condition ........ no error
```

3000 octets out, 3000 octets in, byte for byte, having lost 512 of them on the way.

## Things that will bite you

**`ok == false` is not `Done`.** `NextPDU` returning false means the machine has nothing pending, which during a transfer usually means it is waiting for the other end. Looping until `!ok` and then declaring success will report a half-delivered file as complete.

**`AckFinished` hands out an ACK every time you ask.** It is owed once. Ask once, remember that you did, and stop.

**Class 1 does not tell you anything.** With `Acknowledged: false` there is no NAK and no Finished unless you set `ClosureRequested`, and even then the closure only says the receiver saw the EOF. If the file matters, use Class 2.

**Nothing retransmits on its own.** There is no timer in this package. A NAK that gets lost stalls the transfer until your scheduler resends something. That is the same contract as FOP-1's T1 timer, and the same trap.

**Segment size is not free.** Smaller segments lose less per dropped PDU and pay the header cost more often. 512 or 1024 octets is a common starting point; match it to your frame size rather than picking a round number.

**The checksum is over the file, not the PDUs.** A PDU CRC (`CRCFlag`) catches a corrupt PDU. The EOF checksum catches a file that assembled wrongly. They are different failures and you want both.

## Next

- [Handle a lossy link](/docs/guides/lossy-link), what the layers underneath do about loss
- [Store and forward for deep space](/docs/guides/dtn-deep-space), when the link is not there at all
- [CFDP protocol page](/protocols/transport/cfdp) | [Conformance](/conformance/cfdp) | [CLI](/cli/cfdp)
