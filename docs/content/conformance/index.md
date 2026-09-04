---
title: Conformance
description: What each package implements, clause by clause, and what it does not.
order: 0
---

Every protocol package ships a conformance statement. Where the standard
publishes a PICS proforma, it is filled in. Where it does not, there is a
coverage matrix written against the normative text.

The **Vectors** column links each statement to the octets that back it, in
the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors).
Those are JSON data files, so any implementation can check itself against
the same octets. A dash means the package has no
extractable wire goldens — its pinned values are structural or
behavioural, not whole-field encodings.

**Gaps are recorded, not hidden.** An honest "not implemented" is worth more
than a silent one, and it is the first thing a mission assurance reader looks
for. Rows checked against a standard's prose rather than a published test
vector are marked as derived.

Start with [how this is verified](/docs/reference/verification). It says how much
of this rests on a published vector, how much on a reading of the clause, and
what has never been tested against another implementation.

| Package | Go | Vectors | |
|---|---|--:|---|
| [Space Packet Protocol](/conformance/spp) | `pkg/spp` | [28](https://github.com/ravisuhag/astro/tree/main/vectors/spp) | [Protocol](/protocols/transport/spp) |
| [Encapsulation Packet Protocol](/conformance/epp) | `pkg/epp` | [16](https://github.com/ravisuhag/astro/tree/main/vectors/epp) | [Protocol](/protocols/transport/epp) |
| [CCSDS File Delivery Protocol](/conformance/cfdp) | `pkg/cfdp` | [14](https://github.com/ravisuhag/astro/tree/main/vectors/cfdp) | [Protocol](/protocols/transport/cfdp) |
| [Licklider Transmission Protocol](/conformance/ltp) | `pkg/ltp` | [9](https://github.com/ravisuhag/astro/tree/main/vectors/ltp) | [Protocol](/protocols/transport/ltp) |
| [Bundle Protocol](/conformance/bp) | `pkg/bp` | [5](https://github.com/ravisuhag/astro/tree/main/vectors/bp) | [Protocol](/protocols/transport/bp) |
| [Bundle Protocol Security](/conformance/bpsec) | `pkg/bpsec` | [13](https://github.com/ravisuhag/astro/tree/main/vectors/bpsec) | [Protocol](/protocols/transport/bpsec) |
| [TM Space Data Link Protocol](/conformance/tmdl) | `pkg/tmdl` | [20](https://github.com/ravisuhag/astro/tree/main/vectors/tmdl) | [Protocol](/protocols/data-link/tmdl) |
| [TM Space Data Link (ECSS)](/conformance/tmdl-ecss) | `pkg/tmdl` | [20](https://github.com/ravisuhag/astro/tree/main/vectors/tmdl) | [Protocol](/protocols/data-link/tmdl) |
| [TC Space Data Link Protocol](/conformance/tcdl) | `pkg/tcdl` | — | [Protocol](/protocols/data-link/tcdl) |
| [AOS Space Data Link Protocol](/conformance/aos) | `pkg/aos` | [9](https://github.com/ravisuhag/astro/tree/main/vectors/aos) | [Protocol](/protocols/data-link/aos) |
| [Unified Space Data Link Protocol](/conformance/usdl) | `pkg/usdl` | [9](https://github.com/ravisuhag/astro/tree/main/vectors/usdl) | [Protocol](/protocols/data-link/usdl) |
| [Proximity-1 Data Link Layer](/conformance/pxdl) | `pkg/pxdl` | — | [Protocol](/protocols/data-link/pxdl) |
| [COP-1](/conformance/cop) | `pkg/cop` | [15](https://github.com/ravisuhag/astro/tree/main/vectors/cop) | [Protocol](/protocols/data-link/cop) |
| [Space Data Link Security](/conformance/sdls) | `pkg/sdls` | [7](https://github.com/ravisuhag/astro/tree/main/vectors/sdls) | [Protocol](/protocols/data-link/sdls) |
| [TM Sync and Channel Coding](/conformance/tmsc) | `pkg/tmsc` | [7](https://github.com/ravisuhag/astro/tree/main/vectors/tmsc) | [Protocol](/protocols/coding/tmsc) |
| [TC Sync and Channel Coding](/conformance/tcsc) | `pkg/tcsc` | [7](https://github.com/ravisuhag/astro/tree/main/vectors/tcsc) | [Protocol](/protocols/coding/tcsc) |
| [Proximity-1 Coding and Sync](/conformance/pxsc) | `pkg/pxsc` | [4](https://github.com/ravisuhag/astro/tree/main/vectors/pxsc) | [Protocol](/protocols/coding/pxsc) |
| [Optical Coding and Sync](/conformance/ocsc) | `pkg/ocsc` | [1](https://github.com/ravisuhag/astro/tree/main/vectors/ocsc) | [Protocol](/protocols/coding/ocsc) |
| [Space Link Extension](/conformance/sle) | `pkg/sle` | [23](https://github.com/ravisuhag/astro/tree/main/vectors/sle) | [Protocol](/protocols/ground/sle) |
| [Lossless Data Compression](/conformance/ldc) | `pkg/ldc` | [107 files](https://github.com/ravisuhag/astro/tree/main/vectors/ldc) | [Protocol](/protocols/compression/ldc) |
| [Housekeeping Compression](/conformance/rhc) | `pkg/rhc` | — | [Protocol](/protocols/compression/rhc) |
| [Time Code Formats](/conformance/tcf) | `pkg/tcf` | [14](https://github.com/ravisuhag/astro/tree/main/vectors/tcf) | [Protocol](/protocols/mission/tcf) |
| [Packet Utilization Standard](/conformance/pus) | `pkg/pus` | [10](https://github.com/ravisuhag/astro/tree/main/vectors/pus) | [Protocol](/protocols/mission/pus) |
| [XTCE](/conformance/xtce) | `pkg/xtce` | [8 files](https://github.com/ravisuhag/astro/tree/main/vectors/xtce) | [Protocol](/protocols/mission/xtce) |
| [Orbit Data Messages](/conformance/odm) | `pkg/odm` | [2](https://github.com/ravisuhag/astro/tree/main/vectors/odm) | [Protocol](/protocols/mission/odm) |
