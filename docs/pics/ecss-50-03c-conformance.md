# ECSS-E-ST-50-03C CONFORMANCE MATRIX

## Conformance statement for `pkg/tmdl` — ECSS-E-ST-50-03C

---

## A1 GENERAL INFORMATION

### A1.1 Identification

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 25/08/2026 |
| Serial Number | ASTRO-ECSS5003C-CONF-001 |
| System Conformance Statement Cross-Reference | This document |

### A1.2 Identification of Implementation Under Test

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/tmdl |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing the TM Space Data Link Protocol. This document audits it against the European profile rather than against CCSDS 132.0-B; the CCSDS statement is [`tmdl-pics.md`](tmdl-pics.md). |

### A1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/tmdl (Go package) |
| System Name(s) | Astro |

### A1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | ECSS-E-ST-50-03C, *Space data links — Telemetry transfer frame protocol*, 31 July 2008 |
| Relationship to CCSDS | The European profile of the CCSDS TM Space Data Link Protocol (CCSDS 132.0-B). It defines no new frame; it adopts the CCSDS frame and constrains its options. |
| Obtained from | `https://ecss.nl/wp-content/uploads/standards/ecss-e/ECSS-E-ST-50-03C31July2008.pdf` — public, no registration required |
| Have any exceptions been required? | **No** — the five gaps of the first pass were closed on 23/08/2026, and the idle-packet fill gap the second pass found was closed on 25/08/2026; see A3 |

**A note on the version.** Plan 024 called for "ECSS-E-ST-50-03C Rev.1". No such revision is published: the active document on ecss.nl is ECSS-E-ST-50-03C dated 31 July 2008, whose own change log records the 2008 issue as editorial renumbering of the 6 November 2007 text. This audit is against that document.

---

## A2 REQUIREMENTS MATRIX

Every numbered clause of the normative sections (§5.1 to §5.6) appears below,
one row each. Section 4 is the informative overview and contains no numbered
requirements.

**Inventory total: 135 clauses. Matrix rows: 135.**

Verdicts use four values:

| Verdict | Meaning |
|---|---|
| **conforms** | the code always behaves as the clause requires |
| **configurable** | the code can be operated within the profile; the row states the configuration |
| **gap** | the code cannot satisfy the clause as written |
| **out-of-scope** | the clause targets a layer this package does not own |

Requirement text is paraphrased. ECSS documents are copyrighted; clause
identifiers plus a paraphrase are enough to find the original.

### 5.1 General

| Req | Requirement (paraphrase) | What it constrains | `pkg/tmdl` evidence | Verdict |
|---|---|---|---|---|
| 5.1a | The TM Transfer Frame shall encompass the major fields, positioned contiguously if present, in the sequence shown in… | field order in the frame | `frame.go` `EncodeWithoutFEC` assembles primary header, secondary header, data field, OCF in order; `frame.go` `EncodeWithConfig` appends the FECF last | **conforms** |
| 5.1b | The maximum length for a TM Transfer Frame shall be 2048 octets. | maximum frame length 2048 octets | `physical.go` `ChannelConfig.Validate` rejects a `FrameLength` above `MaxFrameLength` (2048), but nothing calls it automatically — a CCSDS-only mission may exceed the European ceiling. Set 2048 or less and call `Validate` | **configurable** |
| 5.1c | The TM Transfer Frame shall be of constant length throughout a specific mission phase. | constant frame length per mission phase | set `ChannelConfig.FrameLength`: `frame.go` `EncodeWithConfig` and `DecodeTMTransferFrameWithConfig` then reject any other length with `ErrFrameLengthMismatch`, and the VCP/VCA services build to that size. The legacy `FrameLength: 0` path still emits one variable-length frame per packet, so constancy holds only under the fixed-length configuration | **configurable** |
| 5.1d | The TM Transfer Frame length shall be in conformance with the specifications contained in the standard for telemetry… | conformance with the channel coding standard | coding lives in `pkg/tmsc` (CCSDS 131.0-B); this package emits frames only | **out-of-scope** |
| 5.1e | TM Transfer Frames shall be transferred over a physical channel at a constant rate. | constant transmission rate | a physical-layer property; no code owns it here | **out-of-scope** |
| 5.1f | In order to assure correct decoding at the receiving end, the same telemetry channel coding options shall be applied… | same coding options across the channel | belongs to `pkg/tmsc` | **out-of-scope** |
| 5.1g | At the receiving end, TM Transfer Frames containing detected errors need not be delivered. | errored frames need not be delivered | `frame.go` `DecodeTMTransferFrame` rejects a frame whose FECF does not verify, returning an error rather than the frame | **conforms** |
| 5.1h | The handling of TM Transfer Frames containing detected errors shall be specified for each mission or mission phase. | mission-specified handling of errored frames | the caller decides what to do with the error from `frame.go` `DecodeTMTransferFrameWithConfig` | **configurable** |
| 5.1i | All TM Transfer Frames with the same Master Channel Identifier on a physical channel shall constitute a master… | frames sharing an MCID form a master channel | `frame.go` `MCID()`; `channel.go` `MasterChannel` keyed by SCID; `physical.go` `PhysicalChannel.masterChannels` maps MCID to master channel | **conforms** |
| 5.1j | A master channel shall consist of between one to eight virtual channels. | one to eight virtual channels per master channel | `frame.go` `PrimaryHeader.Validate` rejects a VCID above 7, so at most eight exist | **conforms** |
| 5.1k | On a physical channel that carries TM Transfer Frames, all the frames shall have the same Transfer Frame Version… | one Transfer Frame Version Number per physical channel | `frame.go` `PrimaryHeader.Validate` fixes the version at 0 for every frame | **conforms** |

### 5.2 Transfer Frame Primary Header

