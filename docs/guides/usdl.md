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

- **Unified format**: a single transfer frame structure replaces TM, TC, and AOS frames.
- **Fixed or variable length**: frames can be fixed-length (for traditional TDM links) or variable-length (for packet-switched links). The frame layout tells the receiver which construction rule is in use.
- **MAP multiplexing**: up to 16 MAP channels per Virtual Channel (4-bit MAP ID).
- **16-bit Spacecraft ID**: supports up to 65,536 spacecraft (vs. 1,024 in TM/TC).
- **Managed frame counting**: a per-VC Virtual Channel Frame Count of 0 to 7 octets, sized by a managed parameter and declared in the header.
- **Flexible error control**: 16-bit CRC-16-CCITT or the 32-bit CCSDS CRC-32 (the Proximity-1 polynomial `0x00A00805` — not CRC-32C).
- **Bidirectional**: the same frame format is used for uplink and downlink, with a source/destination flag.
- **Truncated frames**: a minimal 4-octet header variant for short telecommands (annex D).

## Channel Hierarchy

USLP organizes data transmission through a four-level channel hierarchy:

```
Physical Channel
  └── Master Channel (one per spacecraft)
        ├── Virtual Channel 0
        │     ├── MAP 0 (e.g., real-time housekeeping packets)
        │     ├── MAP 1 (e.g., science data stream)
        │     └── ...up to 16 MAPs (0-15)
        ├── Virtual Channel 1
        └── ...up to 64 Virtual Channels (0-63; 63 reserved for idle)
```

### Physical Channel

The physical communication link (e.g., S-band, X-band). All frames on a physical channel share the same fixed length (for fixed-length mode) and FECF configuration.

### Master Channel

Identified by SCID (Spacecraft ID, 16 bits). Groups all Virtual Channels belonging to the same spacecraft.

### Virtual Channel

Identified by VCID (6 bits, 0-63). Provides logical separation of data streams within a Master Channel. Each VC maintains an independent frame count (the VCF Count) for gap detection. VCID 63 is reserved for Only Idle Data (OID) frames.

### MAP Channel

Identified by MAP ID (4 bits, 0-15). Provides fine-grained multiplexing within a Virtual Channel. Each MAP can run a different service type (packet, access, or octet stream). The receive side of every service filters frames by MAP ID.

## Transfer Frame Structure

```
┌────────────────┬────────┬─────────────┬────────┬───┬────┐
│ Primary Header │ Insert │ TFDF Header │ TFDZ   │OCF│FECF│
│ (4 or 7-14 B)  │ Zone   │ (1 or 3 B)  │        │   │    │
└────────────────┴────────┴─────────────┴────────┴───┴────┘
```

### Primary Header

The first 4 octets are common to every frame:

- **TFVN** (4 bits): Transfer Frame Version Number, always `1100` (12) for USLP.
- **SCID** (16 bits): Spacecraft Identifier (0-65535).
- **Source/Dest** (1 bit): 0 = SCID is the frame's source, 1 = SCID is its destination.
- **VCID** (6 bits): Virtual Channel Identifier (0-63).
- **MAP ID** (4 bits): Multiplexer Access Point Identifier (0-15).
- **End of Frame Primary Header flag** (1 bit): 1 = the header ends here — this is a *truncated* frame (annex D). 0 = the full header continues:
- **Frame Length** (16 bits): total frame octets minus 1. The decoder cross-checks it against the delivered buffer.
- **Bypass/Sequence Control flag** (1 bit): 0 = sequence-controlled QoS, 1 = expedited.
- **Protocol Control Command flag** (1 bit): 0 = user data, 1 = protocol control information.
- **Reserved spares** (2 bits): always `00`; the decoder rejects frames with these set.
- **OCF flag** (1 bit): signals the presence of the Operational Control Field.
- **VCF Count Length** (3 bits): size of the following count field, 0-7 octets.
- **VCF Count** (0-56 bits): per-VC frame counter, big-endian, as wide as declared.

A full header is therefore 7 octets plus the VCF Count. A truncated header is exactly 4 octets, used only for short telecommands on variable-length channels; truncated frames carry no insert zone, OCF, or FECF.

### Insert Zone

An optional field between the primary header and the TFDF header, for mission-specific periodic data (e.g., time stamps). Its length is fixed per physical channel and configured externally.

### TFDF Header (Transfer Frame Data Field Header)

One mandatory octet:

- **TFDZ Construction Rules** (3 bits): how the data zone is organized.
- **UPID** (5 bits): USLP Protocol Identifier from the SANA registry.

Plus a **16-bit pointer**, present only for rules `000`, `001`, and `010`:

