---
title: Cross Support Transfer Service
short: CSTS
description: CCSDS 921.1-B-2, the framework that succeeds SLE.
identifiers:
  - "CCSDS 921.1-B-2 * Cross Support Transfer Service—Specification Framework"
  - "pkg/csts"
order: 41
---

> **CCSDS 921.1-B-2** | [Blue Book](https://public.ccsds.org/Pubs/921x1b2e1.pdf) | [`pkg/csts`](https://github.com/ravisuhag/astro/tree/main/pkg/csts)

## Overview

CSTS is the successor to [Space Link Extension](/protocols/ground/sle). Where
SLE defines four services as four separate standards with four sets of protocol
data units, CSTS defines a **framework** — a set of reusable procedures and
operations — and a service is assembled from them.

That is the whole idea. A new cross support service does not need a new
protocol; it picks the procedures it needs and says which parameters and events
it has. The Monitored Data service of CCSDS 922.1-B-2 and the Tracking Data
service of CCSDS 922.2-B-2 are the first two built this way.

| Piece | What it is |
|---|---|
| **Operations** | 13 message types: BIND, START, GET, TRANSFER-DATA, NOTIFY and the rest (section 3) |
| **Procedures** | 12 reusable behaviours, from Association Control to Sequence-Controlled Data Processing (section 4) |
| **A service** | One prime procedure, any number of secondary ones, and always Association Control |

## Scope

**Implemented.** The framework's spine: the object identifier tree of annex
F3.1, the common types of F3.3, the standard operation headers of clause 3.3,
the Association Control operations of F3.5, and the framework PDU of F3.15 that
carries them all.

**Not implemented: the procedures as state machines.** This package reads and
writes their messages; it does not run Buffered Data Delivery or
Sequence-Controlled Data Processing. That is the same split `pkg/sle` makes,
where the codecs are pure and the association machine is caller-pumped.

**Carried, not modelled.** Three of the twenty PDU alternatives keep their
octets rather than being decoded further: the EXECUTE-DIRECTIVE invocation and
the two buffer messages. So do several fields — a `Time`, a `Name`, an
`EventValue`, a `ListOfParametersEvents`. Each is built from identifiers
registered with SANA rather than fixed by this document, so a Go type for them
would be a type for a registry that changes without this package.

## A CSTS PDU says what it is; an SLE PDU does not

This is the difference that matters most in practice.

An SLE PDU's wire tag means one operation in Return All Frames and another in
Forward CLTU. Nothing in the octets says which service they came from — the
association does, and that is out of band. It is why `astro sle decode`
requires `--service` and refuses to guess.

A framework PDU's tag means the same operation everywhere (annex F3.15), and
the message carries the name of the procedure instance it belongs to (clause
3.3.2.5). So the same octets mean the same thing wherever they arrive, and
`Decode` needs nothing but the octets.

```go
pdu, err := csts.Decode(data)
fmt.Println(pdu.Type)                     // "BIND invocation"
if header, ok := pdu.Header(); ok {
    fmt.Println(header.Procedure.Type)     // which procedure it belongs to
    fmt.Println(header.InvokeID)           // what pairs a response with it
}
```

## The tags run in tens

Annex F3.15 numbers the CHOICE alternatives `[0]`–`[4]`, then `[10]`, `[11]`,
`[20]`, `[21]`, `[30]`–`[32]`, `[40]`, `[41]`, `[50]`, `[60]`–`[62]`, `[70]`,
`[71]`.

The gaps are deliberate: each operation's messages get a decade, so a future
issue can add one without renumbering anything. A tag *in* a gap is not an
operation, and `Decode` refuses it rather than passing it along as something it
half-understood.

## Implicit tagging replaces the SEQUENCE tag

Every module in annex F is declared `IMPLICIT TAGS`. A CHOICE alternative's
context tag therefore **replaces** the tag of the type beneath it rather than
nesting inside it.

Several alternatives are declared as a bare type reference:

```
UnbindReturn ::= StandardReturnHeader
StartReturn  ::= StandardReturnHeader
GetReturn    ::= StandardReturnHeader
```

`StandardReturnHeader` is a `SEQUENCE`, so `[3] UnbindReturn` encodes as
`a3 <len> <fields>` — there is **no** `30` in it. Writing a SEQUENCE there as
well produces a PDU one level too deep, which round-trips perfectly against
itself and against nothing else. `TestReturnHeaderIsNotDoubleWrapped` checks
the octets for exactly that.

## The PEER-ABORT diagnostic is shared across three layers

A peer abort carries one octet, and its value space is partitioned across the
whole cross support family (annex F3.5):

| Range | Allocated by |
|---|---|
| 0–39 | SLE transfer services |
| 40–69 | the CSTS Association Control procedure |
| 70–125 | a CSTS procedure, aborting the association with its own reason |
| 126 | `otherReason` |
| 128–199 | ISP1, the underlying protocol |
| 200–250 | the application, chosen per service type |

So the same octet arrives from three different layers and means different things
in each. `PeerAbortDiagnostic.Origin` says which layer allocated a value, and
`String` names it only where the framework defines it. An application value
means whatever its service type says, and this package has no service type to
ask — the same reason `pkg/sle` will not decode a PDU without being told the
service.

## The transport is SLE's

Clause 2.6 makes ISP1 (CCSDS 913.1-B-2) the default underlying protocol, and
says an implementation using it uses that document's credentials algorithm.

So the transport and the credential octets are what `pkg/sle` already builds.
The BER codec is shared between the two packages for the same reason: CSTS and
SLE share no data type, but they share the encoding.

This package carries credentials rather than interpreting them, and a caller
that needs the algorithm reaches for `pkg/sle`.

## Derived vectors, and why

CCSDS 921.1-B-2 prints no worked example and no octets. It is an abstract
specification with an ASN.1 annex rather than a wire format document, so there
is nothing to transcribe the way annex G of the navigation standards lets us
transcribe a file.

The vectors are therefore **derived**, and the corpus says so. Each one carries
its derivation from annex F octet by octet in its note, so a reader can check
the derivation against the module rather than against this package.

## Reference

- [CCSDS 921.1-B-2, Cross Support Transfer Service—Specification Framework](https://public.ccsds.org/Pubs/921x1b2e1.pdf)
- [CCSDS 922.1-B-2, Monitored Data](https://public.ccsds.org/Pubs/922x1b2.pdf) and [CCSDS 922.2-B-2, Tracking Data](https://public.ccsds.org/Pubs/922x2b2.pdf), the first two services built on it
- [CCSDS 920.0-G-1](https://public.ccsds.org/Pubs/920x0g1.pdf), the Green Book
- [Conformance](/conformance/csts) | [Space Link Extension](/protocols/ground/sle)
