# PICS PROFORMA FOR OPTICAL COMMUNICATIONS CODING AND SYNCHRONIZATION

## Conformance Statement for `pkg/ocsc` — CCSDS 142.0-B-1

---

## A1.1 GENERAL INFORMATION

### A1.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 23/08/2026 |
| PICS Serial Number | ASTRO-OCSC-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A1.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/ocsc |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | Code rate is a caller parameter; `Recover` takes the mission's frame length |
| Other Information | Go library implementing the deterministic data-conditioning chain of the High Photon Efficiency scheme: sync marker attachment, slicing with zero fill, pseudo-randomization, CRC-32 attachment, and termination bits — batch (`Condition`) or streaming (`Conditioner`). On the receive side, `Recover` performs the post-decoder steps of §3.14 and §3.15: frame synchronization with a lock on the expected marker position, a per-frame quality indicator, and a per-frame sequence indicator. The SCPPM encoder, everything modulation-coupled, and the SCPPM decoder are out of scope. All operations are bit-addressable, because the block lengths of table 3-1 are not multiples of eight. |

### A1.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/ocsc (Go package) |
| System Name(s) | Astro |

### A1.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 142.0-B-1 (Optical Communications Coding and Synchronization, Blue Book, Issue 1, August 2019) |
| Have any exceptions been required? | Yes [X] No [ ] — see A1.5 |

---

## A1.2 DATA CONDITIONING

| Feature | Reference | Status | Support |
|---|---|---|---|
| CCSDS transfer frames as input (HPE-1) | §3.2 | M | Y — caller-supplied TM/AOS/USLP frames; the §3.2 NOTE's streaming encoding is supported by `Conditioner` |
| ASM attachment to form SMTFs | §3.3.1 | M | Y |
| ASM sequence 1ACFFC1D | §3.3.2 | M | Y |
| Slicing into k-digit information blocks | §3.4.1 | M | Y |
| Information block sizes of table 3-1 | table 3-1 | M | Y — 5006, 7526, 10046 for rates 1/3, 1/2, 2/3 |
| Zero fill to a multiple of k | §3.4.2.1.1 | M | Y |
| Pseudo-randomization | §3.5.1.1 | M | Y |
| Generator g(D) = D^8+D^7+D^5+D^3+1 | §3.5.2.1 | M | Y — verified against the published 40-digit vector |
| Sequence period 255 | §3.5.3.1 | M | Y |
| Register initialized to all ones per block | §3.5.3.2 | M | Y |
| CRC-32 attachment | §3.6.1.1 | M | Y |
| Generator h(X) = X^32+X^29+X^18+X^14+X^3+1 | §3.6.2.2 | M | Y — 0x20044009, register preset to all ones |
| Two termination digits | §3.7 | M | Y |
| Encoder input block of k-hat digits | §3.7 | M | Y — 5040, 7560, 10080 |

---

## A1.3 CODE RATES

| Feature | Reference | Status | Support |
|---|---|---|---|
| Code rate 1/3 | table 3-1 | O | Y |
| Code rate 1/2 | table 3-1 | O | Y |
| Code rate 2/3 | table 3-1 | O | Y |

---

## A1.4 CHANNEL CODING AND MODULATION

| Feature | Reference | Status | Support |
|---|---|---|---|
| SCPPM encoder | §3.8 | M | N — see A1.5 |
| Code interleaver | §3.8 | M | N |
| Accumulator and PPM mapping | §3.8 | M | N |
| Channel interleaver | §3.9 | M | N |
| Codeword synchronization marker | §3.10 | M | N |
| Repeat factor (HPE-11) | §3.11 | 1+ | N |
| Slot mapper | §3.12 | M | N |
| Guard slot insertion | §3.13 | M | N |

---

## A1.4a TRANSFER FRAME VALIDATION AND SEQUENCE INDICATION

