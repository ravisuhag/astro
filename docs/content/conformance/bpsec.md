---
title: Bundle Protocol Security
short: BPSec
description: "Coverage matrix: what this package implements, clause by clause."
order: 55
---

## Conformance Statement for `pkg/bpsec`, RFC 9172 and RFC 9173

RFC 9172 ships no PICS proforma, so what follows is a coverage matrix in the
same shape as the rest of this section. Status **M** marks what the RFC makes
mandatory for an implementation of the feature, **O** what it leaves optional.

---

## A1.1 GENERAL INFORMATION

### A1.1.1 Identification

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 04/09/2026 |
| Serial Number | ASTRO-BPSEC-COV-001 |
| System Conformance Statement Cross-Reference | This document |

### A1.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/bpsec |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing the BPSec block formats and the two default security contexts of RFC 9173: the Abstract Security Block, the Block Integrity Block, the Block Confidentiality Block, both canonicalization algorithms, and AES Key Wrap. Security policy and key management are out of scope, as are bundle agent behaviour and any convergence layer. |

### A1.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub, github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/bpsec (Go package) |
| System Name(s) | Astro |

### A1.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | RFC 9172 (Bundle Protocol Security, Standards Track, January 2022) with RFC 9173 (Default Security Contexts for BPSec, Standards Track, January 2022) and RFC 3394 (AES Key Wrap, Informational, September 2002) |
| Have any exceptions been required? | Yes [X] No [ ], see A1.6 |

---

## A1.2 SECURITY BLOCK STRUCTURE

| Feature | Reference | Status | Support |
|---|---|---|---|
| BIB block type code 11 | 9172 clause 11.1 | M | Y |
| BCB block type code 12 | 9172 clause 11.1 | M | Y |
| Security blocks use the canonical block format | 9172 clause 3.5 | M | Y: they are `bp.CanonicalBlock` values |
| Abstract Security Block field order | 9172 clause 3.6 | M | Y |
| Targets as a CBOR array of unsigned integers | 9172 clause 3.6 | M | Y |
| At least one target | 9172 clause 3.6 | M | Y: `ErrNoTargets` |
| No duplicate targets | 9172 clause 3.6 | M | Y: `ErrDuplicateTarget` |
| Every target names a block that exists | 9172 clause 3.6 | M | Y: `ErrTargetNotInBundle` |
| Targets named by block number | 9172 clause 3.4 | M | Y: block number 0 is the primary block |
| Context flags bit 0 means parameters present | 9172 clause 3.6 | M | Y |
| Reserved context flag bits written as zero | 9172 clause 3.6 | M | Y: `ErrReservedContextFlag` on encode |
| Reserved context flag bits ignored on read | 9172 clause 3.6 | M | N: refused instead, see A1.6 |
| Security source as an endpoint ID | 9172 clause 3.6 | M | Y: decoded by `pkg/bp`, so the `dtn` and `ipn` rules are shared |
| Parameters as [id, value] two-tuples | 9172 clause 3.6 | M | Y: values held as raw CBOR |
| Results ordered to match the targets | 9172 clause 3.6 | M | Y: one set per target, `ErrResultCountMismatch` otherwise |
| One security block may carry several operations | 9172 clause 3.3 | O | Y |

---

## A1.3 BLOCK INTERACTION RULES

| Feature | Reference | Status | Support |
|---|---|---|---|
| Security operations in a bundle are unique | 9172 clause 3.2 | M | Y: `ErrDuplicateSecurityOperation` |
| A BIB must not target a security block | 9172 clause 3.7 | M | Y: `ErrIntegrityTargetsSecurityBlock` |
| A BCB must not target another BCB | 9172 clause 3.8 | M | Y: `ErrConfidentialityTargetsBCB` |
| A BCB must not target the primary block | 9172 clause 3.8 | M | Y: `ErrConfidentialityTargetsPrimary` |
| A BCB targets a BIB only when they share a target | 9172 clause 3.8 | M | Y: `ErrBCBTargetsUnsharedBIB` |
| A BCB targeting the payload sets the fragment replication flag | 9172 clause 3.8 | M | Y: `ErrBCBFragmentFlag` |
| A BCB must not set the discard-if-unprocessable flag | 9172 clause 3.8 | M | Y: `ErrBCBRemovableFlag` |
| A BIB must not be added for a target a BCB encrypts | 9172 clause 3.9 | M | Y: `ErrIntegrityAfterConfidentiality` |
| A BIB is not checked when its target is encrypted | 9172 clause 3.9 | M | Y: `Verify` refuses |
| A BIB is not checked when the BIB itself is encrypted | 9172 clause 3.9 | M | Y: an encrypted BIB is skipped when scanning, not treated as malformed |
| A BCB encrypts its target in place | 9172 clause 3.8 | M | Y |
| When a BIB is encrypted at the same time, both go in one bundle | 9172 clause 3.9 | M | Y: either as a second BCB target or a separate BCB, at the caller's choice |
| Automatic splitting of a partly-covered BIB | 9172 clause 3.9 | M | N: refused instead, see A1.6 |

