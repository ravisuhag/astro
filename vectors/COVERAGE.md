# Coverage

What the vector corpus covers, what it does not, and which values are not
established against a standard. Read this before relying on the corpus as
evidence of conformance.

## What is covered

| Package | Standard | Vectors | Corpus files |
|---|---|--:|--:|
| `aos` | CCSDS 732.0-B-4 | 12 | — |
| `bp` | CCSDS 734.2-B-1 / RFC 5050 | 5 | — |
| `cfdp` | CCSDS 727.0-B-5 | 14 | — |
| `cmac` | RFC 4493 (CMAC-AES128), NIST SP 800-38B (CMAC-AES256) | 8 | — |
| `cop` | CCSDS 232.0-B-4 (CLCW), CCSDS 232.1-B-2 (FARM-1) | 29 | — |
| `crc` | CCSDS 132.0-B-3 clause 4.1.6 (CRC-16-CCITT) | 9 | — |
| `epp` | CCSDS 133.1-B-3 | 16 | — |
| `ldc` | CCSDS 121.0-B-3 | — | 107 |
| `ltp` | CCSDS 734.1-B-1 / RFC 5326 | 10 | — |
| `ocsc` | CCSDS 142.0-B-1 | 1 | — |
| `pn` | CCSDS 131.0-B-5 clause 10.4.2 (TM), CCSDS 231.0-B-4 clause 6.2 (TC), CCSDS 132.0-B-3 clause 4.1.4.6.2 (OID) | 3 | — |
| `pus` | ECSS-E-ST-70-41C (PUS-C) | 10 | — |
| `pxsc` | CCSDS 211.2-B-3 (Proximity-1 coding and synchronization), code per CCSDS 131.0 | 4 | — |
| `sdls` | CCSDS 355.0-B-2 | 7 | — |
| `sdnv` | RFC 5050 clause 4.1 (SDNV), adopted by CCSDS 734.1-B-1 via RFC 5326 clause 1.6 item 20 | 17 | — |
| `sle` | ITU-T X.690 (BER), CCSDS 913.1-B-2 (SLE ISP1 TML) | 23 | — |
| `spp` | CCSDS 133.0-B-2 | 28 | — |
| `tcf` | CCSDS 301.0-B-4 | 14 | — |
| `tcsc` | CCSDS 231.0-B-4 | 7 | — |
| `tmdl` | CCSDS 132.0-B-3 | 21 | — |
| `tmsc` | CCSDS 131.0-B-5 | 7 | — |
| `usdl` | CCSDS 732.1-B-3 | 12 | — |
| `xtce` | CCSDS 660.0-B-2 (XTCE) | — | 8 |
| **Total** | | **257** | **115** |

257 vectors and 115 referenced corpus files across 23 packages.
Every value is traced to a clause or a published corpus; none is marked unverified.

## What is not covered

**State machines, partly.** COP-1 has `sequence` vectors on both halves.
`cop/farm1.json` runs the receiving machine against state table 6-1:
acceptance, the sliding window, lockout and recovery, the control
commands, and the modulo-256 wrap. `cop/fop1.json` runs the sending
side's retransmission rules from clause 5.1.10 — the timer, the
transmission count and limit, and the alert-or-suspend branch.

Two limits inside COP-1. FOP-1's full state table 5-1 is six states
against about twenty-five events and is not covered; only what clause
5.1.10 states in prose is. And FARM-1's Wait state is untested, because
reaching it needs a buffer-availability signal that clause 6.3.2.3
leaves optional, so a vector requiring it would pin an implementation
choice rather than the standard.

The other machines are still uncovered: the CFDP transaction engines,
the LTP session and receiver, SLE association, bundle reassembly and
custody transfer, and frame multiplexing and flush ordering. The form
they need now has two worked examples to follow.

Within FARM-1, the Wait state is deliberately untested: reaching it
needs a buffer-availability signal that clause 6.3.2.3 leaves optional,
so a vector requiring it would pin an implementation choice rather than
the standard.

**Layers with nothing to pin as octets.** Some layers a full stack needs
are absent here, and deliberately so — what they define is not an octet
string a vector can carry:

| Layer | Why |
|---|---|
| TC data link (CCSDS 232.0-B-4) | its framing is covered by `tcsc` and `cop`; the layer itself contributes inputs rather than expected octets |
| Proximity-1 data link (CCSDS 211.0-B-6) | defines bit positions within fields rather than whole-field encodings |
| Robust header compression (CCSDS 123.0) | defines mask positions rather than encoded streams |

