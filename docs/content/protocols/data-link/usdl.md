---
title: Unified Space Data Link Protocol
short: USDL
description: CCSDS 732.1-B-3, one frame format that replaces TM, TC, and AOS.
identifiers:
  - "CCSDS 732.1-B-3 * Unified Space Data Link Protocol"
  - "pkg/usdl * astro usdl"
order: 23
---

> **CCSDS 732.1-B-3** | [Blue Book](https://ccsds.org/Pubs/732x1b3e1.pdf) | [`pkg/usdl`](https://github.com/ravisuhag/astro/tree/main/pkg/usdl) | [`astro usdl`](/cli/usdl)

## Overview

USLP folds [TM](/protocols/data-link/tmdl), [TC](/protocols/data-link/tcdl), and [AOS](/protocols/data-link/aos) into one frame format. It runs in both directions, does fixed *or* variable length frames, and adds a MAP layer for finer multiplexing.

It is the protocol for a mission that would otherwise run three data link stacks and maintain all of them.

## Scope

**Implemented.** The frame format, both fixed and variable length, truncated frames, all eight TFDZ construction rules, and the three MAP services: MAPP, MAPA, and MAPO. Master and virtual channel multiplexing, gap detection over a variable-width counter, and OID idle frames with the mandatory PN fill.

**Somewhere else.** Sync and coding are [`pkg/tmsc`](/protocols/coding/tmsc) or [`pkg/tcsc`](/protocols/coding/tcsc) depending on direction.

**Left to you.** Insert zone contents, and the OCF, you install a supplier callback and Astro asks it for each frame.

## Field map

The primary header. Its size varies, because the frame count field is 0 to 7 octets wide. Go fields on `usdl.PrimaryHeader`.

| Field | Bits | Go | Notes |
|---|---|---|---|
| Transfer Frame Version Number | 4 | `TFVN` | Always `12` (`0b1100`) |
| Spacecraft Identifier | 16 | `SCID` | 0-65535. TM and TC get 10 bits; USLP gets 16. |
| Source or Destination | 1 | `SourceOrDest` | `0` = SCID is the source, `1` = the destination |
| Virtual Channel Identifier | 6 | `VCID` | 0-63. VCID 63 carries Only Idle Data. |
| MAP Identifier | 4 | `MAPID` | 0-15 |
| End of Frame Primary Header | 1 | `EndOfFPH` | Set on a truncated frame |
| Frame Length | 16 | `FrameLength` | Total octets minus 1. Absent on truncated frames. |
| Bypass / Sequence Control | 1 | `BypassSeqCtrl` | `true` = expedited |
| Protocol Control Command | 1 | `ProtCtrlCmd` | `true` = protocol control |
| OCF Flag | 1 | `OCFFlag` | Signals the OCF in-band |
| VCF Count Length | 3 | `VCFCountLen` | 0-7 octets |
| VC Frame Count | 0-56 | `VCFCount` | Width set by the field above |

The rest of the frame, on `usdl.TransferFrame`:

| Part | Size | Go | Notes |
|---|---|---|---|
| Insert Zone | fixed | `InsertZone` | Optional, fixed per physical channel |
| TFDF Header | 1 or 3 B | `DataFieldHeader` | Construction rule, UPID, and sometimes a pointer |
| Transfer Frame Data Zone | variable | `DataField` | |
| Operational Control Field | 4 B | `OCF` | Present when `OCFFlag` is set |
| Frame Error Control Field | 2 B | `FECF` | Optional CRC-16-CCITT |

Channel settings live on `usdl.ChannelConfig`: `FrameLength` (0 means variable), `HasOCF`, `HasFECF`, `InsertZoneLen`, `VCFCountLen`, `IdlePattern`.

### Construction rules

The TFDF header's 3-bit rule says how to read the data zone. Rules `000`, `001`, and `010` add a 16-bit pointer; the rest do not.

| Rule | Length | Data zone holds | Pointer |
|---|---|---|---|
| `000` | fixed | CCSDS packets, spanning frames | First Header Pointer (`0xFFFF` = none starts here) |
| `001` | fixed | Start of a MAPA or VCA SDU | Last Valid Octet Pointer (`0xFFFF` = continues) |
| `010` | fixed | Continuation of that SDU | Last Valid Octet Pointer |
| `011` | variable | A continuous octet stream | - |
| `100` | variable | First segment of an SDU | - |
| `101` | variable | Middle segment | - |
| `110` | variable | Last segment | - |
| `111` | variable | Complete, unsegmented SDUs or packets | - |

The 5-bit UPID names what is inside. Constants are in the package. The common ones: `0` Space or Encapsulation Packets, `1` COP-1 control, `2` COP-P control, `4` user octet stream, `5` Mission Specific Information-1, `7` Proximity-1 SPDUs, `31` Idle Data. UPID 3 is not assigned by SANA or by any USLP issue.

## Gotchas

**Two different idle fills, and they are not interchangeable.** An OID frame's data zone carries the mandatory PN sequence from clause 4.1.4.1.10, a 32-cell LFSR, polynomial D0+D1+D2+D22+D32, all-ones seed, never restarted. `MasterChannel` holds one persistent `OIDSequence` for this. Your `ChannelConfig.IdlePattern` is a different thing: it fills spare data-zone space *behind* a Last Valid Octet Pointer on a non-OID frame. Mixing them up puts a project pattern where the standard wants PN.

**A partly full packet zone gets an Encapsulation Idle Packet.** Under rule `000`, Astro completes the zone with a real idle packet and points the First Header Pointer at it, so a receiver can resynchronize. Not raw fill.

**Truncated frames give up almost everything.** No insert zone, no OCF, no FECF, no pointer, `ErrTruncatedFrameFields` if you try. The data zone needs at least one octet (`ErrTruncatedFrameTooShort`, minimum frame is 6 octets) and the whole frame stops at 32 octets (`ErrTruncatedFrameTooLong`).

**An OCF channel needs a supplier or nothing sends.** Set one with `SetOCFSupplier` on each service, and on the `MasterChannel` for idle frames. Without it you get `ErrNoOCFSupplier`. Astro refuses rather than making up an all-zero Type-1 report, because a fabricated CLCW is worse than a missing frame.

**Octet streams need a variable-length channel.** Clause 4.2.4.1. Calling MAPO on a fixed-length channel gives `ErrOctetStreamFixedLength`.

**The FECF is 16 bits or nothing.** USLP has no 32-bit variant, unlike some other CCSDS links. `ErrInvalidFECSize` if you ask for one.

**Two MAPs on one VC both get their traffic.** The virtual channel keeps a per-MAP receive demultiplexer, so a service pulling its own MAP's frames does not consume and discard another MAP's. That is a real bug in naive implementations.

## Quick start

```go
import "github.com/ravisuhag/astro/pkg/usdl"

// Spacecraft 100, virtual channel 1, MAP 0.
frame, err := usdl.NewTransferFrame(100, 1, 0, payload,
    usdl.WithConstructionRule(usdl.RuleNoSegmentation),
    usdl.WithUPID(usdl.UPIDSpacePackets),
    usdl.WithVCFCount(2, 42), // 2-octet count, value 42
)
encoded, err := frame.Encode()
```

```go
// OCF presence is read from the in-band flag, so the config only needs
// to say whether the channel carries a FECF and how long its insert zone is.
back, err := usdl.DecodeTransferFrameWithConfig(encoded, usdl.ChannelConfig{HasFECF: true})

fmt.Println(back.Header.SCID, back.Header.VCID, back.Header.MAPID, back.Header.VCFCount)
// 100 1 0 42
```

USLP signals OCF presence with a header flag, so the decoder works that one out for itself and the config only carries FECF presence and insert zone length. `DecodeTransferFrame(data, fecSize, insertZoneLen)` still works as a deprecated positional form of the same call.

## Construction rules

The rule tells a receiver how to read the data zone. Constants match the layout table on the [protocol page](/protocols/data-link/usdl#construction-rules).

| Constant | Value | Data zone |
|---|---|---|
| `RulePacketsSpanning` | `000` | Packets spanning frames, with a First Header Pointer |
| `RuleStartOfSDU` | `001` | Start of a MAPA or VCA SDU |
| `RuleContinuingSDU` | `010` | Continuation of that SDU |
| `RuleOctetStream` | `011` | A continuous octet stream |
| `RuleStartingSegment` | `100` | First segment |
| `RuleContinuingSegment` | `101` | Middle segment |
| `RuleLastSegment` | `110` | Last segment |
| `RuleNoSegmentation` | `111` | Complete, unsegmented |

UPID constants name the payload: `UPIDSpacePackets` (0), `UPIDCOPPControl` (2), `UPIDUserOctetStream` (4), `UPIDIdle` (31), and the rest of the SANA registry.

## Frame options

| Option | Effect |
|---|---|
| `WithConstructionRule(rule)` | Sets the TFDZ construction rule |
| `WithUPID(upid)` | Names what is in the data zone |
| `WithPointer(p)` | First Header Pointer or Last Valid Octet Pointer, for rules `000`/`001`/`010` |
| `WithVCFCount(len, n)` | Sets the count width in octets and its value |
| `WithInsertZone(data)` | Fills the insert zone |
| `WithOCF(ocf)` | Attaches the 4-byte OCF and sets the flag |
| `WithoutFECF()` | Drops the CRC on this frame |
| `WithSourceOrDest(flag)` | Whether the SCID names the source or the destination |
| `WithBypassSeqCtrl()` | Marks the frame expedited |
| `WithProtCtrlCmd()` | Marks it a protocol control command |

## Truncated frames

For a very short telecommand, annex D allows a stripped-down frame:

```go
tc, err := usdl.NewTruncatedFrame(100, 1, 0, cmdBytes)
```

The whole frame stays within 6 to 32 octets. No insert zone, no OCF, no FECF, no pointer, asking for any of those gives `ErrTruncatedFrameFields`.

## Channel configuration

```go
config := usdl.ChannelConfig{
    FrameLength:   256,  // 0 means variable length
    HasOCF:        false,
    HasFECF:       true,
    InsertZoneLen: 0,
    VCFCountLen:   2,    // 0 to 7 octets
    IdlePattern:   nil,
}
```

`FrameLength: 0` puts the channel in variable-length mode, which is what the octet stream service needs.

## Services

All three operate at the MAP level. A virtual channel keeps a per-MAP receive demultiplexer, so two MAPs sharing a VC each get their own traffic.

### MAPPacketService

```go
vc := usdl.NewVirtualChannel(1, 32)
counter := usdl.NewFrameCounter()

svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, counter)
svc.SetPacketSizer(spp.PacketSizer)

svc.Send(packetBytes)
svc.Flush()
```

On a fixed-length channel this uses rule `000` with the First Header Pointer, and `Flush` completes a partial frame with an Encapsulation Idle Packet. On a variable-length channel each frame carries complete packets under rule `111`.

### MAPAccessService

Constant-length SDUs:

```go
svc := usdl.NewMAPAccessService(100, 1, 1, sduSize, vc, config, counter)
```

Fixed-length channels start an SDU under rule `001` and continue under `010`, delimited by the Last Valid Octet Pointer.

### MAPOctetStreamService

```go
svc := usdl.NewMAPOctetStreamService(100, 1, 2, vc, config, counter)
```

Rule `011`, variable-length channels only. Clause 4.2.4.1. On a fixed-length channel you get `ErrOctetStreamFixedLength`.

## OCF suppliers

On a channel configured with `HasOCF`, install a supplier or nothing sends:

```go
svc.SetOCFSupplier(func() []byte { return clcw.Encode() })
mc.SetOCFSupplier(func() []byte { return clcw.Encode() })  // for idle frames
```

Without one you get `ErrNoOCFSupplier`. Astro refuses rather than fabricating an all-zero Type-1 report, because a made-up CLCW is worse than a missing frame.

## Channel hierarchy

```go
mc := usdl.NewMasterChannel(100, config)
mc.AddVirtualChannel(vc, 1)

pc := usdl.NewPhysicalChannel("X-band", config)
pc.AddMasterChannel(mc, 1)

frame, err := pc.GetNextFrame()
```

`GetNextFrameOrIdle()` emits an OID frame when nothing is queued. The master channel holds one persistent `OIDSequence` so back-to-back idle frames carry different PN fill.

Gap detection needs the count width, since it is a managed parameter:

```go
det := usdl.NewFrameGapDetector(config.VCFCountLen)
```

## Errors

| Error | Cause |
|---|---|
| `ErrInvalidVersion` | TFVN is not 12 |
| `ErrInvalidSpacecraftID` | Outside 0-65535 |
| `ErrInvalidVCID` | Outside 0-63 |
| `ErrInvalidMAPID` | Outside 0-15 |
| `ErrInvalidVCFCountLen` | Outside 0-7 octets |
| `ErrInvalidVCFCount` | Exceeds the configured field width |
| `ErrInvalidConstructionRule` | Outside 0-7 |
| `ErrInvalidPointer` | Exceeds the data zone length |
| `ErrInvalidFECSize` | Not 0 or 2, USLP has only the 16-bit FECF |
| `ErrTruncatedFrameFields` | Truncated frame asked for an insert zone, OCF, FECF, or pointer |
| `ErrTruncatedFrameTooShort` | Data zone under 1 octet |
| `ErrTruncatedFrameTooLong` | Frame over 32 octets |
| `ErrNoOCFSupplier` | Channel needs an OCF but none was installed |
| `ErrOctetStreamFixedLength` | MAPO on a fixed-length channel |
| `ErrFrameLengthMismatch` | Length field disagrees with the buffer |
| `ErrNoPacketSizer` | `SetPacketSizer` was not called before receiving |
| `ErrCRCMismatch` | FECF did not match |

## Notes

Commentary, not sourced from the standard.

**Why unify at all?** A mission running TM down, TC up, and AOS for the high-rate instrument maintains three frame formats, three multiplexers, and three sets of test tooling. USLP is one of each.

**Why a variable-width frame counter?** A quiet housekeeping channel does not need 24 bits and a high-rate instrument channel does. Making the width a managed parameter lets each virtual channel pay only for what it uses.

**Why 16 bits of spacecraft ID?** Constellations. Ten bits is 1,024 spacecraft, which felt endless when TM was written and does not any more.

**Why so many construction rules?** They are the seams where TM, TC, and AOS were stitched together. Each old protocol's data zone layout survives as a rule, which is what makes migration possible without changing the receiver's mental model all at once.

## Reference

- [CCSDS 732.1-B-3](https://ccsds.org/Pubs/732x1b3e1.pdf), Unified Space Data Link Protocol (Blue Book)
- [SANA UPID registry](https://sanaregistry.org/r/uslp_protocol_id), USLP Protocol Identifiers
- [CLI](/cli/usdl) | [Conformance](/conformance/usdl) | [The stack](/docs/start/concepts)
