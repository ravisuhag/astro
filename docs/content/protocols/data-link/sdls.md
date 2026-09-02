---
title: Space Data Link Security
short: SDLS
description: CCSDS 355.0-B-2, encrypting and authenticating transfer frame contents.
identifiers:
  - "CCSDS 355.0-B-2 * Space Data Link Security"
  - "pkg/sdls * astro sdls"
order: 26
---

> **CCSDS 355.0-B-2** | [Blue Book](https://public.ccsds.org/Pubs/355x0b2.pdf) | [`pkg/sdls`](https://github.com/ravisuhag/astro/tree/main/pkg/sdls) | [`astro sdls`](/cli/sdls)

## Overview

SDLS protects the contents of a Transfer Frame. It adds a **Security Header**
before the frame's data field and a **Security Trailer** after it. The header
says which key and settings apply; the trailer carries a Message
Authentication Code that proves the frame was not altered.

It solves two problems at once. Encryption keeps a passive listener from
reading telemetry or commands. Authentication keeps an active attacker from
forging or replaying them. For a command link, the second matters more: an
attacker who cannot read your commands but can replay yesterday's can still do
real harm.

SDLS does not replace the frame protocols. It sits inside them.

### Where SDLS fits

```
┌─────────────────────────────────────────────┐
│  Application data (packets, files, ...)     │
├─────────────────────────────────────────────┤
│  Space Data Link Security (SDLS)            │  <- protects the data field
│  Security Header │ data │ Security Trailer  │
├─────────────────────────────────────────────┤
│  TM / TC / AOS / USLP Transfer Frame        │  <- carries the protected bytes
├─────────────────────────────────────────────┤
│  Synchronization and Channel Coding         │
└─────────────────────────────────────────────┘
```

The protected bytes become the frame's data field. Nothing in `pkg/tmdl`,
`pkg/tcdl`, `pkg/aos`, or `pkg/usdl` changes to carry them.

### Key characteristics

- **The wire format is not self-describing.** Field widths come from the
  Security Association, which both ends agree on before the link opens. Only
  the 16-bit Security Parameter Index travels in the clear, and it just names
  which agreement to use (clause 2.3.1.4).
- **One service per association.** An SA does authentication, encryption, or
  authenticated encryption, never a mix (clause 4.2.2.4).
- **The frame header is authenticated but not encrypted.** A receiver needs to
  read the header to route the frame, so SDLS covers it with the MAC instead of
  hiding it.
- **Not every header bit is covered.** An authentication bit mask selects which
  ones are. Counters that legitimately change in flight are masked out (clause 4.2.2.6.2).
- **Replay protection is built in.** A sequence counter travels with each frame
  and the receiver refuses anything it has already seen (clause 2.3.2.3).

## Scope

**Implemented.** All four **baselines of Annex E**:

- **AES-256-GCM** authenticated encryption, with a 96-bit initialization
  vector and a 128-bit MAC. That is the interoperability profile for TM
  (clause E1), AOS (clause E3), and USLP (clause E4).
- **AES-CMAC** authentication for telecommand (clause E2): a 256-bit key, a 32-bit
  sequence number, and a 128-bit MAC. The Go standard library has no CMAC and
  this package takes no outside dependencies, so it is implemented in
  `internal/cmac` and verified against the RFC 4493 and NIST SP 800-38B
  vectors. Select it with `AuthAlgorithm: sdls.AuthCMAC`, see
  [Telecommand: the clause E2 CMAC baseline](/protocols/data-link/sdls#telecommand-the-clause-e2-cmac-baseline).

It also offers **GMAC** for authentication without encryption. GMAC is not an
annex baseline itself; it is the natural authentication-only companion of the
GCM baselines, same cipher, same key and IV layout, nothing encrypted.

**Not here, on purpose.** Encryption without authentication. Clause 2.3.3 warns
against it, and so do we. Asking for it gives you `ErrUnsupportedMode`.

**Not here yet.** A CLI. `astro sdls apply` and friends are a follow-up, once
this API has seen some use. And the SDLS Extended Procedures of CCSDS 355.1:
key management and over-the-air rekeying are a separate standard.

**Left to you.** Key storage and distribution. An SA takes a 32-byte key and
does not care where it came from.

## Field map: the Security Header

Four fields, in this order, with no gaps between them (clause 4.1.1.1.3):

| Field | Width | Notes |
|---|---|---|
| Security Parameter Index | 16 bits, always present | Names the SA. 0 and 65535 are reserved (clause 4.1.1.2.3) |
| Initialization Vector | managed, may be 0 | 96 bits in the baseline |
| Sequence Number | managed, may be 0 | 0 in the baseline, the IV doubles as the counter |
| Pad Length | managed, may be 0 | 0 in the baseline, GCM needs no padding |

The whole header is capped at 64 octets (clause 4.1.1.1.4). The baseline uses 14.

The trailer is just the MAC: 16 octets in the baseline (clause E1.3).

## Building a security association

```go
import "github.com/ravisuhag/astro/pkg/sdls"

sa := &sdls.SecurityAssociation{
    SPI:  1,                            // names this SA on the wire
    Mode: sdls.AuthenticatedEncryption, // the Annex E baseline
    Key:  key,                          // exactly 32 bytes, from your key store
    FieldLengths: sdls.FieldLengths{
        IV:     sdls.GCMIVSize, // 12
        SeqNum: 0,              // the IV serves as the anti-replay counter
        PadLen: 0,              // GCM needs no padding
        MAC:    16,
    },
    SeqWindow: 100, // how far ahead of the last frame we still accept
}

if err := sa.Validate(); err != nil {
    log.Fatal(err)
}
```

`Validate` catches the mistakes that matter: a reserved SPI, a key that is not
32 bytes, an IV that is not 12, a MAC outside 12-16 octets (1-16 for CMAC,
which Go's GCM floor does not constrain), a header over 64 octets.

**An SA is not safe for concurrent use.** It holds the sender's IV counter and
the receiver's replay state, and both change on every frame. Give each
direction of each channel its own SA, or serialize access yourself.

## Sending

```go
// The frame header bytes SDLS authenticates but does not encrypt.
frameHeader := encodedPrimaryHeader

protected, err := sa.ApplySecurity(frameHeader, payload)
if err != nil {
    return err
}

// The protected bytes are the frame's data field.
frame, err := aos.NewTransferFrame(scid, vcid, protected, aos.WithFECF())
```

Every call advances the IV counter. It will never hand out the same IV twice
for one key, when the counter space runs out it returns `ErrIVExhausted`
rather than wrapping. Reusing an IV under one GCM key is catastrophic, so this
is a refusal, not a warning.

## Receiving

```go
frame, err := aos.DecodeTransferFrame(raw, 0, false, true)
if err != nil {
    return err
}

lookup := sdls.StaticLookup(sa) // or your own SPI -> SA function

header, payload, err := sdls.ProcessSecurity(frame.DataField, frameHeader, lookup)
if err != nil {
    return err // authentication failed, replay, unknown SPI, ...
}
```

On any failure the payload is `nil`. Partial plaintext never escapes a frame
that did not verify.

The replay window only advances after the MAC checks out. A forged frame
cannot push the counter forward and lock out the real sender.

If the SA lists the channels it serves in `Channels`, use
`ProcessSecurityForChannel` and pass the channel the frame arrived on. The
frame is refused with `ErrSAChannelMismatch` when the SA is not bound to that
channel (clause 4.2.4.3). Plain `ProcessSecurity` cannot know the channel, so it
checks the SPI only.

```go
rx.Channels = []sdls.ChannelID{{TFVN: 0, SCID: 0x2A, VCID: 3, MAPID: sdls.NoMAP}}

ch := sdls.ChannelID{TFVN: 0, SCID: scid, VCID: vcid, MAPID: sdls.NoMAP}
header, payload, err := sdls.ProcessSecurityForChannel(frame.DataField, frameHeader, ch, lookup)
```

## Telecommand: the clause E2 CMAC baseline

The TC baseline authenticates with **AES-CMAC** instead of GMAC. There is no
initialization vector at all: the Security Header is six octets, the 16-bit
SPI and a 32-bit sequence number that carries the anti-replay counter.

```go
sa := &sdls.SecurityAssociation{
    SPI:           7,
    Mode:          sdls.Authentication, // clause E2 authenticates, never encrypts
    AuthAlgorithm: sdls.AuthCMAC,       // AES-CMAC instead of GMAC
    Key:           key,                 // exactly 32 bytes
    FieldLengths: sdls.FieldLengths{
        IV:     0, // CMAC needs no IV; Validate rejects anything else
        SeqNum: 4, // 32-bit sequence number, per clause E2.1 b
        PadLen: 0,
        MAC:    16, // 128 bits, per clause E2.1 c
    },
    SeqWindow: 100,
}

protected, err := sa.ApplySecurity(frameHeader, command)
```

Sending and receiving work exactly as above; `ProcessSecurity` picks the
right algorithm from the SA the SPI names. The MAC covers the same
Authentication Payload as GMAC does, masked frame header, security header,
then the data field.

## The authentication bit mask

This is the part most likely to trip you up, and it is where the two ends must
agree exactly.

The MAC does not cover every bit of the frame header. Some fields change
legitimately between sender and receiver, or are added downstream. The mask
picks which bits count: a `1` means "include this bit", a `0` means "treat it
as zero". Clause 4.2.2.6.2 sets the rules:

| Field | Mask bits | Why |
|---|---|---|
| Virtual Channel ID | all ones | An attacker must not be able to move a frame between channels |
| Security Header, except the IV | all ones | The SPI and counter are covered; the IV is not |
| Frame Data Field | all ones | The payload itself |
| MAP ID (USLP only) | all ones | Same reasoning as the VCID |
| Segment Header (TC only) | all ones | |
| Master Channel Frame Count (TM only) | all zeros | Changes as frames are multiplexed |
| Frame Header Error Control (AOS only) | all zeros | Computed downstream |
| Insert Zone (AOS and USLP) | all zeros | Not part of the secured data |
| Everything else | all zeros by default | Missions may override, see clause 4.2.2.6.2 j |

Set the mask with `sa.AuthMask`. It must be at least as long as the frame
header plus the security header. Don't build it by hand: one constructor per
frame type returns the strictest mask that honors the mandatory exclusions.

```go
// TM: everything covered except the Master Channel Frame Count and the IV.
sa.AuthMask = sdls.BaselineAuthMaskTM(secondaryHeaderLen, sa.FieldLengths)

// TC: the whole header covered, segment header included.
sa.AuthMask = sdls.BaselineAuthMaskTC(hasSegmentHeader, sa.FieldLengths)

// AOS: everything covered except the FHEC, the insert zone, and the IV.
sa.AuthMask = sdls.BaselineAuthMaskAOS(hasFHEC, insertZoneLen, sa.FieldLengths)

// USLP: everything covered except the insert zone and the IV.
sa.AuthMask = sdls.BaselineAuthMaskUSLP(primaryHeaderLen, insertZoneLen, sa.FieldLengths)
```

To exclude more fields (clause 4.2.2.6.2 j allows it), clear further octets in the
returned slice. Both ends must use the same mask.

A `nil` mask means every octet of the frame header is authenticated. For TM
and AOS that violates the mandatory exclusions: the moment a multiplexer
rewrites the MCFC or a coder fills in the FHEC, the MAC breaks at the
receiver. Leave the mask `nil` only when the header you pass contains no
field that changes in flight, otherwise use the constructors.

**The IV is always excluded, whatever the mask says.** clause 4.2.2.6.2 h) makes that
mandatory, so this package enforces it rather than trusting the mask.

## Sequence numbers and replay

The sender bumps its counter by one per frame (clause 4.2.3.4 a). The receiver keeps
the highest value it has accepted and applies two rules (clause 2.3.2.3.2, clause 2.3.2.3.3):

- A counter at or below the stored value is a replay. Discard it.
- A counter more than `SeqWindow` ahead is discarded too. The window absorbs
  normal gaps and delays without accepting anything from far in the future.

In the baseline the IV *is* the counter, so there is no separate field to
manage. If your SA declares a Sequence Number field instead, this package uses
that one.

Setting `SeqWindow` to 0 turns the check off. Only do that in tests.

## Reference

- [CCSDS 355.0-B-2](https://public.ccsds.org/Pubs/355x0b2.pdf), Space Data Link Security Protocol
- [CCSDS 352.0-B-2](https://public.ccsds.org/Pubs/352x0b2.pdf), Cryptographic Algorithms
- [CLI](/cli/sdls) | [Conformance](/conformance/sdls) | [The stack](/docs/start/concepts)
