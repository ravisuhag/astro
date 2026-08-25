# CCSDS File Delivery Protocol

> CCSDS 727.0-B-5 — CCSDS File Delivery Protocol (CFDP)

## Overview

CFDP moves files across space links. It is how images, logs, and software
loads actually travel: everything below it in this library carries bytes, and
CFDP is the layer that turns a stream of bytes back into a file on the far end.

One file transfer is a **transaction**. It runs as a sequence of PDUs:

```
Metadata  →  File Data  →  File Data  →  ...  →  EOF
```

The Metadata PDU names the file and its size. File Data PDUs carry the
contents, each stamped with the offset where it belongs. The EOF PDU closes the
stream and carries a checksum so the receiver can tell whether it got
everything intact.

CFDP PDUs are ordinary payload bytes. They ride inside Space Packets or
Encapsulation Packets, so this package sits alongside `pkg/spp` and `pkg/epp`
rather than changing them.

### Where CFDP fits

```
┌─────────────────────────────────────────────┐
│  Files (images, logs, software loads)       │
├─────────────────────────────────────────────┤
│  CFDP — transactions, PDUs, checksums       │  ← this package
├─────────────────────────────────────────────┤
│  Space Packet / Encapsulation Packet        │  ← carries the PDU bytes
├─────────────────────────────────────────────┤
│  TM / TC / AOS / USLP Transfer Frame        │
└─────────────────────────────────────────────┘
```

## Two classes of service

**Class 1, unacknowledged.** The sender pushes the file once and stops. There
is no return link and nothing is retransmitted. Fine for a link you trust, or
where a lost file can simply be sent again later.

**Class 2, acknowledged.** The receiver answers. It sends NAK PDUs naming the
byte ranges it is still missing, the sender fills the gaps, and the exchange
closes with a Finished PDU and its ACK. Use this when the file has to arrive.

The class is a single bit in the PDU header. Note it is inverted on the wire:
table 5-1 gives `0` for acknowledged and `1` for unacknowledged. This package
exposes it as `Acknowledged bool`, in the logical sense.

## Sending a file

```go
import "github.com/ravisuhag/astro/pkg/cfdp"

fs := cfdp.NewOSFilestore("/var/spool/downlink")

sender, err := cfdp.NewSender(fs, cfdp.SenderConfig{
    Source:              cfdp.NewEntityID(1),
    Destination:         cfdp.NewEntityID(2),
    TransactionSeq:      cfdp.NewEntityID(42),
    Acknowledged:        true,
    SegmentSize:         1024,
    SourceFileName:      "image.raw",
    DestinationFileName: "incoming/image.raw",
    ChecksumType:        cfdp.ChecksumModular,
})
if err != nil {
    return err
}

for {
    pdu, ok, err := sender.NextPDU()
    if err != nil {
        return err
    }
    if !ok {
        break // nothing to send right now
    }

    raw, err := pdu.Encode()
    if err != nil {
        return err
    }
    // raw is the user data of a Space Packet, or the payload of anything else.
    transmit(raw)
}
```

`NextPDU` returning `false` does not mean the transaction is over. It means
nothing is pending at this instant — a Class 2 sender will have more to do once
a NAK arrives. Check `Done()` for completion.

## Receiving a file

```go
receiver := cfdp.NewReceiver(fs, cfdp.ReceiverConfig{
    Source:         cfdp.NewEntityID(1),
    Destination:    cfdp.NewEntityID(2),
    TransactionSeq: cfdp.NewEntityID(42),
    Acknowledged:   true,
})

for raw := range incoming {
    pdu, err := cfdp.DecodePDU(raw)
    if err != nil {
        continue // malformed, or a CRC failure — discard it
    }
    if err := receiver.HandlePDU(pdu); err != nil {
        log.Printf("cfdp: %v", err)
    }

    // Send back whatever the receiver owes: ACKs, NAKs, Finished.
    for {
        out, ok, err := receiver.NextPDU()
        if err != nil || !ok {
            break
        }
        encoded, _ := out.Encode()
        transmit(encoded)
    }
}

if receiver.Complete() && receiver.ConditionCode() == cfdp.CondNoError {
    // The file is written and its checksum verified.
}
```

## The library owns no clock

This is the design decision that shapes everything else, and it matches
`pkg/cop`'s FOP-1.

CFDP has timers: how long to wait for an EOF ACK, when to send a NAK, when to
declare a transaction dead from inactivity. **None of them live here.** No
goroutines, no `time.Timer`, no background work. You call the methods; nothing
happens on its own.

That means your scheduler drives retransmission:

```go
// Your timer fired and no EOF ACK came back.
sender.ResendEOF()

// Your timer fired and the file is still incomplete.
receiver.RequestNAK()

// You sent Finished but its ACK never came back.
receiver.ResendFinished()

// The check limit expired and the EOF never arrived: give up and close
// out with Finished (check limit reached).
receiver.ExpireCheckLimit()

// A retry limit ran out for good. Raise the fault; table 4-1 decides
// what happens next (the default is to cancel the transaction).
sender.DeclareFault(cfdp.CondPositiveACKLimitReached)
receiver.DeclareFault(cfdp.CondNAKLimitReached)
```

