---
title: Bundle Protocol Security
short: BPSec
description: RFC 9172, integrity and confidentiality blocks for BPv7 bundles.
identifiers:
  - "RFC 9172 * Bundle Protocol Security"
  - "RFC 9173 * Default Security Contexts for BPSec"
  - "pkg/bpsec"
order: 15
---

> **RFC 9172**, with contexts from **RFC 9173** | [RFC 9172](https://www.rfc-editor.org/rfc/rfc9172) | [RFC 9173](https://www.rfc-editor.org/rfc/rfc9173) | [`pkg/bpsec`](https://github.com/ravisuhag/astro/tree/main/pkg/bpsec)

## Overview

BPSec protects the blocks inside a [BPv7 bundle](/protocols/transport/bp). It
adds two block types. A **Block Integrity Block**, or BIB, carries a keyed hash
over one or more other blocks. A **Block Confidentiality Block**, or BCB,
replaces the contents of its targets with ciphertext.

Security in a delay-tolerant network is a different problem from security on a
live link. There is no handshake, because there may be no round trip for hours.
There is no session, because the two ends are rarely up at the same time. What
travels is a bundle that already carries everything a receiver needs to check
it, and that bundle may sit on a relay for a long time before it moves on.

That is why the protection is per block rather than per hop. A waypoint can
check a BIB without holding the key that would let it read the payload. A relay
can forward an encrypted bundle it cannot open. Each block is protected on its
own, and each protection names the node that applied it.

RFC 9172 defines the block structure and leaves the cryptography to a *security
context*. RFC 9173 defines the two default contexts, and those are what this
package implements: `BIB-HMAC-SHA2` and `BCB-AES-GCM`.

## Scope

**Implemented.** Both security blocks, the Abstract Security Block codec they
share, both RFC 9173 security contexts, the canonicalization that feeds them,
and the block interaction rules of RFC 9172 clause 3.9. AES Key Wrap
(RFC 3394) is included, because both contexts define a wrapped key parameter
and neither is usable across a real network without it.

**Deliberately absent: security policy.** RFC 9172 clause 7 leaves every
deployment to decide which security operations are required, which are
optional, and what happens when one fails. This package reports whether an
operation succeeded. Acting on the answer is the caller's job, and it has to
be, because the right answer differs between a lunar relay and a cubesat.

**Deliberately absent: key management.** RFC 9172 clause 6 says outright that
key management is out of its scope, and it is out of this package's scope too.
You hand a key in. Where it came from is your business.

**Left to the caller: the initialisation vector.** RFC 9173 clause 4.6 is blunt
about what reusing one costs — a single repeat of an IV with the same key loses
the integrity protection, not merely the confidentiality. A library that
generated IVs from its own hidden state would take that decision away from the
mission that has to live with it.

**Left to the caller: when to remove a security block.** RFC 9172 clause 5.1
requires a security acceptor to strip a block once it has processed every
operation in it. `Decrypt` does that, because a BCB sitting beside plaintext
actively misdescribes the bundle. `Verify` does not, because verifying is also
what a waypoint does on the way past, and only the caller knows which one this
node is. `Remove` is there for when it is the acceptor.

**In a different package.** Bundles themselves are [`pkg/bp`](/protocols/transport/bp).
Moving a bundle is nobody's package yet: astro has no convergence layer, so
there is no daemon here and no sockets.

## Field map

Both security blocks are ordinary canonical blocks whose block-type-specific
data holds an **Abstract Security Block**. The ASB is a bare CBOR sequence, not
an array — the fields sit one after another with no head around them.

| Field | CBOR | Go | Notes |
|---|---|---|---|
| Security Targets | array of uint | `ASB.Targets` | Block numbers. At least one, no duplicates. 0 is the primary block. |
| Security Context Id | uint | `ASB.ContextID` | 1 is BIB-HMAC-SHA2, 2 is BCB-AES-GCM. |
| Security Context Flags | uint | `ASB.ContextFlags` | Only bit 0 is defined: parameters present. |
| Security Source | EID array | `ASB.Source` | The node that added the block. |
| Security Context Parameters | array of [id, value] | `ASB.Parameters` | Present if and only if bit 0 is set. |
| Security Results | array of array of [id, value] | `ASB.Results` | One set per target, in target order. |

Parameter and result values are held as raw CBOR. RFC 9172 clause 3.10 leaves
their encoding to whichever security context defines them, so the ASB itself
cannot know what shape a given number takes. `DecodeIntegrityParameters` and
`DecodeConfidentialityParameters` read them for the two contexts RFC 9173
defines.

### Block type codes

| Code | Block | Go |
|---|---|---|
| 11 | Block Integrity Block | `BlockTypeIntegrity` |
| 12 | Block Confidentiality Block | `BlockTypeConfidentiality` |

## The parameter numbers are not shared between contexts

This is the trap most likely to produce a working implementation that talks to
nobody.

| Number | In a BIB (clause 3.3.4) | In a BCB (clause 4.3.5) |
|---|---|---|
| 1 | SHA variant | Initialisation vector |
| 2 | **Wrapped key** | AES variant |
| 3 | Integrity scope flags | **Wrapped key** |
| 4 | — | AAD scope flags |

The wrapped key is 2 in one and 3 in the other. The scope flags are 3 in one
and 4 in the other. Read the security context identifier before you read any
parameter. Code that keys off the number alone will pull an integrity scope
flag and treat it as a wrapped key.

The result numbers do not collide, but only because each context defines
exactly one: 1 is the expected HMAC in a BIB and the authentication tag in a
BCB.

## The primary block is framed two ways in one IPPT

The Integrity-Protected Plaintext is the octet string a BIB's hash is taken
over. RFC 9173 clause 3.7 builds it in five steps, and the primary block can
enter it twice over, framed differently each time.

When the primary block flag pulls the primary block in as **context** — step 2
— it goes in raw. Appendix A.4 shows the IPPT starting `07 8807…`, where `8807`
is the primary block's own first octets.

When the primary block is the **target** — step 5 — it is wrapped in a CBOR
byte string head that appears nowhere in the bundle. Appendix A.3 shows that
IPPT starting `00 581c 8807…`, where `581c` is a 28-octet byte string head
around the 28-octet primary block.

The reason is that step 5 quotes a data field, and the canonical form of a data
field includes its own CBOR encoding (RFC 9172 clause 4). The primary block has
no block-type-specific data field to quote, so the step quotes the block — and
still quotes it as a byte string. Step 2 is not quoting a field at all.

Only appendix A.3 shows this, because it is the only worked example whose BIB
targets the primary block. An implementation that never signs the primary block
will never notice, and will fail the first time it meets one that does.

## The IPPT includes the byte string head; the BCB plaintext does not

For an ordinary target block the two contexts disagree about the same field.

A BIB hashes the block-type-specific data **with** the CBOR byte string head it
carries on the wire. Appendix A.1 prints the IPPT as `00 5823 5265…`, where
`5823` is the head.

A BCB encrypts the block-type-specific data **without** it. Clause 4.7.1 says
so directly and prints a table of examples. Appendix A.2 lists the payload
plaintext as `5265…`, 35 octets, and the ciphertext replaces exactly those.

Both are deliberate. Encrypting the head would change the block's length
framing; hashing without it would leave the length unprotected.

## Clause 4.7.2 says BIB where it means BCB

Step 4 of the AAD construction reads "the block type code, block number, and
block processing control flags associated with the BIB". It is describing how a
BCB builds its additional authenticated data, and there may be no BIB in the
bundle at all.

The worked example settles it. Appendix A.4 prints the payload AAD ending
`0c0201` — type 12, block number 2, replicate-in-every-fragment set — which is
the BCB's own header. No BIB header appears in it.

Read step 4 as "the security block this AAD belongs to". That is what
`AAD` implements.

## The scope flags go into the hash even when they are not in the block

Both contexts begin the same way: the canonical form starts as the CBOR
encoding of the scope flags, with every unset, reserved and unassigned bit
zeroed. That happens whether or not the block carries the flags as a parameter.

Clause 3.7 states it as an aside — "while integrity scope flags might not be
included in the BIB representing the security operation, they MUST be included
in the IPPT value itself" — and it is easy to read past. Leave them out and
every hash you compute will be wrong by one leading octet.

Appendix A.2 prints the AAD for a no-scope BCB as `h'00'`. That is not an
absent AAD. It is the scope flags byte, alone.

## The RFC breaks its own key length rule

Clause 3.5 says an HMAC key "MUST have a key length equal to the output of the
HMAC". Appendix A.1 then signs with HMAC 512/512 — a 64-octet output — under a
16-octet key.

This package does not enforce the clause. Enforcing it would reject the
document's own worked example, and every implementation that pinned itself to
those octets. HMAC accepts any key length by construction (RFC 2104), so a
short key works; it is simply weaker than clause 3.5 intends.

If your deployment wants the rule, check the key length before you call.

## There is no A192GCM

RFC 8152 assigns 1, 2 and 3 to A128GCM, A192GCM and A256GCM. RFC 9173 table 4
lists only 1 and 3. A 192-bit content encryption key has no way to be named in
a BCB, so `AESVariant(2)` is rejected here rather than quietly accepted.

The SHA variants have no such gap: 5, 6 and 7 are all defined.

## Adding a BCB changes the bundle in place

`Confidentiality.Add` encrypts. The target blocks it names come back holding
ciphertext, and any checksum on them is stripped first, as RFC 9173
clause 4.8.1 requires. There is no copy kept.

`Decrypt` goes the other way, and it does keep the original until it is sure:
every target is authenticated and decrypted before any of them is written back.
A bundle that fails to decrypt is the bundle that arrived. RFC 9172 clause 5.1.1
requires a node that cannot decrypt an encrypted payload to discard the whole
bundle, and that decision needs the bundle it was given.

`Integrity.Add` also strips a target's checksum, for the same clause-level
reason, but that one changes nothing a verifier sees: a block's CRC is not part
of the IPPT.

## The worked example bundles are not bundles you may create

Every example in RFC 9173 appendix A has a creation time of zero and no Bundle
Age block, except A.3. RFC 9171 clause 4.4.2 forbids a node to create such a
bundle, because nothing then says when it expires.

`pkg/bp` splits the difference: `Decode` accepts them, `Encode` refuses them.
That split is deliberate and predates this package — a decoder that rejected
the RFC's own example would reject what real implementations copied from it.
It does mean you cannot round-trip appendix A.1 through `bp.Bundle.Encode`.
Add a Bundle Age block, or assemble the blocks yourself.

## Using the package

### Quick start

Sign the payload:

```go
integrity := bpsec.Integrity{
    BlockNumber: 2,
    Source:      bp.IPN(2, 1),
    Variant:     bpsec.HMACSHA384,
    Scope:       bpsec.ScopeAll,
    Key:         key,
}
bib, err := integrity.Add(bundle, bp.PayloadBlockNumber)
```

Check it at the far end:

```go
err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: key})
```

Encrypt it, carrying the key wrapped so the receiver can recover it:

```go
confidentiality := bpsec.Confidentiality{
    BlockNumber: 3,
    Flags:       bp.BlockFlagReplicateInEveryFragment,
    Source:      bp.IPN(2, 1),
    Variant:     bpsec.AES256GCM,
    Scope:       bpsec.ScopeAll,
    Key:         contentKey,
    KEK:         keyEncryptionKey,
    IV:          iv,
}
bcb, err := confidentiality.Add(bundle, bp.PayloadBlockNumber)
```

And open it, holding only the key encryption key:

```go
err := bpsec.Decrypt(bundle, bcb, bpsec.Keys{KEK: keyEncryptionKey})
```

### Scope

`ScopeFlags` says how much beyond the target's own contents a security
operation binds itself to. The same three bits mean the same three things in
both contexts.

| Flag | Binds to |
|---|---|
| `ScopePrimaryBlock` | The bundle's primary block, so a target moved into another bundle stops verifying. |
| `ScopeTargetHeader` | The target's type code, block number and processing control flags. |
| `ScopeSecurityHeader` | The security block's own type code, block number and flags. |
| `ScopeAll` | All three. This is what RFC 9173 says to assume when the parameter is absent. |
| `ScopeNone` | The target's contents only. |

Wider scope is not free — every flag is one more thing that must be identical
at the far end — but `ScopeNone` protects less than people expect. It leaves
the block headers, and the bundle the block sits in, unprotected.

### Canonicalization on its own

`IPPT` and `AAD` are exported. They are the part of BPSec two implementations
are most likely to disagree about, and a disagreement shows up as a failed
check with nothing to point at. Building the octets on their own and comparing
is the fastest way to find out who is right.

```go
ippt, err := bpsec.IPPT(bundle, bpsec.ScopeAll, bp.PayloadBlockNumber, bib)
```

### Errors

| Error | Means |
|---|---|
| `ErrIntegrityCheckFailed` | A BIB's recomputed hash does not match the one it carries. |
| `ErrDecryptionFailed` | A BCB's ciphertext did not authenticate. Wrong key, wrong IV, altered ciphertext or altered AAD — AEAD cannot say which, so neither does this. |
| `ErrNoKey` | No key to use: the block carries no wrapped key and none was supplied. |
| `ErrDuplicateSecurityOperation` | The bundle already applies this service to that target (clause 3.2). |
| `ErrIntegrityAfterConfidentiality` | A BIB was added for, or checked against, a target a BCB already encrypts (clause 3.9). |
| `ErrConfidentialityTargetsPrimary` | A BCB tried to target the primary block (clause 3.8). |
| `ErrBCBTargetsUnsharedBIB` | A BCB tried to encrypt a BIB it shares no target with (clause 3.8). |
| `ErrBCBFragmentFlag` | A BCB targets the payload without the replicate-in-every-fragment flag (clause 3.8). |
| `ErrBCBRemovableFlag` | A BCB sets the discard-if-unprocessable flag (clause 3.8). |
| `ErrUnknownContext` | A security context this package does not implement. The block still decodes. |
| `ErrUnknownAESVariant` | An AES variant other than 1 or 3. |
| `ErrIVLength` | An initialisation vector outside 8 to 16 octets. |

## Notes

A BIB gives you integrity, not authentication of a peer. Both contexts here are
symmetric: anyone who can check a hash could also have produced it. RFC 9172
clause 1.1 says this plainly, and clause 3.1 adds that BIBs do not give
hop-by-hop authentication either, because policy at each node differs. If you
need to prove *who* signed something, you need a security context this document
does not define.

The choice of symmetric cryptography is not an oversight. RFC 9173 clause 3.1
gives the reason: a delay-tolerant network may have no way to reach a
certificate authority, so a context that needs one is a context that cannot be
used. This is commentary on the RFC's stated reasoning, not a claim about what
any mission should do.

## Reference

- [RFC 9172, Bundle Protocol Security](https://www.rfc-editor.org/rfc/rfc9172)
- [RFC 9173, Default Security Contexts for BPSec](https://www.rfc-editor.org/rfc/rfc9173)
- [RFC 3394, AES Key Wrap](https://www.rfc-editor.org/rfc/rfc3394)
- [Conformance](/conformance/bpsec) | [Bundle Protocol](/protocols/transport/bp) | [The stack](/docs/start/concepts)
