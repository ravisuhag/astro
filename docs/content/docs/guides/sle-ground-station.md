---
title: Pull frames from a ground station
short: Ground station
description: The link between two ground systems, where a control centre asks an antenna for what it heard.
order: 13
---

The [downlink guide](/docs/guides/downlink) ends at the ground station. That is not where the data is needed. It is needed at a mission control centre, which is usually somewhere else entirely, and the two are joined by a TCP connection rather than a radio.

[SLE](/protocols/ground/sle) is the protocol for that hop. This guide opens a Return All Frames session and pulls a frame across it.

The complete program is [`examples/sle`](https://github.com/ravisuhag/astro/tree/main/examples/sle). Run it:

```bash
go run ./examples/sle/
```

## What we are building

```
Mission control                             Ground station
(SLE user)                                  (SLE provider)
      │                                                  │
      │  TML context (heartbeat, dead factor) ──────────►│
      │  BIND ──────────────────────────────────────────►│
      │◄─────────────────────────────────── BIND return  │
      │  START (all qualities, from now) ───────────────►│
      │◄────────────────────────────────── START return  │
      │                                                  │
      │◄─────── transfer buffer, a CADU off the antenna  │
      │                                                  │
      │  STOP ──────────────────────────────────────────►│
      │  UNBIND ────────────────────────────────────────►│
      │                                                  │
```

A real TCP connection on localhost, and a real CADU that `pkg/tmdl` framed and `pkg/tmsc` wrapped.

## Four services, one shape

| Service | Standard | Carries |
|---|---|---|
| RAF | CCSDS 911.1 | Every frame on a physical channel |
| RCF | CCSDS 911.2 | Frames from one virtual channel |
| ROCF | CCSDS 911.5 | Just the operational control fields, so the [CLCWs](/protocols/data-link/cop) |
| FCLTU | CCSDS 912.1 | [CLTUs](/protocols/coding/tcsc) going up |

The first three come down and the last goes up, but the session is the same shape in all four: bind, start, move data, stop, unbind. This guide uses RAF, the one that hands you everything.

## An association, then a service on top of it

The association is the connection: who both ends are, how often they check on each other, and how they authenticate.

```go
association, err := sle.NewAssociation(sle.AssociationConfig{
    Role:              sle.RoleUser,
    LocalIdentifier:   "CTRL-CENTRE",
    PeerIdentifier:    "GROUND-STN",
    HeartbeatInterval: 30,
    DeadFactor:        3,
    UserName:          "CTRL-CENTRE",
    Password:          []byte("user-secret"),
    PeerPassword:      []byte("provider-secret"),
    AcceptableDelay:   time.Minute,
})
```

`AcceptableDelay` is how far a peer's credential timestamp may be from now before it is refused. That is the replay defence, and it means both ends need a clock that agrees to within about a minute.

Credentials hash with SHA-256. The previous issue of the standard used SHA-1, and a 20-octet legacy digest still decodes here, but it cannot be verified, because this package does not implement the superseded scheme.

The service instance sits on the association and names one configured service at the provider:

```go
user, err := sle.NewRAFUser(sle.ServiceConfig{
    Association:   association,
    DeliveryMode:  sle.DeliveryReturnCompleteOnline,
    Version:       5,
    ResponderPort: "GROUND-PORT",
    Instance: sle.ServiceInstanceIdentifier{
        {Identifier: "sagr", Value: "DEMOSAT"},
        {Identifier: "spack", Value: "PASS-0417"},
        {Identifier: "rsl-fg", Value: "1"},
        {Identifier: "raf", Value: "onlc1"},
    },
})
```

Those four attributes are the usual RAF form: the service agreement, the pass, the functional group, and the instance. A provider serving several missions routes an inbound BIND by matching this.

## The context message comes first

Before any SLE PDU, the user sends a TML context message. It is the user's to send, and a user that receives one rejects it:

```go
sle.WriteMessage(conn, association.ContextMessage(now))
```

It carries the heartbeat interval and the dead factor, and the provider adopts them rather than proposing its own. So the user decides how chatty the connection is.

## Nothing here reads a clock

Every call takes a `now`, and `pkg/sle` starts no goroutines and holds no timers. The heartbeat is a good example: the package tells you when one is due and when the peer has gone quiet, and your own scheduler acts on it:

```go
if association.HeartbeatDue(now) {
    sle.WriteMessage(conn, sle.HeartbeatMessage())
}
if association.PeerDead(now) {
    // three intervals of silence: the far end is gone
}
```

The TCP I/O is yours too. `WriteMessage` and `ReadMessage` take an `io.Writer` and an `io.Reader`, and `ReadMessage` reads exactly the octets its header promises so that it does not swallow the start of the next message.

## Bind, start, and the session states

Each operation queues a PDU. `NextPDU` hands it over to write, `HandlePDU` takes the answer:

```go
user.Bind(now, 1)
for {
    pdu, ok := user.NextPDU()
    if !ok {
        break
    }
    sle.WriteMessage(conn, &sle.Message{Type: sle.MessageSLEPDU, Body: pdu})
}
```

The second argument to `Bind` is the credential nonce. The example passes a counter so the output is stable; use a real random source in anything that matters.

START is where you say what you want:

```go
user.Start(now, 2, sle.ConditionalTime{}, sle.ConditionalTime{}, sle.FrameQualityAll)
```

Two undefined `ConditionalTime` values mean "from now until I say stop", which is what an online service is for. A pair of known times asks an offline service for a stretch of the archive instead.

`FrameQualityAll` takes good and bad frames both. Asking for good only is tempting and usually wrong: a frame that failed error control still tells you the pass was live and the antenna was pointed.

## Taking delivery

Frames arrive unprompted once started, and they arrive in buffers:

```go
event, err := user.HandlePDU(message.Body, now)
frames := event.TransferBuffer.Frames()
```

The buffering is why RAF scales. A provider recovering frames at line rate does not send one PDU per frame; it fills a buffer and ships it, so the connection carries a few large messages instead of thousands of small ones.

```
  transfer buffer with 1 frame(s)
    antenna ........... DSS-25
    Earth receive time  2026-04-17T08:30:00Z (day 24943, 30600000 ms, 0 us)
    quality ........... good
    continuity ........ -1
    data .............. 262 octets
    spacecraft ........ 42, VC 1, frame 0
    survived intact ... true
```

Each frame comes with the metadata that only the ground station knows: which antenna heard it, when it reached the ground, whether it passed error control, and how many frames were lost since the last delivery.

`continuity -1` means the provider cannot tell. That is a real answer and worth handling, because a station that cannot count gaps is not the same as one reporting zero.

The `Data` is the frame content, so it comes apart the same way it would at the station:

```go
recovered, err := tmsc.UnwrapCADU(frame.Data, tmsc.DefaultASM(), true)
decoded, err := tmdl.DecodeTMTransferFrame(recovered)
```

## Then close it properly

```
  STOP ........ ready
  UNBIND ...... unbound
```

STOP goes back to ready, which means you can START again with different parameters without rebinding. UNBIND ends the association.

## Things that will bite you

**The context message is not optional.** Skip it and the provider has no heartbeat parameters. It is also directional: the user sends, the provider receives, and a user that gets one treats it as an error.

**Answer the START before you deliver.** A transfer buffer that arrives before the START return reaches a user still in the ready state, which is a protocol error rather than an early frame. The provider in the example flushes its queue between the two for exactly that reason.

**Heartbeats are yours to send.** There is no timer in this package. A connection that goes quiet for `HeartbeatInterval * DeadFactor` seconds is dead as far as the peer is concerned, and nothing will have stopped that happening.

**Handle heartbeats before the state machine.** A heartbeat is a TML message with an empty body, not an SLE PDU. Feeding one to `HandlePDU` is an error. Filter on `message.Type` first.

**The BER here is a deliberate subset.** Go's `encoding/asn1` is DER-oriented and rejects the context-specific CHOICE tagging that SLE relies on, so this package carries just enough BER for what SLE actually sends. It emits definite lengths and accepts the indefinite form, because real providers emit it.

**Clocks have to agree.** `AcceptableDelay` rejects credentials whose timestamp is too far from now. A control centre whose clock has drifted a few minutes cannot bind, and the diagnostic does not say "check your clock".

## Next

- [Build a downlink](/docs/guides/downlink), where the frames this service delivers come from
- [Build an uplink](/docs/guides/uplink), which is what FCLTU carries in the other direction
- [SLE protocol page](/protocols/ground/sle) | [Conformance](/conformance/sle) | [CLI](/cli/sle)
