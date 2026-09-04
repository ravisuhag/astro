# Coverage

What the vector corpus covers, what it does not, and which values are not
established against a standard. Read this before relying on the corpus as
evidence of conformance.

## What is covered

| Package | Standard | Vectors | Corpus files |
|---|---|--:|--:|
| `aos` | CCSDS 732.0-B-4 | 12 | — |
| `bp` | RFC 9171 (BPv7), with ipn scheme per RFC 9758 | 24 | — |
| `bpsec` | RFC 9172 (BPSec), with default security contexts per RFC 9173 | 13 | — |
| `cfdp` | CCSDS 727.0-B-5 | 25 | — |
| `cmac` | RFC 4493 (CMAC-AES128), NIST SP 800-38B (CMAC-AES256) | 8 | — |
| `cop` | CCSDS 232.0-B-4 (CLCW), CCSDS 232.1-B-2 (FARM-1) | 29 | — |
| `crc` | CCSDS 132.0-B-3 clause 4.1.6 (CRC-16-CCITT) | 13 | — |
| `epp` | CCSDS 133.1-B-3 | 16 | — |
| `keywrap` | RFC 3394 (AES Key Wrap) | 6 | — |
| `ldc` | CCSDS 121.0-B-3 | — | 107 |
| `ltp` | CCSDS 734.1-B-1 / RFC 5326 | 19 | — |
| `ocsc` | CCSDS 142.0-B-1 | 1 | — |
| `odm` | CCSDS 502.0-B-3 (OPM, OMM, OEM) | 6 | 6 |
| `pn` | CCSDS 131.0-B-5 clause 10.4.2 (TM), CCSDS 231.0-B-4 clause 6.2 (TC), CCSDS 132.0-B-3 clause 4.1.4.6.2 (OID) | 4 | — |
| `pus` | ECSS-E-ST-70-41C (PUS-C) | 13 | — |
| `pxsc` | CCSDS 211.2-B-3 (Proximity-1 coding and synchronization), code per CCSDS 131.0 | 6 | — |
| `sdls` | CCSDS 355.0-B-2 | 7 | — |
| `sdnv` | RFC 5050 clause 4.1 (SDNV), adopted by CCSDS 734.1-B-1 via RFC 5326 clause 1.6 item 20 | 19 | — |
| `sle` | ITU-T X.690 (BER), CCSDS 913.1-B-2 (SLE ISP1 TML) | 29 | — |
| `spp` | CCSDS 133.0-B-2 | 35 | — |
| `stack` | CCSDS 132.0-B-3 composed with CCSDS 131.0-B-5 | 5 | — |
| `tcf` | CCSDS 301.0-B-4 | 14 | — |
| `tcsc` | CCSDS 231.0-B-4 | 13 | — |
| `tdm` | CCSDS 503.0-B-2 | 1 | 1 |
| `tmdl` | CCSDS 132.0-B-3 | 29 | — |
| `tmsc` | CCSDS 131.0-B-5 | 7 | — |
| `usdl` | CCSDS 732.1-B-3 | 20 | — |
| `xtce` | CCSDS 660.0-B-2 (XTCE) | — | 8 |
| **Total** | | **374** | **122** |

374 vectors and 122 referenced corpus files across 28 packages.
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

**Every state machine in the library is now covered.** `cop`, `ltp`,
`cfdp`, `sle` and `stack`, 29 sequence vectors across the five. All of it
was added once the runner existed. Before that, sequence vectors were data no test read:
`internal/vectors` skipped every one of them with "no runner is wired for
them", so COP-1's ten sat in the corpus and in this table's counts
without ever running against `pkg/cop`. They run now, and so do the rest.

`ltp/session.json` drives a sender and receiver across a link that drops
segments: the clean run, a lost data segment recovering with no timer,
the lost *checkpoint* that recovers with nothing until the caller resends
it, and a green part that is never retransmitted.

`cfdp/transaction.json` drives a file transfer the same way. The pair of
class 1 runs is the point: the same dropped PDU is silently lost in
class 1 and recovered by a NAK in class 2, so the difference a caller
chooses between is a fact the corpus states rather than prose. A fourth
run alters a PDU without changing its length or offset, which no length
check can catch and only the end-of-file checksum does.

`sle/association.json` drives a user and a provider through the bind and
unbind handshakes, a provider refused permission to send BIND, and the
heartbeat stepped one second at a time across both its deadlines. Two
rules it pins were written down only after the vectors disagreed with a
first reading of them: UNBIND closes the association rather than
returning it to unbound, and the dead-peer check is strict, so a peer is
still alive at exactly dead_factor intervals and gone one second later.

