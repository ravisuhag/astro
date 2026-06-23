# PICS PROFORMA FOR SPACE DATA LINK SECURITY

## Conformance Statement for `pkg/sdls` — CCSDS 355.0-B-2

---

## A1.1 GENERAL INFORMATION

### A1.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 23/08/2026 |
| PICS Serial Number | ASTRO-SDLS-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A1.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/sdls |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing the CCSDS Space Data Link Security Protocol. Ships the §E1 baseline: AES-256-GCM authenticated encryption with a 96-bit IV and a 128-bit MAC, plus GMAC for authentication without confidentiality. Composes with `pkg/tmdl`, `pkg/tcdl`, `pkg/aos`, and `pkg/usdl` from the outside — the protected data field is built here and handed to the frame constructor. |

### A1.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/sdls (Go package) |
| System Name(s) | Astro |

### A1.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 355.0-B-2 (Space Data Link Security Protocol, Blue Book, Issue 2, July 2022) |
| Have any exceptions been required? | Yes [X] No [ ] — see A1.5 |

---

## A1.2 PROTOCOL DATA UNITS

| Feature | Reference | Status | Support |
|---|---|---|---|
| Security Header | §4.1.1 | M | Y — SPI, IV, Sequence Number, Pad Length, contiguous and in that order |
| Security Parameter Index | §4.1.1.2 | M | Y — 16 bits, big-endian, leading field |
| SPI reserved values rejected | §4.1.1.2.3 | M | Y — 0 and 65535 rejected by `SecurityAssociation.Validate` |
| Initialization Vector field | §4.1.1.3 | O | Y — width managed per SA; 12 octets for the GCM baseline |
| Sequence Number field | §4.1.1.4 | O | Y — width managed per SA; zero octets when the IV serves as the counter |
| Pad Length field | §4.1.1.5 | O | Y — carried and honored on receive; never generated (GCM needs no padding) |
| Security Header maximum 64 octets | §4.1.1.1.4 | M | Y — enforced by `FieldLengths.Validate` |
| Security Trailer | §4.1.2 | O | Y — MAC, fixed length per SA |

---

## A1.3 SECURITY ASSOCIATION

| Feature | Reference | Status | Support |
|---|---|---|---|
| One service type per SA | §4.2.2.4 | M | Y — `Mode` is exactly one of authentication, encryption, authenticated encryption |
| Common SA parameters | §4.2.2.5 | M | Y — SPI plus IV, Sequence Number, Pad Length, and MAC widths |
| Authentication algorithm and mode | §4.2.2.6.1 a | M | Y — AES-256-GCM / GMAC |
| Authentication bit mask | §4.2.2.6.1 b, §4.2.2.6.2 | M | Y — `AuthMask`, applied bitwise-AND before MAC computation |
| IV excluded from authenticated data | §4.2.2.6.2 h | M | Y — enforced in code regardless of the mask supplied |
| Managed anti-replay sequence number | §4.2.2.6.1 c | M | Y — sender counter; receiver stored value |
| Managed sequence number window | §4.2.2.6.1 d | M | Y — `SeqWindow` |
| Managed initialization vector | §4.2.2.7 b | M | Y — big-endian counter, never reused |

---

## A1.4 PROCEDURES

| Feature | Reference | Status | Support |
|---|---|---|---|
| ApplySecurity — encryption | §4.2.3.2.2.1 | O | N — see A1.5 |
| ApplySecurity — authentication | §4.2.3.2.2.2 | O | Y — data field unencrypted, MAC in the trailer |
| ApplySecurity — authenticated encryption | §4.2.3.2.2.3 | O | Y — AEAD split: data field is plaintext, masked prefix is the AAD |
| Sequence number incremented per frame | §4.2.3.4 a | M | Y |
| Authentication bit mask applied | §4.2.3.4 d | M | Y |
| MAC truncation to trailer width | §4.2.3.4 f | O | Y — 12 to 16 octets, most significant bits kept |
| ProcessSecurity — SA verification | §4.2.4.3 | M | Y — unknown SPI rejected before any cryptographic work |
| ProcessSecurity — authentication | §4.2.4.2.3.1 | O | Y |
| ProcessSecurity — authenticated encryption | §4.2.4.2.3.2 | O | Y |
| No data returned on verification failure | §4.2.4.2.3 | M | Y — every failure path returns a nil data field |
| Anti-replay check | §2.3.2.3 | M | Y — applied only after the MAC verifies |

---

## A1.5 EXCEPTIONS AND UNSUPPORTED FEATURES

| Feature | Reference | Support | Rationale |
|---|---|---|---|
| AES-CMAC authentication (TC baseline) | §E2.1 | N | Absent from the Go standard library; this package takes no external dependencies. Tracked as `TODO(sdls): AES-CMAC`. |
| Encryption without authentication | §2.3.3, §4.2.3.2.2.1 | N | `ApplySecurity` returns `ErrUnsupportedMode`. §2.3.3 itself warns that encryption without authentication can give a false sense of security. |
| Block padding generation | §4.2.3.3 b | N | GCM is a stream mode and needs none (§E1.2 note 2). A non-zero Pad Length is still honored on receive. |
| SDLS Extended Procedures (key management, OTAR) | CCSDS 355.1 | N | A separate standard, out of scope for this package. |
| Over-the-air SA negotiation | §2.3.1.5 note | N | An Application Layer function, undefined by the standard. |

---

## A1.6 BASELINE MODE CONFORMANCE

The §E1 (TM), §E3 (AOS), and §E4 (USLP) baselines are supported in full.

| Baseline parameter | Reference | Value | Support |
|---|---|---|---|
| Algorithm | §E1.1 | AES-GCM | Y |
| Key length | §E1.1 a | 256 bits | Y — exactly 32 octets, enforced |
| IV length | §E1.1 b | 96 bits, transmitted in-line | Y |
| MAC length | §E1.1 c | 128 bits | Y |
| Security Header length | §E1.2 | 14 octets | Y |
| Security Trailer length | §E1.3 | 16 octets | Y |
| Sequence Number field | §E1.2 note 1 | 0 octets — IV serves as the counter | Y |
| Pad Length field | §E1.2 note 2 | 0 octets | Y |

The §E2 (TC) baseline is **not** supported: it specifies AES-CMAC. See A1.5.