Channel multiplexing has no wire format of its own and so has nothing to
pin either.

**Bit-string layers.** Most of `ocsc` works on bit strings — bit-level
lengths, termination digits, sequence indicators — which an octet-string
format cannot express. Only its sync marker is pinned.

**Expected parse trees.** The XTCE documents in `xtce/` are shared
inputs, but what a parser should produce from them is a tree, not an octet
string. No vector kind expresses that, so only load-or-refuse behaviour is
checkable from the corpus.

**Civil-time conversion.** The `tcf` vectors pin P-field and T-field
packing from explicit field values. They do not cover conversion from a
civil timestamp: Level 1 CUC applies a TAI-UTC offset from a leap-second
table, and pinning octets through that path would fix the table rather
than the standard's layout. Treat leap-second handling as outside the
corpus.

**SLE operation encodings.** The BER primitives and TML framing are
pinned; the operation encodings above them are not. Several
GET-PARAMETER alternatives have no published vectors to test a typed
shape against, which is the blocker.

## Values not established against a standard

None. Every vector cites a clause or a published corpus.

The three `aos` frame header error control vectors were carried as
`"source": "unverified"` because the RS(10,6) arithmetic was reproducible
but the choice of information symbols was not established. Clause
4.1.2.6.5 f) of CCSDS 732.0-B-4 settles it: the bit-to-symbol mapping
makes the six information symbols header bits 0-15 and 40-47, leaving the
24-bit virtual channel frame count unprotected. Encoding on that basis
reproduces `ce8e`, so the vectors now cite the clause.

## Field-width rules

Every field narrower than the type that carries it has a reject vector.
This matters more than it sounds: these fields are packed by shifting, so
an unchecked out-of-range value is not dropped but *substituted* — a 6-bit
virtual channel identifier given 64 goes out as 0. For a routing or
addressing field that means a frame delivered to somewhere the caller did
not name, with nothing on the wire to show it happened.

Pinned across `aos`, `cop`, `epp`, `pus`, `spp`, `tmdl` and `usdl`:
virtual channel identifiers, MAP IDs, APIDs, sequence counts and flags,
frame counts and cycles, FARM-B counters, status fields, and every
protocol version field.

## What these values rest on

The clause is the authority and the vector is evidence of it. Nothing
below changes that, and no implementation's agreement adds to it.

Every octet here has been reproduced from its cited clause by a second
implementation, and separately put to implementations that have never
seen this corpus. Nothing disagreed. That is worth knowing once and is
not worth tracking per package: it says the derivations are sound, not
that the clauses were read correctly.

Three limits remain, and they are properties of the corpus rather than of
any one check.

**A derivation cannot catch a clause misread twice.** Where a value comes
from a published corpus or a worked example the standard prints, that
risk is gone — the CCSDS 121.0 data set, the annex F checksum, the RFC
5050 SDNV examples, the RFC 4493 and NIST CMAC sets, the published
randomizer sequences. Everywhere else it stands.

**Reject vectors carry weaker evidence than encode vectors, and always
will.** Another implementation confirms octets by producing them. It can
only confirm a rejection by refusing something, and implementations are
routinely more permissive than the standard they implement — accepting an
out-of-range identifier rather than refusing it. A rejection rule can be
corroborated by reading the clause and in almost no other way.

**The citations are audited structurally, not semantically.** Every
vector names a clause. 239 of the 249 citations have been checked against
the document they name, at the revision they name, and every one exists
as a real heading. The ten not checked are ECSS-E-ST-70-41C, which is not
freely available.

That check says the citation points somewhere real. It does not say the
clause means what the `note` claims. The clauses read in full so far are
the ones a defect was found in or that settled a value:

| Document | Clauses read | What it settled |
|---|---|---|
| RFC 5050 | 4.1, 6.1 | the SDNV rule; the administrative record layout |
| RFC 5326 | 3.1, 3.1.2 | the segment header; the segment type codes |
| CCSDS 732.0-B-4 | 4.1.2.6, 4.1.2.6.5 | the frame header error control, which had been unverified |
| CCSDS 232.1-B-2 | 5.1.2, 5.1.10, 6.1-6.3 | the FOP-1 and FARM-1 rules the `sequence` vectors run |

Reading the rest in full is the remaining work. Three citation errors
have been found and fixed this way, which is the argument for finishing
it.
