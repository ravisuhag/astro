# Bundle Protocol

> CCSDS 734.2-B-1, profiling RFC 5050 — Bundle Protocol version 6

## Overview

Bundle Protocol is the network layer of Delay-Tolerant Networking. It moves
application data units — **bundles** — across networks where no end-to-end
path exists at any single moment.

That is the whole idea. IP assumes a path exists right now. In space, a relay
orbiter may be behind a planet, a ground station may not be in view for six
hours, and the far end may be light-minutes away. BP stores a bundle at each
hop and forwards it when a link opens, rather than holding a session open
across the gap.

### Version 6, not version 7

**CCSDS 734.2-B-1 profiles RFC 5050, which is Bundle Protocol version 6.**

This matters. BPv7 (RFC 9171) encodes bundles in CBOR and is wire-incompatible
with v6 — the two cannot talk to each other. This package implements what
CCSDS specifies. If you need BPv7, it would be a separate package.

The CCSDS profile adds two things to RFC 5050: the **IPN naming scheme** with
Compressed Bundle Header Encoding (RFC 6260), and a mandatory **Extended Class
of Service** block.

### Where BP fits

```
┌─────────────────────────────────────────────┐
│  Application data (files, telemetry)        │
├─────────────────────────────────────────────┤
│  Bundle Protocol (pkg/bp)                   │  ← this package
├─────────────────────────────────────────────┤
│  Convergence layer: LTP (pkg/ltp) or        │
│  Encapsulation Packets (pkg/epp)            │
├─────────────────────────────────────────────┤
│  Space Data Link (pkg/aos, pkg/usdl, ...)   │
└─────────────────────────────────────────────┘
```

## Bundle structure

A bundle is a primary block followed by one or more canonical blocks:

```
[ primary block │ extension blocks... │ payload block ]
```

The **primary block** carries everything needed to route the bundle:
destination, source, report-to and custodian endpoints, a creation timestamp,
a lifetime, and the processing flags.

**Canonical blocks** are everything else. The payload is one of them, always
last, always carrying the last-block flag.

Nearly every field is a Self-Delimiting Numeric Value, which is why this
package builds on `pkg/sdnv`.

## Endpoints and the dictionary

An endpoint is a URI: a scheme and a scheme-specific part. CCSDS requires the
**IPN scheme** (§3.2.1), where the scheme-specific part is a node number, a
period, and a service number:

```go
dest := bp.IPNEndpoint(2, 1)   // ipn:2.1
src  := bp.IPNEndpoint(1, 1)   // ipn:1.1
```

Node numbers run 1 to 2^64−1 and are assigned by SANA. Service numbers run
0 to 2^64−1.

Here is the part worth understanding. The primary block does not store four
endpoint URIs. It stores a **dictionary** — a run of null-terminated strings —
and each endpoint travels as a *pair of offsets* into it, one for the scheme
and one for the scheme-specific part.

Four endpoints all in the IPN scheme therefore store the string `ipn` once, not
four times. That is Compressed Bundle Header Encoding, and on a link where
every octet is paid for in watts, it is worth the indirection. This package
builds the dictionary for you.

The null endpoint `dtn:none` names nobody, and is what you use for a custodian
when nobody has custody.

## Building a bundle

```go
import "github.com/ravisuhag/astro/pkg/bp"

primary := &bp.PrimaryBlock{
    Flags:       bp.BundleFlags(0).WithPriority(bp.PriorityNormal) | bp.FlagSingleton,
    Destination: bp.IPNEndpoint(2, 1),
    Source:      bp.IPNEndpoint(1, 1),
    ReportTo:    bp.IPNEndpoint(1, 0),
    Custodian:   bp.NullEndpoint,
    CreationTimestamp: bp.CreationTimestamp{Time: secondsSince2000(), SequenceNumber: 1},
    Lifetime:    3600, // seconds
}

bundle, err := bp.NewBundle(primary, payload)
if err != nil {
    return err
}

raw, err := bundle.Encode()
```

The creation timestamp counts seconds since the start of the year 2000. It and
the source endpoint together identify the bundle — which is how status reports
and custody signals refer back to it.

## Processing flags

The flags field is an SDNV, so it has no fixed width. Three groups (§4.2):

**Handling requests**, bits 0 to 5: fragment, administrative record, must not
fragment, custody transfer requested, singleton destination, application
acknowledgement.

**Class of service**, bits 7 and 8: bulk, normal, expedited. Use
`WithPriority`, which leaves the other bits alone.