`stack/downlink.json` composes a sender and receiver from one
configuration and pins the ordering the multiplexer imposes: a frame
count per channel, a flush that emits in channel order on every run
rather than in Go's randomised map order, and what Priority actually
means. That last one also corrected a first reading — it is a
weighted round-robin share, not a rank, so a higher-weighted channel
neither drains first nor starves the others.

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

**A derivation cannot catch a clause misread twice.** Two things remove
that risk, and they are not the same thing.

*Published data* — the CCSDS 121.0 data set, the annex F checksum, the
RFC 5050 SDNV examples, the RFC 4493 and NIST CMAC sets, the published
randomizer sequences, and the bundle encodings of RFC 9173 appendix A.
The standards body settles these.

*Another implementation's octets*, which is stronger, because it is the
only thing that catches a clause two readers misread the same way. Four
files hold them:

| File | Source | Covers |
|---|---|--:|
| `bp/interop.json` | dtn7-go 0.10.2 | 6 |
| `spp/interop.json` | spacepackets 0.32.0 | 7 |
| `tmdl/interop.json` | spacepackets 0.32.0 | 8 |
| `usdl/interop.json` | spacepackets 0.32.0 | 8 |
| `pus/interop.json` | spacepackets 0.32.0 | 3 |
| `cfdp/interop.json` | spacepackets 0.32.0 | 7 |
| `tcsc/interop.json` | Yamcs 5.13.5 | 6 |
| `crc/interop.json` | Yamcs 5.13.5 | 4 |
| `pxsc/interop.json` | Yamcs 5.13.5 | 2 |
| `pn/interop.json` | Yamcs 5.13.5 | 1 |
| `ltp/interop.json` | ION-DTN, off the wire | 5 |
| `pxsc/convolutional.json` | a deployed CCSDS 171/133 realization | 4 |

That is 61 vectors across eleven packages, and it covers the layers where a
shared misreading is likeliest — the places where a mistake produces an
implementation that agrees with itself perfectly:

- **The TC uplink coding.** `tcsc` had no outside check at all before
  Yamcs. A BCH(63,56) parity octet, the complement applied to it, and the
  0x55 fill padding a short codeblock are three such places. The start
  sequence 0xeb90 is a fourth, and the worst of them: it does not survive
  text extraction from the standard, which yields a pattern reading FF00
  from a page that plainly shows EB90, so a vector taken from the
  extracted text would have condemned a correct implementation.
- **The pseudo-randomizer.** Plan 026 found astro generating the TM
  sequence from the wrong feedback taps. Every randomized frame was
  unreadable by a conforming receiver and no round trip could tell,
  because XOR is self-inverse. The published digits caught it; Yamcs
  agreeing means two sources rather than one reading of one table.

- **The two frame headers.** TM packs twelve fields into six octets with
  only two octet-aligned; USLP's header is variable length because a
  three-bit field declares how many octets of frame count follow it. A
  boundary off by one and a header assumed fixed both round-trip.
- **CFDP's transmission mode bit**, which table 5-1 inverts: '0' means
  acknowledged. An implementation using the obvious sense marks every
  acknowledged transaction unacknowledged.
- **PUS**, for the reason below.

The `pus` three matter out of proportion to their number.
ECSS-E-ST-70-41C is behind registration, so every other `pus` vector
rests on a reading of clauses this project cannot publish, and its ten
citations are the only ones in this corpus never audited against the
document. An independent implementation agreeing on the octets is the
strongest check available on that layer.

Everywhere else the derivation risk stands.

**Three layers cannot be corroborated from outside at all, and the reason
is structural.** `cop`, `epp` and `sdls` rest on clause derivation alone.
`ocsc` is a fourth in practice, though for a different reason: most of it
is bit-string work an octet format cannot express, so there is nothing to
compare even if an implementation were reachable.

`ltp` used to be on this list and came off it the hard way. ION exposes
no callable codec — its serializers are static functions reaching into an
SDR database — so the octets came from building ION in a container,
driving it with ltpdriver over a UDP link service, and recording what
arrived at the far end. 8359 segments, of which five are kept as
representatives of the four types that transmission produced. astro
decodes each and re-encodes it identically.

That is worth stating plainly for anyone attempting the same on the
remaining three: it is not a library call, it is a running system, and it
took a Docker image, three config files and a listener. It is possible.
It is just not cheap.

`bp` used to be the sixth, and no longer is — twice over.

First, replacing Bundle Protocol version 6 with version 7 brought
RFC 9173 appendix A into reach: it prints four worked example bundles in
CBOR diagram notation beside their hex, and says plainly they can inform
unit and interoperability test suites. Four `bp` vectors are those
published octets.