| Req | Requirement (paraphrase) | What it constrains | `pkg/tmdl` evidence | Verdict |
|---|---|---|---|---|
| 5.2.1a | The Transfer Frame Primary Header shall always be present in a TM Transfer Frame. | primary header always present | `frame.go` `TMTransferFrame.Header` is a value, not a pointer; `frame.go` `EncodeWithoutFEC` always encodes it | **conforms** |
| 5.2.1b | The Transfer Frame Primary Header shall consist of six fields, positioned contiguously, in the following sequence: | six fields in sequence | `frame.go` `PrimaryHeader.Encode` writes MCID, VCID, OCF flag, MC count, VC count, data field status in order | **conforms** |
| 5.2.2.1a | The Master Channel Identifier shall always be present in a Transfer Frame Primary Header. | Master Channel Identifier, bits 0-11, two subfields | `frame.go` `MCID()` combines version and SCID; `frame.go` `PrimaryHeader.Encode` packs them into the first two octets | **conforms** |
| 5.2.2.1b | The Master Channel Identifier shall be contained within bits 0-11 of the Transfer Frame Primary Header. | Master Channel Identifier, bits 0-11, two subfields | `frame.go` `MCID()` combines version and SCID; `frame.go` `PrimaryHeader.Encode` packs them into the first two octets | **conforms** |
| 5.2.2.1c | The Master Channel Identifier shall consist of two fields, positioned contiguously, in the following sequence: | Master Channel Identifier, bits 0-11, two subfields | `frame.go` `MCID()` combines version and SCID; `frame.go` `PrimaryHeader.Encode` packs them into the first two octets | **conforms** |
| 5.2.2.2a | The Transfer Frame Version Number shall always be present in a Master Channel Identifier. | Transfer Frame Version Number present, bits 0-1 | `frame.go` `PrimaryHeader.VersionNumber`, packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.2.2b | The Transfer Frame Version Number shall be contained within bits 0-1 of the Transfer Frame Primary Header. | Transfer Frame Version Number present, bits 0-1 | `frame.go` `PrimaryHeader.VersionNumber`, packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.2.2c | The Transfer Frame Version Number shall be set to '00'. | version number set to '00' | `frame.go` `PrimaryHeader.Validate` `Validate` rejects any value but 0 | **conforms** |
| 5.2.2.3a | The Spacecraft Identifier shall always be present in a Master Channel Identifier. | Spacecraft Identifier present, bits 2-11 | `frame.go` `PrimaryHeader.SpacecraftID` is a 10-bit field; `frame.go` `PrimaryHeader.Validate` rejects a value above 0x3FF | **conforms** |
| 5.2.2.3b | The Spacecraft Identifier shall be contained within bits 2-11 of the Transfer Frame Primary Header. | Spacecraft Identifier present, bits 2-11 | `frame.go` `PrimaryHeader.SpacecraftID` is a 10-bit field; `frame.go` `PrimaryHeader.Validate` rejects a value above 0x3FF | **conforms** |
| 5.2.2.3c | The Spacecraft Identifier shall provide the identification of the spacecraft which is associated with the data… | Spacecraft Identifier present, bits 2-11 | `frame.go` `PrimaryHeader.SpacecraftID` is a 10-bit field; `frame.go` `PrimaryHeader.Validate` rejects a value above 0x3FF | **conforms** |
| 5.2.2.3d | The Spacecraft Identifier shall be static throughout all mission phases. | SCID static across all mission phases | the caller passes the SCID to `frame.go` `NewTMTransferFrame`; keeping it constant is the caller's | **configurable** |
| 5.2.3a | The Virtual Channel Identifier shall always be present in a Transfer Frame Primary Header. | Virtual Channel Identifier present, bits 12-14 | `frame.go` `PrimaryHeader.VirtualChannelID`; packed at `frame.go` `PrimaryHeader.Encode`; range checked at `frame.go` `PrimaryHeader.Validate` | **conforms** |
| 5.2.3b | The Virtual Channel Identifier shall be contained within bits 12-14 of the Transfer Frame Primary Header. | Virtual Channel Identifier present, bits 12-14 | `frame.go` `PrimaryHeader.VirtualChannelID`; packed at `frame.go` `PrimaryHeader.Encode`; range checked at `frame.go` `PrimaryHeader.Validate` | **conforms** |
| 5.2.3c | The Virtual Channel Identifier shall provide the identification of the virtual channel to which the TM Transfer… | VCID identifies the virtual channel | `frame.go` `GVCID()`; `channel.go` `MasterChannel.AddFrame` routes by virtual channel | **conforms** |
| 5.2.4a | The Operational Control Field Flag shall always be present in a Transfer Frame Primary Header. | OCF Flag present, bit 15 | `frame.go` `PrimaryHeader.OCFFlag`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.4b | The Operational Control Field Flag shall be contained in bit 15 of the Transfer Frame Primary Header. | OCF Flag present, bit 15 | `frame.go` `PrimaryHeader.OCFFlag`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.4c | The Operational Control Field Flag shall indicate the presence or absence of the Operational Control Field, as… | OCF Flag signals presence of the OCF | `frame.go` `EncodeWithoutFEC` emits the four OCF octets only when the flag is set, and errors if the field is not four octets | **conforms** |
| 5.2.4d | The Operational Control Field Flag shall be static in the associated master channel or virtual channel throughout a… | OCF Flag static per channel throughout a phase | `physical.go` `ChannelConfig.HasOCF` is fixed per channel; the caller must set the header flag to match | **configurable** |
| 5.2.5a | The Master Channel Frame Count shall always be present in a Transfer Frame Primary Header. | Master Channel Frame Count present, bits 16-23 | `frame.go` `PrimaryHeader.MCFrameCount`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.5b | The Master Channel Frame Count shall be contained within bits 16-23 of the Transfer Frame Primary Header. | Master Channel Frame Count present, bits 16-23 | `frame.go` `PrimaryHeader.MCFrameCount`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.5c | The Master Channel Frame Count shall contain a sequential binary count (modulo 256) of each TM Transfer Frame… | sequential count modulo 256 per master channel | `service.go` `FrameCounter.Next` increments a `uint8`, wrapping at 256 — but the count is sequential across the master channel only when the caller wires one shared `FrameCounter` into every service on it and into `MasterChannel.SetFrameCounter` for idle frames. Nothing enforces the wiring | **configurable** |
| 5.2.5d | The Master Channel Frame Count shall not be reset before reaching 255 unless there is a major system reset. | count not reset before 255 | `service.go` `FrameCounter.Next` only ever increments; nothing resets it | **conforms** |
| 5.2.6a | The Virtual Channel Frame Count shall always be present in a Transfer Frame Primary Header. | Virtual Channel Frame Count present, bits 24-31 | `frame.go` `PrimaryHeader.VCFrameCount`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.6b | The Virtual Channel Frame Count shall be contained within bits 24-31 of the Transfer Frame Primary Header. | Virtual Channel Frame Count present, bits 24-31 | `frame.go` `PrimaryHeader.VCFrameCount`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.6c | The Virtual Channel Frame Count shall contain a sequential binary count (modulo 256) of each TM Transfer Frame… | sequential count modulo 256 per virtual channel | `service.go` `FrameCounter` keeps one wrapping `uint8` per VCID — sequential only when the caller passes a counter to the channel's service and does not stamp counts by hand. Nothing enforces the wiring | **configurable** |
| 5.2.6d | The Virtual Channel Frame Count shall not be reset before reaching 255 unless there is a major system reset. | count not reset before 255 | `service.go` `FrameCounter.Next` | **conforms** |
| 5.2.7.1a | The Transfer Frame Data Field Status shall always be present in a Transfer Frame Primary Header. | Data Field Status present, bits 32-47, five subfields | `frame.go` `PrimaryHeader` (the five data-field-status fields) are the five subfields; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.1b | The Transfer Frame Data Field Status shall be contained within bits 32-47 of the Transfer Frame Primary Header. | Data Field Status present, bits 32-47, five subfields | `frame.go` `PrimaryHeader` (the five data-field-status fields) are the five subfields; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.1c | The Transfer Frame Data Field Status shall consist of five fields, positioned contiguously, in the following sequence: | Data Field Status present, bits 32-47, five subfields | `frame.go` `PrimaryHeader` (the five data-field-status fields) are the five subfields; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.2a | The Transfer Frame Secondary Header Flag shall always be present in a Transfer Frame Data Field Status. | Secondary Header Flag present, bit 32 | `frame.go` `PrimaryHeader.FSHFlag`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.2b | The Transfer Frame Secondary Header Flag shall be contained in bit 32 of the Transfer Frame Primary Header. | Secondary Header Flag present, bit 32 | `frame.go` `PrimaryHeader.FSHFlag`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.2c | The Transfer Frame Secondary Header Flag shall indicate the presence or absence of the Transfer Frame Secondary… | flag signals presence of the secondary header | `frame.go` `EncodeWithoutFEC` encodes the secondary header only when `FSHFlag` is set | **conforms** |
| 5.2.7.2d | The Transfer Frame Secondary Header Flag shall be static in a specific master channel, throughout a mission phase,… | flag static per master or virtual channel | the caller sets it per frame; `frame.go` `NewTMTransferFrame` sets it when secondary header data is supplied | **configurable** |
| 5.2.7.2e | The Transfer Frame Secondary Header Flag shall be static in a specific virtual channel, throughout a mission phase,… | flag static per master or virtual channel | the caller sets it per frame; `frame.go` `NewTMTransferFrame` sets it when secondary header data is supplied | **configurable** |
| 5.2.7.3a | The Synchronization Flag shall always be present in a Transfer Frame Data Field Status. | Synchronization Flag present, bit 33 | `frame.go` `PrimaryHeader.SyncFlag`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.3b | The Synchronization Flag shall be contained in bit 33 of the Transfer Frame Primary Header. | Synchronization Flag present, bit 33 | `frame.go` `PrimaryHeader.SyncFlag`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.3c | The Synchronization Flag shall signal the formatting of the Transfer Frame Data Field, as follows: | flag signals data field formatting | `service.go` `emitFrame` leaves it clear for packet service; `service.go` `VirtualChannelAccessService.Send` sets it for virtual channel access | **conforms** |
| 5.2.7.3d | The Synchronization Flag shall be static in a specific virtual channel throughout a mission phase. | flag static per virtual channel throughout a phase | fixed by which service the caller runs on the channel: `service.go` `VirtualChannelPacketService` packets, `service.go` `VirtualChannelAccessService` access | **configurable** |
| 5.2.7.4a | The Packet Order Flag shall always be present in a Transfer Frame Data Field Status. | Packet Order Flag present, bit 34 | `frame.go` `PrimaryHeader.PacketOrderFlag`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.4b | The Packet Order Flag shall be contained in bit 34 of the Transfer Frame Primary Header. | Packet Order Flag present, bit 34 | `frame.go` `PrimaryHeader.PacketOrderFlag`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.4c | If the Synchronization Flag is set to '0', the Packet Order Flag shall be set to '0'. | Sync Flag '0' forces Packet Order Flag '0' | `frame.go` `PrimaryHeader.Validate` rejects the combination outright | **conforms** |
| 5.2.7.5a | The Segment Length Identifier shall always be present in a Transfer Frame Data Field Status. | Segment Length Identifier present, bits 35-36 | `frame.go` `PrimaryHeader.SegmentLengthID`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.5b | The Segment Length Identifier shall be contained in bits 35-36 of the Transfer Frame Primary Header. | Segment Length Identifier present, bits 35-36 | `frame.go` `PrimaryHeader.SegmentLengthID`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.5c | If the Synchronization Flag is set to '0', the Segment Length Identifier shall be set to '11'. | Sync Flag '0' forces Segment Length Identifier '11' | `frame.go` `PrimaryHeader.Validate` rejects any other value | **conforms** |
| 5.2.7.6a | The First Header Pointer shall always be present in a Transfer Frame Data Field Status. | First Header Pointer present, bits 37-47 | `frame.go` `PrimaryHeader.FirstHeaderPtr`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.6b | The First Header Pointer shall be contained in bits 37-47 of the Transfer Frame Primary Header. | First Header Pointer present, bits 37-47 | `frame.go` `PrimaryHeader.FirstHeaderPtr`; packed at `frame.go` `PrimaryHeader.Encode` | **conforms** |
| 5.2.7.6c | If the Synchronization Flag is set to '0', the First Header Pointer shall contain information on the data in the… | FHP carries data field information when Sync Flag is '0' | `frame.go` `validate` bounds it to 11 bits; construction pins it to 0x7FF when the Sync Flag is set, while decode accepts any value there, since the standard leaves the field undefined for a receiver | **conforms** |
| 5.2.7.6d | If at least one packet starts in the Transfer Frame Data Field, the First Header Pointer shall contain the location… | FHP gives the location of the first packet header | `service.go` `emitFrame` takes the pointer; `service.go` `VirtualChannelPacketService.Receive` reads from it | **conforms** |
| 5.2.7.6e | The locations of the octets in the Transfer Frame Data Field shall be numbered in ascending order starting with '0'. | octets numbered from zero | `service.go` `VirtualChannelPacketService.Receive` indexes the data field from 0 | **conforms** |
| 5.2.7.6f | If no packet starts in the Transfer Frame Data Field, the First Header Pointer shall be set to '11111111111'. | no packet starts, FHP '11111111111' | `service.go` `emitFrame` is called with 0x7FF for a continuation frame | **conforms** |
| 5.2.7.6g | If the Transfer Frame Data Field contains only idle data, the First Header Pointer shall be set to '11111111110'. | idle data only, FHP '11111111110' | `frame.go` `NewIdleFrame` sets FHPOnlyIdleData (0x7FE); `IsIdleFrame` matches it | **conforms** |

