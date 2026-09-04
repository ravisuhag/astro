---
title: Cross Support Transfer Service
short: CSTS
description: "PICS proforma: what this package implements, clause by clause."
order: 175
---

## Conformance Statement for `pkg/csts`, CCSDS 921.1-B-2

**Read this first.** Annex A1.1 of CCSDS 921.1-B-2 says the framework is not
meant to be implemented on its own: its prime intent is to provide a framework
for the specification of Cross Support Transfer *Services*, and a service
specification "will provide a specification for all elements that are left
abstract in this document". The annex ships a Requirements List with a blank
support column, which service specifications are expected to import and profile.

So conformance is properly claimed by a service — the Monitored Data service of
CCSDS 922.1-B-2, say — and not by the framework. What follows states which of
the framework's protocol elements this package encodes and decodes, which is
what a service built on it would need. It is not a claim to implement a CSTS.

---

## A1 IDENTIFICATION

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 04/09/2026 |
| ICS Serial Number | ASTRO-CSTS-ICS-001 |
| Implementation Name | astro/pkg/csts |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub, github.com/ravisuhag/astro |
| Specification | CCSDS 921.1-B-2 (Cross Support Transfer Service—Specification Framework, Blue Book, February 2021) |
| Underlying protocol | CCSDS 913.1-B-2 (ISP1), implemented by `pkg/sle` |
| Have any exceptions been required? | Yes [X] No [ ], see A3 |

---

## A2 REQUIREMENTS

### Framework protocol data unit, annex F3.15

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| The 20 CHOICE alternatives and their context tags | F3.15 | M | Y: all 20, pinned against the module |
| Implicit tagging — the context tag replaces the type's own | F3.15, and every module's `IMPLICIT TAGS` | M | Y: alternatives declared `X ::= StandardReturnHeader` carry no SEQUENCE inside the context tag |
| A tag outside the 20 | F3.15 | — | Y: refused, not passed along |
| A primitive alternative tag | F3.15 | — | Y: refused — every alternative is a SEQUENCE |

### Common operations, clause 3 and annex F3.4

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| Standard invocation header | 3.3.2, F3.3 | M | Y: credentials, invoke-id, procedure-name |
| Standard return header | 3.3.2, F3.3 | M | Y: credentials, invoke-id, the result CHOICE and its diagnostic |
| Standard acknowledge header | 3.3.2, F3.3 | M | Y: the same type as the return header, told apart by the PDU tag per the note under 3.3.1.3 |
| PEER-ABORT has no header | 3.3.1.1 | M | Y: the one exception, and `Header` reports none for it |
| The invoke-id is copied unchanged into a response | 3.3.2.4.2 | M | Y: carried; pairing invocations with responses is the caller's, as the association machine is |
| BIND, UNBIND, PEER-ABORT | 3.4–3.6, F3.5 | M | Y |
| START, STOP | 3.7, 3.8, F3.4 | M | Y |
| TRANSFER-DATA | 3.9, F3.4 | M | Y |
| PROCESS-DATA | 3.10, F3.4 | M | Y |
| NOTIFY | 3.11, F3.4 | M | Y |
| GET | 3.12, F3.4 | M | Y: the parameter list is carried encoded, see A3 |
| EXECUTE-DIRECTIVE | 3.13, F3.4 | M | P: the acknowledgement and return are the standard header; the invocation is carried as octets, see A3 |

### Common types, annex F3.3

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| `Credentials` CHOICE, and the SIZE (8..256) on the used alternative | F3.3 | M | Y: checked on encode as well as decode |
| `ProcedureName`: type, and the three-way role CHOICE | F3.3, 3.3.2.5 | M | Y |
| Secondary procedure instance number is `IntPos` | F3.3 | M | Y: zero refused |
| `Diagnostic`, four defined alternatives | F3.3 | M | Y |
| `Diagnostic` extension `[100]` | F3.3 | O | Y: carried as octets — its syntax is named by the procedure, not by this document |
| `Extended` CHOICE, `notUsed` default | F2.1, F3.3 | M | Y: written as `notUsed` |
| `AuthorityIdentifier` SIZE (3..16) | F3.3 | M | Y |
| `IdentifierString` excludes the blank | F3.3 | M | Y: `VisibleString (FROM (ALL EXCEPT " "))` |
| `IntUnsigned` 0..2³²−1, `IntPos` 1..2³²−1 | F3.3 | M | Y: Go's int64 holds values neither permits, so the range is checked |
| `ServiceInstanceIdentifier` | F3.2 | M | Y: three OIDs and an instance number |

