# PICS PROFORMA FOR TM SYNCHRONIZATION AND CHANNEL CODING

## Conformance Statement for `pkg/tmsc` — CCSDS 131.0-B-5

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 24/08/2026 |
| PICS Serial Number | ASTRO-TMSC-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/tmsc |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS TM Synchronization and Channel Coding sublayer. Provides Attached Sync Marker (ASM) framing, CCSDS pseudo-randomization (255-bit legacy sequence), Channel Access Data Unit (CADU) wrapping/unwrapping, and Reed-Solomon coding with RS(255,223) and RS(255,239), dual (Berlekamp) basis symbol representation, symbol interleaving, and shortened codeblocks via virtual fill. |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/tmsc (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 131.0-B-5 (TM Synchronization and Channel Coding, Blue Book, Issue 5, September 2023) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE — Of the coding methods, only Reed-Solomon is implemented. Convolutional (section 3), concatenated (section 5), turbo (section 6), and LDPC (sections 7 and 8) coding are not implemented. Of the two pseudo-randomizer sequences, only the 255-bit legacy sequence (10.4.2) is implemented; the 131071-bit sequence (10.4.1) is not. Non-supported capabilities are identified in section A2.2.

### Status Notation

| Status | Meaning |
|---|---|
| M | Mandatory for every implementation of the selected options. |
| O.1 | Coding method. The coding method used on a Physical Channel is a mission-selected managed parameter (12.3, table 12-1); no individual method is mandatory, but at least one must be provided. |
| O.2 | Randomizer sequence. Long (131071-bit), short (255-bit), or absent is a mission-selected managed parameter (12.3, table 12-1). |
| O.3 | Shortened codeblock. The virtual fill length Q is a mission-selected managed parameter (12.5, table 12-3). |

---

## A2.2 REQUIREMENTS LIST

### Table A-1: Frame Synchronization (section 9)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TMSC-1 | ASM bit pattern 0x1ACFFC1D for uncoded, convolutional, Reed-Solomon, concatenated, rate-7/8 LDPC (Transfer Frame), and all LDPC (stream of SMTFs) coded data | 9.2, 9.3.1 | M | Yes | `DefaultASM()` returns the 32-bit pattern 0x1ACFFC1D. Fresh copy returned each call to prevent mutation. |
| TMSC-2 | ASM attachment: the ASM immediately precedes the codeblock or Transfer Frame, with no intervening bits | 9.4.1, 9.4.2 | M | Yes | `WrapCADU()` prepends the ASM directly to the (optionally randomized) codeblock or frame data. Custom ASM supported via parameter. |
| TMSC-3 | ASM detection/stripping (receive) | 9.1, 9.4 | M | Yes | `UnwrapCADU()` validates and strips the ASM. Returns `ErrSyncMarkerMismatch` if the ASM is not found at the expected position. |
| TMSC-4 | ASM excluded from the Reed-Solomon encoder/decoder data space | 9.5.1 | M | Yes | The ASM is handled by `WrapCADU()`/`UnwrapCADU()` entirely outside `RSCodec`; it is never randomized or encoded. |
| TMSC-5 | Scheme-specific ASMs (64-bit for rate-1/2 turbo and rates 1/2, 2/3, 4/5 LDPC; 96/128/192-bit for low-rate turbo) | 9.3.2–9.3.5 | O.1 | No | Not applicable: the corresponding turbo and LDPC coding schemes are not implemented (see table A-4). |
| TMSC-6 | ASM for embedded data stream (0x352EF853) | 9.6 | O | No | Not implemented. A custom ASM can be passed to `WrapCADU()`/`UnwrapCADU()` if needed. |

### Table A-2: Pseudo-Randomizer (section 10)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TMSC-7 | Randomization method: exclusive-OR of each bit of the codeblock/frame with a standard pseudo-random sequence | 10.2.1, 10.2.2 | M | Yes | `Randomize(data)` XORs data with the PN sequence. Returns a new slice; input not modified. Applied after RS encoding, ASM excluded. |
| TMSC-8 | Synchronization and application: sequence starts at the first bit after the ASM; generator reinitialized for each codeblock/frame; ASM itself never randomized | 10.3, 10.4.3 | M | Yes | `WrapCADU()` randomizes before prepending the ASM; `UnwrapCADU()` de-randomizes after stripping it. The generator restarts on every call. |
| TMSC-9 | 131071-bit pseudo-random sequence, h(x) = x^17 + x^14 + 1 | 10.4.1 | O.2 | No | Not implemented. This is the preferred sequence in Issue 5 for obviating spectral spikes on high-data-rate links. |
| TMSC-10 | 255-bit legacy pseudo-random sequence, h(x) = x^8 + x^7 + x^5 + x^3 + 1, initialized to all ones | 10.4.2, 10.4.3 | O.2 | Yes | `GeneratePNSequence(length)` implements the 8-bit LFSR; `Randomize()` applies it. Kept in Issue 5 for backward compatibility with legacy systems. |
| TMSC-11 | De-randomization (receive) using the same sequence | 10.3.3, 10.3.4 | M | Yes | Same `Randomize()` function — XOR is self-inverse. Integrated into `UnwrapCADU()` when randomize=true. |

### Table A-3: Reed-Solomon Coding (section 4)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TMSC-12 | Parameters: J = 8 bits per symbol; E = 16 (255,223) | 4.3.1 | M | Yes | `NewRS255_223()` — 32 check symbols, corrects up to 16 symbol errors per codeword. |
| TMSC-13 | Parameters: J = 8 bits per symbol; E = 8 (255,239) | 4.3.1 | M | Yes | `NewRS255_239()` — 16 check symbols, corrects up to 8 symbol errors per codeword. E is a mission-selected managed parameter (12.5, table 12-3). |
| TMSC-14 | General characteristics: n = 255 symbols per codeword, 2E check symbols, k = n − 2E information symbols | 4.3.2 | M | Yes | `DataLen()` = 223 or 239; `NRoots()` = 32 or 16. |
| TMSC-15 | Field generator polynomial F(x) = x^8 + x^7 + x^2 + x + 1 over GF(2) | 4.3.3 | M | Yes | 0x187; lookup tables (`gfExp[512]`, `gfLog[256]`) precomputed in `init()`. |
| TMSC-16 | Code generator polynomial g(x) = ∏(x − α^11j) for j = 128−E … 127+E; equivalently roots β^(112+j), j = 0 … 2E−1, with β = α^11 | 4.3.4 | M | Yes | Generator built from roots β^112 … β^(111+2E) with β = α^11 (`rsFCR` = 112, `rsPrim` = 11 in `rs.go`). |
| TMSC-17 | Systematic code: information symbols unchanged, check symbols appended | 4.3.4 NOTE 2 | M | Yes | `Encode()` copies data and appends parity; data portion is byte-identical on the wire. |
| TMSC-18 | Symbol interleaving, depth I ∈ {1, 2, 3, 4, 5, 8} | 4.3.5.1 | M | Yes | `EncodeInterleaved()` / `DecodeInterleaved()`. `validInterleaveDepth()` rejects other depths with `ErrInvalidInterleaveDepth`. I is a managed parameter (12.5). |
| TMSC-19 | Maximum codeblock length 255·I symbols | 4.3.6 | M | Yes | Enforced by exact input length checks. |
| TMSC-20 | Shortened codeblock length via virtual fill | 4.3.7 | O.3 | Yes | `EncodeShortened(data, depth, virtualFill)` / `DecodeShortened(...)` logically prepend Q zero symbols before encoding and strip them after decoding; the fill is never transmitted. |
| TMSC-21 | Virtual fill constraints: all zeros, not transmitted, at the beginning of the codeblock only, an integer multiple of 8·I bits (Q a multiple of I), Q < kI, fixed per Mission Phase | 4.3.7.3, 4.3.8.2 | M | Yes | Validated: Q must be a non-negative multiple of the depth and smaller than depth × DataLen(), else `ErrInvalidVirtualFill`. Q is a managed parameter that both ends must agree on (12.5, table 12-3). |
| TMSC-22 | Dual basis representation: symbols cross the channel in the dual (Berlekamp) basis, z0 transmitted first | 4.3.9 | M | Yes | Wire bytes are dual-basis; encoder/decoder arithmetic runs in the conventional basis. Transform per 4.3.9.3 and annex F in `tal.go`; verified against libfec-compatible golden vectors. |
| TMSC-23 | Codeblock synchronization achieved via the ASM | 4.3.10 | M | Yes | Fixed-length codeblocks delimited by the ASM through `WrapCADU()`/`UnwrapCADU()`. |
| TMSC-24 | Error-correcting decoder | 4.1 | M | Yes | The Blue Book specifies the code, not a decoding algorithm. Implementation: syndromes, Berlekamp-Massey, Chien search, Forney, then a post-correction syndrome recheck; any inconsistency (including σ′(X⁻¹) = 0) returns `ErrUncorrectable` rather than silently mis-decoding. |
| TMSC-25 | Uncorrectable error detection | 4.1, 4.2.2 | M | Yes | Returns `ErrUncorrectable` when errors exceed E per codeword or the corrected word fails the syndrome recheck. |

### Table A-4: Coding Methods (sections 3, 5–8; selection per table 12-1)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TMSC-26 | Basic convolutional coding (rate 1/2, K = 7, G1 = 171 octal, G2 = 133 octal, G2 inverted) | 3.3 | O.1 | No | Not implemented. |
| TMSC-27 | Punctured convolutional coding (rates 2/3, 3/4, 5/6, 7/8) | 3.4 | O.1 | No | Not implemented. |
| TMSC-28 | Reed-Solomon coding | 4 | O.1 | Yes | See table A-3. |
| TMSC-29 | Concatenated coding (Reed-Solomon outer, convolutional inner) | 5 | O.1 | No | Convolutional inner code not implemented. The RS outer code is available. |
| TMSC-30 | Turbo coding (rates 1/2, 1/3, 1/4, 1/6) | 6 | O.1 | No | Not implemented. |
| TMSC-31 | LDPC coding of a Transfer Frame (rates 1/2, 2/3, 4/5, 7/8) | 7 | O.1 | No | Not implemented. |
| TMSC-32 | LDPC coding of a stream of Sync-Marked Transfer Frames | 8 | O.1 | No | Not implemented. |

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Not Supported |
|----------|-------------|-----------|---------------|
| Mandatory within supported options (M) | 18 | 18 | 0 |
| Coding methods (O.1) | 8 | 1 | 7 |
| Randomizer sequences (O.2) | 2 | 1 | 1 |
| Shortened codeblock (O.3) | 1 | 1 | 0 |
| Other optional (O) | 1 | 0 | 1 |
| **Total** | **30** | **21** | **9** |

NOTE — TMSC-5 is counted with the coding methods it belongs to.

### Non-Conformances (Mandatory Items Not Supported)

None. Every mandatory requirement of the supported options — Reed-Solomon coding with the 255-bit randomizer — is implemented, including the dual basis representation (4.3.9) and the virtual fill rules (4.3.8.2).

### Non-Supported Optional Items

| Item | Description | Reason |
|------|-------------|--------|
| TMSC-5 | Scheme-specific ASMs | The turbo/LDPC schemes they belong to are not implemented. |
| TMSC-6 | Embedded data stream ASM | Not implemented; custom ASMs can be supplied by the caller. |
| TMSC-9 | 131071-bit pseudo-randomizer | Not implemented; the legacy 255-bit sequence is provided. Missions requiring ITU power-flux-density compliance at high data rates should note 10.4.2's caveats. |
| TMSC-26/27 | Convolutional coding | Not implemented. |
| TMSC-29 | Concatenated coding | Requires the convolutional inner code. |
| TMSC-30 | Turbo coding | Not implemented. Specialized application. |
| TMSC-31/32 | LDPC coding | Not implemented. Specialized application. |

### Fully Supported Areas

| Area | Items | Implementation |
|------|-------|----------------|
| Frame synchronization | TMSC-1–4 | `DefaultASM()`, `WrapCADU()`, `UnwrapCADU()`; ASM outside the coded/randomized data space. |
| Pseudo-randomization (255-bit) | TMSC-7, 8, 10, 11 | `GeneratePNSequence()`, `Randomize()` (self-inverse), integrated into the CADU pipeline. |
| Reed-Solomon coding | TMSC-12–25 | GF(2^8) over 0x187, roots β^(112+j) with β = α^11, dual-basis wire representation, systematic encode, full decode pipeline with post-correction syndrome recheck, shortened codeblocks via virtual fill, interleaving depths 1, 2, 3, 4, 5, 8. |
