# PICS PROFORMA FOR TM SYNCHRONIZATION AND CHANNEL CODING

## Conformance Statement for `pkg/tmsc` — CCSDS 131.0-B-4

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 19/03/2026 |
| PICS Serial Number | ASTRO-TMSC-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/tmsc |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS TM Synchronization and Channel Coding sublayer. Provides Attached Sync Marker (ASM) framing, CCSDS pseudo-randomization via PN sequence, Channel Access Data Unit (CADU) wrapping/unwrapping, and Reed-Solomon error correction coding with RS(255,223) and RS(255,239) codes including symbol interleaving. |

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
| Specification | CCSDS 131.0-B-4 (TM Synchronization and Channel Coding, Blue Book, Issue 4, September 2022) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE — Convolutional coding and turbo coding are not implemented. Non-supported optional capabilities are identified in section A2.2.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: Synchronization Functions

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TMSC-1 | Attached Sync Marker (ASM) | 6.2 | M | Yes | `DefaultASM()` returns the standard 4-byte ASM: 0x1ACFFC1D. Fresh copy returned each call to prevent mutation. |
| TMSC-2 | ASM Attachment (Send) | 6.3 | M | Yes | `WrapCADU()` prepends ASM to Transfer Frame data. Custom ASM supported via parameter. |
| TMSC-3 | ASM Detection/Stripping (Receive) | 6.4 | M | Yes | `UnwrapCADU()` validates and strips ASM. Returns `ErrSyncMarkerMismatch` if ASM not found at expected position. Custom ASM supported. |
| TMSC-4 | CADU Construction | 6.5 | M | Yes | `WrapCADU(frameData, asm, randomize)` produces complete CADU: ASM + (optionally randomized) frame data. |
| TMSC-5 | CADU Deconstruction | 6.6 | M | Yes | `UnwrapCADU(cadu, asm, randomize)` extracts frame data from CADU, stripping ASM and optionally de-randomizing. |

### Table A-2: Pseudo-Randomization

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TMSC-6 | PN Sequence Generation | 7.2 | M | Yes | `GeneratePNSequence(length)` generates the CCSDS pseudo-random sequence using 8-bit LFSR with polynomial h(x) = x^8 + x^7 + x^5 + x^3 + 1, initialized to 0xFF. |
| TMSC-7 | Randomization (Send) | 7.3 | M | Yes | `Randomize(data)` XORs data with PN sequence. Returns new slice; input not modified. Applied to Transfer Frame data only (ASM excluded). |
| TMSC-8 | De-Randomization (Receive) | 7.4 | M | Yes | Same `Randomize()` function — XOR is self-inverse. Integrated into `UnwrapCADU()` when randomize=true. |
| TMSC-9 | Randomization in CADU Pipeline | 7.5 | M | Yes | `WrapCADU()` applies randomization before ASM prepend when randomize=true. `UnwrapCADU()` applies de-randomization after ASM strip. |

### Table A-3: Reed-Solomon Coding

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TMSC-10 | GF(2^8) Field | 8.2 | M | Yes | Galois Field GF(2^8) with primitive polynomial x^8 + x^7 + x^2 + x + 1 (0x187). Lookup tables (`gfExp[512]`, `gfLog[256]`) precomputed in `init()`. `gfMul()`, `gfInv()`, `gfPow()` operations. |
| TMSC-11 | RS(255,223) Code | 8.3.1 | M | Yes | `NewRS255_223()` — 32 parity symbols, corrects up to 16 symbol errors per codeword. FCR=112. Generator polynomial precomputed. |
| TMSC-12 | RS(255,239) Code | 8.3.2 | M | Yes | `NewRS255_239()` — 16 parity symbols, corrects up to 8 symbol errors per codeword. FCR=112. Generator polynomial precomputed. |
| TMSC-13 | RS Encoding | 8.4 | M | Yes | `RSCodec.Encode(data)` — systematic encoding. Input: DataLen() bytes. Output: 255-byte codeword (data + parity). Input not modified. |
| TMSC-14 | RS Decoding | 8.5 | M | Yes | `RSCodec.Decode(codeword)` — full error correction pipeline: syndrome computation, Berlekamp-Massey algorithm, Chien search, Forney algorithm. Returns corrected data, error count, and error status. Input not modified. |
| TMSC-15 | Syndrome Computation | 8.5.1 | M | Yes | Syndromes S_i = R(α^(FCR+i)) computed for all nroots values. All-zero syndromes indicate no errors (early exit). |
| TMSC-16 | Berlekamp-Massey Algorithm | 8.5.2 | M | Yes | Computes error-locator polynomial σ(x). Returns degree (number of errors). Returns `ErrUncorrectable` if degree > nroots/2. |
| TMSC-17 | Chien Search | 8.5.3 | M | Yes | Exhaustive evaluation of σ(α^{-i}) over all 255 field elements to find error positions. Returns nil if root count doesn't match expected error count. |
| TMSC-18 | Forney Algorithm | 8.5.4 | M | Yes | Computes error magnitudes using error-evaluator polynomial Ω(x) and formal derivative σ'(x). Corrects codeword in-place. |
| TMSC-19 | Uncorrectable Error Detection | 8.6 | M | Yes | Returns `ErrUncorrectable` when errors exceed correction capability (>16 for RS(255,223), >8 for RS(255,239)). |

