# Space Packet Protocol

> CCSDS 133.0-B-2 — Space Packet Protocol

## Overview

The Space Packet Protocol (SPP) is the fundamental **Network Layer** protocol for transferring application data in space missions. It provides a standardized way to package telemetry, telecommands, and other application data into discrete units called **Space Packets** that can be routed across a spacecraft's onboard data network and between spacecraft and ground systems.

SPP is used by virtually every major space agency — NASA, ESA, JAXA, ISRO, and others — making it the lingua franca of spacecraft application data. Whether a temperature sensor reading leaves a CubeSat or a high-resolution image leaves a Mars rover, the data travels inside Space Packets.

### Where SPP Fits

```
┌─────────────────────────────────────────────┐
│  Application Process (sensors, instruments) │
├─────────────────────────────────────────────┤
│  Space Packet Protocol (SPP)                │  ← Network Layer
│  Packages application data into packets     │
├─────────────────────────────────────────────┤
│  Data Link Layer                            │
│  TM/TC/AOS Space Data Link Protocol         │
│  Carries packets inside Transfer Frames     │
├─────────────────────────────────────────────┤
│  Sync & Channel Coding Layer                │
│  Error correction, synchronization          │
├─────────────────────────────────────────────┤
│  Physical Layer (RF/Optical link)           │
└─────────────────────────────────────────────┘
```

SPP sits between the application processes that generate data and the Data Link Layer protocols (like TMDL) that carry packets over the space link. A single Space Packet is the smallest addressable unit of application data in the CCSDS architecture.

### Key Characteristics

- **Connectionless**: Each packet is self-contained with its own header — no session setup required.
- **Fixed header, variable payload**: The 6-byte primary header is always the same structure; the data field varies from 1 to 65,536 bytes.
- **Dual direction**: The same format serves both telemetry (spacecraft → ground) and telecommand (ground → spacecraft).
- **Application addressing**: Each packet is tagged with an Application Process Identifier (APID) that identifies the source or destination application.
- **Sequencing**: Built-in per-APID sequence counters detect lost or out-of-order packets.

## Packet Structure

Every Space Packet consists of two parts: a **Packet Primary Header** (6 bytes, mandatory) and a **Packet Data Field** (variable length, 1–65,536 bytes).

```
                         Space Packet
┌──────────────────────────┬──────────────────────────────┐
│   Packet Primary Header  │     Packet Data Field        │
│        (6 bytes)         │    (1 to 65,536 bytes)       │
└──────────────────────────┴──────────────────────────────┘
```

Total packet size ranges from **7 bytes** (6-byte header + 1 byte of data) to **65,542 bytes**.

### Packet Primary Header

The 6-byte header is divided into three 16-bit words:

```
Word 1: Packet Identification
┌─────────┬──────┬────────────┬─────────────────────┐
│ Version │ Type │ Sec Header │        APID         │
│  (3b)   │ (1b) │   Flag(1b) │       (11b)         │
└─────────┴──────┴────────────┴─────────────────────┘

Word 2: Packet Sequence Control
┌──────────────────┬─────────────────────────────────┐
│  Sequence Flags  │       Sequence Count            │
│      (2b)        │          (14b)                  │
└──────────────────┴─────────────────────────────────┘

Word 3: Packet Data Length
┌───────────────────────────────────────────────────┐
│              Packet Data Length                    │
│                   (16b)                           │
└───────────────────────────────────────────────────┘
```

#### Packet Version Number (3 bits)

Always `000` for the current version of SPP. This field identifies the packet as a CCSDS Space Packet and distinguishes it from other packet types that might share the same link.

#### Packet Type (1 bit)

| Value | Meaning |
|-------|---------|
| `0` | Telemetry (TM) — data from spacecraft |
| `1` | Telecommand (TC) — commands to spacecraft |

This bit determines the direction of data flow. A spacecraft's onboard router uses this field to decide whether a packet should be processed locally (TC) or forwarded to the downlink (TM).

#### Secondary Header Flag (1 bit)

| Value | Meaning |
|-------|---------|
| `0` | No secondary header present |
| `1` | Secondary header is present |

When set, the Packet Data Field begins with a mission-defined secondary header (typically containing a timestamp). The CCSDS standard does not prescribe the format of the secondary header — it is entirely mission-specific.

#### Application Process Identifier — APID (11 bits)

The APID is the **addressing mechanism** of SPP. It identifies which application process on the spacecraft generated (for TM) or should receive (for TC) the packet. Valid range: 0–2047.

| APID | Usage |
|------|-------|
| `0x000–0x7FE` | Mission-defined application processes |
| `0x7FF` (2047) | Idle packet (fill data, no application meaning) |

A spacecraft might assign APIDs like this:

| APID | Application |
|------|-------------|
| 1 | Attitude determination |
| 2 | Thermal subsystem housekeeping |
| 10 | Star tracker images |
| 100 | Payload science data |
| 200 | Command echo/verification |

The APID space is managed per mission. Ground systems use the APID to route received packets to the correct processing pipeline.

#### Sequence Flags (2 bits)

These flags indicate where a packet falls within a sequence of related packets:

| Value | Name | Meaning |
|-------|------|---------|
| `00` | Continuation | Middle segment of a multi-packet group |
| `01` | First Segment | First packet in a group |
| `10` | Last Segment | Last packet in a group |
| `11` | Unsegmented | Complete, standalone packet |

Most packets use `11` (unsegmented). The segmentation flags are used when a single application data unit is too large to fit in one packet and must be split across multiple packets. The receiving application reassembles the segments in order using the Sequence Count.

