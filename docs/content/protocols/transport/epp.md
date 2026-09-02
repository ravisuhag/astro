---
title: Encapsulation Packet Protocol
short: EPP
description: CCSDS 133.1-B-3 — a thin wrapper for carrying IP and other non-CCSDS data.
order: 11
---

> **CCSDS 133.1-B-3** | [Blue Book](https://public.ccsds.org/Pubs/133x1b3e1.pdf) | [`pkg/epp`](https://github.com/ravisuhag/astro/tree/main/pkg/epp) | [`astro epp`](/cli/epp)

EPP wraps data that is not a Space Packet so it can travel on a CCSDS data link. An IP datagram, an LTP segment, anything with its own addressing. The header is 1, 2, 4, or 8 octets and does almost nothing except say what is inside.

That is the whole point. [SPP](/protocols/transport/spp) gives you APID routing, sequence counting, and segmentation. If your payload already has all that, EPP is the cheaper way to get it down the link.

The first three bits are always `111`, where a Space Packet has `000`. That is what lets both share one data link.

## Scope

**Implemented.** All four header sizes, every Protocol ID including the extension mechanism, idle packets in both the 1-octet and fill forms, and a service layer over any `io.ReadWriter`.

**Left to you.** What is inside. EPP identifies the payload protocol and gets out of the way.

## Field map

The header is variable length. Its size is decided entirely by the 2-bit Length of Length field in octet 0 (table 4-1). Go fields on `epp.Header`.

| Field | Bits | Go | Present in |
|---|---|---|---|
| Packet Version Number | 3 | `PVN` | every header. Always `7`. |
| Encapsulation Protocol ID | 3 | `ProtocolID` | every header |
| Length of Length | 2 | `LengthOfLength` | every header |
| User Defined Field | 4 | `UserDefined` | 4- and 8-octet headers |
| Protocol ID Extension | 4 | `ExtendedProtocolID` | 4- and 8-octet headers |
| CCSDS Defined Field | 16 | `CCSDSDefined` | 8-octet header only |
| Packet Length | 8/16/32 | `PacketLength` | all but the 1-octet header |

`Header.Size()` returns the octet count: `1 << LengthOfLength`.

| LoL | Header | Packet Length field | Max total packet |
|---|---|---|---|
| `00` | 1 octet | none | 1 octet — idle only |
| `01` | 2 octets | 8 bits | 255 octets |
| `10` | 4 octets | 16 bits | 65,535 octets |
| `11` | 8 octets | 32 bits | 4,294,967,295 octets |

### Protocol IDs

From clause 4.1.2.3 and the SANA Encapsulation Protocol ID registry.

| Value | Constant | Carries |
|---|---|---|
| 0 | `ProtocolIDIdle` | Fill data |
| 1 | `ProtocolIDLTP` | [Licklider Transmission Protocol](/protocols/transport/ltp) |
| 2 | `ProtocolIDIPE` | IPv4 or IPv6, via the Internet Protocol Extension |
| 3-5 | — | Reserved |
| 6 | `ProtocolIDExtended` | Whatever the Protocol ID Extension names |
| 7 | `ProtocolIDMission` | Mission-specific, privately defined |

## Gotchas

**Packet Length is the whole packet, header included.** This is the opposite of [SPP](/protocols/transport/spp), where the length field is the data field size minus one. Clause 4.1.2.8.2. Getting the two confused is the classic EPP bug.

**A 1-octet header must be idle.** Length of Length `00` means no length field and no data zone, so the only thing it can legally be is the idle packet `0xE0`. Anything else gives `ErrNonIdleOneOctetHeader`.

**Protocol ID 6 needs a 4- or 8-octet header.** The extension field does not exist in the shorter headers, so asking for `ProtocolIDExtended` with a 2-octet header gives `ErrExtendedNeedsLongHeader`.

**The extension field must be zero unless Protocol ID is 6.** `ErrExtensionMustBeZero`. Same idea in reverse: setting a field the Protocol ID does not activate.

**Fields that do not fit are refused, not dropped.** Setting a User Defined Field on a 2-octet header gives `ErrFieldNeedsLongerHeader` rather than silently discarding it.

**A packet with no data field is only legal when idle.** Non-idle packets must carry something — `ErrEmptyData`. For multi-octet fill, use `NewIdleFillPacket(totalLength, fillByte)`.

header rules, see the [protocol page](/protocols/transport/epp).

## Quick Start

```go
// Create a service over any io.ReadWriter (TCP conn, serial port, etc.)
svc := epp.NewService(conn, epp.ServiceConfig{})

// Send raw bytes — packet construction is handled automatically
err := svc.SendBytes(epp.ProtocolIDIPE, ipv4Datagram)

// Receive — returns Protocol ID and data
pid, data, err := svc.ReceiveBytes()
```

## Service Layer

The `Service` type provides send/receive operations over an `io.ReadWriter` transport:

```go
svc := epp.NewService(conn, epp.ServiceConfig{
    MaxPacketLength: 65535, // optional; defaults to 4,294,967,295 (no spec-valid packet rejected)
})
```

### Byte-Level Service

The simplest way to send and receive data. The service wraps your bytes in a valid encapsulation packet automatically:

```go
// Send an IPv4 datagram
err := svc.SendBytes(epp.ProtocolIDIPE, ipv4Datagram)

// Send with an extended protocol ID
err := svc.SendBytes(epp.ProtocolIDExtended, data,
    epp.WithExtendedProtocolID(9),
)

// Receive — returns Protocol ID and data zone
pid, data, err := svc.ReceiveBytes()
```

### Packet-Level Service

For full control over the packet structure, build an `EncapsulationPacket` and send it directly:

```go
// Send a pre-built packet
err := svc.SendPacket(packet)

// Receive and decode a packet
packet, err := svc.ReceivePacket()
```

## Creating Packets

For use cases outside the Service layer (testing, offline encoding, custom transports), construct packets directly:

```go
// Internet Protocol Extension packet (carries IPv4/IPv6)
packet, err := epp.NewIPEPacket(ipDatagram)

// LTP packet
packet, err := epp.NewLTPPacket(segment)

// Mission-specific (privately defined) packet
packet, err := epp.NewMissionPacket(payload)

// 1-octet idle packet (0xE0)
packet, err := epp.NewIdlePacket()

// Idle fill packet of an exact total size (for frame fill)
packet, err := epp.NewIdleFillPacket(120, 0xFF)

// Generic constructor with explicit Protocol ID
packet, err := epp.NewPacket(epp.ProtocolIDIPE, data)
```

`NewPacket` picks the smallest header that fits the data. Passing more data
than an 8-bit length field can describe automatically selects the 4-octet
header, and so on.

### Packet Options

Options configure the header size and optional fields:

```go
// Force at least a 4-octet header (2-octet length field)
packet, err := epp.NewIPEPacket(data, epp.WithLongLength())

// Set the 4-bit User Defined Field (needs a 4- or 8-octet header)
packet, err := epp.NewMissionPacket(data, epp.WithUserDefined(0xA))

// Extended Protocol ID: sets PID '110' and the 4-bit extension value
packet, err := epp.NewPacket(epp.ProtocolIDExtended, data,
    epp.WithExtendedProtocolID(9),
)

// CCSDS Defined Field (forces the 8-octet header)
packet, err := epp.NewPacket(epp.ProtocolIDExtended, data,
    epp.WithExtendedProtocolID(9),
    epp.WithCCSDSDefined(0x1234),
)
```

### Inspecting Packets

```go
// Check if a packet is an idle packet
if packet.IsIdle() { ... }

// Human-readable dump for debugging
fmt.Println(packet.Humanize())
```

### Packet Sizing

The `PacketSizer` function returns the total packet length from the header bytes of an Encapsulation Packet. It implements the `sdl.PacketSizer` signature, allowing EPP packets to be extracted from fixed-length transfer frames by the `tmdl` and `tcdl` service layers:

```go
// PacketSizer returns the total packet length from header bytes.
totalLen := epp.PacketSizer(headerBytes)
```

## Encoding and Decoding

```go
// Encode a packet to bytes for transmission
encoded, err := packet.Encode()

// Decode bytes back into a packet
decoded, err := epp.Decode(encoded)

// Access decoded fields
fmt.Println(decoded.Header.ProtocolID)
fmt.Println(decoded.Header.Size())
fmt.Println(decoded.Data)
```

## Full Pipeline Example

### Send Path

```go
// Create an EPP packet carrying an IPv4 datagram
packet, err := epp.NewIPEPacket(ipv4Datagram)

// Encode to bytes
encoded, err := packet.Encode()

// Frame in a TM Transfer Frame (via tmdl)
frame, err := tmdl.NewTMTransferFrame(0x1A, 1, encoded, nil, nil)
```

### Receive Path

```go
// Extract packet bytes from a transfer frame
data := frame.DataField

// Decode the Encapsulation Packet
packet, err := epp.Decode(data)

// Access the encapsulated datagram
fmt.Printf("Protocol ID: %d\n", packet.Header.ProtocolID)
fmt.Printf("Data: %x\n", packet.Data)
```

## Errors

All errors are exported package-level variables, suitable for use with `errors.Is`:

| Error | Meaning |
|-------|---------|
| `ErrInvalidPVN` | PVN is not 7 ('111') |
| `ErrInvalidProtocolID` | Protocol ID outside 0-7 |
| `ErrInvalidLengthOfLength` | Length of Length field outside 0-3 |
| `ErrInvalidUserDefined` | User Defined Field does not fit in 4 bits |
| `ErrInvalidExtendedProtocolID` | Protocol ID Extension does not fit in 4 bits |
| `ErrNonIdleOneOctetHeader` | 1-octet header with a non-idle Protocol ID |
| `ErrExtendedNeedsLongHeader` | Protocol ID '110' with a header under 4 octets |
| `ErrExtensionMustBeZero` | Non-zero extension with a Protocol ID other than '110' |
| `ErrFieldNeedsLongerHeader` | Field set that the selected header size cannot carry |
| `ErrIdleWithData` | 1-octet idle packet given a data zone |
| `ErrEmptyData` | Non-idle packet has no data |
| `ErrDataTooShort` | Input data too short to decode |
| `ErrPacketLengthMismatch` | Packet length field doesn't match actual size |
| `ErrPacketTooLarge` | Packet exceeds maximum for header format |
| `ErrInvalidIdleLength` | Idle fill packet requested with total length < 1 |
| `ErrNilPacket` | Nil packet provided |

## Notes

Commentary, not sourced from the standard.

**Why four header sizes?** A 1-octet idle packet costs almost nothing, and a mission moving multi-gigabyte files needs a 32-bit length. Making the size a function of two bits means a receiver knows how much to read after the very first octet.

**Why `111` for the version?** It had to be a value SPP would never use, so the two can share a link and a receiver can tell them apart from bit one.

**Why total length instead of length-minus-one?** EPP's job is to be simple to parse for things that are not CCSDS-native. "How many bytes is this packet" is the question a wrapper should answer directly.

## Reference

- [CCSDS 133.1-B-3](https://public.ccsds.org/Pubs/133x1b3e1.pdf) — Encapsulation Packet Protocol (Blue Book)
- [CLI](/cli/epp) | [Conformance](/conformance/epp) | [The stack](/docs/start/concepts)