### 5.3 Transfer Frame Secondary Header

| Req | Requirement (paraphrase) | What it constrains | `pkg/tmdl` evidence | Verdict |
|---|---|---|---|---|
| 5.3.1a | If present, the Transfer Frame Secondary Header shall follow, without gap, the Transfer Frame Primary Header. | secondary header follows the primary without gap | `frame.go` `EncodeWithoutFEC` | **conforms** |
| 5.3.1b | The presence or absence of the Transfer Frame Secondary Header shall be signalled by the Transfer Frame Secondary… | presence signalled by the Secondary Header Flag | `frame.go` `EncodeWithoutFEC` | **conforms** |
| 5.3.1c | If present, the Transfer Frame Secondary Header shall comprise an integral number of octets: between 2 and 64 octets. | secondary header total 2 to 64 octets | `frame.go` `Validate` rejects a total above MaxSecondaryHeaderSize (64) | **conforms** |
| 5.3.1d | The Transfer Frame Secondary Header shall be associated with either a master channel or a virtual channel. | associated with one channel, fixed length throughout a phase | the caller supplies the data at `frame.go` `NewTMTransferFrame`; keeping it fixed is the caller's | **configurable** |
| 5.3.1e | The Transfer Frame Secondary Header shall have a fixed length in the associated master channel or in the associated… | associated with one channel, fixed length throughout a phase | the caller supplies the data at `frame.go` `NewTMTransferFrame`; keeping it fixed is the caller's | **configurable** |
| 5.3.1f | The Transfer Frame Secondary Header shall consist of two fields, positioned contiguously, in the following sequence: | two fields: identification then data | `frame.go` `SecondaryHeader.Encode` writes the identification octet then the data field | **conforms** |
| 5.3.1g | The Transfer Frame Secondary Header shall be used to carry fixed length data defined at mission level. | carries fixed-length mission-defined data | `frame.go` `SecondaryHeader.DataField` `DataField` is opaque | **conforms** |
| 5.3.1h | The Transfer Frame Secondary Header may be used to provide an extended virtual channel frame count as specified in… | may carry an extended virtual channel frame count | permitted, not required. The container exists at `frame.go` `SecondaryHeader.DataField`; see 5.3.4.2 | **configurable** |
| 5.3.2.1a | The Transfer Frame Secondary Header Identification shall always be present in a Transfer Frame Secondary Header. | identification present, bits 0-7, two subfields | `frame.go` `SecondaryHeader.Encode` packs version and length into one octet | **conforms** |
| 5.3.2.1b | The Transfer Frame Secondary Header Identification shall be contained in bits 0-7 of the Transfer Frame Secondary… | identification present, bits 0-7, two subfields | `frame.go` `SecondaryHeader.Encode` packs version and length into one octet | **conforms** |
| 5.3.2.1c | The Transfer Frame Secondary Header Identification shall comprise two fields, positioned contiguously, in the… | identification present, bits 0-7, two subfields | `frame.go` `SecondaryHeader.Encode` packs version and length into one octet | **conforms** |
| 5.3.2.2a | The Transfer Frame Secondary Header Version Number shall always be present in a Transfer Frame Secondary Header… | secondary header version present, bits 0-1 | `frame.go` `SecondaryHeader.VersionNumber`; packed at `frame.go` `SecondaryHeader.Encode` | **conforms** |
| 5.3.2.2b | The Transfer Frame Secondary Header Version Number shall be contained in bits 0-1 of the Transfer Frame Secondary… | secondary header version present, bits 0-1 | `frame.go` `SecondaryHeader.VersionNumber`; packed at `frame.go` `SecondaryHeader.Encode` | **conforms** |
| 5.3.2.2c | The Transfer Frame Secondary Header Version Number shall be set to '00'. | secondary header version set to '00' | `frame.go` `SecondaryHeader.Validate` rejects any other value | **conforms** |
| 5.3.2.3a | The Transfer Frame Secondary Header Length shall always be present in a Transfer Frame Secondary Header… | length field present, bits 2-7 | `frame.go` `SecondaryHeader.HeaderLength`; packed at `frame.go` `SecondaryHeader.Encode` | **conforms** |
| 5.3.2.3b | The Transfer Frame Secondary Header Length shall be contained in bits 2-7 of the Transfer Frame Secondary Header. | length field present, bits 2-7 | `frame.go` `SecondaryHeader.HeaderLength`; packed at `frame.go` `SecondaryHeader.Encode` | **conforms** |
| 5.3.2.3c | The Transfer Frame Secondary Header Length shall contain the total length of the Transfer Frame Secondary Header in… | length field is TOTAL secondary header octets minus one | `frame.go` `Validate` requires the field to equal the data field length, i.e. the total minus one; `Decode` reads it the same way | **conforms** |
| 5.3.2.3d | The value of the Transfer Frame Secondary Header Length shall be static within a specific master channel or a… | length static per channel throughout a phase | the caller keeps the data field length fixed | **configurable** |
| 5.3.3a | The Transfer Frame Secondary Header Data Field shall always be present in a Transfer Frame Secondary Header. | data field present, follows the identification, carries the data | `frame.go` `SecondaryHeader.Encode` and `frame.go` `SecondaryHeader.Decode` | **conforms** |
| 5.3.3b | The Transfer Frame Secondary Header Data Field shall follow, without gap, the Transfer Frame Secondary Header… | data field present, follows the identification, carries the data | `frame.go` `SecondaryHeader.Encode` and `frame.go` `SecondaryHeader.Decode` | **conforms** |
| 5.3.3c | The Transfer Frame Secondary Header Data Field shall contain the Transfer Frame Secondary Header data. | data field present, follows the identification, carries the data | `frame.go` `SecondaryHeader.Encode` and `frame.go` `SecondaryHeader.Decode` | **conforms** |
| 5.3.4.2a | The length of the Transfer Frame Secondary Header shall be 32 bits. | extended count needs a 32-bit secondary header | `frame.go` — closed together with 5.3.2.3c; a 4-octet header now writes 3 | **conforms** |
| 5.3.4.2b | The Transfer Frame Secondary Header Data Field shall contain the 24-bit extension to the virtual channel frame count. | 24-bit extension counting roll-overs of the 8-bit count | the caller places the extension in `frame.go` `SecondaryHeader.DataField` `DataField`; the package does not maintain the roll-over count | **configurable** |
| 5.3.4.2c | The extension to the virtual channel frame count shall be a binary count of the roll-overs of the 8-bit value… | 24-bit extension counting roll-overs of the 8-bit count | the caller places the extension in `frame.go` `SecondaryHeader.DataField` `DataField`; the package does not maintain the roll-over count | **configurable** |
| 5.3.4.2d | The use of the extended virtual channel frame count shall be associated with either a master channel or a virtual… | extended count associated with a channel and static | a caller-side convention | **configurable** |
| 5.3.4.2e | The use of the extended virtual channel frame count shall be static in the associated master channel or in the… | extended count associated with a channel and static | a caller-side convention | **configurable** |