### Object identifiers, annex F3.1

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| The framework arc, `{1 3 112 4 4 1 1}` | F3.1 | M | Y |
| The 18 operation identifiers | F3.1 | M | Y |
| The 7 procedure types and 3 derived procedures | F3.1 | M | Y: a derived procedure sits under the one it derives from, not beside it |
| A procedure type outside the framework's own | F3.1 | O | Y: carried and reported by OID, not named — see A3 |

### PEER-ABORT diagnostic partition, annex F3.5

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| The 12 Association Control values, 40–51 | 3.6.2.2, F3.5 | M | Y |
| `forwardBufferTooLarge` (70) | F3.5 | O | Y |
| `otherReason` (126) | F3.5 | M | Y |
| Values allocated by SLE, ISP1 and the application | F3.5 | — | Y: the origin is reported; the value is not named, see A3 |

---

## A3 EXCEPTIONS AND UNSUPPORTED FEATURES

**Conformance belongs to a service, not to this package.** Annex A1.1 says so
directly, and it is the first thing on this page for that reason. This package
is a codec for the framework's protocol elements; a CSTS is what a service
specification defines on top of them.

**The procedures are not state machines here.** Section 4 specifies twelve
procedures with their own states, timers and rules. This package reads and
writes their messages and runs none of them. That is the same split `pkg/sle`
makes — pure codecs, and an association machine the caller pumps — and for the
same reason: a library that owned timers would own a scheduling policy the
standard does not specify.

**Three PDU alternatives are carried as octets.** The EXECUTE-DIRECTIVE
invocation, whose directive qualifier is a four-way CHOICE over
SANA-registered identifiers; and the forward and return buffers, which belong
to the Buffered Data Processing and Buffered Data Delivery procedures rather
than to the common operations of annex F3.4. Their octets are kept and
re-encode unchanged, so nothing is lost by decoding one.

**Several fields are carried as encoded octets.** A `Time`, a `Name`, an
`EventValue`, a `ListOfParametersEvents`, a `TypeAndValue`. Each is built from
identifiers registered with SANA rather than fixed by this document. A Go type
for them would be a type for a registry that changes without this package, and
the registry is where their meaning lives.

**Values outside the framework's own registrations are not named.** A procedure
type a service defines comes back as an OID; a PEER-ABORT diagnostic in the
application range comes back with its origin and its number. Naming either
would mean guessing which service is on the other end, which is exactly what
`pkg/sle` refuses to do when it demands `--service`.

**Credentials are carried, not computed.** Annex F3.3 says the structure
depends on the algorithm and does not specify it; clause 2.6 says an
implementation over ISP1 uses that document's algorithm. So the octets are the
ones `pkg/sle` builds, and a caller that needs them reaches for that package.

**The extension mechanism is written as `notUsed`.** Clause F2.1 makes that the
value when the capability is unused, and every extension point in the framework
is defined by a procedure or a service rather than here. An extension that
arrives on decode is not refused; it is read past.

**The vectors are derived, not published.** CCSDS 921.1-B-2 prints no worked
example and no octets — it is an abstract specification with an ASN.1 annex.
Each vector carries its derivation from annex F octet by octet in its note, so
the derivation can be checked against the module rather than against this
package. That is a weaker footing than the navigation packages have, where
annex G prints files, and it is stated rather than glossed over.

---

## A4 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| PDU alternatives | 20 | Annex F3.15 |
| Credentials length | 8 to 256 octets | Annex F3.3 SIZE constraint |
| Authority identifier | 3 to 16 characters, no blanks | Annex F3.3 |
| Logical port name | 1 to 128 characters, no blanks | Annex F3.3 |
| Appellation | 1 to 128 characters | Annex F3.3 |
| Integer range | 0 to 2³²−1, or 1 to 2³²−1 for `IntPos` | Annex F3.3 |
| PEER-ABORT diagnostic | 1 octet | Annex F3.5, which notes ISP1 allows no more |
| BER length ceiling | 16 MiB by default | `internal/ber`, shared with `pkg/sle` |

---

## Wire test vectors

The files backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/csts) — 10 vectors, 5 encode and 5 decode.

| File | |
|---|---|
| [`csts/framework.json`](https://github.com/ravisuhag/astro/blob/main/vectors/csts/framework.json) | 10 vectors |

These are **derived, not published**, and the corpus note says so. CCSDS 921.1-B-2 prints no octets at all. Each vector's note carries its derivation from annex F octet by octet, which is the most a reader can be given: it lets the derivation be checked against the module instead of against this package.

What they pin hardest is the implicit tagging. A CHOICE alternative's context tag replaces the tag of the type beneath it, so an alternative declared `X ::= StandardReturnHeader` encodes with no SEQUENCE inside its context tag. Writing one there as well produces a PDU one level too deep — which round-trips perfectly against itself, and against nothing else.

See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how to consume these, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