Second, `bp/interop.json` holds six bundles captured from **dtn7-go**, a
version 7 implementation written from RFC 9171 by other authors. astro
decodes every one and re-encodes it to the identical octets. That covers
both CRC algorithms, both endpoint schemes, the checksum-over-its-own-
zeroed-field rule, the indefinite-length bundle array, and the
double-wrapped extension data — each of them a place where a wrong
implementation agrees with itself perfectly.

The X-25 CRC-16 is the sharpest case. It is not the CCSDS CRC-16 that
`pkg/crc` computes, so `pkg/bp` carries its own, and a wrong one would
pass every round trip. A separate implementation computing the same
0x4893, 0x2987 and 0x5114 over the same three blocks is what settles
it.

Worth noting how it came about. The corroboration was not the reason for
the change; the reason was that nothing runs version 6. It came free with
picking the version people actually deploy, which is the general lesson:
a live standard has a published record around it, and a dead one does
not.

What usually makes the difference is shape rather than effort. dtn7-go
exposes a bundle as a value with a marshal method, spacepackets does the
same for packets and frames, and Yamcs — a whole mission control system —
turns out to expose its BCH generator, randomizer and CRC calculators as
plain classes callable without a server. Each of those captures was a
short program.

ION is the counter-example that shows the shape is not the whole story.
It exposes nothing callable, and its octets were still obtainable by
running it and watching the link. So the remaining three are not
impossible, only expensive: `cop`, `epp` and `sdls` would each need a
peer implementation configured to exchange with a test harness, and the
managed parameters agreed on both sides before a single octet matches.

Corroboration works where an implementation exposes a *codec*: a
function from field values to octets, callable in isolation. Where the
only implementations expose a protocol *engine* instead -- a daemon with
its own database, a whole self-consistent frame with matching managed
parameters, a server component that reads a stream -- there is nothing
to call. Their state lives across a session rather than in a return
value.

That is why these four are the gap and the others are not, and why
closing it means standing up a running system rather than installing a
library. Anyone attempting it should know that before starting.

**Reject vectors carry weaker evidence than encode vectors, and always
will.** Another implementation confirms octets by producing them. It can
only confirm a rejection by refusing something, and implementations are
routinely more permissive than the standard they implement — accepting an
out-of-range identifier rather than refusing it. A rejection rule can be
corroborated by reading the clause and in almost no other way.

**The citations are audited.** Every vector names a clause. Each cited
clause has been read against what its vectors claim, across 20 documents
covering all but the ten ECSS-E-ST-70-41C citations, which are not
freely available.

Nineteen citations pointed at the wrong clause and are corrected. The
values were right in every case; it was the pointers that were wrong,
which is precisely what a check for "does this clause exist" cannot
catch, because the wrong clause exists too. One of the nineteen, an
RFC 5050 citation in `bp`, has since gone with the version 6 vectors it
belonged to; the table below lists the eighteen that remain.

Two limits worth knowing for anyone repeating this. Bit diagrams do not
survive PDF text extraction -- the TC start sequence extracts as a
pattern reading FF00 where the rendered page plainly shows EB90 -- so
figures must be read as images. And a clause number matched in a table of
contents looks identical to one matched in the body.

What the reading corrected:

| Package | Was | Is | Why |
|---|---|---|---|
| `ltp` | 3.2 | 3.1 | RFC 5326 3.2 is Segment Content, not the header |
| `ltp` | type 12 as a report segment | type 8 | RFC 5326 3.1.2 makes 12 a cancel segment |
| `tcf` | 3.2.1 on five P-field vectors | 3.2.2 | 3.2.1 is the T-field |
| `tcf` | 3.2.2 on five T-field vectors | 3.2.1 | 3.2.2 is the P-field |
| `tcf` | 3.3 on three CDS vectors | 3.3.1 | 3.3 is the whole CDS section |
| `tcsc` | 3.4 on both CLTU vectors | 5.2.2, 5.2.4 | 3.4 is Fill Data |
| `aos` | 4.1.2.6.5, 4.1.2.6 | 4.1.2.5.4, 4.1.2.5.5 | the signalling field, not the error control code |
| `aos` | 4.1 on two frame vectors | 4.1.2 | 4.1 is the whole PDU section |

Separately, ten notes across `cop`, `spp` and `tcf` numbered bits from
the least significant end while every cited standard numbers from the
most significant. And six `sdnv` notes claimed RFC 5050 printed values it
does not; the reading found two genuine published examples the corpus was
missing, which are now vectors.

What remains unread is ECSS-E-ST-70-41C, behind registration, covering
the ten `pus` citations.