---

## A1.4 CANONICAL FORMS

| Feature | Reference | Status | Support |
|---|---|---|---|
| Deterministic CBOR in canonical forms | 9172 clause 4 | M | Y: shared with `pkg/bp`, which emits shortest-form arguments |
| A canonical data field includes its own CBOR encoding | 9172 clause 4 | M | Y: the IPPT quotes the block-type-specific data with its byte string head |
| No enclosing CBOR in a canonical field | 9172 clause 4 | M | Y: the array framing around a block is not included |
| Reserved and unassigned block flags zeroed in canonical form | 9172 clause 4 | M | Y: masked to bits 0, 1, 2 and 4 |
| Only the block-type-specific data is encrypted | 9172 clause 4 | M | Y |
| IPPT step order and content | 9173 clause 3.7 | M | Y: pinned to appendices A.1, A.3 and A.4 |
| IPPT skips the primary and target header steps when the target is the primary block | 9173 clause 3.7 | M | Y: pinned to appendix A.3 |
| AAD step order and content | 9173 clause 4.7.2 | M | Y: pinned to appendices A.2 and A.4 |
| Scope flags always included in the IPPT and the AAD | 9173 clauses 3.7, 4.7.2 | M | Y |
| BCB plaintext excludes the CBOR byte string head | 9173 clause 4.7.1 | M | Y |

---

## A1.5 SECURITY CONTEXTS

### BIB-HMAC-SHA2, context identifier 1

| Feature | Reference | Status | Support |
|---|---|---|---|
| HMAC 256/256, 384/384, 512/512 | 9173 clause 3.3.1 | M | Y: variants 5, 6 and 7 |
| Output length equals the SHA-2 size | 9173 clause 3.1 | M | Y: checked on verify, `ErrHMACLength` |
| Default variant 6 when the parameter is absent | 9173 clause 3.3.1 | O | Y |
| Wrapped key parameter, number 2 | 9173 clause 3.3.2 | O | Y: RFC 3394 |
| Integrity scope flags, number 3 | 9173 clause 3.3.3 | O | Y |
| Default scope 7 when the parameter is absent | 9173 clause 3.3.3 | O | Y |
| Reserved and unassigned scope bits zeroed | 9173 clause 3.3.3 | M | Y: `ErrReservedScopeFlag` |
| Expected HMAC as security result 1 | 9173 clause 3.4 | M | Y |
| Constant-time tag comparison | 9173 clause 3.6 | O | Y: `hmac.Equal` |
| Target CRC removed before the hash | 9173 clause 3.8.1 | M | Y |
| Key length equal to the HMAC output | 9173 clause 3.5 | M | N: not enforced, see A1.6 |
| CRC restored at a non-destination security acceptor | 9173 clause 3.8.2 | M | N: not implemented, see A1.6 |

### BCB-AES-GCM, context identifier 2

| Feature | Reference | Status | Support |
|---|---|---|---|
| A128GCM and A256GCM | 9173 clause 4.3.2 | M | Y: variants 1 and 3. Value 2 is not assigned and is refused |
| Default variant 3 when the parameter is absent | 9173 clause 4.3.2 | O | Y |
| Initialisation vector 8 to 16 octets, number 1 | 9173 clause 4.3.1 | O | Y: `ErrIVLength` |
| A missing IV treated as an error | 9173 clause 4.8.2 | O | Y: `ErrMissingIV` |
| Wrapped key parameter, number 3 | 9173 clause 4.3.3 | O | Y: RFC 3394 |
| AAD scope flags, number 4 | 9173 clause 4.3.4 | O | Y |
| Authentication tag of 128 bits | 9173 clause 4.4.1 | M | Y: `ErrTagLength` |
| Tag carried as security result 1 | 9173 clause 4.4 | O | Y: always, rather than appended to the ciphertext |
| Tag appended to the ciphertext instead | 9173 clause 4.4 | O | N: not implemented, see A1.6 |
| Ciphertext replaces the plaintext in place | 9173 clause 4.8.1 | M | Y |
| Target CRC removed before encryption | 9173 clause 4.8.1 | M | Y |
| Plaintext restored on decryption | 9173 clause 4.8.2 | M | Y |
| A failed authentication yields no plaintext | 9173 clause 4.8.2 | M | Y: the bundle is left as it arrived |

### AES Key Wrap