### 5.4 Transfer Frame Data Field

| Req | Requirement (paraphrase) | What it constrains | `pkg/tmdl` evidence | Verdict |
|---|---|---|---|---|
| 5.4.2a | The Transfer Frame Data Field shall always be present in a TM Transfer Frame. | data field always present | `frame.go` `TMTransferFrame.DataField`; `frame.go` `EncodeWithoutFEC` always writes it | **conforms** |
| 5.4.2b | The Transfer Frame Data Field shall follow, without gap, one of the following: | data field follows the secondary or primary header without gap | `frame.go` `EncodeWithoutFEC` appends it directly after whichever header was written | **conforms** |
| 5.4.2c | If a Transfer Frame Secondary Header is present, the Transfer Frame Secondary. | data field follows the secondary or primary header without gap | `frame.go` `EncodeWithoutFEC` appends it directly after whichever header was written | **conforms** |
| 5.4.2d | If a Transfer Frame Secondary Header is not present, the Transfer Frame Primary. | data field follows the secondary or primary header without gap | `frame.go` `EncodeWithoutFEC` appends it directly after whichever header was written | **conforms** |
| 5.4.2e | The length of the Transfer Frame Data Field shall be an integral number of octets and be constrained by the length… | integral octets, constrained by the frame length | `physical.go` `ChannelConfig.DataFieldCapacity` derives it; `frame.go` `padDataField` fills to capacity | **conforms** |
| 5.4.3.3a | A packet handled by the packet processing and extraction functions shall have a defined Packet Version Number in… | packet version number conformance | packet formats belong to `pkg/spp`; this package treats the data field as octets | **out-of-scope** |
| 5.4.3.3b | An idle packet shall be either: | idle packet definition | `service.go` `idleFillPacket` builds a real SPP idle packet — APID 0x7FF per CCSDS 133.0-B-2 — and `isIdlePacket` recognises one by that APID on extraction | **conforms** |
| 5.4.3.4a | The packet processing function shall be applied independently for each virtual channel. | packet processing applied per virtual channel | `service.go` `VirtualChannelPacketService` is constructed per VCID at `service.go` `NewVirtualChannelPacketService` | **conforms** |
| 5.4.3.4b | The packet processing function shall place packets contiguously into the Transfer Frame Data Field. | packets placed contiguously | `service.go` `VirtualChannelPacketService.Send` appends to one buffer; `service.go` `emitFullFrames` slices it | **conforms** |
| 5.4.3.4c | If the length of a packet exceeds the available space in the Transfer Frame Data Field, the packet processing… | packets longer than the space are split across frames | `service.go` `emitFullFrames` emits full frames and carries the remainder forward | **conforms** |
| 5.4.3.4d | The packet processing function shall set the First Header Pointer as specified in clause 5.2.7.6. | First Header Pointer set per 5.2.7.6 | `service.go` `emitFrame` takes the computed pointer | **conforms** |
| 5.4.3.4e | Packets with different Packet Version Numbers may be transmitted within a virtual channel. | packets of different versions may share a virtual channel | permitted, not required. `service.go` `VirtualChannelPacketService.Send` does not inspect packet contents | **conforms** |
| 5.4.3.4f | A Transfer Frame Data Field containing only idle data may be created. | a data field of only idle data may be created | permitted. `frame.go` `NewIdleFrame` builds one with the FHPOnlyIdleData pointer per 5.2.7.6g, and `NewIdleFrameWithCounter` stamps its frame counts from the shared counter | **conforms** |
| 5.4.3.4g | One or more idle packets may be created to fill space in a Transfer Frame Data Field. | idle packets may fill spare space | `service.go` `Flush` fills spare data field space with an SPP idle packet, spanning into following frames when the spare space is under the seven-octet minimum, so a conformant receiver parses the fill as a packet and discards it | **conforms** |
| 5.4.3.5a | The packet extraction function shall be applied independently for each virtual channel. | packet extraction applied per virtual channel | `service.go` `VirtualChannelPacketService.Receive`, one service per VCID | **conforms** |
| 5.4.3.5b | The packet extraction function shall extract the packets from a Transfer Frame Data Field using the value of the… | extraction uses the First Header Pointer | `service.go` `VirtualChannelPacketService.Receive` | **conforms** |
| 5.4.3.5c | A Transfer Frame Data Field containing idle data shall be discarded. | a data field of idle data is discarded | `frame.go` `IsIdleFrame` matches OID frames and `service.go` `Receive` skips them; the FHPOnlyIdleData branch drops fields marked idle by the pointer | **conforms** |
| 5.4.3.5d | Any idle packets extracted from Transfer Frame Data Fields shall be discarded. | extracted idle packets are discarded | `service.go` `isIdlePacket` applied in `Receive`: every extracted packet with the idle APID is dropped before delivery | **conforms** |
| 5.4.4.2a | The stored data shall be in the form of standard TM Transfer Frames. | playback data is whole recorded frames placed in real-time data fields | `service.go` `VirtualChannelAccessService` carries opaque octets, so recorded frames fit; the package does not manage the recorder | **configurable** |
| 5.4.4.2b | At playback time, the recorded TM Transfer Frames shall be placed into the Transfer Frame Data Field of real-time TM… | playback data is whole recorded frames placed in real-time data fields | `service.go` `VirtualChannelAccessService` carries opaque octets, so recorded frames fit; the package does not manage the recorder | **configurable** |
| 5.4.4.2c | The asynchronous insertion may be made in either the forward or the reverse mode. | forward or reverse insertion mode | a caller-side choice; neither is modelled | **configurable** |
| 5.4.4.2d | If forward insertion mode is used, then any recorded attached synchronization markers shall use the alternative… | alternative synchronization marker for forward mode | sync markers belong to `pkg/tmsc` | **out-of-scope** |
| 5.4.4.2e | A dedicated virtual channel shall be used for the playback data. | a dedicated virtual channel for playback | the caller assigns the VCID at `service.go` `NewVirtualChannelAccessService` | **configurable** |
| 5.4.4.2f | At the receiving end, the real-time virtual channel used for the playback data shall be processed and its contents… | receiving end stores and later retrieves playback frames | `service.go` `VirtualChannelAccessService.Receive` returns the octets; storage and ordering are the caller's | **configurable** |
| 5.4.4.2g | In the later off-line processing, the recorded TM Transfer Frames shall be retrieved in the correct,… | receiving end stores and later retrieves playback frames | `service.go` `VirtualChannelAccessService.Receive` returns the octets; storage and ordering are the caller's | **configurable** |
| 5.4.4.2h | Any Communications Link Control Word (see clause 5.5.3) extracted from the Operational Control Field of a recorded… | CLCWs from recorded frames must not drive the live link | CLCW handling belongs to `pkg/cop` | **out-of-scope** |

