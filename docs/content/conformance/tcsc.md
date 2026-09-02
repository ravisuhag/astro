---
title: TC Sync and Channel Coding
short: TCSC
description: "PICS proforma: what this package implements, clause by clause."
order: 140
---

## Conformance Statement for `pkg/tcsc` — CCSDS 231.0-B-4

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 25/08/2026 |
| PICS Serial Number | ASTRO-TCSC-PICS-002 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/tcsc |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS TC Synchronization and Channel Coding sublayer. Provides CLTU wrapping/unwrapping with BCH(63,56) forward error correction per codeblock (SEC and TED decoding modes), CCSDS pseudo-randomization, configurable start/tail sequences, and PLOP-1/PLOP-2 acquisition and idle sequence assembly. |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/tcsc (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 231.0-B-4 (TC Synchronization and Channel Coding, Blue Book, Issue 4, November 2019) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE — Non-supported optional capabilities (LDPC and convolutional coding) are identified in section A2.2. Clause references follow the Issue 4 numbering: section 3 (BCH coding), section 4 (LDPC coding), section 5 (CLTU), section 6 (randomizer), section 7 (physical layer operations procedures), section 8 (managed parameters).

NOTE — CCSDS 231.0-B-4 publishes no PICS proforma; its annex A is the service definition. This document is therefore a coverage matrix written against the clauses above, not a filled-in proforma, and the item numbers (TCSC-n) are local to this repository.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: BCH(63,56) Coding (section 3)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TCSC-1 | BCH Generator Polynomial | 3.3 | M | Yes | g(x) = x^7 + x^6 + x^2 + 1, represented as `bchPoly = 0xC5`. |
| TCSC-2 | BCH Codeblock Structure | 3.2 | M | Yes | 64 bits per codeblock: 56 information bits (7 bytes) + 7 parity bits + 1 filler bit. Constants: `InfoBytes = 7`, `CodeblockBytes = 8`. |
| TCSC-3 | BCH Systematic Encoding with Complemented Parity | 3.3 | M | Yes | `BCHEncode(info)` — 7-bit LFSR systematic encoding. The transmitted parity bits are the COMPLEMENT of the LFSR remainder (`parity = ^sr & 0x7F`), per the standard. Information bytes unchanged in first 7 bytes; parity in high 7 bits of the 8th byte. Pinned to the known vector: all-zeros information octets encode to last octet 0xFE. |
| TCSC-4 | BCH Filler Bit | 3.3.2 | M | Yes | Filler bit (bit 0 of byte 7) is always '0' per 3.3.2. Pinned by `TestBCHEncode_FillerBitAlwaysZero`. |
| TCSC-5 | BCH Syndrome Computation | 3.5 | M | Yes | `BCHDecodeWithMode()` complements the received parity bits (undoing the on-the-wire inversion) before the LFSR syndrome pass over all 63 code bits. Zero syndrome indicates no errors. |
| TCSC-6 | SEC Decoding Mode | 3.5 | M | Yes | `BCHDecode()` / `ModeSEC` corrects 1 bit error per codeblock; `findErrorPosition()` searches all 63 bit positions for a syndrome match. Returns corrected information bytes and correction count. NOTE: in SEC mode a 3-bit error pattern can miscorrect; guaranteed detection of up to 3 bit errors requires TED mode. |
| TCSC-7 | TED Decoding Mode | 3.5 | M | Yes | `BCHDecodeWithMode(cb, ModeTED)` — Triple Error Detection: no correction attempted; any non-zero syndrome returns `ErrUncorrectable`. Guaranteed detection of up to 3 bit errors per codeblock. Also selectable end-to-end via `UnwrapCLTUWithMode()`. |
| TCSC-8 | BCH Uncorrectable Detection | 3.5 | M | Yes | SEC mode returns `ErrUncorrectable` when the syndrome is non-zero and no single-bit error position matches (2+ bit errors). |
| TCSC-9 | BCH Error-Free Pass-Through | 3.5 | M | Yes | Zero syndrome: information bytes returned with 0 corrections and nil error. |

### Table A-2: Pseudo-Randomization (section 6)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TCSC-10 | PN Sequence Generation | 6.2 | M | Yes | `GeneratePNSequence(length)` — 8-bit LFSR, h(x) = x^8 + x^6 + x^4 + x^3 + x^2 + x + 1, preset to all ones. Pinned to the published first 40 digits (0xFF 0x39 0x9E 0x5A 0x68). This is the TC polynomial; the TM randomizer of CCSDS 131.0-B-5 clause 10.4.2 is a different sequence and is not used here. |
| TCSC-11 | Randomization (Send) | 6.3 | M | Yes | `Randomize(data)` XORs data with the PN sequence. Returns new slice; input not modified. |
| TCSC-12 | De-Randomization (Receive) | 6.3 | M | Yes | Same `Randomize()` function — XOR is self-inverse. Integrated into `UnwrapCLTU()` when randomize=true. |
| TCSC-13 | Randomization Coverage in the CLTU | 6.3 | M | Yes | `WrapCLTU()` pads to the codeblock boundary with 0x55 fill FIRST, then randomizes the padded buffer, so the fill octets go out randomized like the rest of the data. `UnwrapCLTU()` de-randomizes the full recovered buffer (fill included). Pinned by `TestWrapCLTU_RandomizesFillOctets`. |

### Table A-3: CLTU (section 5)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TCSC-14 | CLTU Start Sequence | 5.2.2 | M | Yes | `DefaultStartSequence()` returns the standard 2-byte start sequence 0xEB90. Custom sequences supported via `WrapCLTU()` parameter. |
| TCSC-15 | CLTU Tail Sequence | 5.2.4 | M | Yes | `DefaultTailSequence()` returns the standard 8-byte tail sequence 0xC5C5C5C5C5C5C579. The pattern is chosen so that it fails BCH decoding even after any single bit error, which is what terminates reception. |
| TCSC-16 | CLTU Assembly (Send) | 5.2 | M | Yes | `WrapCLTU(frameData, startSeq, tailSeq, randomize)`: 0x55 padding to the 7-byte boundary, optional randomization of the padded buffer, BCH encoding of each block, start sequence prepend, tail sequence append. |
| TCSC-17 | CLTU Data Padding | 3.4, 5.2 | M | Yes | Frame data padded to a multiple of `InfoBytes` (7) with the fill data of 3.4 (0x55 octets) before randomization and BCH encoding, as part of CLTU assembly. |
| TCSC-18 | CLTU Reception and Termination | 5.3 | M | Yes | `UnwrapCLTU()` / `UnwrapCLTUWithMode()`: validates the start sequence, decodes codeblocks, and terminates on the tail sequence or on the FIRST codeblock that fails to decode — an exact tail match is not required, so bit errors in the tail are tolerated. Trailing octets after the CLTU are ignored. |
| TCSC-19 | Start Sequence Validation | 5.3 | M | Yes | `UnwrapCLTU()` validates the start sequence; returns `ErrStartSequenceMismatch` otherwise. |
| TCSC-20 | Minimum Length Validation | 5.3 | M | Yes | `UnwrapCLTU()` requires at least the start sequence plus one codeblock; returns `ErrDataTooShort` otherwise. |

### Table A-4: Acquisition, Idle, and PLOP (section 7)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TCSC-21 | Acquisition Sequence | 7.2.2 | M | Yes | `AcquisitionSequence(octets)` — alternating '01' pattern (0x55 octets); defaults to the recommended minimum of 16 octets (128 bits). |
| TCSC-22 | Idle Sequence | 7.2.4 | M | Yes | `IdleSequence(octets)` — alternating '01' pattern between CLTUs. The length is unconstrained by 7.2.4 and is a managed parameter; `DefaultIdleOctets = 8` is this library's practical default, not a requirement. |
| TCSC-23 | PLOP-1 | 7.4 | M | Yes | `UplinkSequence(PLOP1, ...)` — each CLTU preceded by its own acquisition sequence (session ends after each CLTU). |
| TCSC-24 | PLOP-2 | 7.5 | M | Yes | `UplinkSequence(PLOP2, ...)` — one acquisition sequence starts the session; idle sequence keeps the channel modulated between CLTUs. CCSDS-recommended procedure. |
| TCSC-25 | Carrier Modulation Modes (CMM state machine) | 7 | M | No | The CMM-1..CMM-4 state machine itself is not modeled; the library assembles the symbol stream (acquisition/idle/CLTU ordering) and leaves carrier control to the ground station equipment. |

### Table A-5: Optional Coding Schemes

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TCSC-26 | LDPC Coding | 4 | O | No | Not implemented. |
| TCSC-27 | Concatenated BCH + Convolutional | — | O | No | Convolutional inner code not implemented. BCH outer code is available. |

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Not Supported |
|----------|-------------|-----------|---------------|
| Mandatory (M) | 25 | 24 | 1 |
| Optional (O) | 2 | 0 | 2 |
| **Total** | **27** | **24** | **3** |

### Non-Conformances (Mandatory Items Not Supported)

| Item | Description | Reason |
|------|-------------|--------|
| TCSC-25 | CMM state machine | The library produces the physical-channel symbol stream (`UplinkSequence`) but does not model the carrier modulation mode transitions; carrier on/off control belongs to the transmitting equipment, not to a codec library. |

### Non-Supported Optional Items

| Item | Description | Reason |
|------|-------------|--------|
| TCSC-26 | LDPC Coding | Not implemented. Specialized application for high-data-rate TC links. |
| TCSC-27 | Concatenated BCH + Convolutional | Convolutional inner code not implemented. |

### Key Implementations

| Area | Items | Implementation |
|------|-------|----------------|
| BCH(63,56) Coding | TCSC-1-9 | `BCHEncode()` (complemented parity, filler '0', all-zeros -> 0xFE vector), `BCHDecode()` / `BCHDecodeWithMode()` with SEC and TED modes. |
| Pseudo-Randomization | TCSC-10-13 | `GeneratePNSequence()`, `Randomize()`; fill-then-randomize order in `WrapCLTU()`. |
| CLTU | TCSC-14-20 | `WrapCLTU()`, `UnwrapCLTU()` / `UnwrapCLTUWithMode()` terminating on the first failed codeblock. |
| Acquisition/Idle/PLOP | TCSC-21-24 | `AcquisitionSequence()`, `IdleSequence()`, `UplinkSequence()` for PLOP-1 and PLOP-2. |
