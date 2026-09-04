---
title: Glossary
short: Glossary
description: The acronyms CCSDS runs on, expanded, with the package that implements each one.
order: 1
---

CCSDS is dense with acronyms and most of these docs use them without stopping to expand. This page stops.

Ordered by where it sits in [the stack](/docs/start/concepts) rather than alphabetically, because the shape is the useful part. Every expansion here is the one the standards and the package documentation use.

## Packets

| | | |
|---|---|---|
| **APID** | Application Process Identifier | Names the on-board application a packet came from or is going to. 11 bits, so 0 to 2047; `0x7FF` is reserved for idle packets. [`pkg/spp`](/protocols/transport/spp) |
| **SPP** | Space Packet Protocol | CCSDS 133.0-B-2, the packet every mission uses. [`pkg/spp`](/protocols/transport/spp) |
| **EPP** | Encapsulation Packet Protocol | CCSDS 133.1-B-3, a thin wrapper for data that is not a Space Packet, such as IP. [`pkg/epp`](/protocols/transport/epp) |
| **PDU** | Protocol Data Unit | One protocol's unit of transmission. Used throughout CFDP, LTP and Proximity-1. |
| **SDU** | Service Data Unit | What a service hands down to the layer below, or up to the layer above. |
| **PUS** | Packet Utilization Standard | ECSS-E-ST-70-41C, what goes *inside* a packet: services, subtypes, reports. [`pkg/pus`](/protocols/mission/pus) |

## Data link

| | | |
|---|---|---|
| **TM** | Telemetry | Fixed-length downlink frames, CCSDS 132.0-B-3. [`pkg/tmdl`](/protocols/data-link/tmdl) |
| **TC** | Telecommand | Variable-length uplink frames, CCSDS 232.0-B-4. [`pkg/tcdl`](/protocols/data-link/tcdl) |
| **AOS** | Advanced Orbiting Systems | High-rate downlink frames that also carry bitstreams and opaque blocks, CCSDS 732.0-B-4. [`pkg/aos`](/protocols/data-link/aos) |
| **USLP** | Unified Space Data Link Protocol | One frame format for both directions, CCSDS 732.1-B-3. [`pkg/usdl`](/protocols/data-link/usdl) |
| **SCID** | Spacecraft Identifier | Which spacecraft a frame belongs to. 10 bits in TM and TC, 8 in AOS. |
| **VCID** | Virtual Channel Identifier | Which stream inside the physical channel. 3 bits in TM, 6 in AOS and TC. |
| **GVCID** | Global Virtual Channel Identifier | Frame version, spacecraft and virtual channel together, so a channel is unique across missions. |
| **MCID** | Master Channel Identifier | Frame version and spacecraft, without the virtual channel. |
| **FHP** | First Header Pointer | Where the first packet *starts* inside a frame's data field. What lets a receiver find packet boundaries again after losing a frame. |
| **OCF** | Operational Control Field | Four octets in every frame of a channel that carries one. Where the CLCW rides home. [Full duplex](/docs/guides/full-duplex) |
| **FECF** | Frame Error Control Field | The frame's CRC, two octets at the end. |
| **FSH** | Transfer Frame Secondary Header | Optional per-frame header, fixed length for the channel. |
| **FHEC** | Frame Header Error Control | Reed-Solomon over an AOS frame header, so a corrupted VCID is caught rather than mis-routed. AOS only. |
| **MAP** | Multiplexer Access Point | A subdivision of a TC or USLP virtual channel. |
| **VCA** | Virtual Channel Access | The service that carries one opaque fixed-length block per frame, with no protocol header. |
| **M_PDU** | Multiplexing PDU | The AOS data field shape that carries variable-length packets. |
| **B_PDU** | Bitstream PDU | The AOS data field shape that carries a raw octet stream. |
| **OID** | Only Idle Data | The reserved virtual channel, 63, that carries nothing but fill. |

## Reliable commanding

| | | |
|---|---|---|
| **COP-1** | Communications Operation Procedure-1 | CCSDS 232.1-B-2, what makes telecommand reliable. Two state machines. [`pkg/cop`](/protocols/data-link/cop) |
| **FOP-1** | Frame Operation Procedure | The ground half of COP-1: assigns sequence numbers, holds a sliding window, retransmits. |
| **FARM-1** | Frame Acceptance and Reporting Mechanism | The spacecraft half: accepts, discards or locks out, and reports what it did. |
| **CLCW** | Communications Link Control Word | Four octets FARM-1 generates saying what it has accepted. Travels back in the OCF of a telemetry frame. |
| **AD, BD, BC** | Type-A Data, Type-B Data, Type-B Control | The three TC frame kinds. AD is sequence-controlled and reliable; BD bypasses the sequence check; BC carries a control directive such as Unlock. |

## Coding and synchronization