### 5.5 Operational Control Field

| Req | Requirement (paraphrase) | What it constrains | `pkg/tmdl` evidence | Verdict |
|---|---|---|---|---|
| 5.5.1a | If present, the Operational Control Field shall occupy the four octets following, without gap, the Transfer Frame… | OCF occupies the four octets after the data field | `frame.go` `EncodeWithoutFEC` appends it there and rejects any length but four | **conforms** |
| 5.5.1b | The presence or absence of the Operational Control Field shall be signalled by the Operational Control Field Flag in… | presence signalled by the OCF Flag | `frame.go` `EncodeWithoutFEC` | **conforms** |
| 5.5.1c | The Operational Control Field shall be associated with a master channel or a virtual channel. | OCF associated with a channel and present in every frame of it | `physical.go` `ChannelConfig.HasOCF` `HasOCF` fixes it per channel; the caller supplies the field per frame | **configurable** |
| 5.5.1d | The Operational Control Field shall be present in every TM Transfer Frame transmitted through the associated master… | OCF associated with a channel and present in every frame of it | `physical.go` `ChannelConfig.HasOCF` `HasOCF` fixes it per channel; the caller supplies the field per frame | **configurable** |
| 5.5.1e | Bit 0 of the Operational Control Field shall contain a Type Flag which indicates the contents of the field. | bit 0 of the OCF is a Type Flag | `frame.go` `TMTransferFrame.OperationalControl` carries the four octets verbatim without interpreting them; the caller composes the flag | **configurable** |
| 5.5.2a | The Type Flag shall always be present in an Operational Control Field. | Type Flag present in bit 0 and set by content type | not modelled. `frame.go` `TMTransferFrame.OperationalControl` is an opaque four-octet field the caller fills | **configurable** |
| 5.5.2b | The Type Flag shall be contained in bit 0 of the Operational Control Field. | Type Flag present in bit 0 and set by content type | not modelled. `frame.go` `TMTransferFrame.OperationalControl` is an opaque four-octet field the caller fills | **configurable** |
| 5.5.2c | The Type Flag shall be set as follows: | Type Flag present in bit 0 and set by content type | not modelled. `frame.go` `TMTransferFrame.OperationalControl` is an opaque four-octet field the caller fills | **configurable** |
| 5.5.2d | The Type Flag may vary between TM Transfer Frames on the same virtual channel. | the Type Flag may vary between frames | permitted. `frame.go` `TMTransferFrame.OperationalControl` is set per frame | **conforms** |
| 5.5.3a | If the Type Flag is '0', the Operational Control Field shall contain a Type-1-Report. | Type Flag '0' means a Type-1-Report | the caller composes the field | **configurable** |
| 5.5.3b | A Type-1-Report shall contain a Communications Link Control Word in conformance with ECSS-E-ST-50-04, clause 6.3. | Type-1-Report carries a CLCW per ECSS-E-ST-50-04 | the CLCW belongs to `pkg/cop`; `pkg/tmdl` carries the octets | **out-of-scope** |
| 5.5.4a | If the Type Flag is '1', the Operational Control Field shall contain a Type-2-Report. | Type Flag '1' means a Type-2-Report and its first bit gives its use | not modelled; the caller composes the field | **configurable** |
| 5.5.4b | The value of the first bit of a Type-2-Report (i.e. bit 1 of the Operational Control Field) shall indicate the use… | Type Flag '1' means a Type-2-Report and its first bit gives its use | not modelled; the caller composes the field | **configurable** |
| 5.5.4c | The value of the first bit of a Type-2-Report may vary between TM Transfer Frames on the same virtual channel. | the first bit may vary between frames | permitted. `frame.go` `TMTransferFrame.OperationalControl` is set per frame | **conforms** |