### Table A-4: Symbol Interleaving

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TMSC-20 | Interleaved Encoding | 9.2 | M | Yes | `RSCodec.EncodeInterleaved(data, depth)` — de-interleaves input into `depth` blocks, encodes each independently, re-interleaves output. Input: depth × DataLen() bytes. Output: depth × 255 bytes. |
| TMSC-21 | Interleaved Decoding | 9.3 | M | Yes | `RSCodec.DecodeInterleaved(data, depth)` — de-interleaves into `depth` codewords, decodes each independently, re-interleaves corrected data. Returns total corrections across all codewords. |
| TMSC-22 | Interleave Depth 1 | 9.4 | M | Yes | Supported. Equivalent to non-interleaved coding. |
| TMSC-23 | Interleave Depth 2 | 9.4 | M | Yes | Supported. |
| TMSC-24 | Interleave Depth 3 | 9.4 | M | Yes | Supported. |
| TMSC-25 | Interleave Depth 4 | 9.4 | M | Yes | Supported. |
| TMSC-26 | Interleave Depth 5 | 9.4 | M | Yes | Supported. Common for deep-space missions. |
| TMSC-27 | Interleave Depth 8 | 9.4 | M | Yes | Supported. Maximum burst error protection. |
| TMSC-28 | Invalid Depth Rejection | 9.5 | M | Yes | `validInterleaveDepth()` rejects depths other than 1, 2, 3, 4, 5, 8. Returns `ErrInvalidInterleaveDepth`. |

### Table A-5: Optional Coding Schemes

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| TMSC-29 | Convolutional Coding | 10 | O | No | Not implemented. |
| TMSC-30 | Turbo Coding | 11 | O | No | Not implemented. |
| TMSC-31 | LDPC Coding | 12 | O | No | Not implemented. |
| TMSC-32 | Concatenated Coding (RS + Convolutional) | 13 | O | No | Convolutional inner code not implemented. RS outer code is available. |

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Not Supported |
|----------|-------------|-----------|---------------|
| Mandatory (M) | 28 | 28 | 0 |
| Optional (O) | 4 | 0 | 4 |
| **Total** | **32** | **28** | **4** |

### Non-Conformances (Mandatory Items Not Supported)

None. All 28 mandatory items are fully supported.

### Non-Supported Optional Items

| Item | Description | Reason |
|------|-------------|--------|
| TMSC-29 | Convolutional Coding | Not implemented. Can be added as a future enhancement. |
| TMSC-30 | Turbo Coding | Not implemented. Specialized application. |
| TMSC-31 | LDPC Coding | Not implemented. Specialized application. |
| TMSC-32 | Concatenated Coding | RS outer code available; convolutional inner code not implemented. |

### Fully Supported Mandatory Items

All 28 mandatory items (TMSC-1 through TMSC-28) are supported. Key implementations:

| Area | Items | Implementation |
|------|-------|----------------|
| Synchronization | TMSC-1–5 | `DefaultASM()`, `WrapCADU()`, `UnwrapCADU()` with custom ASM support. |
| Pseudo-Randomization | TMSC-6–9 | `GeneratePNSequence()`, `Randomize()` (self-inverse), integrated into CADU pipeline. |
| Reed-Solomon Coding | TMSC-10–19 | GF(2^8) arithmetic with lookup tables, RS(255,223) and RS(255,239), full decode pipeline (syndromes, Berlekamp-Massey, Chien search, Forney), uncorrectable error detection. |
| Symbol Interleaving | TMSC-20–28 | `EncodeInterleaved()`, `DecodeInterleaved()`, all valid depths (1,2,3,4,5,8), invalid depth rejection. |
