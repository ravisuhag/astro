---
title: NDM Combined Instantiation
short: NDM
description: "Coverage matrix: what this package implements, clause by clause."
order: 226
---

## Conformance Statement for `pkg/ndm`, CCSDS 505.0-B-3 clause 4.11

CCSDS 505.0-B-3 ships no Implementation Conformance Statement proforma. What
follows takes the shape the navigation standards use, applied to clause 4.11,
the NDM combined instantiation.

This package implements that clause and nothing else in the document. The rest
of CCSDS 505.0-B-3 — the element naming, the header and body structure, the
units attribute — is implemented by `pkg/odm`, `pkg/adm`, `pkg/tdm` and
`pkg/cdm`, each in its own conformance statement, because it is what their
own standards' XML sections point at.

---

## A1 IDENTIFICATION

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 04/09/2026 |
| ICS Serial Number | ASTRO-NDM-ICS-001 |
| Implementation Name | astro/pkg/ndm |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub, github.com/ravisuhag/astro |
| Specification | CCSDS 505.0-B-3 (XML Specification for Navigation Data Messages, Blue Book, May 2023) |
| Also | CCSDS 502.0-B-3 clause 8.12 and CCSDS 504.0-B-2 clause 7.8, which repeat the rules for their own messages |
| Have any exceptions been required? | Yes [X] No [ ], see A3 |

---

## A2 REQUIREMENTS

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| `<ndm></ndm>` root element | 4.11.3 | M | Y |
| Standard attributes on the root | 4.11.4 | M | Y: `xmlns:xsi`, `xmlns:ndm`, `xsi:noNamespaceSchemaLocation` |
| Neither `id` nor `version` on the root | 4.11.4 | M | Y: a root carrying either is refused |
| Only `id` and `version` on a constituent tag | 4.11.5 | M | Y: written that way, and a location found on a constituent is discarded in favour of the root's |
| Constituents from table 3-1 | 4.11.6 | M | P: the nine messages this repository implements, see A3 |
| Any combination of message types | 4.11.7 | M | Y: the standards may be mixed in one file |
| At least one constituent message | 4.11.8 | O | Y: a `should`, so an empty file is read rather than refused |
| `<COMMENT>` elements under the root | 4.11.9, figure 4-2 | O | Y |
| Constituents validated as whole messages | 4.11.5, and each message's own standard | M | Y: each is handed to its own package's decoder |
| Schema instance namespace, exactly as given | 4.3.3 | M | Y: `http`, not `https` — the string names a namespace |
| Schema location as one unbroken string | 4.3.6 | O | Y |

---

## A3 EXCEPTIONS AND UNSUPPORTED FEATURES

**The Re-entry Data Message is not a constituent this package reads.** Clause
4.11.6 draws the constituents from table 3-1, which lists the RDM of
CCSDS 508.1 alongside the nine implemented here. That standard has no package
in this repository, so a file carrying an `<rdm>` is refused outright with
`ErrUnknownMessageType` rather than half-read. A combined file is all-or-
nothing on purpose: returning the eight messages a reader understood and
silently dropping the ninth would misrepresent the file.

**The qualified schema set is not used.** Clause 4.3.5 offers `qualified` and
`unqualified` element forms and clause 4.3.6 a schema location for each. This
package writes the unqualified form, which every worked example in every
navigation standard uses. An instantiation that arrives qualified is not
rejected on that ground — the decoder does not look at namespace prefixes —
but re-encoding it produces the unqualified form.

**One file names one master schema.** Clause 4.11.4 puts a single
`xsi:noNamespaceSchemaLocation` on the root, and each navigation standard names
a different master: CCSDS 502.0-B-3 gives `3.0` and CCSDS 504.0-B-2 gives `4.0`.
A file mixing their messages can only name one, and the documents do not settle
which — figure 7-3 of CCSDS 504.0-B-2 writes the `4.0` master over a file of
ADM messages and its own figure G-12 writes the `3.0` master over another. This
package carries whatever the file had and defaults a new one to the schema its
first message names. A caller who needs a different one sets `Combined.Schema`.

**There is no key-value equivalent.** Aggregation is defined for XML only.
Clause 5.2.2 of CCSDS 504.0-B-2 says a sequence of ACMs "may be aggregated into
a single Navigation Data Message (NDM) XML file", and neither standard defines
a way to do it in the `keyword = value` notation. Nothing is missing here; there
is nothing to implement.

**A constituent is re-serialised on the way in and out.** Each message is
written as a document of its own and handed to its package's decoder, and back
again on encode. That costs a serialise and a parse per message. It is
deliberate: it means there is one decoder per message type rather than two, so
a message cannot be accepted inside a combined file and refused outside it.

---

## A4 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| Constituents per file | bounded by the input | Clause 4.11.2 says "any number"; no ceiling is imposed |
| Constituent message types | 9 | `opm`, `omm`, `oem`, `ocm`, `apm`, `aem`, `acm`, `tdm`, `cdm` |
| Nesting | one level | A combined instantiation may not contain another; clause 4.11.6 draws constituents from table 3-1, which does not list `<ndm>` |
| Whole file in memory | yes | Read into memory rather than streamed, as every other navigation decoder here is |

---

## Wire test vectors

The files backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/ndm) — 2 decode vectors and 2 corpus files.

| File | |
|---|---|
| [`ndm/combined.json`](https://github.com/ravisuhag/astro/blob/main/vectors/ndm/combined.json) | 2 vectors |
| `ndm/combined-omm.xml` | figure G-21 of CCSDS 502.0-B-3, published text |
| `ndm/combined-mixed.xml` | an orbit and an attitude message in one file — **derived**, not published |

The first is published text: figure G-21 of CCSDS 502.0-B-3 prints it. The second is this package's own output, and is marked as derived in the corpus note, because no figure in either standard prints a file that mixes the standards even though clause 4.11.7 allows one.

What the vectors assert is the structure the wrapping adds: how many messages a file holds, of what types, in what order, and which master schema the root names. The constituents themselves are asserted by the vectors of `pkg/odm`, `pkg/adm`, `pkg/tdm` and `pkg/cdm` — a message inside a combined file is the same message, read by the same decoder.

See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how to consume these, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