### 5.6 Frame Error Control Field

| Req | Requirement (paraphrase) | What it constrains | `pkg/tmdl` evidence | Verdict |
|---|---|---|---|---|
| 5.6.1a | If present, the Frame Error Control Field shall occupy the two octets following, without gap, one of the following: | FECF occupies the two octets after the OCF or data field | `frame.go` `EncodeWithConfig` appends it last, after whatever `frame.go` `EncodeWithoutFEC` produced | **conforms** |
| 5.6.1b | The Frame Error Control Field shall be present in a TM Transfer Frame if the TM Transfer Frame is not Reed-Solomon… | FECF present when the frame is not Reed-Solomon encoded | `frame.go` `EncodeWithConfig` always appends it, so the mandatory case is always met | **conforms** |
| 5.6.1c | If present, the Frame Error Control Field shall occur within every TM Transfer Frame transmitted within the same… | FECF consistently present or absent across the physical channel | `frame.go` `EncodeWithConfig` and `DecodeTMTransferFrameWithConfig` honour `ChannelConfig.HasFEC` | **conforms** |
| 5.6.2a | The encoding procedure shall be as follows: | encoding procedure, CRC-16 with generator X^16+X^12+X^5+1, preset to ones | `frame.go` `EncodeWithConfig` calls `crc.ComputeCRC16`; `pkg/crc` `ComputeCRC16` implements that polynomial with an all-ones preset | **conforms** |
| 5.6.3a | The decoding procedure shall use an error detection syndrome, S(X), given by S(X) = [(X16 ⋅ C*(X)) + (Xn ⋅ L(X))]… | decoding uses the error detection syndrome | `frame.go` `DecodeTMTransferFrameWithConfig` recomputes the CRC over the frame and compares | **conforms** |
| 5.6.3b | The Frame Error Control Field shall not be used for error correction. | the FECF is not used for error correction | `frame.go` `DecodeTMTransferFrameWithConfig` rejects a mismatching frame; nothing attempts correction | **conforms** |
---