#### Sequence Count (14 bits)

A per-APID counter that increments with each packet sent. Range: 0–16,383, wrapping back to 0 after 16,383.

**Purpose:**
- **Loss detection**: If the ground receives packets with counts 41, 42, 44, it knows packet 43 was lost.
- **Ordering**: If packets arrive out of order (possible with some link configurations), the sequence count restores the original order.
- **Duplicate detection**: Two packets with the same APID and sequence count indicate a duplicate.

Each APID maintains its own independent sequence counter. Packet 42 from APID 1 and packet 42 from APID 2 are unrelated.

#### Packet Data Length (16 bits)

The number of bytes in the Packet Data Field **minus one**. This "minus one" convention means:
- A value of `0` indicates 1 byte of data.
- A value of `65,535` indicates 65,536 bytes of data.
- The minimum Packet Data Field is 1 byte; the maximum is 65,536 bytes.

This field allows the receiver to know exactly how many bytes to read after the header without any framing ambiguity.

### Packet Data Field

The Packet Data Field contains the actual payload and optional framing fields:

```
┌─────────────────────┬───────────────────┬──────────────────┐
│  Secondary Header   │   User Data       │  Error Control   │
│  (optional, 1-63B)  │   (variable)      │  (optional, 2B)  │
└─────────────────────┴───────────────────┴──────────────────┘
```

**Constraint (C1/C2):** A packet must contain at least a Secondary Header or User Data. An empty Packet Data Field (no secondary header and no user data) is not valid.

#### Secondary Header

When the Secondary Header Flag is set, the first bytes of the Packet Data Field are a mission-defined secondary header. Common uses:

- **Timestamp**: When the data was acquired (most common use)
- **Packet subcategory**: Further classifying data within an APID
- **Data quality flags**: Indicating sensor status at acquisition time

The CCSDS standard limits the secondary header to **1–63 bytes** and requires the format to be fixed for a given APID — all packets from the same APID must use the same secondary header structure.

#### User Data

The application payload. This is the actual telemetry measurement, command parameters, image data, or whatever the application process needs to transfer. The structure is entirely application-defined.

#### Error Control (optional)

An optional 2-byte CRC-16-CCITT checksum appended at the end of the Packet Data Field. When present, it covers the entire packet (header + data field, excluding the CRC itself) and allows the receiver to detect bit errors.

The polynomial used is the standard CCITT CRC-16: `x^16 + x^12 + x^5 + 1` (0x1021), with an initial value of `0xFFFF`.

## Idle Packets

Idle packets (APID = `0x7FF`) serve as **fill data**. They are used when:
- A fixed-rate link has no real data to send but must keep transmitting.
- Transfer Frame slots need to be filled to maintain the fixed frame length.
- Timing synchronization requires continuous transmission.

Receivers should recognize and discard idle packets. Their data field content is meaningless (typically all `0xFF` or zeros).

## Services

The standard defines two service interfaces for applications to send and receive data:

### Packet Service

The application constructs a complete `SpacePacket` and hands it to the service layer. The service stamps it with a sequence count and transmits it. On the receive side, the service delivers complete decoded packets.

This service gives the application full control over the packet structure, including secondary headers and segmentation.

### Octet String Service

The application provides raw bytes and an APID. The service layer wraps them in a valid Space Packet automatically — constructing the header, assigning the sequence count, and optionally computing the CRC.

This is the simpler interface, suitable for applications that just need to move data without worrying about packet structure.

## Relationship with Data Link Protocols

Space Packets do not travel alone — they are carried inside **Transfer Frames** at the Data Link Layer. The relationship works as follows:

1. **Multiplexing**: Multiple packets from different APIDs can be packed into the same Transfer Frame. The Data Link Layer uses the **First Header Pointer** in the frame header to locate the first packet boundary.

2. **Spanning**: A single large packet can span multiple Transfer Frames. The receiver accumulates data across frames and uses the packet length field to know when the packet is complete.

3. **Virtual Channels**: Different Virtual Channels can carry different sets of APIDs, providing bandwidth allocation and priority between data streams.

```
Transfer Frame 1           Transfer Frame 2
┌──────────────────────┐   ┌──────────────────────┐
│ HDR │ [Pkt A][Pkt B  │   │  Pkt B cont][Pkt C]  │
│     │       ↑        │   │  ↑                   │
│     │  FHP=len(PktA) │   │  FHP=remaining_B     │
└──────────────────────┘   └──────────────────────┘
```

## Design Rationale

Several design choices in SPP reflect the unique constraints of space communication:

**Why 11-bit APID?** 2,048 addresses is enough for even complex spacecraft (most missions use fewer than 100 APIDs) while keeping the header compact. Every byte matters when your downlink is 1 kbps.

**Why a separate Packet Type bit?** Spacecraft often have separate telemetry and telecommand processing chains. A single bit in a fixed position lets hardware routers make forwarding decisions without parsing the payload.

**Why "length minus one"?** This eliminates the ambiguity of whether length=0 means "empty" or "one byte" and guarantees every packet carries at least one byte of data, which is a CCSDS architectural requirement.

**Why per-APID sequence counts?** Different applications generate data at different rates. A shared counter would wrap too quickly for high-rate instruments and waste counter space for low-rate housekeeping. Per-APID counting also means a lost packet from one application doesn't affect gap detection for another.

## Reference

- [CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2e2.pdf) — Space Packet Protocol (Blue Book)
- [CCSDS 133.0-G-1](https://public.ccsds.org/Pubs/133x0g1.pdf) — Space Packet Protocol Summary (Green Book)
