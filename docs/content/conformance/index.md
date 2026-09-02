---
title: Conformance
description: What each package implements, clause by clause, and what it does not.
order: 0
---

Every protocol package ships a conformance statement. Where the standard
publishes a PICS proforma, it is filled in. Where it does not, there is a
coverage matrix written against the normative text.

**Gaps are recorded, not hidden.** An honest "not implemented" is worth more
than a silent one, and it is the first thing a mission assurance reader looks
for. Rows checked against a standard's prose rather than a published test
vector are marked as derived.

Start with [how this is verified](/docs/reference/verification). It says how much
of this rests on a published vector, how much on a reading of the clause, and
what has never been tested against another implementation.

| Package | Go | |
|---|---|---|
| [Space Packet Protocol](/conformance/spp) | `pkg/spp` | [Protocol](/protocols/transport/spp) |
| [Encapsulation Packet Protocol](/conformance/epp) | `pkg/epp` | [Protocol](/protocols/transport/epp) |
| [CCSDS File Delivery Protocol](/conformance/cfdp) | `pkg/cfdp` | [Protocol](/protocols/transport/cfdp) |
| [Licklider Transmission Protocol](/conformance/ltp) | `pkg/ltp` | [Protocol](/protocols/transport/ltp) |
| [Bundle Protocol](/conformance/bp) | `pkg/bp` | [Protocol](/protocols/transport/bp) |
| [TM Space Data Link Protocol](/conformance/tmdl) | `pkg/tmdl` | [Protocol](/protocols/data-link/tmdl) |
| [TM Space Data Link (ECSS)](/conformance/tmdl-ecss) | `pkg/tmdl` | [Protocol](/protocols/data-link/tmdl) |
| [TC Space Data Link Protocol](/conformance/tcdl) | `pkg/tcdl` | [Protocol](/protocols/data-link/tcdl) |
| [AOS Space Data Link Protocol](/conformance/aos) | `pkg/aos` | [Protocol](/protocols/data-link/aos) |
| [Unified Space Data Link Protocol](/conformance/usdl) | `pkg/usdl` | [Protocol](/protocols/data-link/usdl) |
| [Proximity-1 Data Link Layer](/conformance/pxdl) | `pkg/pxdl` | [Protocol](/protocols/data-link/pxdl) |
| [COP-1](/conformance/cop) | `pkg/cop` | [Protocol](/protocols/data-link/cop) |
| [Space Data Link Security](/conformance/sdls) | `pkg/sdls` | [Protocol](/protocols/data-link/sdls) |
| [TM Sync and Channel Coding](/conformance/tmsc) | `pkg/tmsc` | [Protocol](/protocols/coding/tmsc) |
| [TC Sync and Channel Coding](/conformance/tcsc) | `pkg/tcsc` | [Protocol](/protocols/coding/tcsc) |
| [Proximity-1 Coding and Sync](/conformance/pxsc) | `pkg/pxsc` | [Protocol](/protocols/coding/pxsc) |
| [Optical Coding and Sync](/conformance/ocsc) | `pkg/ocsc` | [Protocol](/protocols/coding/ocsc) |
| [Space Link Extension](/conformance/sle) | `pkg/sle` | [Protocol](/protocols/ground/sle) |
| [Lossless Data Compression](/conformance/ldc) | `pkg/ldc` | [Protocol](/protocols/compression/ldc) |
| [Housekeeping Compression](/conformance/rhc) | `pkg/rhc` | [Protocol](/protocols/compression/rhc) |
| [Time Code Formats](/conformance/tcf) | `pkg/tcf` | [Protocol](/protocols/mission/tcf) |
| [Packet Utilization Standard](/conformance/pus) | `pkg/pus` | [Protocol](/protocols/mission/pus) |
| [XTCE](/conformance/xtce) | `pkg/xtce` | [Protocol](/protocols/mission/xtce) |