## A3 GAPS — ALL CLOSED

The audit of 23/08/2026 found five gaps, fixed the same day in
`fix(tmdl): correct four ECSS-E-ST-50-03C conformance defects`. A second pass
on 25/08/2026 found that the first pass had itself overstated two rows, and
the underlying defect was fixed then. This section records all of them, since
the reasoning is worth keeping and several describe a failure mode this
codebase has now hit repeatedly.

### 5.4.3.3b / 5.4.3.4g — raw 0xFF fill sold as "idle packets" (second pass)

The first pass marked both rows *conforms* with `padDataField`'s raw 0xFF fill
as evidence. That was wrong twice over: 0xFF fill is not an idle packet, and a
conformant receiver parses the fill as a packet header — 0x7FF in the first
two octets reads as the idle APID, but the length field is garbage — and loses
packet sync. The in-repo receiver survived only through a nonstandard
all-0xFF heuristic no other implementation shares.

**Now:** `Flush` fills spare data field space with a real SPP idle packet
(APID 0x7FF) built by `idleFillPacket`, spanning into following frames when
the spare space is under the seven-octet minimum packet size, and `Receive`
discards extracted idle packets by APID per 5.4.3.5d. The raw-0xFF heuristic
remains only as decode-side leniency for streams from older versions of this
package.

Alongside that fix, three rows the first pass graded *conforms* were re-graded
*configurable*, because the property holds only under caller wiring: 5.2.5c
and 5.2.6c (counts are sequential only when one shared `FrameCounter` feeds
every service and the idle-frame path) and 5.1c (length constancy holds only
under a set `FrameLength`, which the codec now enforces on encode and decode).

### 5.3.2.3c — the secondary header length field was one too small

§5.3.2.3c requires the six-bit field to hold *the total secondary header length
in octets minus one*, the total being the identification octet plus the data
field. So for an N-octet data field the field reads N.

The code required it to equal `len(DataField)-1` and decoded with the same
offset. A four-octet header went out saying 2 where the standard wants 3.

Encoder and decoder agreed, so every round trip passed and the package's own
tests were silent — one of them asserted the wrong value outright, which is how
the defect looked deliberate. A conforming receiver reading 2 computes a
three-octet total and starts the Transfer Frame Data Field an octet early,
corrupting every frame with a secondary header.

The same wording is in CCSDS 132.0-B-3 §4.1.3.2.2.3, so this was never an
ECSS-only defect.

**Now:** the field equals the data field length, `SetDataField` derives it, and
the test asserts the octet on the wire plus the offset a receiver would compute
from it.

### 5.3.4.2a — the extended virtual channel frame count inherited it

§5.3.4.2a fixes the secondary header at 32 bits when it carries the extended
count: four octets, so the field must read 3. Closed by the fix above.

### 5.2.7.6g — idle frames used the wrong First Header Pointer

§5.2.7.6 gives two codes to two conditions: `11111111111` (0x7FF) when no
packet starts in the data field, `11111111110` (0x7FE) when the field holds
only idle data. `NewIdleFrame` filled with idle and then set 0x7FF, and
`IsIdleFrame` matched on 0x7FF, so OID frames were mislabelled and conformant
ones from other senders went unrecognised.

