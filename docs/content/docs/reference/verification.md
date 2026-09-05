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
| Test functions | 2014 |
| Wire test vectors | 400, across 32 packages |
| Fuzz targets | 77 |
| Benchmarks | 33 |
| Numbered PICS items | 500 |
| Statement coverage | 86.5% mean across 33 packages, 59.6% lowest |

## The vector corpus

The expected octets live in
[`vectors/`](https://github.com/ravisuhag/astro/tree/main/vectors) as JSON,
one directory per package. The Go tests read them; so can anything else.

That matters because a value locked inside a test in one language can only
check that language. A value in a data file can check any implementation,
and agreement between two independent implementations is the only thing
that catches a clause misread the same way twice.

Every vector carries the clause it comes from and the arithmetic that
produced it. A vector without a derivation does not load, and one without a
clause is marked `unverified` instead, which says plainly that agreeing with
it proves an implementation matches the corpus rather than the standard.
None of the 400 carry that marker today.
[`COVERAGE.md`](https://github.com/ravisuhag/astro/blob/main/vectors/COVERAGE.md)
explains the one case that did, until the clause was found, alongside what
the corpus does not cover at all.

[`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md)
is what a consumer needs: field dictionaries per package, the hex and
bit-order conventions, the error vocabulary, and the deliberate absences.
The test of it is that someone with no access to this source can run the
fixtures without asking a question.

```bash
make vectors
```

validates every fixture and runs in CI ahead of the tests. It fails on a
missing derivation, an error name outside the vocabulary, a duplicate
vector name, or a corpus path that does not exist.

## Published vectors, and where they come from

These are the cases where somebody outside this project published the expected answer.

| Source | What it covers | Where |
|---|---|---|
| CCSDS 121.0-B-2 test data, from the SLS Data Compression working group | 72 coded bit streams over 35 sample files, every parameter set | `vectors/ldc/corpus/` |
| CCSDS 727.0-B-5 annex F | The CFDP checksum, over a 15-octet file sent in three segments | `vectors/cfdp/wire.json` |
| RFC 5050 clause 4.1, reaffirmed by RFC 5326 clause 1.6 | The worked SDNV examples both DTN standards depend on | `vectors/sdnv/sdnv.json` |
| CCSDS 211.2-B-3 annex C | The Proximity-1 CRC-32, its polynomial, preset and syndrome behaviour | `vectors/crc/crc32.json` |
| CCSDS 142.0-B-1 clause 3.5.2.1 | The first 40 digits of the pseudo-randomizer sequence | `vectors/pn/sequences.json` |
| CCSDS 132.0-B-3 clause 4.1.4.6.2.2, CCSDS 732.1-B-3 annex H | The Only Idle Data fill sequence, published by two standards independently | `vectors/pn/sequences.json`, `vectors/usdl/frame.json` |
| RFC 4493 section 4, NIST SP 800-38B | The CMAC-AES128 and CMAC-AES256 example sets | `vectors/cmac/aes.json` |
| libfec / gr-satellites | The rate-1/2 convolutional code, in the convention deployed receivers use | `vectors/pxsc/convolutional.json` |

The 121.0 corpus is the strongest evidence in the repository. Every one of those 72 streams must encode byte-identically and decode back to the exact input samples, so the Rice coder is checked against an answer nobody here chose. One parameter set is deliberately not vendored: `ExtendedParameters` uses per-reference-interval byte alignment that this package does not implement, and [the LDC conformance page](/conformance/ldc) says so.

## Hand-derived vectors, and why they still help

For the rest, the vectors pin the exact wire octets and carry the arithmetic that produced them. From `vectors/usdl/frame.json`:

```json
{
  "name": "non-truncated-with-vcf-count-and-crc16",
  "clause": "4.1.2",
  "note": "TFVN '1100', SCID 1234 (0x04d2), source/dest 1, VCID 42, MAP ID 5 ... byte 0 = 1100 | scid[15:12] = 0xc0; byte 1 = scid[11:4] = 0x4d; byte 2 = scid[3:0]|S/D|vcid[5:3] = 0x2d ... FECF is CRC-16-CCITT over everything before it = 0x0e51, recomputed independently.",
  "want": "c04d2d4a0011020102000000deadbeef0e51"
}
```

The `note` is required, and that is the point. A test that only checks a round trip proves the code agrees with itself, which is exactly the failure mode [the contributing guide](/docs/contribute) describes: three defects found in this repository were self-consistent and wrong.

Two variations are worth naming:

**Two independent computations.** `pkg/sdls` computes each protected frame twice, once through `ApplySecurity` and once from first principles with the standard library, then compares both against the constant pinned in `vectors/sdls/protected-frame.json`. A change to the authentication payload ordering or the IV placement fails loudly rather than round-tripping quietly. The vector records the answer; that test records the derivation, so both stay.

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

- [The vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors) — `README.md` for the format, `CONTRACT.md` for consuming it, `COVERAGE.md` for what is and is not covered
- [Conformance index](/conformance), the clause-by-clause result for each package
- [Contributing](/docs/contribute), the rule about never coding from memory, and the testing conventions
- [Glossary](/docs/reference/glossary) | [Performance](/docs/reference/performance) | [Security](/docs/reference/security)
