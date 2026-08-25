# PICS PROFORMA FOR LOSSLESS DATA COMPRESSION

## Conformance Statement for `pkg/ldc` — CCSDS 121.0-B-3

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 23/08/2026 |
| PICS Serial Number | ASTRO-LDC-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/ldc |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing the Rice adaptive entropy coder and its preprocessor, encoder and decoder both. Integer arithmetic throughout; no floating point appears anywhere in the package. Byte-slice in, byte-slice out — packetization is the caller's. |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/ldc (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 121.0-B-3 (Lossless Data Compression, Blue Book, Issue 3, August 2020) |
| Companion report | CCSDS 120.0-G-4 (Lossless Data Compression, Green Book, Issue 4, November 2021) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE — Non-supported capabilities are identified in section A2.3. All of them
are options the standard leaves to the application or defines for transport
layers this package does not touch.

NOTE — This implementation is validated against the official CCSDS 121.0-B-2
test vectors published by the SLS Data Compression working group (as mirrored
in libaec's `data/121B2TestData`). All 72 AllOptions and LowEntropyOptions
vectors — 36 in each set, covering resolutions 1 through 32 — encode
byte-identically and decode back to the exact samples; they are vendored in
`pkg/ldc/testdata/` and every one runs as `TestVectors_*` in
`vectors_test.go`. The ExtendedParameters set is excluded for the reason given
in section A2.3.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: Adaptive entropy coder — parameters

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| LDC-1 | Block size J | 3.1.6 | M | 8, 16, 32, 64 | Yes | `Params.BlockSize`. Any other value is `ErrInvalidBlockSize`. |
| LDC-2 | Sample resolution n | 3.1.6 | M | 1 to 32 bits | Yes | `Params.Resolution`. |
| LDC-3 | Unsigned sample range | 3.1.6, 4.4 | M | 0 to 2^n−1 | Yes | |
| LDC-4 | Signed sample range | 3.1.6, 4.4 | M | −2^(n−1) to 2^(n−1)−1 | Yes | `Params.Signed`. Two's complement, sign extended from n bits. Requires the unit-delay predictor — see the interpretation note in A2.3. |
| LDC-5 | Option identifier attached to every coded data set | 3.1.1, 5.2.1.3 | M | — | Yes | Always written, even when a subset of options is in use. |

### Table A-2: Code options

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| LDC-6 | Fundamental sequence option | 3.2 | M | Yes | Table 3-1 transcribed as a test. Implemented as the split-sample option with k=0, which §3.3.2 says it is. |
| LDC-7 | Split-sample options | 3.3 | M | Yes | Every k the resolution allows: up to 5, 13 or 29 in the three basic columns of table 5-1. |
| LDC-8 | Split-sample field order | 3.3.3 | M | Yes | All FS codewords for the block first, then all split bits. Not interleaved — the spec is explicit and the obvious implementation is wrong. |
| LDC-9 | Second-extension option | 3.4 | M | Yes | The transform of §3.4.1, with δ₁ = 0 substituted on a reference block. |
| LDC-10 | Second-extension overflow | 3.4.2 | M | Yes | At 32-bit resolution the transform can exceed a 64-bit integer. The option reports itself unusable rather than wrapping. §3.4.2's note that it "is only designed to be a useful option when all of the transformed symbols are small" is why this can never lose data. |
| LDC-11 | Zero-block option | 3.5 | M | Yes | Including the ROS codeword and the 64-block segments of §3.5.2. |
| LDC-12 | Zero-block run codewords | 3.5.3, table 3-2 | M | Yes | Table transcribed in full, including the ROS codeword displaced between four and five. |
| LDC-13 | Zero-block spans multiple blocks | 3.1.4, 3.5.1 | M | Yes | One coded data set covers the whole run. |
| LDC-14 | All-zeros with a reference sample | 3.5.1, 3.7.2 | M | Yes | A reference block counts as all zeros when the J−1 samples after the reference are zero, whatever the reference itself is. |
| LDC-15 | No-compression option | 3.6 | M | Yes | |

### Table A-3: Code selection

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| LDC-16 | Select the option minimizing encoded bits, identifier included | 3.7.1, 3.7.3 | M | Yes | Every option is priced without being emitted, which matters: an FS codeword at 32-bit resolution can be four billion bits long. |
| LDC-17 | Zero-block always selected for all-zeros runs | 3.7.2 | M | Yes | Not priced against the others; imposed. |
| LDC-18 | Tie-breaking order | 3.7.4 | M | Yes | No compression, then second extension, then smallest k. Pinned by test — this is not the order an implementer would guess. |

### Table A-4: Preprocessor

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| LDC-19 | Preprocessor may be omitted | 4.1 | O | Yes | `PredictorNone`. |
| LDC-20 | Unit-delay predictor | 4.2.5 | M | Yes | The only predictor the standard specifies. |
| LDC-21 | First sample of a reference interval predicts itself | 4.2.5 | M | Yes | So its prediction error is zero. |
| LDC-22 | Bypass predictor | 4.2.3 | O | Yes | Predicts zero, keeps the mapper. |
| LDC-23 | Application-specific predictor | 4.2.4 | O | No | The standard names it and does not define it. A file header requesting it is refused with `ErrUnsupportedPredictor`. |
| LDC-24 | Reference samples required only for previous-sample predictors | 4.2.6 | M | Yes | Inserted for the unit-delay predictor, and for nothing else. |
| LDC-25 | Reference sample is the first of a block, uncoded, leading the CDS | 4.2.6, 5.2.2 | M | Yes | |
| LDC-26 | Reference sample interval r | 4.3 | M | Yes | 1 to 4096 blocks. Bounds the zero-block segments even when no reference samples are used. |
| LDC-27 | Prediction error mapper | 4.4 | M | Yes | The three-branch equation, with θ = min(x̂ − xmin, xmax − x̂). Verified against the worked table of Green Book §3.3.3, including the two rows past θ. |
| LDC-28 | Mapper is a bijection | 4.4 | M | Yes | Checked exhaustively at low resolution across every predictor value. |

### Table A-5: Coded data set formats

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| LDC-29 | Option identification key | 5.2.1, table 5-1 | M | Yes | All five resolution columns, transcribed in full as a test. |
| LDC-30 | Basic code option set | 5.2.1.1 | M | Yes | 3, 4 and 5-bit identifiers by resolution. |
| LDC-31 | Restricted code option set | 5.2.1.1 | O | Yes | 1 and 2-bit identifiers, allowed only at n ≤ 4. Requesting it above that is `ErrRestrictedNotAllowed`. |
| LDC-32 | CDS format, FS and split-sample | 5.2.3 | M | Yes | |
| LDC-33 | CDS format, no compression | 5.2.4 | M | Yes | |
| LDC-34 | CDS format, zero block | 5.2.5 | M | Yes | |
| LDC-35 | CDS format, second extension | 5.2.6 | M | Yes | J/2 symbols, per §3.4.1 and figure 5-4. The prose of §5.2.6 says "2J transformed pairs", which contradicts both and is read as a typo. |
| LDC-36 | Bit order MSB first | 1.5.2 | M | Yes | |

### Table A-6: File format

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| LDC-37 | File header, 12 octets | 7.2.2, table 7-1 | O | Yes | Every field, encoded and decoded. |
| LDC-38 | Output word size B | 7.2.1.2 | O | Yes | 1 to 8 octets. |
| LDC-39 | Reserved fields are zero | table 7-1 | M | Yes | All three checked on decode; a set bit is `ErrReservedFieldSet`. |
| LDC-40 | Data Sense | table 7-1 | M | Yes | '0' two's complement, '1' positive. The one field that reads the opposite way from the rest, and pinned by its own test for that reason. |
| LDC-41 | Application-specific mapper | table 7-1 | O | No | Refused with `ErrUnsupportedMapper`. |
| LDC-42 | File body is concatenated CDSes | 7.2.3.1 | O | Yes | |
| LDC-43 | Zero fill to the word boundary | 7.2.3.2 | O | Yes | |

### Table A-7: Packet formats

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| LDC-44 | CDSes inserted into space packets | 5.3 | O | No | Out of scope by design: this package emits coded data sets and the caller packetizes, with `pkg/spp`. |
| LDC-45 | Compression identification packet | 6 | O | No | The standard marks section 6 optional. The file header of section 7 covers the same need and is implemented instead. |

---

## A2.3 EXCEPTIONS AND LIMITATIONS

### Non-Supported Items

| Item | Description | Reason |
|------|-------------|--------|
| LDC-23 | Application-specific predictor | §4.2.4 names it and leaves it undefined: "such a predictor is unique to the application and is not specified in this Recommended Standard." There is nothing to implement. A file header requesting it is refused rather than silently decoded with the wrong predictor. |
| LDC-41 | Application-specific mapper | Same reasoning, from table 7-1. |
| LDC-44 | Insertion into space packets | The caller composes coded data sets into packets. Keeping the two apart is what lets this package be used with the file format, with packets, or with neither. |
| LDC-45 | Compression identification packet | Section 6 is optional and duplicates what the file header carries. |
| — | Reference-interval byte alignment | Some encoders pad the coded stream to a byte boundary at the end of each reference sample interval, an application framing choice the standard leaves open rather than a numbered requirement (libaec exposes it as its `-p` option). This decoder reads the coded data set as one continuous bit stream and cannot consume such streams. Seen in the official 121.0-B-2 `ExtendedParameters/sar32bit.j16.r256.rz` vector, which is why that set is not in `testdata/`. |

### Interpretations

Two places where the standard's text admits more than one reading, and the
reading taken here:

| Where | Reading taken |
|---|---|
| Signed samples and the predictor (table 7-1, Data Sense) | Table 7-1 makes the positive Data Sense "mandatory if preprocessor is bypassed or preprocessor absent". Read narrowly, that constrains only a section-7 file header field; read broadly, it says signed samples are meaningful only under the unit-delay predictor. This implementation takes the broad, conservative reading and enforces it in `Params.Validate` everywhere, not just in the file path: `Signed` with any predictor other than unit delay is refused with `ErrUnsupportedPredictor`. The narrow reading would let signed samples through with the bypass predictor outside the file format — and would then produce parameter sets a section-7 header cannot describe. Refusing keeps every compressible stream expressible as a file. |
| Second-extension CDS symbol count (§5.2.6) | The prose says "2J transformed pairs", which contradicts §3.4.1 and figure 5-4 (J/2 symbols). Read as a typo; J/2 is implemented. Also recorded at LDC-35. |

### Implementation-Defined Limits

None of these limits are in the standard. The first two exist because a
decoder must not be able to be told to exhaust memory; the third is a
documented limitation of the one entry point that decodes without a sample
count.

| Limit | Value | Why |
|---|---|---|
| Decodable sample count | 2^28 samples | The header's Number of Samples field is 48 bits, so a twelve-octet file can claim 2^48 samples — a terabyte of output — and the decoder would size a slice from it before reading a coded bit. |
| FS codeword length | Bounded by the resolution | A run of zero octets in a corrupt stream would otherwise be read as an enormous sample value. `ReadFS` takes a limit and refuses past it. |
| Fill skipped by the unbounded `Decompress` | 7 bits | `Decompress` (no sample count) treats a trailing all-zero run of fewer than eight bits as §7.2.3.2 fill, which covers a B=1 file body exactly. A file written with an output word size B > 1 octet can carry up to 8B−1 fill bits, and without the count that tail cannot be told from a truncated coded data set — so `Decompress` fails with an error rather than guessing. `DecompressCount` and `DecompressFile` know the count and skip any fill. Pinned by `TestDecompressRefusesLongWordFill`. |

### Fully Supported Mandatory Items

Every mandatory item is supported. All five code options are implemented in
both directions, along with both specified predictors, the mapper, the coded
data set formats, and the option selection rule including its tie-breaks.

| Area | Items | Implementation |
|------|-------|----------------|
| Parameters | LDC-1–5 | `params.go` |
| Bit packing | LDC-36 | `bits.go` |
| Code options | LDC-6–15 | `options.go` |
| Selection | LDC-16–18 | `encoder.go` |
| Preprocessor | LDC-19–28 | `preprocessor.go` |
| CDS formats | LDC-29–35 | `encoder.go`, `decoder.go` |
| File format | LDC-37–43 | `file.go` |

### Verification

| Source | What it pins |
|---|---|
| Blue Book table 3-1 | Fundamental sequence codewords |
| Blue Book table 3-2 | Zero-block run codewords, including ROS |
| Blue Book table 5-1 | Every option identifier at every resolution |
| Blue Book table 7-1 | File header, field by field |
| Blue Book §3.4.1 | The second-extension transform |
| Blue Book §3.7.4 | The tie-breaking order |
| Green Book §3.3.3 | The preprocessor and mapper, as a worked table |
| Green Book figure 3-4 | Zero-block segmentation and ROS |

Annex A of the Green Book names a fuller vector set at
`cwe.ccsds.org/sls/docs/sls-dc/BB121B3TestData`. That location requires a
CCSDS login and returned HTTP 403, so it has not been run against this
implementation. Anyone with access is encouraged to.