| | | |
|---|---|---|
| **TMSC** | TM Synchronization and Channel Coding | CCSDS 131.0-B-5, the downlink coding layer. [`pkg/tmsc`](/protocols/coding/tmsc) |
| **TCSC** | TC Synchronization and Channel Coding | CCSDS 231.0-B-4, the uplink coding layer. [`pkg/tcsc`](/protocols/coding/tcsc) |
| **ASM** | Attached Sync Marker | The known pattern, `0x1ACFFC1D` for TM, that says a frame starts here. |
| **CADU** | Channel Access Data Unit | A sync marker followed by a coded, randomized frame. What actually goes on a downlink. |
| **CLTU** | Command Link Transmission Unit | The uplink equivalent: start sequence, BCH codeblocks, tail sequence. |
| **PLTU** | Proximity Link Transmission Unit | The Proximity-1 equivalent. [`pkg/pxsc`](/protocols/coding/pxsc) |
| **RS** | Reed-Solomon | The block code CCSDS 131.0 uses on a downlink. RS(255,223) corrects 16 symbols per codeword, RS(255,239) corrects 8. |
| **BCH** | Bose-Chaudhuri-Hocquenghem | The code inside a CLTU codeblock. BCH(63,56) corrects one bit. |
| **LDPC** | Low-Density Parity-Check | A modern code CCSDS 131.0 also defines. **Not implemented**; `pkg/tmsc` stops at Reed-Solomon. |
| **PLOP** | Physical Layer Operations Procedures | How a CLTU sequence is delimited on the physical layer. |
| **OCSC** | Optical Coding and Synchronization | CCSDS 142.0-B-1, the coding layer for a laser link. Works in bits, not octets. [`pkg/ocsc`](/protocols/coding/ocsc) |

## Security

| | | |
|---|---|---|
| **SDLS** | Space Data Link Security | CCSDS 355.0-B-2, encrypting or authenticating a frame's data field. [`pkg/sdls`](/protocols/data-link/sdls) |
| **SA** | Security Association | The agreed parameters both ends configure before the link opens: algorithm, key, field widths, channels. |
| **SPI** | Security Parameter Index | The only part of the SA that travels on the wire. A pointer to the agreement. |
| **MAC** | Message Authentication Code | The tag in the security trailer that proves the frame was not forged or altered. |
| **IV** | Initialization Vector | The per-frame nonce for AES-GCM. Never reused, which is why an SA is not concurrency safe. |

## Files and networking

| | | |
|---|---|---|
| **CFDP** | CCSDS File Delivery Protocol | CCSDS 727.0-B-5, moving whole files. [`pkg/cfdp`](/protocols/transport/cfdp) |
| **DTN** | Delay-Tolerant Networking | Store and forward, for links that are never all up at once. |
| **BP** | Bundle Protocol | The DTN network layer, RFC 9171. Version 7, which encodes with CBOR; version 6 (RFC 5050) is a different wire format and is not implemented. [`pkg/bp`](/protocols/transport/bp) |
| **LTP** | Licklider Transmission Protocol | The DTN convergence layer for one hop, RFC 5326 as profiled by CCSDS 734.1-B-1. [`pkg/ltp`](/protocols/transport/ltp) |
| **SDNV** | Self-Delimiting Numeric Value | The variable-length integer BP and LTP encode nearly every field with. |
| **DTN time** | Delay-Tolerant Networking time | Milliseconds since 2000-01-01, ignoring leap seconds. Bundle Protocol version 7 uses it; version 6 counted seconds. |

## Ground to ground

| | | |
|---|---|---|
| **SLE** | Space Link Extension | Moving frames between ground systems over TCP. The only protocol here that never touches a spacecraft. [`pkg/sle`](/protocols/ground/sle) |
| **ISP1** | Internet SLE Protocol One | The transport SLE runs over, CCSDS 913.1-B-2. |
| **TML** | Transport Mapping Layer | ISP1's message framing: a header, then a context, heartbeat or PDU body. |
| **RAF** | Return All Frames | The SLE service that delivers every frame on a physical channel. |
| **RCF** | Return Channel Frames | Delivers frames from one virtual channel. |
| **ROCF** | Return Operational Control Fields | Delivers just the OCFs, so the CLCWs. |
| **FCLTU** | Forward CLTU | The uplink direction: the control centre sends CLTUs to the station. |
| **BER** | Basic Encoding Rules | How SLE's ASN.1 messages are encoded. |

## Time

| | | |
|---|---|---|
| **CUC** | CCSDS Unsegmented Time Code | A binary counter: coarse seconds and fine fractions. Smallest, and what a link should carry. [`pkg/tcf`](/protocols/mission/tcf) |
| **CDS** | CCSDS Day Segmented Time Code | Day count, milliseconds of day, optional sub-milliseconds. |
| **CCS** | CCSDS Calendar Segmented Time Code | BCD calendar fields. Widest, and readable off a hex dump. |
| **TAI** | International Atomic Time | Continuous, no leap seconds. A CUC Level 1 code counts TAI seconds since 1958. |
| **UTC** | Coordinated Universal Time | What people use. 37 seconds behind TAI since 2017. [Time correlation](/docs/guides/time-correlation) |

## Mission data

| | | |
|---|---|---|
| **XTCE** | XML Telemetric and Command Exchange | The mission database format: what each parameter is, how it is encoded, which packet carries it. [`pkg/xtce`](/protocols/mission/xtce) |
| **LDC** | Lossless Data Compression | CCSDS 121.0-B-3, the Rice adaptive entropy coder, for science data. [`pkg/ldc`](/protocols/compression/ldc) |
| **RHC** | Robust Compression of Housekeeping Data | CCSDS 124.0-B-1, POCKET+, for telemetry that barely changes. [`pkg/rhc`](/protocols/compression/rhc) |

## Assurance

| | | |
|---|---|---|
| **PICS** | Protocol Implementation Conformance Statement | The proforma a standard ships for an implementer to fill in, clause by clause. What the [conformance pages](/conformance) are. |
| **IUT** | Implementation Under Test | What a PICS is filled in about. Here, the package. |
| **M, O, C** | Mandatory, Optional, Conditional | The status a PICS gives each item. There are 626 mandatory and 202 optional rows across these pages. |

## Reference

- [The stack](/docs/start/concepts), how these layers fit together
- [Protocol index](/protocols), one page per standard
- [How this is verified](/docs/reference/verification)
