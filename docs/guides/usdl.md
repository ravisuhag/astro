# Unified Space Data Link Protocol

> CCSDS 732.1-B-2 — Unified Space Data Link Protocol

## Overview

The Unified Space Data Link Protocol (USLP) is a **Data Link Layer** protocol that unifies the TM, TC, and AOS space data link protocols into a single, flexible frame format. It supports both fixed-length and variable-length frames, bidirectional communication, and a richer multiplexing model through MAP (Multiplexer Access Point) channels.

USLP is the modern successor to the separate TM (CCSDS 132.0-B-3), TC (CCSDS 232.0-B-4), and AOS (CCSDS 732.0-B-4) data link protocols. It was designed for next-generation missions that need a single, configurable protocol stack instead of maintaining three separate ones.

### Where USLP Fits

```
┌─────────────────────────────────────────────┐
│  Space Packet Protocol / Other Upper Layer  │
│  Application data in packets                │
├─────────────────────────────────────────────┤
│  Unified Space Data Link Protocol (USLP)   │  ← Data Link Layer
│  Packs data into transfer frames            │
│  Virtual Channels, MAP multiplexing         │
├─────────────────────────────────────────────┤
│  Sync & Channel Coding                      │
│  ASM attachment, FEC, randomization         │
├─────────────────────────────────────────────┤
│  Physical Layer (RF/Optical link)           │
└─────────────────────────────────────────────┘
```

### Key Characteristics

- **Unified format**: A single transfer frame structure replaces TM, TC, and AOS frames.
- **Fixed or variable length**: Frames can be fixed-length (for traditional TDM links) or variable-length (for packet-switched links).
- **MAP multiplexing**: Up to 64 MAP channels per Virtual Channel, enabling fine-grained data stream separation.
- **16-bit Spacecraft ID**: Supports up to 65,536 spacecraft (vs. 1,024 in TM/TC).
- **Flexible error control**: Supports both CRC-16-CCITT and CRC-32C.
- **Bidirectional**: The same frame format is used for both uplink and downlink, with a source/destination flag.

## Channel Hierarchy

USLP organizes data transmission through a four-level channel hierarchy:

```
Physical Channel
  └── Master Channel (one per spacecraft)
        ├── Virtual Channel 0
        │     ├── MAP 0 (e.g., real-time housekeeping packets)
        │     ├── MAP 1 (e.g., science data stream)
        │     └── ...up to 64 MAPs (0-63)
        ├── Virtual Channel 1
        └── ...up to 64 Virtual Channels (0-63)
```

### Physical Channel

The physical communication link (e.g., S-band, X-band). All frames on a physical channel share the same fixed length (for fixed-length mode) and FECF configuration.

### Master Channel

Identified by SCID (Spacecraft ID, 16 bits). Groups all Virtual Channels belonging to the same spacecraft.

### Virtual Channel

Identified by VCID (6 bits, 0-63). Provides logical separation of data streams within a Master Channel. Each VC maintains an independent sequence counter for gap detection.

### MAP Channel

Identified by MAP ID (6 bits, 0-63). Provides fine-grained multiplexing within a Virtual Channel. Each MAP can run a different service type (packet, access, or octet stream).

## Transfer Frame Structure

```
┌─────────────────────────────────────────────────────────┐
│ Primary Header │ Insert │ Data Field │ Data    │OCF│FECF│
│                │ Zone   │ Header     │ Zone    │   │    │
├────────────────┤        ├────────────┤         │   │    │
│ TFVN (4 bits)  │optional│ Constr.    │ User    │opt│opt │
│ SCID (16 bits) │        │ Rule (3b)  │ data    │4B │2/4B│
│ S/D  (1 bit)   │        │ UPID (5b)  │         │   │    │
│ VCID (6 bits)  │        │ FHO (16b)  │         │   │    │
│ MAPID (6 bits) │        │ SeqNum(16b)│         │   │    │
│ EOFPH (1 bit)  │        │            │         │   │    │
│ [FrameLen(16b)]│        │            │         │   │    │
└────────────────┘        └────────────┘         └───┘────┘
```

### Primary Header

- **TFVN** (4 bits): Transfer Frame Version Number, always `1100` (12) for USLP.
- **SCID** (16 bits): Spacecraft Identifier (0-65535).
- **Source/Dest** (1 bit): 0 = frame originates from SCID, 1 = frame destined for SCID.
- **VCID** (6 bits): Virtual Channel Identifier (0-63).
- **MAP ID** (6 bits): Multiplexer Access Point Identifier (0-63).
- **EOFPH** (1 bit): End of Frame Primary Header. When 1, the primary header ends here (fixed-length mode). When 0, a 16-bit Frame Length field follows (variable-length mode).
- **Frame Length** (16 bits, conditional): Total frame octets minus 1. Present only when EOFPH = 0.

### Insert Zone

An optional field between the primary header and the data field header. Used for mission-specific purposes (e.g., time stamps, quality indicators). Its length is fixed per physical channel and configured externally.

### Data Field Header (TFDFH)

- **Construction Rule** (3 bits): Defines how the Transfer Frame Data Zone is structured.
  - `0` = Packet spanning (MAPP service)
  - `1` = VCA SDU (MAPA service)
  - `2` = Octet stream (MAPO service)
  - `7` = Idle data
- **UPID** (5 bits): USLP Protocol Identifier, identifies the upper-layer protocol.
- **First Header Offset** (16 bits): Offset to the first packet header in the data zone. Special values: `0xFFFF` = no packet start, `0xFFFE` = all idle fill.
- **Sequence Number** (16 bits): Per-VC frame sequence counter for gap detection.

## Services

USLP provides three data service types, all operating at the MAP level:

### MAP Packet Service (MAPP)

Equivalent to the TM VCP service. Multiplexes variable-length packets into frames using the First Header Offset for boundary detection. Supports packet spanning across frames with FHO-based resynchronization after frame loss.

### MAP Access Service (MAPA)

Equivalent to the TM VCA service. Transfers fixed-length SDUs (Service Data Units). Each frame carries exactly one SDU.

### MAP Octet Stream Service (MAPO)

New in USLP. Transfers an unstructured octet stream without packet boundaries. Useful for bulk data transfer where framing is handled at a higher layer.

## Library Usage

```go
import "github.com/ravisuhag/astro/pkg/usdl"

// Create a USLP Transfer Frame
frame, _ := usdl.NewTransferFrame(100, 1, 0, payload,
    usdl.WithConstructionRule(usdl.RulePacketSpanning),
    usdl.WithSequenceNumber(42),
)
encoded, _ := frame.Encode()

// Decode a frame (CRC-16, no insert zone)
decoded, _ := usdl.DecodeTransferFrame(data, usdl.FECSize16, 0)

// Use CRC-32 instead
frame32, _ := usdl.NewTransferFrame(100, 1, 0, payload,
    usdl.WithCRC32(),
)

// Channel hierarchy
vc := usdl.NewVirtualChannel(1, 100)
mc := usdl.NewMasterChannel(100, config)
mc.AddVirtualChannel(vc, 1)
pc := usdl.NewPhysicalChannel("X-band", config)
pc.AddMasterChannel(mc, 1)

// MAPP service for packet multiplexing
svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, counter)
svc.Send(packetData)
```

## References

- [CCSDS 732.1-B-2](https://public.ccsds.org/Pubs/732x1b2.pdf) — Unified Space Data Link Protocol (Blue Book)
- [CCSDS 130.0-G-3](https://public.ccsds.org/Pubs/130x0g3.pdf) — Overview of Space Communications Protocols
