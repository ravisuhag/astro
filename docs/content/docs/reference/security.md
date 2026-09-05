---
title: Security
short: Security
description: A library whose input arrives from a radio. The threat model, the resource limits, and how to report a problem.
order: 4
---

Astro parses octets that came off a radio link. Nothing upstream of the decoder is trusted, or can be: a ground station receives whatever is transmitted on its frequency, and a spacecraft receives whatever is transmitted at it.

So the decoders are the attack surface, and this page is what is done about that.

## The threat model

**In scope.** Malformed, truncated, or hostile octets reaching any `Decode` function. The failure modes that matter are a panic that takes down a ground system, and a length field that makes the decoder allocate memory the attacker chose.

**Out of scope for this library.** Whether the sender is who they claim to be. That is [SDLS](/docs/guides/secure-a-link)'s job, and it is a separate decision a mission makes: authenticate the uplink so a command cannot be forged, encrypt the downlink if the data is sensitive. `pkg/sdls` implements the mechanism; it cannot make you turn it on.

**Also out of scope.** Key management. `pkg/sdls` takes the key octets and does not load, store, rotate or protect them.

## Every decoder is fuzzed

77 fuzz targets across 29 packages. The property each one asserts is that arbitrary input never panics and never allocates from a length field the input controls.

```bash
make fuzz-smoke
```

runs a short burst (15 seconds each) over 67 of them, covering every frame and packet decoder plus most everything else that parses untrusted octets. The remaining 10 (extra targets in `pkg/pus`, `pkg/pxsc` and `pkg/xtce`) run only under `go test -fuzz` directly. A new decoder is expected to arrive with a target, per [the contributing guide](/docs/contribute/adding-a-protocol).

## Resource limits

A length field in a header is a number an attacker picks. Where a standard sets no ceiling, this library sets one, because the alternative is `make([]byte, attackerChoice)`.

| Package | Limit | Value | Why |
|---|---|---|---|
| `pkg/bp` | none | — | Version 7 decodes from a caller-supplied slice rather than a stream, so a length field can only claim what is already in memory. The caller's own bound is the ceiling |
| `pkg/ltp` | `DefaultMaxBlockSize` | 64 MiB | Not in RFC 5326: a segment offset is an SDNV reaching 2^64, so one corrupt segment could claim a huge offset |
| `pkg/xtce` | `MaxDocumentSize` | 64 MiB | A very large database file |
| `pkg/xtce` | `MaxDepth` | 100 | A deeply nested document. The check runs as a token scan before any decoding, so deep input is refused rather than recursed into |
| `pkg/sle` | `DefaultMaxMessageSize` | 16 MiB | A TML header promising more than exists |
| `pkg/sle` | `MaxSpaceLinkDataUnit` | 65536 | Ceiling on a delivered frame |
| `pkg/sle` | `MaxEventQualifier` | 1024 | Bounded opaque field |
| `pkg/sdnv` | `MaxEncodedSize` | 10 | A 64-bit value needs at most 10 octets; more is a malformed or padded encoding |
| `pkg/cfdp` | `MaxIDWidth` | 8 | Entity ID widths are caller-declared |

Where the limit is a `Default`, it is a configurable field: set it to what your mission actually sends rather than leaving headroom you do not need.

## XML, and why the usual attacks do not apply

`pkg/xtce` reads XML, which normally means worrying about external entities. Go's `encoding/xml` does not fetch DTDs and does not expand external entities, so XXE, entity expansion bombs and network callbacks from a document are not reachable. What remains is plain resource abuse, and that is what `MaxDocumentSize` and `MaxDepth` bound.

There is no XSD validation. The standard library has no XSD validator and this library takes no dependencies, so `Validate` runs semantic checks written in Go: references resolve, inheritance does not loop, names do not collide. A file that breaks the schema in a way those checks miss will load. Run `xmllint` over it first if that matters.

## Cryptography

`pkg/sdls` uses the Go standard library for AES-GCM and AES-CMAC. No cryptographic primitive is implemented here.

Two details worth knowing:

**ISP1 credentials hash with SHA-256**, per CCSDS 913.1-B-2 clause 3.1.2.3. SHA-1 belonged to the previous issue of the standard. A 20-octet legacy digest still decodes, but it cannot be verified, because the superseded scheme is not implemented. Only SHA-256 is ever generated.

**A Security Association is not safe for concurrent use.** It holds the sender's IV counter and the receiver's anti-replay state, both of which change on every frame. Sharing one across goroutines corrupts the IV counter, and reusing an IV under GCM is the failure that loses you the key stream. One SA per direction per channel.

## What a mission still has to do

- **Turn on authentication for commanding.** An unauthenticated uplink is forgeable by anyone with a transmitter and the published frame format. See [encrypt and authenticate a link](/docs/guides/secure-a-link).
- **Set `SeqWindow`.** `pkg/sdls` disables the anti-replay check when it is zero, which is only ever right in a test.
- **Bind each SA to its channels.** Use `ProcessSecurityForChannel`, not `ProcessSecurity`, so a genuine frame moved to another virtual channel is refused.
- **Keep clocks agreeing.** SLE credentials are rejected when the peer's timestamp is too far from now, which is the replay defence and also a way to lock yourself out.
- **Decide what a decode failure means.** `Accept` treats an undecodable CADU as an error because only the caller can tell a corrupt frame from a misconfigured channel. A station should log and continue rather than stop.

## Reporting a vulnerability

Use [private vulnerability reporting](https://github.com/ravisuhag/astro/security/advisories) on the repository. Please do not open a public issue, discussion or pull request for it.

Include what you investigated and why you believe an exploit is possible. A proof of concept and a link to the code are the most useful things you can send. Reports are not eligible for a bounty.

## Reference

- [How this is verified](/docs/reference/verification), the fuzzing and vector picture in full
- [Encrypt and authenticate a link](/docs/guides/secure-a-link), SDLS in practice
- [SDLS protocol page](/protocols/data-link/sdls) | [XTCE protocol page](/protocols/mission/xtce)