| Feature | Reference | Status | Support |
|---|---|---|---|
| Transfer frame validation (HPE-14) | §3.14 | M | P — the sync-marker synchronization of §3.14.1 and the quality indicator of §3.14.2 are implemented in `Recover`; the SCPPM decoding that precedes them is out of scope (see A1.5), so block correctness comes from the attached CRC-32 rather than from a decoder |
| Quality Indicator per transfer frame | §3.14.2 | M | Y — `RecoveredFrame.Valid` is false when any block carrying the frame's bits failed its CRC |
| Optional FECF in transfer frame | §3.14.3 | O | N — the Data Link Protocol Sublayer's concern; see `pkg/crc` |
| Sequence Indicator | §3.15 | M | Y — `RecoveredFrame.Gap` is 'zero' (false) when the frame is the direct successor of the previous one, 'one' (true) when a gap was detected |

---

## A1.4b MANAGED PARAMETERS FOR TELEMETRY SIGNALING (table 5-1)

| Managed Parameter | Reference | Allowed Values | Support |
|---|---|---|---|
| TM/AOS/USLP transfer frame length (octets) | §5.2 | Integer (max 65536) | Y — enforced as `MaxFrameLength` on `AttachASM`, `Condition`, `Conditioner.Push`, and `Recover` |
| PPM order, M | §5.2 | 4, 8, 16, 32, 64, 128, 256 | N — SCPPM stages out of scope (see A1.5) |
| Code rate, r | §5.2 | 1/3, 1/2, 2/3 | Y — all three |
| Channel interleaver rows N, shift increment B | §5.2 | see table 5-1 | N — channel interleaver out of scope (see A1.5) |
| Repeat factor, q_d | §5.2 | 1, 2, 3, 4, 8, 16, 32 | N — repeat stage out of scope (see A1.5) |

---

## A1.5 EXCEPTIONS AND UNSUPPORTED FEATURES

| Feature | Reference | Support | Rationale |
|---|---|---|---|
| SCPPM encoder and all later stages | §3.8 to §3.13 | N | These stages are coupled to the pulse-position modulation and to the physical layer. This package deliberately stops at the SCPPM encoder input block, which is the last purely deterministic bit transform in the chain. |
| Iterative SCPPM decoding | §2 | N | Decoding SCPPM is a research-grade soft-decision problem. This library ships no soft-decision decoder in any package, and a wire-format library is the wrong home for one. The post-decoder steps of §3.14 — ASM synchronization, frame recovery, the quality indicator — and the §3.15 sequence indicator are implemented in `Recover`. |
| Slot and symbol synchronization, channel estimation | §2 | N | Receiver signal processing, not data format. |
| HPE beacon and optional accompanying data transmission signaling (uplink beacon, AOS or USLP transfer frames, LDPC-coded) | §4 | N | A separate signaling scheme using different codes and a different CSM. A follow-up. |
| CLI subcommands | — | N | A follow-up once the API settles. |

---

## A1.6 IMPLEMENTATION NOTES

| Note | Detail |
|---|---|
| Bit-addressable throughout | Table 3-1's block lengths are 5006, 7526 and 10046 binary digits, none a multiple of eight. Every operation works on a `BitString`. |
| CRC-32 implemented locally | The generator of §3.6.2.2 is 0x20044009, which differs from IEEE CRC-32, from CRC-32C in `pkg/crc`, and from the Proximity-1 CRC in `pkg/pxsc`. |
| Randomizer verified against the spec vector | §3.5.2.1's note publishes the first 40 digits. A short polynomial admits several register layouts producing different sequences, so the published vector is the only safe check. |
| `Recover` requires a frame length | The slicer's zero fill is indistinguishable from frame data once in the stream. Frame length is a managed parameter, so a real receiver has it. With it, `Recover` locks frame synchronization: after each frame the next marker is checked at the one expected offset (§3.14.1), falling back to a bit-by-bit hunt — and a raised sequence indicator — only on mismatch. |
| Quality indicator without a decoder | §3.14.2 marks a frame invalid when it is recovered from an incorrectly decoded codeword. With no SCPPM decoder here, `Recover` uses the attached CRC-32 of §3.6 as the per-block correctness signal and maps failing blocks onto the frames whose bits they carry, sync marker included. |
| Streaming conditioning | `Condition` is a batch call: its input is one complete transmission, and the zero fill of §3.4.2.1.1 lands in its final block. `Conditioner` implements the §3.2 NOTE's streaming encoding: partial blocks carry between `Push` calls, and fill is inserted only at explicit `Close`. |
