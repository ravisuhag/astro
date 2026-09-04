---
title: Bundle Protocol
short: BP
description: RFC 9171, store-and-forward bundles for delay-tolerant networking.
identifiers:
  - "RFC 9171 * Bundle Protocol version 7"
  - "pkg/bp * astro bp"
order: 14
---

> **RFC 9171** | [RFC 9171](https://www.rfc-editor.org/rfc/rfc9171) | [`pkg/bp`](https://github.com/ravisuhag/astro/tree/main/pkg/bp) | [`astro bp`](/cli/bp)

## Overview

Bundle Protocol is the network layer of Delay-Tolerant Networking. It moves
application data units — **bundles** — across networks where no end-to-end path
exists at any single moment.

That is the whole idea. IP assumes a path exists right now. In space, a relay
orbiter may be behind a planet, a ground station may not be in view for six
hours, and the far end may be light-minutes away. BP stores a bundle at each hop
and forwards it when a link opens, instead of holding a session open across the
gap.

### This is version 7

astro implements **RFC 9171, Bundle Protocol version 7**. That is the version
live implementations speak: ION, µD3TN and NASA's HDTN.

Version 6 (RFC 5050, profiled by CCSDS 734.2-B-1) is **a different wire format,
not an earlier revision.** It encodes with SDNV; version 7 encodes with CBOR.
The endpoint naming, the time base and the block structure all changed. The two
cannot talk to each other, and nothing here reads a version 6 bundle.

astro implemented version 6 until it did not. If you need it, it is in git
history at tag `v0.4.0`.

## Bundle structure

A bundle is a CBOR indefinite-length array: a primary block, then one or more
canonical blocks, the last of which is the payload. A break stop code closes it.

```
9f                          indefinite-length array
  88 07 00 00 ...           primary block
  85 01 01 00 00 ...        payload block (type 1, number 1)
ff                          break
```

The primary block is immutable once created. It carries the version, the
processing flags, three endpoint IDs (destination, source, report-to), the
creation timestamp, the lifetime, and optionally the fragment fields and a
checksum.

Every other block shares one shape: type code, block number, flags, checksum
type, then a byte string of type-specific data.

### Endpoint IDs

Two schemes. `dtn` names endpoints with text; `ipn` names them with numbers, for
compactness on a constrained link.

```go
bp.IPN(1, 2)                        // ipn:1.2
bp.IPNWithAllocator(977000, 100, 1) // ipn:977000.100.1
bp.DTN("//receiver/inbox")          // dtn://receiver/inbox
bp.NullEID()                        // dtn:none, the anonymous source
```

astro follows [RFC 9758](https://www.rfc-editor.org/rfc/rfc9758) for the `ipn`
scheme, which added an allocator identifier. Both its encodings decode, and the
one astro writes is byte-identical to RFC 9171's when the allocator is zero —
so bundles astro produces are readable by implementations that predate
RFC 9758.

### Time

DTN time counts **milliseconds since 2000-01-01**, ignoring leap seconds.
Version 6 counted seconds. A bundle read with the wrong unit still parses and
still round-trips inside one implementation, and is wrong by a factor of a
thousand on the wire.

A node with no working clock sends a creation time of zero and carries a Bundle
Age block instead.

## Extension blocks

RFC 9171 defines three, and astro gives each a constructor and an accessor:

| Block | Type | What it carries |
|---|--:|---|
| Previous Node | 6 | the node that forwarded this bundle here |
| Bundle Age | 7 | how long the bundle has existed, in milliseconds |
| Hop Count | 10 | a hop limit and a hop count, against routing loops |

A block of a type astro does not know keeps its bytes and round-trips exactly,
because clause 4.4 requires a node to forward what it cannot parse.

## Fragmentation

A contact window too short for a bundle is the normal case, not the exception.
`Fragment` splits one, and `Reassemble` puts it back:

```go
fragments, _ := bundle.Fragment(1024)
// ... fragments travel by different routes, arriving out of order
original, _ := bp.Reassemble(fragments)
```

Reassembly works in *material extents*, so fragments may overlap. Clause 5.8
allows separate fragmentation episodes in different parts of the network to
produce overlapping slices of the same payload, and clause 5.9 requires a
receiver to cope.

## Status reports

The one administrative record version 7 defines. It reports four assertions —
received, forwarded, delivered, deleted — about a bundle named by its source
node ID and creation timestamp.

```go
b, _ := bp.NewStatusReportBundle(primary, &bp.StatusReport{
    Delivered:        bp.StatusItem{Asserted: true},
    SubjectSource:    bp.IPN(2, 1),
    SubjectTimestamp: subject.Primary.Timestamp,
})
```

A bundle carrying a status report cannot itself request status reports.
Otherwise reports would beget reports, and clause 4.2.3 forbids it.

## What this package does not do

It encodes, decodes and validates bundles. It does not move them.

There is no convergence layer, no routing, no contact graph and no daemon.
Those need timers, sockets and a network, and astro hands that to the caller —
the same rule every package here follows. Compose with
[`pkg/ltp`](/protocols/transport/ltp) when a bundle needs a transport
underneath; the [DTN example](https://github.com/ravisuhag/astro/tree/main/examples/dtn)
shows the two together.

Bundle Protocol Security ([RFC 9172](https://www.rfc-editor.org/rfc/rfc9172))
is a separate standard and would be a separate package.

## Two things the standard makes easy to get wrong

**The bundle array is indefinite-length.** The CDDL grammar in RFC 9171
appendix B writes it as `[primary-block, *extension-block, payload-block]`,
which reads as definite. Clause 4.1 requires the indefinite form, and the
appendix says the prose wins wherever the two disagree. An implementation
trusting the grammar emits bundles no conforming node accepts, while reading
its own output back without complaint. The published bundle in RFC 9173
appendix A.1.1.3 opens `0x9f` and ends `0xff`, settling it.

**Extension block data is two layers deep.** The block-type-specific field is a
byte string whose contents are themselves CBOR. A Bundle Age block carrying
300 ms encodes as `0x43` — a three-octet byte string — wrapping `0x19012c`.
astro's constructors and accessors peel both layers.

## Reference

[Package documentation](https://pkg.go.dev/github.com/ravisuhag/astro/pkg/bp) ·
[conformance statement](/conformance/bp) · [CLI](/cli/bp)