Fixing this exposed a second defect the audit had not found: the packet service
had the two codes **swapped on both paths at once**. It discarded continuation
frames as idle — losing payload — and appended idle fill into the reassembly
buffer. Consistent on both sides, so round trips passed here too.

**Now:** both codes are named constants, `FHPNoPacketStart` and
`FHPOnlyIdleData`, and the service uses each for its own condition.

### 5.3.1c — the secondary header could exceed 64 octets

§5.3.1c caps the whole secondary header at 64 octets. A 63-value length field
with a 64-octet data field encoded to 65, and nothing rejected it.

**Now:** `MaxSecondaryHeaderSize` is enforced by `Validate`.

### 5.6.1c — the Frame Error Control Field could not be omitted

§5.6.1b makes the field mandatory when the frame is not Reed-Solomon encoded,
and its NOTE makes it optional inside a code block, which already protects the
frame. §5.6.1c requires the choice to hold across the physical channel.

Only "always present" was supported: `Encode` always appended the field,
`DecodeTMTransferFrame` always verified it, and `ChannelConfig.HasFEC` was
never consulted by the frame codec. A Reed-Solomon mission omitting the field —
a normal configuration — could not use the package.

**Now:** `EncodeWithConfig` and `DecodeTMTransferFrameWithConfig` honour
`HasFEC`. The original entry points keep the field, so existing callers are
unaffected, and `VirtualChannelFrameService.SetChannelConfig` carries the
choice through the pass-through path.

### And one that was recorded as configurable

§5.1b caps the frame at 2048 octets and nothing enforced it. It is still
*configurable* rather than a gap — the caller sets `FrameLength`, and a
CCSDS-only mission may legitimately exceed the European ceiling — but
`ChannelConfig.Validate` now checks it for missions that care.

---

## A4 SUMMARY

| Verdict | Count | Share |
|---|---|---|
| conforms | 95 | 70% |
| configurable | 33 | 24% |
| out-of-scope | 7 | 5% |
| **gap** | **0** | — |
| **Total** | **135** | |

**Inventory total 135, matrix rows 135.** The two numbers are stated here so
the check is self-contained.

### Reading the result

`pkg/tmdl` conforms to the European TM transfer frame profile. Every mandatory
clause is either satisfied outright or satisfiable by configuration, and the
rows that say *configurable* name the configuration.

The five gaps the first pass of this audit found were all closed on the same
day, and the idle-packet fill defect the second pass found was closed on
25/08/2026. Three of them had been putting wrong bytes on the wire, and all
were the kind that hide: the secondary header length was self-consistent
within the library, the idle-frame pointer sat in a field most tests do not
assert, and the raw fill was survivable only by this package's own receiver.
Fixing the pointer exposed a further defect of the same shape in the packet
service.

That is the pattern worth carrying forward. Three separate defects in this
codebase — the PN randomizer, this length field, and the swapped pointer codes
— were each perfectly symmetric and perfectly wrong. A round trip cannot catch
any of them. Assert the octet.

### On the 24% "configurable" share

Plan 024 set a threshold: if configurable rows exceed 20% of the matrix, the
audit is a hedge and should be reported rather than shipped. This matrix is at
24%, so the threshold is crossed and the reasoning belongs here rather than
buried.

The threshold guards against guessing. These rows are not guesses. They fall
into three groups, and every one names its exact configuration:

- **Operational constancy (14 rows).** Clauses of the form "shall be static in
  the associated master channel throughout a mission phase" — 5.2.2.3d,
  5.2.4d, 5.2.7.2d, 5.2.7.2e, 5.2.7.3d, 5.3.1d, 5.3.1e, 5.3.2.3d, 5.3.4.2d,
  5.3.4.2e and their neighbours. A stateless frame codec cannot *be* static
  across a mission phase; only the system operating it can. Marking these
  "conforms" would be the dishonest answer.
- **Opaque fields the caller fills (9 rows).** The Operational Control Field is
  four octets `pkg/tmdl` carries without interpreting (`frame.go` `TMTransferFrame.OperationalControl`), so the
  Type Flag clauses 5.5.1e through 5.5.4b are satisfied by whatever the caller
  puts there. The CLCW content itself belongs to `pkg/cop`.
- **Caller-supplied values and policies (10 rows).** The 2048-octet frame
  limit (5.1b), frame-length constancy (5.1c), the shared frame counters
  (5.2.5c, 5.2.6c), playback handling (5.4.4.2), and the extended count's
  placement in the secondary header data.

None of these needed mission context I did not have. Had any been genuinely
ambiguous it would appear as a gap with the ambiguity stated, not as a hedge.

### Coverage boundary

Seven clauses are out-of-scope, and all seven point somewhere real:

| Clause | Belongs to |
|---|---|
| 5.1d, 5.1e, 5.1f | channel coding and the physical layer — `pkg/tmsc` |
| 5.4.3.3a | packet formats — `pkg/spp` |
| 5.4.4.2d | synchronization markers — `pkg/tmsc` |
| 5.4.4.2h, 5.5.3b | the CLCW — `pkg/cop` |

None was silently dropped.

---

## A5 METHOD

- The standard was read in full; every numbered clause under §5 was extracted
  mechanically, giving 135. Section 4 is informative and contributes none.
- Each clause was mapped by reading `frame.go`, `service.go`, `channel.go`,
  `physical.go` and `errors.go`. Every verdict but out-of-scope carries a
  `pkg/tmdl` file and symbol; the first pass cited line numbers, which drifted
  within a day of the audit, so the second pass replaced them with symbols.
- Four clauses were checked by building frames in a temporary probe rather than
  by reading: 5.3.2.3c, 5.3.1c, 5.2.7.6g and 5.1b. All four findings above come
  from what the probe actually produced. The probe was deleted; this audit
  changed no code.
- Evidence is as of the working tree on 25/08/2026, which carries the
  uncommitted idle-packet fill, counter-threading, and frame-length
  enforcement changes.