| Rule | TFDZ type | Meaning | Pointer |
|---|---|---|---|
| `000` | fixed | CCSDS packets spanning frames | First Header Pointer (`0xFFFF` = no packet starts) |
| `001` | fixed | start of a MAPA/VCA SDU | Last Valid Octet Pointer (`0xFFFF` = SDU continues) |
| `010` | fixed | continuation of a MAPA/VCA SDU | Last Valid Octet Pointer |
| `011` | variable | continuous octet stream | — |
| `100` | variable | starting segment of an SDU | — |
| `101` | variable | continuing segment | — |
| `110` | variable | last segment | — |
| `111` | variable | complete, unsegmented SDUs/packets | — |

Common UPIDs (constants in this package): 0 = Space/Encapsulation Packets, 1 = COP-1 control, 2 = COP-P control, 3 = SDLS control, 4 = user octet stream, 5 = Mission Specific Information-1 (MAPA_SDU), 7 = Proximity-1 SPDUs, 31 = Idle Data.

### OCF and FECF

- **OCF** (4 bytes): Operational Control Field — its presence is signaled in-band by the OCF flag, so the decoder needs no out-of-band knowledge.
- **FECF** (2 or 4 bytes): Frame Error Control Field, a managed parameter of the physical channel. CRC-16-CCITT, or the CCSDS CRC-32 (polynomial `0x00A00805`, register preset to zero, no inversion — identical to Proximity-1, and *not* CRC-32C).

### Idle (OID) frames

When a fixed-length channel has nothing to send, an Only Idle Data frame is emitted on VCID 63 with MAP ID 0, construction rule `001`, UPID 31 (Idle Data), and the Last Valid Octet Pointer marking the last TFDZ octet. The fill pattern is project-specified (`ChannelConfig.IdlePattern`). A partially filled *packet* zone (rule `000`) is completed with an Encapsulation Idle Packet instead, which the receiver strips.

## Services

USLP provides three data service types, all operating at the MAP level. Every service's Receive filters by MAP ID, so two MAPs sharing a VC do not corrupt each other.

### MAP Packet Service (MAPP)

Multiplexes variable-length packets. On fixed-length channels it uses rule `000` with the First Header Pointer for boundary recovery after frame loss, completing partial frames with an Encapsulation Idle Packet. On variable-length channels each frame carries complete packets under rule `111`.

### MAP Access Service (MAPA)

Transfers constant-length MAPA_SDUs. On fixed-length channels an SDU starts in a rule `001` frame and continues through rule `010` frames, delimited by the Last Valid Octet Pointer. On variable-length channels each SDU rides alone in a rule `111` frame.

### MAP Octet Stream Service (MAPO)

Transfers an unstructured octet stream under rule `011`. Octet streams exist only on variable-length channels (CCSDS 732.1-B-2 §4.2.4.1); sending on a fixed-length channel returns an error.

## Library Usage

```go
import "github.com/ravisuhag/astro/pkg/usdl"

// Create a USLP Transfer Frame (full header, CRC-16 FECF)
frame, _ := usdl.NewTransferFrame(100, 1, 0, payload,
    usdl.WithConstructionRule(usdl.RuleNoSegmentation),
    usdl.WithUPID(usdl.UPIDSpacePackets),
    usdl.WithVCFCount(2, 42), // 2-octet VCF count
)
encoded, _ := frame.Encode()

// Decode a frame (CRC-16, no insert zone). OCF presence is read from
// the in-band OCF flag.
decoded, _ := usdl.DecodeTransferFrame(data, usdl.FECSize16, 0)

// CCSDS CRC-32 FECF instead
frame32, _ := usdl.NewTransferFrame(100, 1, 0, payload, usdl.WithCRC32())

// Truncated frame for a short telecommand (annex D)
tc, _ := usdl.NewTruncatedFrame(100, 1, 0, cmdBytes)

// Channel hierarchy
config := usdl.ChannelConfig{FrameLength: 256, HasFECF: true, VCFCountLen: 2}
vc := usdl.NewVirtualChannel(1, 100)
mc := usdl.NewMasterChannel(100, config)
mc.AddVirtualChannel(vc, 1)
pc := usdl.NewPhysicalChannel("X-band", config)
pc.AddMasterChannel(mc, 1)

// MAPP service for packet multiplexing
counter := usdl.NewFrameCounter()
svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, counter)
svc.Send(packetData)
svc.Flush() // completes the last frame with an Encapsulation Idle Packet
```

## References

- [CCSDS 732.1-B-2](https://ccsds.org/Pubs/732x1b3e1.pdf) — Unified Space Data Link Protocol (Blue Book; link points to the current issue)
- [SANA UPID registry](https://sanaregistry.org/r/uslp_protocol_id) — USLP Protocol Identifiers
- [CCSDS 130.0-G-3](https://public.ccsds.org/Pubs/130x0g3.pdf) — Overview of Space Communications Protocols
