# Coverage

What the vector corpus covers, what it does not, and which values are not
established against a standard. Read this before relying on the corpus as
evidence of conformance.

## What is covered

| Package | Standard | Vectors | Corpus files |
|---|---|--:|--:|
| `aos` | CCSDS 732.0-B-4 | 9 | — |
| `bp` | CCSDS 734.2-B-1 / RFC 5050 | 5 | — |
| `cfdp` | CCSDS 727.0-B-5 | 14 | — |
| `cmac` | RFC 4493 (CMAC-AES128), NIST SP 800-38B (CMAC-AES256) | 8 | — |
| `cop` | CCSDS 232.0-B-4 | 15 | — |
| `crc` | CCSDS 132.0-B-3 clause 4.1.6 (CRC-16-CCITT) | 9 | — |
| `epp` | CCSDS 133.1-B-3 | 16 | — |
| `ldc` | CCSDS 121.0-B-3 | — | 107 |
| `ltp` | CCSDS 734.1-B-1 / RFC 5326 | 9 | — |
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
| `tmdl` | CCSDS 132.0-B-3 | 20 | — |
| `tmsc` | CCSDS 131.0-B-5 | 7 | — |
| `usdl` | CCSDS 732.1-B-3 | 9 | — |
| `xtce` | CCSDS 660.0-B-2 (XTCE) | — | 8 |
| **Total** | | **235** | **115** |

235 vectors and 115 referenced corpus files across 23 packages.
3 vectors are marked unverified; they are listed below.

## What is not covered

**State machines.** Every vector here asserts a single encode, decode or
rejection. Nothing asserts a sequence, so no state machine is covered:
FOP-1 and FARM-1, the CFDP transaction engines, the LTP session and
receiver, SLE association, bundle reassembly and custody transfer, and
frame multiplexing and flush ordering. The schema defines a `sequence`
form for this; no file populates it yet.

**Packages with no wire vectors.** Three packages have nothing to pin at
this level, and their absence is deliberate rather than an oversight:

| Package | Why |
|---|---|
| `tcdl` | its pinned test values are inputs, not expected octets |
| `pxdl` | its pinned values assert bit positions rather than whole-field encodings |
| `rhc` | its pinned values are mask positions rather than encoded streams |

`sdl` is internal channel machinery with no wire format of its own.

**Bit-string layers.** Most of `ocsc` works on bit strings — bit-level
lengths, termination digits, sequence indicators — which an octet-string
format cannot express. Only its sync marker is pinned.

**Expected parse trees.** The XTCE documents in `vectors/xtce/` are shared
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

These carry `"source": "unverified"` and no `clause`. An implementation
agreeing with them has matched this corpus, not the standard.

| File | Vector | What is unresolved |
|---|---|---|
| `aos/frame.json` | `fhec-of-primary-header` | The RS(10,6) arithmetic is reproducible, but whether information symbols `[6,10,14,10,4,3]` are the ones CCSDS 732.0-B-4 specifies is not established. They are header nibbles 0-3 and 10-11, skipping the 24-bit VC frame count in octets 2-4. Clause 4.1.2 settles it. |
| `aos/frame.json` | `frame-with-fhec-and-fecf` | Frame layout and FECF are established; the FHEC value carried inside is subject to the row above. |
| `aos/frame.json` | `frame-with-fhec-inverse` | Same. Pins that FHEC octets survive decode and the data field starts after them. |

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

## The largest gap

No vector here has been exchanged with another implementation.

Every value is derived from a published standard or lifted from a
published corpus, and the derivation is written down. That catches a
misread clause. It does not catch a clause misread the same way by
whoever wrote the encoder and whoever wrote the vector.

The published corpora are the exception and the strongest evidence
present: the CCSDS 121.0 data set, the annex F checksum, the RFC 5050
SDNV examples, the RFC 4493 and NIST CMAC sets, and the published
randomizer digit strings. Everywhere else, agreement between two
implementations is still the missing assurance.
