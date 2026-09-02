---
title: How this is verified
short: Verification
description: Which claims rest on a published test vector, and which rest on a reading of the standard.
order: 2
---

A [conformance statement](/conformance) says what a package implements. This page says how much to trust it, which is a different question and the one worth asking first.

## The short version

Astro runs every published test vector it can find. There are not many. Everywhere else, the expected octets were worked out by hand from the field layouts in the standard.

That distinction matters more than any coverage number. A hand-derived vector catches a misread clause. It cannot catch a clause that was misread the same way twice, by the person writing the encoder and the person writing the test. Only another implementation catches that.

## What the tests are

| | |
|---|---|
| Test functions | 1730 |
| Fuzz targets | 57, across 20 packages |
| Benchmarks | 33 |
| Numbered PICS items | 500 |
| Statement coverage | 87.8% mean across 26 packages, 69.3% lowest |

## Published vectors, and where they come from

These are the cases where somebody outside this project published the expected answer.

| Source | What it covers | Where |
|---|---|---|
| CCSDS 121.0-B-2 test data, from the SLS Data Compression working group | 72 coded bit streams over 35 sample files, every parameter set | `pkg/ldc/testdata/121B2TestData` |
| CCSDS 727.0-B-5 annex F | The CFDP checksum, over a 15-octet file sent in three segments | `pkg/cfdp/checksum_test.go` |
| RFC 5050 clause 4.1, reaffirmed by RFC 5326 clause 1.6 | The worked SDNV examples both DTN standards depend on | `pkg/sdnv/sdnv_test.go` |
| CCSDS 211.2-B-3 annex C | The Proximity-1 CRC-32, its polynomial, preset and syndrome behaviour | `pkg/pxsc/pxsc_test.go` |
| CCSDS 142.0-B-1 clause 3.5.2.1 | The first 40 digits of the pseudo-randomizer sequence | `internal/pn/pn_test.go`, `pkg/ocsc/ocsc_test.go` |

The 121.0 corpus is the strongest evidence in the repository. Every one of those 72 streams must encode byte-identically and decode back to the exact input samples, so the Rice coder is checked against an answer nobody here chose. One parameter set is deliberately not vendored: `ExtendedParameters` uses per-reference-interval byte alignment that this package does not implement, and [the LDC conformance page](/conformance/ldc) says so.

## Hand-derived vectors, and why they still help

For the rest, the tests pin the exact wire octets and show the arithmetic that produced them. From `pkg/usdl`:

```go
// Golden wire vectors, hand-computed from the CCSDS 732.1-B-3 clause 4.1.2 and
// clause 4.1.4 field layouts and checked with independent CRC implementations.
//
//	byte 0 = 1100_0000                        = 0xC0
//	byte 1 = SCID[11:4]                       = 0x4D
//	byte 2 = SCID[3:0] | S/D | VCID[5:3]      = 0x2D
```

Writing the derivation out is the point. A test that only checks a round trip proves the code agrees with itself, which is exactly the failure mode [the contributing guide](/docs/contribute) describes: three defects found in this repository were self-consistent and wrong.

Two variations are worth naming:

**Two independent computations.** `pkg/sdls` computes each protected frame twice, once through `ApplySecurity` and once from first principles with the Go standard library, then compares both against a pinned constant. A change to the authentication payload ordering or the IV placement fails loudly rather than round-tripping quietly.

**Vectors written specifically for past defects.** `pkg/sle/wirevectors_test.go` hand-encodes the ASN.1 for the four encodings an audit found broken, so a regression cannot hide behind a symmetric round trip.

## Fuzzing

Every decoder has a fuzz target: 57 of them across 20 packages. The property is that arbitrary octets never panic and never allocate from an attacker-controlled length field.

```bash
make fuzz-smoke
```

runs a short burst over the six frame and packet decoders that see untrusted input first. The other 51 targets run with `go test -fuzz`. See [security](/docs/reference/security) for the resource limits this pairs with.

## What "derived" means in a conformance table

The conformance pages mark a row derived when it was checked against the standard's prose rather than a published vector. That marker appears 22 times. Read it as: someone read the clause, wrote down what it requires, and the code does that. It is not the same as proof.

## The gap, stated plainly

Astro has never been tested against another implementation. No golden vectors have been exchanged with Yamcs, NASA cFS, ION, or any flight system. That is the single largest piece of missing assurance, and no amount of internal testing substitutes for it.

If you are running Astro against real hardware or another ground system, a bug report with the octets both sides produced is the most valuable thing you can send.

## Reference

- [Conformance index](/conformance), the clause-by-clause result for each package
- [Contributing](/docs/contribute), the rule about never coding from memory, and the testing conventions
- [Glossary](/docs/reference/glossary) | [Performance](/docs/reference/performance) | [Security](/docs/reference/security)