| Feature | Reference | Status | Support |
|---|---|---|---|
| Wrap and unwrap, index-based form | 3394 clauses 2.2.1, 2.2.2 | M | Y |
| Default initial value A6A6A6A6A6A6A6A6 | 3394 clause 2.2.3.1 | M | Y |
| Key data of at least two 64-bit blocks | 3394 clause 2 | M | Y: `ErrKeyDataLength` |
| No key data returned when the check fails | 3394 clause 2.2.2 | M | Y: `ErrIntegrityCheck` |
| Alternative initial values | 3394 clause 2.2.3.2 | O | N: the default only |

---

## A1.6 EXCEPTIONS AND UNSUPPORTED FEATURES

**Security policy is not implemented.** RFC 9172 clause 7 leaves to each
deployment which security operations are required, which are optional, and what
happens when one fails. This package reports success or failure and takes no
action on the result. The status report reason codes of clause 7.1 are
therefore not generated here either; they belong to whatever layer holds the
policy.

**Key management is not implemented.** RFC 9172 clause 6 places it out of
scope, and this package agrees. Keys are supplied by the caller.

**Reserved security context flag bits are refused rather than ignored.**
Clause 3.6 asks a reader to ignore them. This package returns
`ErrReservedContextFlag`. The reasoning matches the rest of astro: a flag this
package does not understand may change how the block should be processed, and
silently proceeding is the failure mode the project exists to avoid. A
deployment that needs the lenient behaviour can read the raw block instead.

**A partly-covered BIB is not split automatically.** Clause 3.9 says that when
a BCB covers some but not all of a BIB's targets, the affected results must be
moved into a new BIB and that new BIB encrypted. Doing this on the caller's
behalf would silently restructure their bundle and re-source a security block
this node did not originate. `Confidentiality.Add` returns
`ErrBCBTargetsUnsharedBIB` instead, and the caller does the split.

**The HMAC key length rule of clause 3.5 is not enforced.** The clause requires
a key as long as the hash output. RFC 9173 appendix A.1 then signs with
HMAC 512/512 under a 16-octet key. Enforcing the clause would reject the
document's own worked example and every implementation pinned to it. HMAC
accepts any key length by construction, so a short key works and is weaker than
the clause intends. Callers who want the rule check before calling.

**The authentication tag is always a security result.** Clause 4.4 allows a tag
to be appended to the ciphertext instead, and requires the target block to be
resized when it is. This package always writes the tag as security result 1,
which is what every worked example in appendix A does. It reads only that form
too, and returns `ErrMissingTag` for a BCB that carries none.

**A CRC is not restored when a security service is removed.** Clauses 3.8.2 and
4.8.2 say that a security acceptor which is not the bundle destination must add
a CRC back to a target it has finished with. The CRC type is a policy choice
those clauses defer to the deployment, and this package holds no policy. The
caller sets `CRCType` on the target after calling `Verify` or `Decrypt`.

**Fragmentation of a bundle carrying security blocks is not handled here.**
RFC 9172 clause 5.2 leaves the rules to the bundle agent, and fragmentation
lives in `pkg/bp`.

---

## A1.7 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| Security context identifier | 0 to 2^64 - 1 as decoded | RFC 9172 clause 11.3 gives the registry a signed 16-bit range; the field itself is a CBOR unsigned integer and is not narrowed here |
| Scope flags | 16 bits | RFC 9173 clauses 3.3.3 and 4.3.4 |
| Initialisation vector | 8 to 16 octets | RFC 9173 clause 4.3.1 |
| Authentication tag | 16 octets | RFC 9173 clause 4.4.1 |
| Key data for AES Key Wrap | a whole number of 8-octet blocks, at least two | RFC 3394 clause 2 |
| Targets per security block | bounded by the input slice | No ceiling is imposed; a decoder reads from a caller-supplied slice |

---

## Wire test vectors

The octets backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/bpsec) — 13 vectors, plus 6 more for AES Key Wrap. Each names the clause it comes from and carries the derivation that produced it.

| File | |
|---|---|
| [`bpsec/security.json`](https://github.com/ravisuhag/astro/blob/main/vectors/bpsec/security.json) | 13 vectors |
| [`keywrap/keywrap.json`](https://github.com/ravisuhag/astro/blob/main/vectors/keywrap/keywrap.json) | 6 vectors |

Nearly all of them are **published octets rather than derived values.** RFC 9173 appendix A prints four worked examples with their keys, their intermediate canonical forms and the blocks that come out; RFC 3394 clause 4 prints six key wrap cases. Both are outside corroboration — different working groups wrote those bytes.

Two vectors run against a bundle assembled from pieces the appendix prints separately rather than one it prints whole, and say so in their own note.

These are data files, so any implementation can check itself against the same octets. See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
