---
title: Space Packet Protocol
short: SPP
description: CCSDS 133.0-B-2, the packet that carries application data across a mission.
identifiers:
  - "CCSDS 133.0-B-2 * Space Packet Protocol"
  - "pkg/spp * astro spp"
order: 10
---

> **CCSDS 133.0-B-2** | [Blue Book](https://public.ccsds.org/Pubs/133x0b2e2.pdf) | [`pkg/spp`](https://github.com/ravisuhag/astro/tree/main/pkg/spp) | [`astro spp`](/cli/spp)

## Overview

A Space Packet is the unit an application sends. It carries a 6-byte header and a payload of 1 to 65,536 bytes. The header names the application it came from or is going to, and counts packets so the receiver can spot gaps.

Packets do not travel alone. A data link protocol packs them into transfer frames for the trip. See [the stack](/docs/start/concepts) for how the layers fit together.

## Scope

**Implemented.** The full packet format. Both service interfaces from the standard: Packet Service (clause 3.3) and Octet String Service (clause 3.4). Per-APID receive configuration, sequence counting with gap detection, and idle packet handling.

**Left to you.** Secondary header contents. The standard says these are mission-defined, so `pkg/spp` takes an interface and moves octets. It does not know what your timestamp looks like.

**Not in the standard, offered anyway.** A 2-byte CRC at the end of the data field. See [Error Control](#error-control) below.

## Field map

The 6-byte Packet Primary Header, and where each field lives in Go.

| Field | Bits | Go | Notes |
|---|---|---|---|
| Packet Version Number | 3 | `PrimaryHeader.Version` | Always `0`. Anything else is rejected. |
| Packet Type | 1 | `PrimaryHeader.Type` | `PacketTypeTM` (0) or `PacketTypeTC` (1) |
| Secondary Header Flag | 1 | `PrimaryHeader.SecondaryHeaderFlag` | Must agree with `SpacePacket.SecondaryHeader` |
| APID | 11 | `PrimaryHeader.APID` | 0-2047. `APIDIdle` (`0x7FF`) means fill. |
| Sequence Flags | 2 | `PrimaryHeader.SequenceFlags` | `SeqFlagContinuation`, `FirstSegment`, `LastSegment`, `Unsegmented` |
| Packet Sequence Count | 14 | `PrimaryHeader.SequenceCount` | 0-16383, counted per APID |
| Packet Data Length | 16 | *derived* | Written as length-1. You never set it. |

The Packet Data Field that follows:

| Part | Go | Notes |
|---|---|---|
| Secondary Header | `SpacePacket.SecondaryHeader` | Optional, mission-defined, at least 1 octet |
| User Data | `SpacePacket.UserData` | Your payload |
| Error Control | `SpacePacket.ErrorControl` | Optional, not CCSDS. See below. |

Constants live in `header.go`. A whole packet is 7 to 65,542 bytes.

## Gotchas

**A packet cannot be empty.** It needs a secondary header or user data, or both. Neither one gives you `ErrEmptyPacket`. This is constraint C1/C2 in the standard.

**Packet Data Length is length minus one.** Astro writes and reads this for you, so the number in `PrimaryHeader.PacketLength` is the raw wire value, not the byte count. Decoded packets carry what was on the wire.

**The Secondary Header Flag and the header must agree.** The flag is the only signal the standard gives a receiver (clause 4.1.3.3.3.2). Setting one without the other fails at encode time with `ErrSecondaryHeaderFlagClear` or `ErrSecondaryHeaderMissing`. `WithSecondaryHeader()` sets both.

**A secondary header has a fixed shape.** Clause 4.1.4.2.1.5 allows exactly three layouts: a Time Code Field alone, an Ancillary Data Field alone, or a Time Code Field followed by an Ancillary Data Field. Clause 4.1.4.2.1.6 says the choice must hold for a managed data path across every mission phase, so one APID means one layout. The `SecondaryHeader` interface sees only octets and cannot check this: it is your job. What Astro does check: the header is at least 1 octet, encodes to exactly the size it declares, and fits in the data field.

**Idle packets carry no secondary header.** Clause 4.1.3.3.3.4 forbids it, and Astro returns `ErrIdleWithSecondaryHeader`. Build them with `NewIdlePacket(fill)`. An idle packet is telemetry unless you pass `WithPacketType(PacketTypeTC)`, nothing in the standard ties the idle APID to a direction.

**A relay must not renumber.** If you decode a packet and send it on, Astro keeps the original sequence count. The count belongs to the application that made the packet (clause 4.1.3.4.3.3), and clause 3.3.1 says Packet Service data travels "without further formatting". Pinning a count yourself with `WithSequenceCount()` has the same effect.

**Sequence counts are per APID.** A gap on APID 100 says nothing about APID 200. The service tracks each separately and reports skips in `Indication.DataLoss`.

### Error control

The optional 2-byte CRC-16-CCITT at the end of the data field is **not part of CCSDS 133.0-B-2**. It is a mission and PUS-style convention that lives inside the data field, which the standard leaves to the mission, so it stays wire-compatible. Astro offers it because most missions want it.

It covers the whole packet up to but not including itself. Polynomial `0x1021`, initial value `0xFFFF`. Turn it on with `WithErrorControl()` when sending and `WithDecodeErrorControl()` when receiving. A receiver that does not expect it will read the two bytes as payload.

## Quick start

```go
// Create a service over any io.ReadWriter (TCP conn, serial port, etc.)
svc := spp.NewService(conn, spp.ServiceConfig{
    PacketType: spp.PacketTypeTM,
})

// Send raw bytes, packet construction is handled automatically
err := svc.SendBytes(100, []byte("temperature=22.5"))

// Receive, returns the OCTET_STRING.indication parameters
ind, err := svc.ReceiveBytes()
fmt.Println(ind.APID, ind.Data, ind.SecondaryHeaderIndicator, ind.DataLoss)
```

## Service layer

The `Service` type provides two CCSDS-defined service interfaces over an `io.ReadWriter` transport:

- **Packet Service** (CCSDS 3.3), send and receive pre-built `SpacePacket` values
- **Octet String Service** (CCSDS 3.4), send and receive raw byte data, with automatic packet wrapping

```go
svc := spp.NewService(conn, spp.ServiceConfig{
    PacketType:      spp.PacketTypeTM,
    MaxPacketLength: 1024, // optional, defaults to 65542
    ErrorControl:    true, // optional, validate CRC on received packets
    DiscardIdle:     true, // optional, drop received idle packets (APID 0x7FF)

    // Optional decoder for inbound secondary headers. It is a factory, not a
    // single value: every received packet gets its own header, so one packet's
    // values are never overwritten by the next one's.
    NewSecondaryHeader: func() spp.SecondaryHeader { return &TimestampHeader{} },
})
```

### Per-APID receive configuration

CCSDS 133.0-B-2 manages the secondary header contents per APID (table 5-1), so two APIDs on one transport may carry different header formats, and one may carry a trailing CRC while another does not. `ServiceConfig.APIDs` overrides the receive-side handling for chosen APIDs:

```go
svc := spp.NewService(conn, spp.ServiceConfig{
    PacketType:         spp.PacketTypeTM,
    NewSecondaryHeader: func() spp.SecondaryHeader { return &TimestampHeader{} },
    ErrorControl:       true,

    APIDs: map[uint16]spp.APIDConfig{
        // APID 200 carries a different secondary header and no CRC.
        200: {NewSecondaryHeader: func() spp.SecondaryHeader { return &PositionHeader{} }},
        // Idle packets carry neither.
        0x7FF: {},
    },
})
```

An entry replaces both service-wide settings for its APID, including their zero values. APIDs without an entry keep the service-wide behavior.

### Octet String Service

The simplest way to send and receive data. The service wraps your bytes in a valid space packet automatically:

```go
// Send raw bytes
err := svc.SendBytes(100, []byte("payload data"))

// Send with all the request parameters of CCSDS 3.4.3.2.2
err := svc.SendBytes(100, data,
    spp.WithSendSecondaryHeader(myHeader),   // Secondary Header Indicator
    spp.WithSendPacketType(spp.PacketTypeTC), // Packet Type
    spp.WithSendSequenceCount(42),            // Packet Sequence Count
    spp.WithSendErrorControl(),
)

// Receive, returns the indication parameters of CCSDS 3.4.3.3.2
ind, err := svc.ReceiveBytes()
```

`ReceiveBytes` returns an `Indication`:

| Field | Meaning |
|---|---|
| `Data` | The octet string |
| `APID` | The managed data path it arrived on |
| `SecondaryHeaderIndicator` | Whether the packet carried a Packet Secondary Header |
| `DataLoss` | Whether the sequence count for this APID skipped ahead |
| `PacketsLost` | How many packets the count skipped |

### Packet service

For full control over the packet structure, build a `SpacePacket` and send it directly:

```go
// Send a pre-built packet (sequence count is stamped automatically)
err := svc.SendPacket(packet)

// Receive and decode a packet
packet, err := svc.ReceivePacket()

// Receive with the PACKET.indication parameters of CCSDS 3.3.3.3.2.
// The loss figures are bound to the returned packet.
ind, err := svc.ReceivePacketIndication()
if ind.PacketLoss {
    log.Printf("%d packet(s) lost on APID %d", ind.PacketsLost, ind.APID)
}
```

### QoS requirement

`WithQoS` attaches the optional QoS Requirement of PACKET.request (CCSDS 3.3.2.4), which selects a service level when the underlying subnetwork offers more than one, for example Type-A versus Type-B service on a telecommand link. The transport declares support by implementing `QoSWriter`; without it, a send carrying a QoS requirement is refused with `ErrQoSUnsupported` before anything reaches the wire:

```go
err := svc.SendPacket(packet, spp.WithQoS(1))
```

What each level means belongs to the transport. The Octet String Service has no QoS parameter (3.4.3.2.2), so `SendBytes` takes no QoS option.

### Sequence counting

The service automatically maintains a per-APID 14-bit sequence counter (CCSDS 133.0-B-2 4.1.3.4.3). Each call to `SendPacket` or `SendBytes` stamps the packet with the next count for its APID and wraps at 16383.

Pinning a count with `WithSequenceCount` (or `WithSendSequenceCount`) does not break the run: the service resynchronizes its counter to one past the pinned value, so the APID's count stays continuous as 4.1.3.4.3.4 requires.

### Loss detection

Every received packet is checked for sequence count continuity on its APID, modulo 16384 (CCSDS 4.3.2.2). `ReceiveBytes` reports the result on the `Indication` and `ReceivePacketIndication` on the `PacketIndication`, bound to the delivered packet. After a plain `ReceivePacket`, read it with `LastDataLoss()`, but note the figure is service-wide, so with concurrent receivers prefer `ReceivePacketIndication`:

```go
packet, err := svc.ReceivePacket()
if lost := svc.LastDataLoss(); lost > 0 {
    log.Printf("%d packet(s) lost before this one", lost)
}

// After a link outage, the gap across the break means nothing.
svc.ResetContinuity()
```

The first packet seen on an APID never reports a loss.

## Creating packets

For use cases outside the Service layer (testing, offline encoding, custom transports), construct packets directly:

```go
// Telemetry packet with APID 100
packet, err := spp.NewTMPacket(100, []byte("temperature=22.5"))

// Telecommand packet with APID 200
packet, err := spp.NewTCPacket(200, []byte("SET_MODE=SAFE"))

// Generic constructor with explicit type
packet, err := spp.NewSpacePacket(100, spp.PacketTypeTM, data)
```

### Packet options

Options configure optional fields:

```go
// With error control (CRC-16-CCITT, auto-computed during Encode)
packet, err := spp.NewTMPacket(100, data, spp.WithErrorControl())

// With a mission-specific secondary header
packet, err := spp.NewTMPacket(100, data, spp.WithSecondaryHeader(myHeader))

// With manual sequence count and flags (for packets built outside a Service)
packet, err := spp.NewTMPacket(100, data,
    spp.WithSequenceCount(42),
    spp.WithSequenceFlags(spp.SeqFlagFirstSegment),
)

// Combining options
packet, err := spp.NewTMPacket(100, data,
    spp.WithSecondaryHeader(myHeader),
    spp.WithErrorControl(),
)
```

### Inspecting packets

```go
// Check if a packet is an idle packet (APID 0x7FF)
if packet.IsIdle() { ... }

// Same question about a raw encoded packet
if spp.IsIdleBytes(raw) { ... }

// Human-readable dump for debugging
fmt.Println(packet.Humanize())
```

### Packet sizing

Two functions read a packet's length, and which one you want depends on whether you already hold the octets.

`PacketSizer` returns the length of the **complete** packet at the front of a buffer, and -1 when the buffer does not hold all of it. That is what the data link packet services need: a packet reaching past the end of the reassembly buffer is not a packet yet, so -1 tells them to pull another frame. Wire it up with `SetPacketSizer`:

```go
vcp.SetPacketSizer(spp.PacketSizer)

n := spp.PacketSizer(buf)
if n > 0 {
    packet := buf[:n] // safe: the whole packet is there
}
```

`DeclaredPacketSize` returns what the 6-octet primary header claims, whether or not the body has arrived. A stream reader needs this, since it must know how many octets to fetch before it has them:

```go
total := spp.DeclaredPacketSize(header) // header need only be 6 octets
```

## Encoding and decoding

```go
// Encode a packet to bytes for transmission
encoded, err := packet.Encode()

// Decode bytes back into a packet
decoded, err := spp.Decode(encoded)

// Decode with a secondary header decoder
decoded, err := spp.Decode(encoded, spp.WithDecodeSecondaryHeader(&MySecondaryHeader{}))

// Decode with error control (CRC) validation
decoded, err := spp.Decode(encoded, spp.WithDecodeErrorControl())

// Combine decode options
decoded, err := spp.Decode(encoded,
    spp.WithDecodeSecondaryHeader(&MySecondaryHeader{}),
    spp.WithDecodeErrorControl(),
)
```

When decoding a packet that has the secondary header flag set, you can pass a `SecondaryHeader` implementation via `WithDecodeSecondaryHeader`. If none is provided, the secondary header bytes are included in `UserData`; the packet still re-encodes byte for byte, so a relay can forward a packet whose secondary header format it does not know.

The Secondary Header Flag is the only signal that a header is present (CCSDS 4.1.3.3.3.2), so it decides both whether the header's octets are written and whether its size counts towards the Packet Data Length. `Encode` and `Validate` refuse a packet where the flag and the `SecondaryHeader` field disagree.

When `WithDecodeErrorControl()` is used, the trailing 2 bytes are extracted as a CRC-16-CCITT checksum and verified against the packet contents. If the CRC does not match, `ErrCRCValidationFailed` is returned.

## Secondary headers

The secondary header's contents and length are mission-defined (CCSDS 4.1.4.2.1.4). Its layout is not free-form: 4.1.4.2.1.5 allows a Time Code Field alone, an Ancillary Data Field alone, or a Time Code Field followed by an Ancillary Data Field, and 4.1.4.2.1.6 requires the same choice throughout a managed data path's life. The interface sees only octets, so keeping to one of those three shapes is your implementation's job.

Implement the `SecondaryHeader` interface:

```go
type SecondaryHeader interface {
    Encode() ([]byte, error)
    Decode([]byte) error
    Size() int  // fixed size in bytes (at least 1)
}
```

Example implementation:

```go
type TimestampHeader struct {
    Seconds     uint32
    Subseconds  uint16
}

func (h *TimestampHeader) Encode() ([]byte, error) {
    buf := make([]byte, 6)
    binary.BigEndian.PutUint32(buf[0:4], h.Seconds)
    binary.BigEndian.PutUint16(buf[4:6], h.Subseconds)
    return buf, nil
}

func (h *TimestampHeader) Decode(data []byte) error {
    if len(data) < 6 {
        return errors.New("insufficient data for timestamp header")
    }
    h.Seconds = binary.BigEndian.Uint32(data[0:4])
    h.Subseconds = binary.BigEndian.Uint16(data[4:6])
    return nil
}

func (h *TimestampHeader) Size() int { return 6 }
```

## Errors

All errors are exported package-level variables, suitable for use with `errors.Is`:

| Error | Meaning |
|---|---|
| `ErrInvalidVersion` | Version is not 0 |
| `ErrInvalidType` | Type is not 0 or 1 |
| `ErrInvalidAPID` | APID outside 0-2047 |
| `ErrInvalidSequenceFlags` | Sequence flags outside 0-3 |
| `ErrInvalidSequenceCount` | Sequence count outside 0-16383 |
| `ErrInvalidHeader` | Header does not conform to CCSDS |
| `ErrEmptyPacket` | Packet has no secondary header and no user data (C1/C2) |
| `ErrNilPacket` | Nil packet provided |
| `ErrPacketTooLarge` | Total packet size outside 7-65542 bytes |
| `ErrDataTooShort` | Input data too short to decode |
| `ErrPacketLengthMismatch` | Data field size doesn't match header length |
| `ErrSecondaryHeaderMissing` | Flag is set on a hand-built packet but no secondary header provided |
| `ErrSecondaryHeaderFlagClear` | A secondary header is attached but the flag is 0 |
| `ErrSecondaryHeaderTooSmall` | Secondary header less than 1 byte |
| `ErrSecondaryHeaderSizeMismatch` | `Encode()` returned a byte count different from `Size()` |
| `ErrSecondaryHeaderExceedsDataField` | The configured decoder is wider than the packet's data field |
| `ErrIdleWithSecondaryHeader` | An idle packet (APID 0x7FF) carries a secondary header |
| `ErrCRCValidationFailed` | CRC integrity check failed |

## Notes

Commentary, not sourced from the standard. Read it as one implementer's reading of why the format looks the way it does.

**Why 11 bits of APID?** 2,048 addresses is generous for a spacecraft (most missions use fewer than a hundred) and it keeps the header at 6 bytes. On a 1 kbps downlink every byte is real money.

**Why a separate type bit?** Telemetry and telecommand usually run through different hardware onboard. One bit at a fixed offset lets a router decide where a packet goes without parsing anything else.

**Why length minus one?** It removes the question of whether zero means "empty" or "one byte", and it makes the standard's rule that every packet carries at least one byte of data fall out of the encoding for free.

**Why count per APID?** A star tracker and a thermistor produce data at wildly different rates. A shared counter would wrap too fast for one and waste range on the other. Per-APID counting also keeps one application's packet loss from looking like everyone's.

## Reference

- [CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2e2.pdf), Space Packet Protocol (Blue Book)
- [CCSDS 133.0-G-1](https://public.ccsds.org/Pubs/133x0g1.pdf), Space Packet Protocol Summary (Green Book)
- [CLI](/cli/spp) | [Conformance](/conformance/spp)
