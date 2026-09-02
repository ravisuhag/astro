---
title: AOS Space Data Link Protocol
short: AOS
description: CCSDS 732.0-B-4 — high-rate downlink frames for Earth observation and deep space.
order: 22
---

> **CCSDS 732.0-B-4** · [Blue Book](https://public.ccsds.org/Pubs/732x0b4.pdf) · [`pkg/aos`](https://github.com/ravisuhag/astro/tree/main/pkg/aos) · [`astro aos`](/cli/aos)

AOS is the downlink protocol for missions that send a lot of data for a long time. Earth observation, lunar, deep space. It does the same job as [TM](/protocols/data-link/tmdl) but scales further in three ways: a 24-bit frame counter instead of 8 bits, an insert zone that puts a fixed field at every frame boundary, and a bitstream service for data that is not packets.

Frames are fixed length per physical channel, same as TM.

## Scope

**Implemented.** The frame format including the optional FHEC, all three data services — M_PDU, B_PDU, and VCA — plus the VC Frame pass-through, master and virtual channel multiplexing, gap detection, and idle frames.

**Somewhere else.** ASM, randomization, and CADU wrapping are [`pkg/tmsc`](/protocols/coding/tmsc).

**Left to you.** Insert zone contents and the CLCW in the OCF. Both are mission-defined.

## Field map

The primary header — 6 bytes, or 8 with FHEC. Go fields on `aos.PrimaryHeader`.

| Field | Bits | Go | Notes |
|---|---|---|---|
| Transfer Frame Version Number | 2 | `TFVN` | Always `1` for AOS. TM uses `0`. |
| Spacecraft Identifier | 8 | `SCID` | 0–255. Narrower than TM's 10 bits. |
| Virtual Channel Identifier | 6 | `VCID` | 0–63. VCID 63 is reserved for Only Idle Data. |
| Virtual Channel Frame Count | 24 | `VCFrameCount` | Wraps at 16,777,215 |
| Replay Flag | 1 | `ReplayFlag` | Marks recorded data being played back |
| VC Frame Count Usage Flag | 1 | `VCFCUsageFlag` | Whether the cycle field is in use |
| VC Frame Count Cycle | 4 | `VCFrameCountCycle` | 0–15, extends the counter further |

The rest of the frame, on `aos.TransferFrame`:

| Part | Size | Go | Notes |
|---|---|---|---|
| Frame Header Error Control | 2 B | `FHEC` | Optional. Reed–Solomon (10,6) over the protected header octets. |
| Insert Zone | fixed | `InsertZone` | Optional, mission-defined, same length on every frame |
| Data Field | variable | `DataField` | Includes the M_PDU or B_PDU header when one applies |
| Operational Control Field | 4 B | `OCF` | Optional, usually a CLCW |
| Frame Error Control Field | 2 B | `FECF` | Optional CRC-16-CCITT over the whole frame |

Channel-wide settings are on `aos.ChannelConfig`: `FrameLength`, `InsertZoneLen`, `HasOCF`, `HasFHEC`, `HasFECF`, and `IdlePattern`.

### The three data services

**M_PDU** carries variable-length packets, usually [Space Packets](/protocols/transport/spp). The data field opens with a 16-bit header: 5 reserved bits and an 11-bit First Header Pointer saying where the next packet starts.

| FHP | Meaning |
|---|---|
| 0–2045 | Offset to the first packet header in the packet zone |
| `0x7FE` | Idle data only |
| `0x7FF` | No packet starts here |

**B_PDU** carries an octet-aligned bitstream — data that is not packets at all. The 16-bit header is 2 reserved bits and a 14-bit Bitstream Data Pointer marking the last valid bit.

| BDP | Meaning |
|---|---|
| 0–16381 | Index of the last valid bit |
| `0x3FFE` | All idle |
| `0x3FFF` | All valid, nothing ends in this frame |

**VCA** carries one opaque fixed-length SDU. No header at all — the data field *is* the SDU.

## Gotchas

**Fill is a real idle packet under M_PDU.** When `Flush` releases a partly full frame, Astro completes the packet zone with a Space Packet at APID `0x7FF`, not raw bytes. A conformant receiver would read raw fill as a packet header. If the leftover room is under 7 octets, the idle packet spans into the next frame.

**A VCA SDU must fill the data field exactly.** Not "at most". A receiver has no in-band way to find where a padded SDU ends, so Astro rejects a short one with `ErrSizeMismatch` rather than padding it and hoping.

**B_PDU cannot mark more than 2047 valid octets in a partial frame.** The BDP counts bits, and its top two values are reserved. On a channel with a bigger bitstream zone, `Flush` splits an oversized partial payload across extra frames instead of writing a pointer that means something else. That is what `ErrBitstreamTooLongForPointer` guards.

**Idle fill is mission-managed, not standardised.** Set `ChannelConfig.IdlePattern` to your repeating pattern. Astro uses `0xFE` when you leave it empty. This is unlike [TM](/protocols/data-link/tmdl), where the PN sequence is mandatory.

**The insert zone is the same length on every frame or it is absent.** It is channel configuration. A length that does not match gives `ErrInvalidInsertZoneLength`.

**FHEC changes the header size.** Turning it on makes the primary header 8 bytes instead of 6, which eats into your data field. It protects only TFVN, SCID, VCID, and the signaling field — not the frame count.

**M_PDU receive needs a `PacketSizer`.** Same as TM and TC. Without one, `ErrNoPacketSizer`.

## Quick start

```go
import "github.com/ravisuhag/astro/pkg/aos"

// Spacecraft 50, virtual channel 1.
frame, err := aos.NewTransferFrame(50, 1, payload,
    aos.WithVCFrameCount(7),
    aos.WithFECF(),
)
encoded, err := frame.Encode()
```

Decoding needs the channel configuration, because AOS does not signal it in band:

```go
// insertZoneLen, hasOCF, hasFECF
back, err := aos.DecodeTransferFrame(encoded, 0, false, true)

fmt.Println(back.Header.SCID, back.Header.VCID, back.Header.VCFrameCount)
// 50 1 7
```

Get those three arguments wrong and the decode either fails or silently reads the wrong bytes as your data field. They come from `ChannelConfig`, which both ends agree on before the link opens.

## Frame options

| Option | Effect |
|---|---|
| `WithInsertZone(data)` | Fills the insert zone. Length must match `ChannelConfig.InsertZoneLen`. |
| `WithOCF(ocf)` | Attaches a 4-byte Operational Control Field |
| `WithFECF()` | Appends the 2-byte CRC |
| `WithFHEC()` | Adds Reed–Solomon header protection, making the primary header 8 bytes |
| `WithVCFrameCount(n)` | Sets the 24-bit count |
| `WithReplayFlag()` | Marks the frame as recorded data being played back |
| `WithVCFCUsage(cycle)` | Sets the usage flag and the 4-bit cycle |

## Channel configuration

One struct, shared by both ends of the link:

```go
config := aos.ChannelConfig{
    FrameLength:   256,
    InsertZoneLen: 0,
    HasOCF:        false,
    HasFHEC:       false,
    HasFECF:       true,
    IdlePattern:   nil, // nil means 0xFE
}
```

Unlike [USLP](/protocols/data-link/usdl), the OCF flag is not carried in the AOS header, so `HasOCF` has to be configured on both sides.

## Services

Pick the service that matches what you are sending.

### MultiplexingService — M_PDU

For variable-length packets. This is the common case.

```go
vc := aos.NewVirtualChannel(1, 32)   // vcid, buffer depth
counter := aos.NewFrameCounter()

svc := aos.NewMultiplexingService(50, 1, vc, config, counter)
svc.SetPacketSizer(spp.PacketSizer)

svc.Send(spacePacketBytes)
svc.Flush()
```

`SetPacketSizer` is required before you can receive — the service has to know how long the packet at the First Header Pointer claims to be. Without it you get `ErrNoPacketSizer`.

`Send` packs into the current frame and releases it when full. `Flush` emits whatever is left, completing the packet zone with a real idle packet at APID `0x7FF`. Skip the flush and your last packets never leave.

### BitstreamService — B_PDU

For octet-aligned data that is not packets:

```go
svc := aos.NewBitstreamService(50, 2, vc, config, counter)
svc.Send(bitstreamBytes)
svc.Flush()
```

`Flush` splits an oversized partial payload across extra frames rather than writing a Bitstream Data Pointer that would mean something else — see [the gotchas](/protocols/data-link/aos#gotchas).

### VirtualChannelAccessService — VCA

One fixed-length SDU per frame:

```go
svc := aos.NewVirtualChannelAccessService(50, 3, sduSize, vc, config, counter)
```

The SDU must fill the data field exactly. A short one gives `ErrSizeMismatch` rather than being padded.

### VirtualChannelFrameService

Pass-through for frames you built yourself:

```go
svc := aos.NewVirtualChannelFrameService(4, vc, config)
```

## Channel hierarchy

```go
mc := aos.NewMasterChannel(50, config)
mc.AddVirtualChannel(vc, 1)   // priority

pc := aos.NewPhysicalChannel("X-band", config)
pc.AddMasterChannel(mc, 1)

frame, err := pc.GetNextFrame()
```

`GetNextFrameOrIdle()` returns an idle frame instead of an error when nothing is queued, which is what a continuous-rate downlink wants.

## Gap detection

```go
det := aos.NewFrameGapDetector()
gap := det.Check(frame)
```

AOS counts per virtual channel only — there is no master channel frame count. The 24-bit counter plus the 4-bit cycle field is why AOS survives high rates where [TM](/protocols/data-link/tmdl) would wrap.

## Errors

| Error | Cause |
|---|---|
| `ErrInvalidVersion` | TFVN is not 1 |
| `ErrInvalidSpacecraftID` | Outside 0–255 |
| `ErrInvalidVCID` | Outside 0–63 |
| `ErrInvalidVCFrameCount` | Exceeds 24 bits |
| `ErrInvalidVCFrameCountCycle` | Outside 0–15 |
| `ErrInvalidFirstHeaderPointer` | Exceeds 11 bits |
| `ErrInvalidBitstreamDataPointer` | Exceeds 14 bits |
| `ErrBitstreamTooLongForPointer` | Partial bitstream cannot be expressed by the BDP |
| `ErrFHECMismatch` | Header error control check failed |
| `ErrCRCMismatch` | FECF did not match |
| `ErrInvalidInsertZoneLength` | Does not match the configured length |
| `ErrSizeMismatch` | VCA SDU does not fill the data field |
| `ErrNoPacketSizer` | `SetPacketSizer` was not called before receiving |
| `ErrSCIDMismatch` | Frame SCID differs from the master channel's |

## Notes

Commentary, not sourced from the standard.

**Why 24 bits of frame count?** An 8-bit counter wraps every 256 frames. On a high-rate downlink that is a fraction of a second, which makes it useless for anything but spotting a single dropped frame. 24 bits plus the 4-bit cycle field covers a whole pass and then some.

**Why a smaller spacecraft ID than TM?** AOS spent those bits on the virtual channel field and the frame count. A mission running AOS knows which spacecraft it is talking to; it needs the counter more.

**Why the insert zone?** Some data has to appear at a known offset in every single frame — a time code, a quality flag — so a receiver can find it without parsing the payload. Putting it in the data field would mean parsing packets first.

**Why B_PDU?** Not everything is a packet. Instrument output is sometimes just a bit stream, and forcing packet framing onto it adds overhead and a boundary problem that nobody wanted.

## Reference

- [CCSDS 732.0-B-4](https://public.ccsds.org/Pubs/732x0b4.pdf) — AOS Space Data Link Protocol (Blue Book)
- [CLI](/cli/aos) · [Conformance](/conformance/aos)