**Status report requests**, bits 14 to 18: reception, custody acceptance,
forwarding, delivery, deletion.

One rule the library enforces: an administrative record must request neither
custody transfer nor any status report. Setting both is rejected.

## Extended Class of Service

CCSDS 734.2-B-1 §3.3 **requires** conformant implementations to support the
ECOS block. Three priority levels are not enough for spacecraft operations.

```go
bundle, err := bp.NewBundle(primary, payload,
    bp.WithECOS(bp.ECOS{
        Flags:   bp.ECOSCritical,
        Ordinal: 200,
    }))
```

What it adds (annex C):

- **Ordinal**, 0 to 255. Within the expedited class, a finer ranking: ordinal
  100 beats ordinal 99. It means nothing unless the class of service is
  expedited. Value 255 is reserved for custody signals.
- **Critical** — send one copy along *every* path that might reach the
  destination, not just the best one. The bundle arrives by whatever route
  turns out to be fastest, at the cost of flooding the network. This is for
  emergency traffic.
- **Streaming** — best efforts, no retransmission. For data where a late copy
  is worse than none, like video.
- **Reliable** — the opposite: use a convergence layer that retransmits.
- **Flow label** — an opaque value passed down to the convergence layer.

Two structural rules the library enforces: the ECOS block must come before the
payload, and a bundle may carry at most one.

## Fragmentation

A bundle too large for a contact window can be split:

```go
fragments, err := bundle.Fragment(1024)
```

Each fragment carries the fragment flag, its offset into the original
application data unit, and the total ADU length so the far end knows when it
has everything.

Extension blocks flagged "replicate in every fragment" are copied into each
piece. Everything else rides with the first fragment only (§5.8) — sending an
extension block five times when once will do wastes the link.

Reassembly takes them in any order:

```go
original, err := bp.Reassemble(fragments)
```

Overlapping fragments are fine. Gaps are not: reassembly fails rather than
handing back a partly-filled buffer.

A bundle with the "must not be fragmented" flag refuses to split.

## Administrative records

Two kinds, both carried as a bundle payload with the administrative-record
flag set (§6.1).

**Status reports** say what a node did with a bundle: received it, took
custody, forwarded it, delivered it, deleted it. Each event has a timestamp,
and — this is the subtle part — **a timestamp is present on the wire only when
its matching status flag is set**. A report of one event is shorter than a
report of five.

**Custody signals** accept or refuse custody. The high bit of the status byte
says which; the low seven bits carry the reason.

```go
record, err := bundle.AdminRecord()
if err != nil {
    return err
}
if record.StatusReport != nil {
    log.Printf("%s", record.StatusReport.Humanize())
}
```

Times in administrative records are "DTN time": seconds since the start of
2000, plus nanoseconds. CCSDS §3.4 relaxes this — where a spacecraft clock
cannot produce meaningful nanoseconds, the onboard precision is used, and this
does not become a requirement on the clock.

## Decoder limits

A block length is an SDNV reaching 2^64. Sizing an allocation from one is a
trivial denial of service, so decoding is bounded:

```go
bundle, err := bp.DecodeBundleWithOptions(raw, bp.DecodeOptions{
    MaxBlockLength: 1 << 20,
    MaxBlocks:      32,
})
```

`DecodeBundle` applies defaults of 16 MiB per block and 64 blocks. Neither cap
is in RFC 5050 — the protocol states no ceiling — but no real implementation
can go without one.

## What is not here yet

- **Bundle Protocol agent** — routing, contact graphs, storage, custody
  timers. This package is the wire format and the block structure; forwarding
  policy is a layer above.
- **Aggregate Custody Signals** (annex D) and **Delay-Tolerant Payload
  Conditioning** (annex E).
- **Bundle Security Protocol** blocks.
- **BPv7** (RFC 9171) — a different wire format, and a separate package if
  anyone needs it.
- **CLI subcommands** — a follow-up once the API settles.

## Reference

- [CCSDS 734.2-B-1](https://public.ccsds.org/Pubs/734x2b1.pdf) — CCSDS Bundle Protocol Specification
- [RFC 5050](https://www.rfc-editor.org/rfc/rfc5050.txt) — Bundle Protocol Specification, the wire format
- [RFC 6260](https://www.rfc-editor.org/rfc/rfc6260.txt) — Compressed Bundle Header Encoding
- [PICS proforma](../pics/bp-pics.md) — conformance statement for this package
