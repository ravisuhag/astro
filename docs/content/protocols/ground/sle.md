---
title: Space Link Extension
description: CCSDS 911.x, 912.1, 913.1 — getting frames to and from a ground station over the internet.
order: 40
---

> **CCSDS 913.1-B-2 (ISP1)**, with RAF, RCF, ROCF and FCLTU on top · [`pkg/sle`](https://github.com/ravisuhag/astro/tree/main/pkg/sle)

## Overview

Everything else in this library speaks to a spacecraft. SLE is the one that
speaks between ground systems.

A mission control centre in one country needs telemetry from a ground station
in another. SLE is the protocol they use: the control centre opens a TCP
connection to the station and either receives frames coming down, or sends
telecommand units going up.

The payloads are things this library already builds. CADUs from `pkg/tmsc`,
CLTUs from `pkg/tcsc`, CLCWs from `pkg/cop`. SLE is the wire between the
ground systems that handle them.

```
┌──────────────────┐                      ┌──────────────────┐
│  Control centre  │ ◄── SLE over TCP ──► │  Ground station  │
└──────────────────┘                      └────────┬─────────┘
                                                   │
                                              space link
                                                   │
                                              ┌────▼─────┐
                                              │Spacecraft│
                                              └──────────┘
```

## The standard numbers

The SLE suite is easy to misattribute, so for the record:

| Standard | What it is |
|---|---|
| CCSDS 913.1-B-2 | **ISP1**, the transport this package implements |
| CCSDS 911.1-B-5 | Return All Frames |
| CCSDS 911.2-B-4 | Return Channel Frames |
| CCSDS 911.5-B-4 | Return Operational Control Fields |
| CCSDS 912.1-B-5 | Forward CLTU |

CCSDS 914.0-M-2 is the SLE Application Program Interface — a Recommended
Practice describing an API, not a wire format.

## Scope

**Implemented** — the transport and the handshake:

- **TML**, the Transport Mapping Layer: message framing over TCP, the context
  message, the heartbeat
- **BER**, a subset codec sized to SLE's ASN.1
- **Credentials**, the ISP1 authentication scheme
- **BIND, UNBIND, PEER-ABORT**, and the association state machine

And the four transfer services built on it:

- **RAF**, Return All Frames — every frame off a physical channel
- **RCF**, Return Channel Frames — one virtual or master channel's frames
- **ROCF**, Return Operational Control Fields — the four-octet control field,
  usually a CLCW
- **FCLTU**, Forward CLTU — telecommand units going up

**Not here yet.**

- **A production provider.** The provider halves answer a user and no more:
  no multi-association management, no transfer-buffer sizing or release
  timers, no production. They are a test double and a starting point.
- **Typed GET-PARAMETER values.** The operation itself works — invocation,
  both return alternatives, and a clean 'unknown parameter' refusal — but the
  per-service parameter CHOICEs travel as raw BER for you to interpret.
- **TLS or any transport security** beyond ISP1's own credentials.
- **CLI subcommands.**

## Three design decisions

### No goroutines, no timers

This package owns neither. Every codec is pure, and the association machine is
caller-pumped — the same contract as `pkg/cop`'s FOP-1.

ISP1 has a heartbeat interval and a dead factor, so there *is* timing involved.
Rather than run a clock, the association answers questions about a time you
hand it:

```go
if assoc.HeartbeatDue(now) {
    conn.Write(heartbeatBytes)
    assoc.RecordSent(now)
}
if assoc.PeerDead(now) {
    conn.Close()
}
```

Your loop drives it. Tests run instantly and deterministically as a result.

### A hand-rolled BER subset

Go's `encoding/asn1` cannot do this job. It implements DER and rejects the
context-specific CHOICE tagging every SLE module relies on. Rather than take a
dependency, this package carries just enough BER for what SLE actually sends.

What is supported: the universal types SLE uses (OBJECT IDENTIFIER included,
which the BIND's service instance identifier needs), context-specific tags in
both primitive and constructed form, multi-octet tag numbers (SLE uses `[100]`
and up), and definite lengths in both forms.

The **indefinite-length form** is accepted on decode — real providers emit
it — by scanning for the end-of-contents octets. This package always emits
the definite form itself. The one refusal left is an indefinite length on a
primitive encoding, which X.690 forbids.

The integer encoding is tested against `encoding/asn1` as an oracle — for
plain universal INTEGERs, BER and DER agree, so the stdlib pins the minimal
two's complement encoding without hand-writing every vector.

### SHA-256, not SHA-1

§3.1.2.3 requires SHA-256. SHA-1 belonged to the previous issue of the
standard. §3.2.3's note keeps a 20-octet digest recognizable only so a new
implementation can talk to an old one: this package decodes a digest only at
20 or 32 octets — no other length is a digest either issue defines — never
generates the 20-octet form, and cannot verify one, because it does not
implement the superseded scheme. Verification requires SHA-256.

## TML: the framing

Every message on the connection is eight octets of header and a body
(§3.3.2.2.1):

```
octet 0:    type (1 = SLE PDU, 2 = context, 3 = heartbeat)
octets 1-3: reserved, zero
octets 4-7: body length, big-endian
octets 8+:  body
```

Three message types (table 3-1):

**Context** opens the connection, before any PDU. Twelve octets: the characters
`ISP1`, three reserved zeros, the version, then the heartbeat interval and dead
factor.

**SLE PDU** carries an encoded operation.

**Heartbeat** is a header with a zero length and no body. It proves an idle
connection is still alive.

```go
assoc, _ := sle.NewAssociation(sle.AssociationConfig{
    Role:              sle.RoleUser,
    LocalIdentifier:   "CTRL-CENTRE",
    PeerIdentifier:    "GROUND-STN",
    HeartbeatInterval: 30,
    DeadFactor:        3,
})

// The context message goes first.
sle.WriteMessage(conn, assoc.ContextMessage(time.Now()))
```

Reading is stream-safe: `ReadMessage` takes exactly the header plus as many
body octets as the header promises, and stops. Reading ahead would swallow the
next message.

## Credentials

Neither end sends its password. The sender hashes a DER-encoded structure
holding the current time, a random number, its user name and its password, then
transmits the time, the random number and the digest. The receiver, who knows
the peer's password, recomputes and compares.

```go
creds, err := sle.GenerateCredentials(now, randomNumber, userName, password)
```

**You supply the random number.** A library has no business picking a mission's
randomness source, and a fixed value makes tests reproducible.

The time is what stops a replay. §3.1.2.2.1 has the receiver reject credentials
whose time is further from now than an acceptable delay:

```go
err := creds.Verify(now, time.Minute, peerName, peerPassword)
// ErrCredentialsExpired if the clock skew is too large
// ErrAuthenticationFailed if the digest does not match
```

The digest comparison is constant time. A timing oracle on a MAC comparison is
a real attack and costs nothing to avoid.

## The association

```go
// User side.
invocation, _ := assoc.Bind(now, randomNumber,
    sle.AppReturnAllFrames, 5, "GS-PORT",
    sle.ServiceInstanceIdentifier{
        {Identifier: "sagr", Value: "MISSION"},
        {Identifier: "spack", Value: "PASS1"},
        {Identifier: "rsl-fg", Value: "1"},
        {Identifier: "raf", Value: "onlc1"},
    })

encoded, _ := invocation.Encode()
sle.WriteMessage(conn, &sle.Message{Type: sle.MessageSLEPDU, Body: encoded})
```

The provider answers, and the state machine handles the rules: an association
already bound refuses a second BIND, an unexpected initiator is refused, bad
credentials are refused with the right diagnostic. Each of those is a test.

States run unbound → bind pending → bound → unbind pending → closed. A
PEER-ABORT jumps straight to closed from anywhere.

## Service instance identifiers

An SLE provider hosts many service instances, and a BIND names one. The
identifier is a sequence of attribute pairs that operators write dotted:

```
sagr=MISSION.spack=PASS1.rsl-fg=1.raf=onlc1
```

Which reads as: the service agreement, the service package, the functional
group, and the RAF instance within it.

On the wire, each attribute name is not the string you type but an **OBJECT
IDENTIFIER** from the SLE-SERVICE-INSTANCE-ID module. This package maps the
operator names for you:

| Name | Object identifier |
|---|---|
| `sagr` | 1.3.112.4.3.1.2.52 |
| `spack` | 1.3.112.4.3.1.2.53 |
| `rsl-fg` | 1.3.112.4.3.1.2.38 |
| `fsl-fg` | 1.3.112.4.3.1.2.14 |
| `raf` | 1.3.112.4.3.1.2.22 |
| `rcf` | 1.3.112.4.3.1.2.46 |
| `rocf` | 1.3.112.4.3.1.2.49 |
| `cltu` | 1.3.112.4.3.1.2.7 |
| `antenna` | 1.3.112.4.3.1.2.3 |

So `{Identifier: "sagr", Value: "MISSION"}` encodes the sagr OID and the
string value. A dotted numeric identifier is passed through as an OID for
anything not in the table. On decode, a peer that sent the legacy
VisibleString form (as old versions of this package did) is still read, and
the attribute's `Legacy` flag says so.

## The four services

Each service is a user half and a provider half over one association. The user
is the mission-control side, and it is the one implemented in full. The
provider is partial — enough to answer a user and to test against, not enough
to run a ground station. See [the PICS](/conformance/sle) for the row-by-row
picture.

| Service | Go types | What it carries |
|---|---|---|
| RAF | `RAFUser`, `RAFProvider` | every frame, good and bad |
| RCF | `RCFUser`, `RCFProvider` | one channel's frames, good only |
| ROCF | `ROCFUser`, `ROCFProvider` | operational control fields |
| FCLTU | `FCLTUUser`, `FCLTUProvider` | CLTUs going up |

### Three states

All four specs use the same three states, and this package numbers them the
way the specs do so a logged state matches the table you are reading:

```
   state 1              state 2                 state 3
  ┌─────────┐  BIND    ┌─────────┐   START     ┌─────────┐
  │ unbound │ ───────► │  ready  │ ──────────► │ active  │
  │         │ ◄─────── │         │ ◄────────── │         │
  └─────────┘  UNBIND  └─────────┘   STOP      └─────────┘
       ▲                                            │
       └────────────── PEER-ABORT ───────────────────┘
```

Data moves only in state 3. An operation the state does not allow is refused
before anything goes on the wire, and a PDU that arrives in the wrong state
draws a PEER-ABORT for protocol error — which is what the state tables say to
do.

### A user-side session

```go
user, err := sle.NewRAFUser(sle.ServiceConfig{
    Association:   assoc,          // already configured, see above
    DeliveryMode:  sle.DeliveryReturnCompleteOnline,
    Version:       5,
    ResponderPort: "GROUND-PORT",
    Instance: sle.ServiceInstanceIdentifier{
        {Identifier: "sagr", Value: "MISSION"},
        {Identifier: "spack", Value: "PASS1"},
        {Identifier: "rsl-fg", Value: "1"},
        {Identifier: "raf", Value: "onlc1"},
    },
})

// Every call queues a PDU. Nothing is sent until you send it.
if err := user.Bind(time.Now(), randomNumber()); err != nil { ... }

for {
    // Push out whatever the machine has queued.
    for {
        pdu, ok := user.NextPDU()
        if !ok {
            break
        }
        sle.WriteMessage(conn, &sle.Message{Type: sle.MessageSLEPDU, Body: pdu})
    }

    // Take in whatever arrived.
    message, err := sle.ReadMessage(conn, sle.DefaultMaxMessageSize)
    if err != nil { ... }
    if message.Type != sle.MessageSLEPDU {
        continue                       // a heartbeat, or the context message
    }

    event, err := user.HandlePDU(message.Body, time.Now())
    if err != nil { ... }

    switch event.Operation {
    case sle.OpBindReturn:
        user.Start(time.Now(), randomNumber(),
            sle.ConditionalTime{}, sle.ConditionalTime{}, sle.FrameQualityAll)
    case sle.OpTransferBuffer:
        for _, frame := range event.TransferBuffer.Frames() {
            handle(frame.Data)         // a CADU; pkg/tmsc.UnwrapCADU opens it
        }
    }
}
```

The loop is yours. The machine holds no socket, starts no goroutine and runs
no clock — `time.Now()` goes in as an argument. `Association.HeartbeatDue` and
`Association.PeerDead` are deadline hints for the same loop.

### Timers are the caller's

The specs put a timer on every confirmed operation: send an invocation, start
a return timer, and PEER-ABORT if it expires. This package does not run that
timer. `ServiceUser.Outstanding()` returns the invoke identifiers still
waiting, so your loop can time them however it already times things.

### What each service does differently

**RCF** filters by channel. Its START carries a `GVCID` — spacecraft, frame
version, virtual channel — instead of a frame quality, because RCF only ever
delivers good frames. Watch the version number: USLP is 12, not 4. The name is
"Version 4" but the wire field is the four-bit Transfer Frame Version Number,
`'1100'`.

**ROCF** filters harder still: a channel, a control word type, and an update
mode. `UpdateChangeBased` delivers a control field only when it differs from
the last one sent, which matters because a CLCW usually repeats unchanged for
many frames. The four octets it delivers go to `pkg/cop`'s CLCW decoder.

**FCLTU** runs the other way and is the only one with counters. Every CLTU
carries an identification number that must climb without gaps: the first is
the number the START asked for, and the count advances as each CLTU is sent —
so CLTUs pipeline without waiting for returns, which is how the spec expects
an uplink to run. `FCLTUUser` keeps the count for you and `TransferData`
returns the number it used. When the provider refuses a CLTU it says which
number it wanted, and the machine resynchronises from the refusal rather than
needing a new START. THROW-EVENT invocations are numbered the same way, by
the machine.

FCLTU is also asynchronous in a way the return services are not. A
TRANSFER-DATA return only says the CLTU was queued. Whether it reached the
antenna arrives later, in an ASYNC-NOTIFY.

## Delivery modes

The delivery mode is fixed by the service agreement before the session, not
negotiated. It decides what happens when data arrives faster than the user
takes it:

| Mode | The provider... | So the caller... |
|---|---|---|
| return timely online | may discard to stay current | reads a discard notification when it happens |
| return complete online | delivers everything in order | must be the brake |
| return offline | reads a store, not a channel | may ask for a past time range |
| forward online | radiates as CLTUs arrive | watches the buffer figure |
| forward offline | queues for a later pass | — |

In this package the mode is state, not an engine. The machines hold one PDU at
a time and never queue, so the buffering that distinguishes the modes lives in
your code. What the library does is refuse what the mode forbids and tell you
what the mode asks of you, through `AllowsDiscard`, `RequiresBackpressure`,
`AllowsPastStartTime` and `AllowsPeriodicStatusReport`.

## Aborts and authentication levels

A PEER-ABORT goes out twice under ISP1 (§3.4): as the `[104]` PDU — a
primitive element holding the bare diagnostic octet, `9F 68 01 xx` on the
wire — and as one octet of TCP **urgent data** before the connection closes.
The library encodes the PDU and gives you the octet (`PeerAbort.UrgentData`);
writing it out of band (`MSG_OOB`) and closing the socket are yours, because
the socket is. An urgent octet you read lands in
`Association.HandleUrgentData`.

Authentication has three levels, picked by the service agreement and set on
`AssociationConfig.AuthLevel`: `AuthLevelNone` checks nothing,
`AuthLevelBind` (the default) checks the BIND exchange, and `AuthLevelAll`
checks every PDU — each `HandlePDU` path verifies the credentials, transfer
buffer entries included, before the machine acts on the PDU.

## Reference

- [CCSDS 913.1-B-2](https://public.ccsds.org/Pubs/913x1b2.pdf) — Internet Protocol for Transfer Services
- [CCSDS 911.1-B-5](https://public.ccsds.org/Pubs/911x1b5e1.pdf) — Return All Frames, whose annex A carries the common ASN.1 modules
- [CCSDS 911.2-B-4](https://public.ccsds.org/Pubs/911x2b4e1.pdf) — Return Channel Frames
- [CCSDS 911.5-B-4](https://public.ccsds.org/Pubs/911x5b4e1.pdf) — Return Operational Control Fields
- [CCSDS 912.1-B-5](https://public.ccsds.org/Pubs/912x1b5e1.pdf) — Forward CLTU
- [ITU-T X.690](https://www.itu.int/rec/T-REC-X.690) — BER, CER and DER
- [Conformance](/conformance/sle)
