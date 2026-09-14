---
title: How this is verified
short: Verification
description: Which claims rest on a published test vector, which on another implementation's octets, and which on a reading of the standard.
order: 2
---

A [conformance statement](/conformance) says what a package implements. This page says how much to trust it, which is a different question and the one worth asking first.

## The short version

Astro runs every published test vector it can find. There are not many. Some of the rest were captured from other implementations of the same standards. Everywhere else, the expected octets were worked out by hand from the field layouts in the standard.

That distinction matters more than any coverage number. A hand-derived vector catches a misread clause. It cannot catch a clause that was misread the same way twice, by the person writing the encoder and the person writing the test. Only another implementation catches that, which is what the captured vectors are for, and they reach eleven of the thirty-two packages.

## What the tests are

| | |
|---|---|
| Test functions | 2111 |
| Wire test vectors | 412, across 32 packages, of which 57 came from other implementations |
| Fuzz targets | 84 |
| Benchmarks | 38 |
| Numbered PICS items | 540 |
| Statement coverage | 87.4% mean across 33 packages, 72.9% lowest |

A numbered PICS item is a distinct identifier — `PREFIX-N` (`SLE-40`), or the
standard's own terser `RN` / `PN` form where a page transcribes it directly
(`rhc.md`'s `R1`, `P1`) — named anywhere in the Item column of a
[conformance page](/conformance). It counts once per page no matter how
many tables on that page mention it: a primary table, a "Non-Conformances"
list and a "Fully Supported Items" summary can all name the same
requirement, and it still counts once. A page whose Item column holds prose
rather than an identifier, such as `tcf.md`'s "Not Implemented" table,
contributes zero. `go test ./internal/conformance/...` recomputes this
number from the pages themselves and fails if it disagrees.

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
None of the 412 carry that marker today.
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

## Vectors captured from other implementations

These are octets produced by somebody else's code from the same standards, then pinned here. They are the only thing in the corpus that can catch a clause this project and its own tests misread the same way.

| Source | What it covers | Vectors |
|---|---|--:|
| [spacepackets](https://pypi.org/project/spacepackets/) 0.32.0, a Python implementation of the packet and frame formats | Space Packet headers, TM and USLP transfer frames, PUS TC headers, CFDP PDUs | 33 |
| [Yamcs](https://yamcs.org) 5.13.5, a mission control system | CLTU codeblocks and their BCH parity, the CCSDS CRC-16, the Proximity-1 CRC-32, and both randomizers | 13 |
| [dtn7-go](https://github.com/dtn7/dtn7-go) v0.10.2 | BPv7 bundles: both endpoint schemes, both CRC algorithms, extension blocks, a status report | 6 |
| ION-DTN, NASA/JPL's reference implementation, captured off the wire | LTP data segments, red and green, with the checkpoint and end-of-block flags | 5 |

Each lives in that package's `interop.json`, and every vector records the fields that were asked of the other implementation, so the capture can be reproduced. The capture programs are not committed: they need dependencies astro does not take.

What made each capture possible was shape rather than effort. dtn7-go hands you a bundle as a value with a marshal method, spacepackets does the same for packets and frames, and Yamcs exposes its BCH generator, randomizer and CRC calculators as plain classes you can call without starting a server. Each of those was a short program.

`pkg/ltp` is the exception and the one worth reading about. ION exposes no callable codec, so the octets came from building it in a container, driving it over a UDP link service, and recording the 8359 segments that arrived; five are kept as representatives. [`COVERAGE.md`](https://github.com/ravisuhag/astro/blob/main/vectors/COVERAGE.md) tells that story in full.

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

Every decoder has a fuzz target: 84 of them across 32 packages. The property is that arbitrary octets never panic and never allocate from an attacker-controlled length field.

```bash
make fuzz-smoke
```

runs a short burst, 15 seconds each, over 74 of them: every frame and packet decoder, and nearly everything else that parses untrusted octets. The remaining 10 are extra targets in `pkg/pus`, `pkg/pxsc` and `pkg/xtce`, and run with `go test -fuzz`. See [security](/docs/reference/security) for the resource limits this pairs with.

## What "derived" means in a conformance table

The conformance pages mark something derived when it was checked against the standard's prose rather than against octets somebody else published. Read it as: someone read the clause, wrote down what it requires, and the code does that. It is not the same as proof.

Most of the corpus is derived, so the marker is used where the distinction is easy to miss rather than on every row. Each package's "Wire test vectors" section says which of the two its own octets are — `pkg/csts` and `pkg/rhc` are the two that spell it out at length, because in both cases the standard publishes nothing to check against at all.

## The gap, stated plainly

Astro has never been run against a flight system, and no vectors have been exchanged with a mission. The captured vectors above are the next best thing and not the same thing: octets taken from four other implementations, checked offline, on the layers those implementations happen to expose.

Three packages have no outside corroboration at all and rest on clause derivation alone: `pkg/cop`, `pkg/epp` and `pkg/sdls`. `pkg/ocsc` is a fourth in practice, because most of it is bit-string work that an octet vector cannot express. `pkg/sle` and `pkg/csts` are derived throughout as well, for the reason [the roadmap](https://github.com/ravisuhag/astro/blob/main/ROADMAP.md) gives: no independent implementation of either is reachable.

So the honest summary is that the wire formats most missions depend on now agree with somebody else's reading, and the rest still rest on one reading of the clause.

If you are running Astro against real hardware or another ground system, a bug report with the octets both sides produced is the most valuable thing you can send.

## Reference

- [The vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors) — `README.md` for the format, `CONTRACT.md` for consuming it, `COVERAGE.md` for what is and is not covered
- [Conformance index](/conformance), the clause-by-clause result for each package
- [Contributing](/docs/contribute), the rule about never coding from memory, and the testing conventions
- [Glossary](/docs/reference/glossary) | [Performance](/docs/reference/performance) | [Security](/docs/reference/security)
