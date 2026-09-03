---
title: Proximity-1 Coding and Sync
short: PXSC
description: "PICS proforma: what this package implements, clause by clause."
order: 150
---

## Conformance Statement for `pkg/pxsc`, CCSDS 211.2-B-3

---

## A1.1 GENERAL INFORMATION

### A1.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 23/08/2026 |
| PICS Serial Number | ASTRO-PXSC-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A1.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/pxsc |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | `Synchronizer` frame-length bounds default to the Version-3 range |
| Other Information | Go library implementing the Proximity Link Transmission Unit, the annex C CRC-32, idle data generation, a stream synchronizer, and the convolutional encoder with a Viterbi decoder. LDPC is out of scope. |

### A1.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub, github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/pxsc (Go package) |
| System Name(s) | Astro |

### A1.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 211.2-B-3 (Proximity-1 Space Link Protocol, Coding and Synchronization Sublayer, Blue Book, Issue 3, October 2019) |
| Have any exceptions been required? | Yes [X] No [ ], see A1.5 |

---

## A1.2 PROXIMITY LINK TRANSMISSION UNIT

| Feature | Reference | Status | Support |
|---|---|---|---|
| PLTU structure: ASM, frame, CRC-32 | clause 3.2.2 | M | Y |
| ASM occupies the first 24 bits | clause 3.2.3.1 | M | Y |
| ASM pattern FAF320 | clause 3.2.3.2 | M | Y |
| Transfer frame immediately follows the ASM | clause 3.2.4.2 | M | Y |
| Version-3 Transfer Frame | clause 3.2.4.1 | M | Y: via `pkg/pxdl` |
| Version-4 (USLP) Transfer Frame | clause 3.2.4.1 | O | Y: carried; `pkg/usdl` decodes it |
| 32-bit CRC as the final field | clause 3.2.2 c) | M | Y |

---

## A1.3 CRC-32 CODING

| Feature | Reference | Status | Support |
|---|---|---|---|
| Generator G(X) = X^32+X^23+X^21+X^11+X^2+1 | annex C, C1.3 | M | Y: 0x00A00805, implemented locally, not from `pkg/crc` |
| Shift register preset to all zeros | annex C, C1 encoder note | M | Y |
| ASM excluded from the CRC computation | annex C, C1.2 note 2 | M | Y |
| Systematic (n, n-32) block code | annex C, C1.2 | M | Y |
| Syndrome of a valid codeword is zero | annex C, C2 | M | Y |
| Error detected if and only if the syndrome is non-zero | annex C, C2.1 | M | Y |

---

## A1.4 IDLE DATA AND SYNCHRONIZATION

| Feature | Reference | Status | Support |
|---|---|---|---|
| Idle data PN sequence 352EF853 | clause 3.3.2.2 | M | Y |
| Sequence repeats from the first bit | clause 3.3.2.4 | M | Y |
| Acquisition sequence | clause 3.3.3 | M | Y: content; duration is a MIB parameter |
| Idle sequence when no PLTU is available | clause 3.3.4 | M | Y |
| Tail sequence before terminating transmission | clause 3.3.5 | M | Y |
| PLTU delimiting in a received bitstream | clause 3.6 | M | Y: `Synchronizer`; the frame's Length field is tried first, brute-force length scan as fallback; octet-aligned input only |
| PLTU validation before delivery | clause 3.6 | M | Y: a failing CRC is skipped, not delivered |

---

## A1.5 CHANNEL CODING

| Feature | Reference | Status | Support |
|---|---|---|---|
| No coding | clause 3.4.2.2 a) | O | Y |
| Convolutional code, rate 1/2, constraint length 7 | clause 3.4.3.1 | O | Y: encoder and Viterbi decoder, pinned to the CCSDS 171/133 convention by independent known-answer vectors |
| G2 output path inverted | clause 3.4.3.1 note 1 | M | Y |
| All transmitted data encoded, PLTUs and idle alike | clause 3.4.3.2 | M | Y: encoder state carries across calls |
| Soft bit decisions, three-bit quantization | clause 3.4.3.3 | O | Y: `DecodeSoft` takes them; the demodulator must supply them |
| LDPC code | clause 3.4.4 | O | N: see A1.6 |
| Codeword Sync Marker | clause 3.4.4 | O | N: LDPC only |
| Pseudo-randomizer | clause 3.4.5 | O | N: LDPC only |

---

## A1.6 EXCEPTIONS AND UNSUPPORTED FEATURES

| Feature | Reference | Support | Rationale |
|---|---|---|---|
| LDPC code, CSM, and pseudo-randomizer | clause 3.4.4, clause 3.4.5 | N | A substantial addition; the randomizer applies only when LDPC is used. A follow-up. |
| Reed-Solomon codes | clause 3.4.1 note | N | Not specified in the CCSDS Proximity-1 standards, and clause 3.4.1 states their use is not intended for cross support. |
| Concatenated convolutional and Reed-Solomon | clause 3.4.2.2 note 2 | N | Explicitly not specified by the standard. |
| Physical layer, modulation, rate control | CCSDS 211.1-B | N | A separate specification. |
| CLI subcommands | - | N | A follow-up once the API settles. |

---

## A1.7 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| ASM | 3 octets, FAF320 | clause 3.2.3 |
| CRC | 4 octets | clause 3.2.2 c) |
| PLTU overhead | 7 octets | The two above |
| Transfer frame length | `MaxFrameLength`, default 2048 | The Version-3 maximum; clause 3.2.2 note 1 leaves the real limit to the mission's `Maximum_Frame_Length` |
| Convolutional code rate | 1/2 | clause 3.4.3.1 |
| Convolutional constraint length | 7 | clause 3.4.3.1 |

---

## Wire test vectors

The octets backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/pxsc) — 4 vectors. Each vector names the clause it comes from and carries the derivation that produced it.

| File | |
|---|---|
| [`pxsc/convolutional.json`](https://github.com/ravisuhag/astro/blob/main/vectors/pxsc/convolutional.json) | 4 vectors |

These are data files, so any implementation can check itself against the same octets. See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
