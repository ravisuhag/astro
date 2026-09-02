---
title: Housekeeping Compression
short: RHC
description: "ICS proforma: what this package implements, clause by clause."
order: 190
---

## Conformance Statement for `pkg/rhc` — CCSDS 124.0-B-1

---

This standard carries its own Implementation Conformance Statement proforma in
annex A, which is normative. What follows fills it in, and then adds the
detail the five-item requirements list does not have room for.

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of ICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 23/08/2026 |
| ICS serial number | ASTRO-RHC-ICS-001 |
| System Conformance statement cross-reference | This document |

### A2.1.2 Identification of Implementation Under Test

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/rhc |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Function Implemented | Compression **Y** — Decompression **Y (derived, see A2.3)** |
| Special Configuration | None |
| Other Information | Go library. Both directions are explicit state types holding the mask, the build and the previous vector; neither is safe for concurrent use. Integer and bitwise operations only. Byte-slice in, byte-slice out with an explicit bit length — framing is the caller's, per clause 2.2. |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Versions | astro/pkg/rhc (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 124.0-B-1 Issue 1, February 2023 |
| Have any exceptions been required? | Yes [ ] No [X] |

Every mandatory requirement is supported. The exceptions section below records
what is *not* in the standard and therefore had to be decided here.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: Requirement List (the standard's own)

| Item | Description | Reference | Status | Support |
|---|---|---|---|---|
| R1 | Input | 3.2 | M | Y |
| R2 | Parameters | 3.3 | M | Y |
| R3 | Mask Update | 4.2 | M | Y |
| R4 | Encoding Functions | 5.2 | M | Y |
| R5 | Encoding Step | 5.3 | M | Y |

### Table A-2: Parameters (the standard's own)

| Item | Description | Reference | Status | Values Allowed | Values Supported |
|---|---|---|---|---|---|
| P1 | Input Vector Length, F | 3.2 | M | 1 .. 65535 | 1 .. 65535 |
| P2 | Configuration of initial mask vector, M0 | 3.3.1 | M | 0 .. 2^F-1 | any; nil means all zeros, the default clause 3.3.1's note suggests |

---

## A2.3 DETAIL BEYOND THE PROFORMA

The five-item list above is coarse, so this section breaks each item down.

### Table B-1: Input and parameters (clause 3)

| Item | Description | Reference | Support | Notes |
|---|---|---|---|---|
| RHC-1 | Fixed-length input vector, 1 to 65535 bits | 3.2 | Yes | `Config.VectorLength`. A wrong-length packet is `ErrInvalidPacketLength`. |
| RHC-2 | Initial mask vector M0 | 3.3.1 | Yes | `Config.InitialMask`. |
| RHC-3 | Minimum required effective robustness level R_t, 0 to 7 | 3.3.2a | Yes | `Config.Robustness`, also per-cycle through `CompressWith`. |
| RHC-4 | New mask flag | 3.3.2b | Yes | Per-cycle. `NewMaskInterval` or `ForceNewMask`. |
| RHC-5 | Send mask flag | 3.3.2c | Yes | Per-cycle, and forced to one while t <= R_t as the clause requires. |
| RHC-6 | Uncompressed flag | 3.3.2d | Yes | Per-cycle, and forced to one while t <= R_t. |
| RHC-7 | Parameters need not be known by the decompressor | 3.3.2 note | Yes | Every flag is recoverable from the output vector. F is the exception, and the note does not list it. |

### Table B-2: Mask update (clause 4)

| Item | Description | Reference | Support | Notes |
|---|---|---|---|---|
| RHC-8 | Build vector, equation 6 | 4.2.1 | Yes | Including B_0 = 0 and the reset when the new mask flag is set. |
| RHC-9 | Mask vector, equation 7 | 4.2.2 | Yes | Both branches: over the mask, or over the build when the flag is set. |
| RHC-10 | Change vector, equation 8 | 4.2.3 | Yes | D_0 = 0. |

### Table B-3: Encoding functions (clause 5.2)

| Item | Description | Reference | Support | Notes |
|---|---|---|---|---|
| RHC-11 | Counter encoding, table 5-1 | 5.2.2 | Yes | All three forms. Table transcribed as a test. |
| RHC-12 | Long-form width E, equation 9 | 5.2.2 | Yes | The leading-zero count encodes the width, which is what lets the decoder parse without a length field. Pinned by its own test. |
| RHC-13 | Run-length encoding, equation 10 | 5.2.3 | Yes | Including the '10' terminator. Figure 5-1's worked counts transcribed as a test. |
| RHC-14 | Trailing zeros not encoded | 5.2.3 note 1 | Yes | Inferred from the vector length. |
| RHC-15 | All-zero vector encodes as the terminator alone | 5.2.3 note 2 | Yes | |
| RHC-16 | Bit extraction, equation 11 | 5.2.4 | Yes | Emitted last selected position first, which is what the equation says once clause 1.6.1's conventions are applied — see the interpretation note in A2.4. |
| RHC-17 | Bit ordering, MSB first | 1.6.1 | Yes | |
| RHC-18 | Vector operations: XOR, OR, AND, inverse, left shift, reversal, Hamming weight | 1.6.1 | Yes | The examples given in clause 1.6.1's text are transcribed as tests. |

### Table B-4: Encoding step (clause 5.3)

| Item | Description | Reference | Support | Notes |
|---|---|---|---|---|
| RHC-19 | Output structure o_t = h_t \|\| q_t \|\| u_t, equation 12 | 5.3.1 | Yes | |
| RHC-20 | d-dot_t, equation 13 | 5.3.2.1 | Yes | |
| RHC-21 | Effective robustness V_t, equation 14 | 5.3.2.2 | Yes | Including C_t, the run of unchanged cycles before the window, which the encoder gets for free and reports. |
| RHC-22 | h_t, equation 15 | 5.3.3.1 | Yes | |
| RHC-23 | Change window X_t, equation 16 | 5.3.3.1 | Yes | All three branches. |
| RHC-24 | y_t, equation 17 | 5.3.3.1 | Yes | |
| RHC-25 | e_t, equation 18 | 5.3.3.1 | Yes | |
| RHC-26 | k_t, equation 19 | 5.3.3.1 | Yes | |
| RHC-27 | c_t, equation 20 | 5.3.3.1 | Yes | The new-mask-set-twice test over the window. |
| RHC-28 | q_t, equation 21 | 5.3.3.2 | Yes | Including the transition coding M XOR M<< before run-length encoding. |
| RHC-29 | u_t, equation 22 | 5.3.3.3 | Yes | All five branches. |

---

## A2.4 EXCEPTIONS AND THINGS THE STANDARD LEAVES OPEN

### The bit extraction order of equation 11

Equation 11 defines the bit extraction as

> BE(a, b) = ȧ_{g(H(b)−1)} ∥ ⋯ ∥ ȧ_{g0}, where g_i denotes the position of
> the ith '1' bit in b, starting from the MSB.

Two readings are possible, and they produce different bit streams. Since g_0
is the first selected position in transmission order, the concatenation as
written leads with the bit at the *last* selected position — but an
implementer could suspect the subscripts of merely reflecting clause 1.6.1's
downward bit numbering, in which case a forward scan would be intended.

This implementation takes the reversed reading, on two grounds:

1. **clause 1.6.1 fixes what the concatenation means.** Equation 1 writes the left
   shift as a« = {ȧ_{N−2}, …, ȧ_1, ȧ_0, 0} and gives the worked example
   '10111' -> '01110', which is only consistent if the first listed term is
   the first transmitted bit. Applying the same convention to equation 11,
   BE transmits ȧ_{g(H−1)} first: the forward scan, reversed. The
   subscripts-are-just-numbering reading would need the example to come out
   '11100', and it does not.
2. **Independent implementations agree.** The VisionSpace PocketPlus C++
   implementation reverses the extracted sequence at every bit-extraction
   site.

So `Vector.Extract` emits the reverse of the forward scan, at all three BE
sites: k_t (equation 17/19) and the two compressed u_t branches
(equation 22). The derived decompressor mirrors the reversal when consuming
them. An earlier revision of this package used the forward reading; streams
it produced do not decode against this one where a k_t or compressed u_t of
two or more differing bits is involved.

No published test vector arbitrates the point — see below — so the reading
rests on the two grounds given.

### The decompressor is derived, not transcribed

CCSDS 124.0-B-1 specifies the compressor and nothing else. Its normative
sections are clause 3, clause 4 and clause 5; the requirements list in annex A2.2.1 has five
items and every one is an encoder item. There is no decoder section.

The decompressor here is the encoder run backwards. Clause 2.1 lists what a
decompressor needs — the last reconstructed vector, a synchronized mask, and
the unpredictable bit values — and the encoding is losslessly invertible, so
the derivation is sound. But it is a derivation, and no published vector
confirms it. What stands behind it instead:

- round trips across vector lengths from 1 to 1000 bits, all robustness levels,
  and every combination of the three flags;
- a mask-tracking test asserting the decompressor's mask equals the
  compressor's after **every** cycle, which catches drift long before it
  changes a reconstructed byte;
- the loss test below;
- two fuzz targets.

### No test vectors are published

The standard contains no worked example of a complete output vector, and no
companion Green Book exists (`124x0g1.pdf` returns 404). What could be pinned
to published material has been: table 5-1, figure 5-1's counts, and the vector
operation examples in clause 1.6.1's text. Everything above that level is verified by
round trip and by hand-derived arithmetic with the equation cited.

### Loss detection is the mission's, and this changes the API

Clause 2.2 says the standard "does not provide a mechanism for identifying the number
of sequential output binary vectors that were lost", and suggests packet
sequence counters. So `Decompressor.NotifyLoss` exists: the caller reports gaps,
and only then can the decompressor tell whether the next output reaches back
far enough.

**Without NotifyLoss, a decompressor fed a stream with holes will reconstruct
wrong bytes and not know.** That is the standard's division of responsibility,
not a defect here, but it is the single most important thing for an integrator
to get right.

Likewise clause 2.2 disclaims sync markers: a corrupt or foreign vector that happens
to parse will be accepted as genuine. Framing with a length field — space
packets, for instance — is assumed.

The loss test drives this: 300-vector streams, robustness 0 through 7, drop
rates 5% to 50%, gaps reported through NotifyLoss. Every vector the
decompressor returns is asserted byte-identical to its original. It is allowed
to refuse; it is never allowed to be wrong.

### The trust model, and strict mode

The recovery gate above has a trust assumption worth stating plainly. After a
reported gap, the decompressor accepts the next output when the gap is at most
that output's effective robustness level V_t — a field the output vector
declares *about itself* (clause 5.3.2.2), which nothing in the format lets a
decompressor verify. The standard offers no integrity mechanism at all: no
checksum, no signature, no sync marker. So the model is:

- **Transport is trusted to deliver what was sent.** Corruption and forgery
  are for the layers below — CRCs and FECFs on the space link, SDLS if the
  mission needs cryptographic integrity.
- **The caller is trusted to report every gap.** clause 2.2 makes loss detection
  the mission's job; NotifyLoss is how it is reported here.
- **Given both, the output vector's own fields are believed** — V_t
  included. A hostile or corrupt vector that survives the layers below can
  claim a reach of up to 15 and be believed, and the reconstruction it
  produces will be wrong.

For callers who will not extend the third trust across a gap, `Config.Strict`
narrows the gate: after any reported loss (NotifyLoss, or an output that
failed to parse), a strict decompressor refuses everything except an
uncompressed output — the one kind that proves itself by carrying the whole
input vector, rather than claiming a reach back to state the decompressor no
longer has. The cost is availability: outputs between the gap and the next
uncompressed one are refused even when their V_t is honest. Strict mode is
this package's addition, not the standard's, and is off by default. Pinned by
`TestStrictModeWaitsForUncompressed`.

### Mission-tunable knobs exposed as configuration

Clause 3.3.2 makes three flags user-specified at every cycle and says nothing about
when to set them. Rather than invent a policy, the flags are exposed directly
through `CompressWith` and `ForceNewMask`/`ForceSendMask`/`ForceUncompressed`,
with simple periodic knobs in `Config` for the common case:

| Knob | Normative? |
|---|---|
| `Robustness` | Yes — clause 3.3.2a, 0 to 7 |
| `NewMaskInterval` | No — this package's convenience |
| `SendMaskInterval` | No — beyond the clause 3.3.2c forcing while t <= R_t |
| `UncompressedInterval` | No — beyond the clause 3.3.2d forcing while t <= R_t |

No adaptive or loss-aware scheduling is built in. That is a mission heuristic
and belongs above this package.

### Vector length must be known in advance

Clause 3.3.2's note lists the parameters a decompressor need not be told: M0, R_t and
the three flags. F is deliberately not among them, so `Config.VectorLength` is
required on both sides. Where an uncompressed output carries F, it is checked
against the configuration and a disagreement is `ErrVectorLengthMismatch`.

### Implementation-defined limits

| Limit | Value | Why |
|---|---|---|
| Remembered history | 16 cycles | clause 5.3.2.2 bounds C_t by min(t,15) - R_t, so nothing older can ever be needed. Not a restriction, just the working set. |
| Counter codeword leading zeros | 10 | A-2 is at most 65533, whose minimal width is 16, so more than ten leading zeros cannot be legal. Without the bound a corrupt stream could spin. |