Note that `ResendFinished` only repeats a Finished PDU that was already sent.
If the far end goes quiet mid-transfer, the tool is `RequestNAK` (to ask for
the missing data again) or, once your retry budget is spent, `DeclareFault`
or `ExpireCheckLimit` to close the transaction out.

The upside is that tests run instantly and deterministically, and scheduling
policy stays where the mission can set it.

## Faults and cancellation

When something goes wrong — a checksum failure, a filestore rejection, a
retry limit — CFDP looks up a **fault handler** for that condition (§4.8).
There are four: cancel, suspend, ignore, and abandon. Table 4-1 defaults
every condition to cancel: the transaction closes out with a Finished PDU
carrying the fault's condition code, and the partial file is discarded.

You can change the disposition per condition:

```go
// At this entity, deliver the file even if its checksum fails.
config.FaultHandlers = map[cfdp.ConditionCode]cfdp.FaultHandler{
    cfdp.CondFileChecksumFailure: cfdp.FaultHandlerIgnore,
}

// Or ask the receiver to do the same, via a TLV in the Metadata PDU.
senderConfig.FaultHandlerOverrides = map[cfdp.ConditionCode]cfdp.FaultHandler{
    cfdp.CondFileChecksumFailure: cfdp.FaultHandlerIgnore,
}
```

Either side can cancel. `sender.Cancel()` sends an EOF whose condition code
is "cancel request received" and whose file size is the progress made so
far. `receiver.Cancel()` discards the partial file and answers with Finished
(delivery incomplete). A receiver that gets an EOF with any fault condition
treats it the same way: it acknowledges the EOF, stops sending NAKs, and
closes out with Finished.

## Checksums

Every file carries a 32-bit checksum, verified on receipt (§4.2).

| Type | Algorithm | Notes |
|---|---|---|
| 0 | Modular | Required by §4.2.2.3. The legacy default |
| 2 | CRC-32C (Castagnoli) | Stronger; recommended for large files |
| 3 | CRC-32 | |
| 15 | Null | Required by §4.2.2.4. Always zero, protects nothing |

The modular checksum is unusual and worth understanding. The file is cut into
4-octet words aligned to file offsets that are multiples of 4, each read
big-endian, and the words added together with the carry discarded.

Because the alignment is by *file offset*, not by arrival order, segments can
arrive out of order and still produce the same answer. That is deliberate: a
Class 2 transfer fills gaps late, and the checksum has to survive it. Annex F
of the spec works through an example, and this package is tested against it
directly.

The CRC-32 variants are order-dependent, so they buffer out-of-order segments
until the file is contiguous.

## Filestores

A `Filestore` is the small interface CFDP needs from a file system: read,
write at an offset, create, delete, rename, size, exists.

Two implementations ship here. `NewMemoryFilestore()` keeps everything in RAM,
which is what the tests use. `NewOSFilestore(dir)` writes under a directory.

**The OS filestore contains every path inside its root.** Filenames arrive over
a radio link, so `../../etc/passwd` must not do what it says. Leading traversal
is stripped rather than rejected, and there is a test that nothing lands
outside the root.

## Filestore requests

A Metadata PDU can carry requests for the receiver to run — delete a file,
rename one, create one — and the Finished PDU carries one response per
request (table 5-7).

```go
config.FilestoreRequests = []cfdp.FilestoreRequest{{
    Action:        cfdp.ActionDeleteFile,
    FirstFileName: cfdp.LV{Value: []byte("old-image.raw")},
}}
```

This package executes create, delete, rename, and deny-file. The rest — append,
replace, and the directory actions — decode correctly but come back with status
"not performed", which table 5-18 provides for.

A transaction with empty filenames carries no file at all. That is how a pure
filestore request travels, and how proxy operations work (§5.2.5).

## The optional PDU CRC

Set `CRCFlag` and every PDU gets a trailing CRC-16, the same CCSDS Telecommand
CRC the frame protocols use (§4.1.3.1). A PDU that fails it is discarded.

Two details worth knowing: the CRC covers everything from the first octet of
the header, and its two octets count toward the PDU data field length
(§4.1.3.2).

Whether you need it depends on what is underneath. If your frames already carry
a Frame Error Control Field, this is a second layer.

## What is not here yet

- **Proxy and remote operations** (Part 2 of the CFDP suite) — a separate
  standard covering store-and-forward overlay.
- **Extended filestore actions** — append, replace, and directory operations
  decode but do not execute.
- **Adaptive flow control** — Keep Alive and Prompt PDUs encode and decode, but
  nothing tunes the send rate from them.
- **CLI subcommands** — `astro cfdp` is a follow-up once this API settles.

## Reference

- [CCSDS 727.0-B-5](https://public.ccsds.org/Pubs/727x0b5e1.pdf) — CCSDS File Delivery Protocol
- [CCSDS 720.1-G-4](https://public.ccsds.org/Pubs/720x1g4.pdf) — CFDP Part 1: Introduction and Overview (Green Book)
- [PICS proforma](../pics/cfdp-pics.md) — conformance statement for this package
