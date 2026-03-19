# PICS PROFORMA FOR TC SYNCHRONIZATION AND CHANNEL CODING

## Conformance Statement for `pkg/tcsc` — CCSDS 231.0-B-4

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 19/03/2026 |
| PICS Serial Number | ASTRO-TCSC-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/tcsc |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS TC Synchronization and Channel Coding sublayer. Provides CLTU wrapping/unwrapping with BCH(63,56) forward error correction per codeblock, CCSDS pseudo-randomization, and configurable start/tail sequences. |

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

NOTE — Non-supported optional capabilities (convolutional and LDPC coding) are identified in section A2.2.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: CLTU Construction

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TCSC-1 | CLTU Start Sequence | 5.2.2 | M | Yes | `DefaultStartSequence()` returns the standard 2-byte start sequence 0xEB90. Custom sequences supported via `WrapCLTU()` parameter. Fresh copy returned each call. |
| TCSC-2 | CLTU Tail Sequence | 5.2.3 | M | Yes | `DefaultTailSequence()` returns the standard 8-byte tail sequence 0xC5C5C5C5C5C5C579. Custom sequences supported. Fresh copy returned each call. |
| TCSC-3 | CLTU Assembly (Send) | 5.2.4 | M | Yes | `WrapCLTU(frameData, startSeq, tailSeq, randomize)` assembles a complete CLTU: optional randomization, padding to 7-byte boundary with 0x55 fill, BCH encoding of each block, start sequence prepend, tail sequence append. |
| TCSC-4 | CLTU Disassembly (Receive) | 5.2.5 | M | Yes | `UnwrapCLTU(cltu, startSeq, tailSeq, randomize)` validates and strips start/tail sequences, BCH-decodes each codeblock with error correction, concatenates information bytes, optionally de-randomizes. Returns total corrections count. |
| TCSC-5 | CLTU Data Padding | 5.2.6 | M | Yes | Frame data padded to a multiple of `InfoBytes` (7) with fill bytes (0x55) before BCH encoding. |
| TCSC-6 | CLTU Minimum Length Validation | 5.2.7 | M | Yes | `UnwrapCLTU()` validates minimum length: start sequence + at least one codeblock + tail sequence. Returns `ErrDataTooShort` if too short. |
| TCSC-7 | CLTU Body Length Validation | 5.2.8 | M | Yes | `UnwrapCLTU()` validates that the body (between start and tail) is a multiple of `CodeblockBytes` (8). Returns `ErrInvalidCLTULength` if not. |
| TCSC-8 | Start Sequence Validation | 5.2.9 | M | Yes | `UnwrapCLTU()` validates the start sequence matches. Returns `ErrStartSequenceMismatch` if not. |
| TCSC-9 | Tail Sequence Validation | 5.2.10 | M | Yes | `UnwrapCLTU()` validates the tail sequence matches. Returns `ErrTailSequenceMismatch` if not. |

### Table A-2: BCH(63,56) Coding

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TCSC-10 | BCH Generator Polynomial | 6.2 | M | Yes | g(x) = x^7 + x^6 + x^2 + 1, represented as `bchPoly = 0xC5`. |
| TCSC-11 | BCH Codeblock Structure | 6.3 | M | Yes | 64 bits per codeblock: 56 information bits (7 bytes) + 7 parity bits + 1 filler bit. Constants: `InfoBytes = 7`, `CodeblockBytes = 8`. |
| TCSC-12 | BCH Systematic Encoding | 6.4 | M | Yes | `BCHEncode(info)` — 7-bit LFSR-based systematic encoding. Information bytes preserved unchanged in first 7 bytes. Parity in high 7 bits of 8th byte. Returns `[8]byte`. |
| TCSC-13 | BCH Filler Bit | 6.5 | M | Yes | Filler bit (bit 0 of byte 7) is the complement of the last parity bit. Ensures tail sequence is distinguishable from valid data. |
| TCSC-14 | BCH Syndrome Computation | 6.6 | M | Yes | `BCHDecode()` computes syndrome by feeding all 63 code bits (56 info + 7 parity) through the LFSR. Zero syndrome indicates no errors. |
| TCSC-15 | BCH Error Detection | 6.7 | M | Yes | Detects up to 3 bit errors per codeblock via syndrome analysis. |
| TCSC-16 | BCH Single-Bit Correction | 6.8 | M | Yes | `BCHDecode()` corrects 1 bit error per codeblock. `findErrorPosition()` searches all 63 bit positions for syndrome match. Returns corrected information bytes and correction count. |
| TCSC-17 | BCH Uncorrectable Detection | 6.9 | M | Yes | Returns `ErrUncorrectable` when syndrome is non-zero but no single-bit error position matches (2+ bit errors). |
| TCSC-18 | BCH Error-Free Pass-Through | 6.10 | M | Yes | Zero syndrome: information bytes returned immediately with 0 corrections and nil error. |

### Table A-3: Pseudo-Randomization

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TCSC-19 | PN Sequence Generation | 7.2 | M | Yes | `GeneratePNSequence(length)` generates the CCSDS pseudo-random sequence using 8-bit LFSR with polynomial h(x) = x^8 + x^7 + x^5 + x^3 + 1, initialized to 0xFF. |
| TCSC-20 | Randomization (Send) | 7.3 | M | Yes | `Randomize(data)` XORs data with PN sequence. Returns new slice; input not modified. Applied to frame data before BCH encoding. |
| TCSC-21 | De-Randomization (Receive) | 7.4 | M | Yes | Same `Randomize()` function — XOR is self-inverse. Integrated into `UnwrapCLTU()` when randomize=true. Applied after BCH decoding. |
| TCSC-22 | Randomization in CLTU Pipeline | 7.5 | M | Yes | `WrapCLTU()` applies randomization before padding and BCH encoding when randomize=true. `UnwrapCLTU()` applies de-randomization after BCH decoding and concatenation. |

### Table A-4: Optional Coding Schemes

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TCSC-23 | LDPC Coding | 8 | O | No | Not implemented. |
| TCSC-24 | Concatenated BCH + Convolutional | 9 | O | No | Not implemented. |

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Not Supported |
|----------|-------------|-----------|---------------|
| Mandatory (M) | 22 | 22 | 0 |
| Optional (O) | 2 | 0 | 2 |
| **Total** | **24** | **22** | **2** |

### Non-Conformances (Mandatory Items Not Supported)

None. All 22 mandatory items are fully supported.

### Non-Supported Optional Items

| Item | Description | Reason |
|------|-------------|--------|
| TCSC-23 | LDPC Coding | Not implemented. Specialized application for high-data-rate TC links. |
| TCSC-24 | Concatenated BCH + Convolutional | Convolutional inner code not implemented. BCH outer code is available. |

### Fully Supported Mandatory Items

All 22 mandatory items (TCSC-1 through TCSC-22) are supported. Key implementations:

| Area | Items | Implementation |
|------|-------|----------------|
| CLTU Construction | TCSC-1–9 | `DefaultStartSequence()`, `DefaultTailSequence()`, `WrapCLTU()`, `UnwrapCLTU()` with custom sequence support, padding, and validation. |
| BCH(63,56) Coding | TCSC-10–18 | `BCHEncode()`, `BCHDecode()` with systematic encoding, LFSR-based syndrome computation, single-bit correction, filler bit, uncorrectable detection. |
| Pseudo-Randomization | TCSC-19–22 | `GeneratePNSequence()`, `Randomize()` (self-inverse), integrated into CLTU pipeline. |
