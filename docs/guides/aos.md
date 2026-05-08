# AOS Space Data Link Protocol

> CCSDS 732.0-B-4 — AOS Space Data Link Protocol

## Overview

The Advanced Orbiting Systems (AOS) Space Data Link Protocol is a **Data Link Layer** protocol designed for high-rate downlinks from spacecraft in advanced orbits — Earth observation, lunar, and deep-space missions where the volume and continuity of telemetry exceeds what the original TM protocol was designed to handle.

AOS introduced three innovations over TM: a 24-bit Virtual Channel frame counter (vs. 8-bit in TM) for long-duration counting without rollover, an Insert Zone for periodic time/quality data inserted at every frame boundary, and the Bitstream PDU service for octet-aligned bitstream transfer outside the Space Packet model.

### Where AOS Fits

```
┌─────────────────────────────────────────────┐
│  Space Packet Protocol / Bitstream / SDU    │
│  Application data in packets or octets      │
├─────────────────────────────────────────────┤
│  AOS Space Data Link Protocol               │  ← Data Link Layer
│  Frames, Virtual Channels, M_PDU/B_PDU/VCA  │
├─────────────────────────────────────────────┤
│  Sync & Channel Coding                      │
│  ASM attachment, FEC, randomization         │
├─────────────────────────────────────────────┤
│  Physical Layer (RF link)                   │
└─────────────────────────────────────────────┘
```

### Key Characteristics

- **Fixed-length frames** per physical channel.
- **Up to 64 Virtual Channels** per spacecraft (VCID 63 reserved for Only Idle Data).
- **24-bit VC frame count** — supports very long downlinks without rollover concerns.
- **Insert Zone**: an optional, mission-defined fixed-length field at every frame for periodic data (typically time codes).
- **Three data services**: M_PDU (packets), B_PDU (bitstream), VCA (opaque SDU).
- **Optional OCF and FECF**: 4-byte Operational Control Field, 2-byte CRC-16-CCITT.

## Channel Hierarchy

```
Physical Channel
  └── Master Channel (one per spacecraft)
        ├── Virtual Channel 0  ── M_PDU service (packets)
        ├── Virtual Channel 1  ── B_PDU service (bitstream)
        ├── Virtual Channel 2  ── VCA service (opaque SDU)
        ├── ...
        └── Virtual Channel 63 ── Only Idle Data (OID)
```

### Physical Channel

The physical communication link. All frames share the same fixed length, insert zone length, and FECF/OCF configuration.

### Master Channel

Identified by SCID (8 bits, 0-255). Groups all Virtual Channels for one spacecraft.

### Virtual Channel

Identified by VCID (6 bits, 0-63). Each VC carries one PDU type. Each VC maintains an independent 24-bit frame count for gap detection.

VCID 63 is reserved for **Only Idle Data (OID)** frames — used as fill when no other VC has data to send.

## Transfer Frame Structure

```
┌────────────────┬────────┬────────────┬───┬────┐
│ Primary Header │ Insert │ Data Field │OCF│FECF│
│   (6 bytes)    │ Zone   │            │   │    │
├────────────────┤        ├────────────┤   │    │
│ TFVN  (2 bits) │optional│ M_PDU,     │opt│opt │
│ SCID  (8 bits) │        │ B_PDU, or  │4B │ 2B │
│ VCID  (6 bits) │        │ VCA        │   │    │
│ VCFC (24 bits) │        │ payload    │   │    │
│ Signaling (8b) │        │            │   │    │
└────────────────┘        └────────────┘   └────┘
```

### Primary Header (6 bytes)

- **TFVN** (2 bits): Transfer Frame Version Number, always `01` for AOS.
- **SCID** (8 bits): Spacecraft Identifier (0-255).
- **VCID** (6 bits): Virtual Channel Identifier (0-63).
- **VC Frame Count** (24 bits): Per-VC frame counter, wraps at 2^24.
- **Signaling Field** (8 bits):
  - Replay Flag (1 bit)
  - VC Frame Count Usage Flag (1 bit)
  - Reserved Spare (2 bits, always `00`)
  - VC Frame Count Cycle (4 bits)

### Insert Zone

Optional, mission-defined fixed-length field placed between the primary header and the data field. Typically carries a time code or status word that must appear at every frame boundary.

### Data Field

The payload format depends on the service running on the Virtual Channel.

#### M_PDU (Multiplexing PDU)

For variable-length packets such as Space Packets. The data field begins with a 16-bit M_PDU header carrying a **First Header Pointer** that indicates where the next packet starts within the packet zone.

```
┌─────────────┬────────────────────────────┐
│ MPDU Header │ Packet Zone                │
│  reserved(5)│ packet | packet | ...      │
│  FHP(11)    │                            │
└─────────────┴────────────────────────────┘
```

Special FHP values: `0x7FE` = no packet starts in this frame, `0x7FF` = idle data only.

#### B_PDU (Bitstream PDU)

For octet-aligned bitstream data. The data field begins with a 16-bit B_PDU header carrying a **Bitstream Data Pointer** that locates the last valid bit within the bitstream zone.

```
┌─────────────┬────────────────────────────┐
│ BPDU Header │ Bitstream Zone             │
│  reserved(2)│                            │
│  BDP(14)    │                            │
└─────────────┴────────────────────────────┘
```

Special BDP values: `0x3FFF` = all valid (no end within frame), `0x3FFE` = all idle.

#### VCA (Virtual Channel Access)

Carries an opaque, fixed-length SDU. The entire data field is the SDU — no protocol header.

### OCF and FECF

- **OCF** (4 bytes, optional): Operational Control Field — typically a CLCW for COP-1 reporting back to the ground.
- **FECF** (2 bytes, optional): Frame Error Control Field, CRC-16-CCITT over the entire frame.

## Library Usage

```go
import "github.com/ravisuhag/astro/pkg/aos"

// Build an AOS Transfer Frame with FECF
frame, _ := aos.NewTransferFrame(50, 1, payload,
    aos.WithVCFrameCount(0),
    aos.WithFECF(),
)
encoded, _ := frame.Encode()

// Decode (knowing the channel config)
decoded, _ := aos.DecodeTransferFrame(encoded, 0, false, true)

// M_PDU service for packet multiplexing
config := aos.ChannelConfig{FrameLength: 1024, HasFECF: true}
vc := aos.NewVirtualChannel(1, 100)
counter := aos.NewFrameCounter()
svc := aos.NewMultiplexingService(50, 1, vc, config, counter)
svc.Send(spacePacketBytes)
svc.Flush() // emit final partial frame

// Channel hierarchy
mc := aos.NewMasterChannel(50, config)
mc.AddVirtualChannel(vc, 1)
pc := aos.NewPhysicalChannel("X-band", config)
pc.AddMasterChannel(mc, 1)
```

## References

- [CCSDS 732.0-B-4](https://public.ccsds.org/Pubs/732x0b4.pdf) — AOS Space Data Link Protocol (Blue Book)
- [CCSDS 130.0-G-3](https://public.ccsds.org/Pubs/130x0g3.pdf) — Overview of Space Communications Protocols
