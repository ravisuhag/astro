// Package bpsec implements Bundle Protocol Security, RFC 9172, with the
// default security contexts of RFC 9173.
//
// BPSec adds two block types to a version 7 bundle. A Block Integrity Block
// (BIB) carries a keyed hash over one or more other blocks, so a receiver can
// tell whether they changed on the way. A Block Confidentiality Block (BCB)
// replaces the contents of its targets with ciphertext. Both name their
// targets by block number, and both may cover several blocks at once.
//
// Security in a delay-tolerant network is not the same problem as security on
// a live link. There is no handshake, because there may be no round trip for
// hours. There is no session, because the two ends are rarely up at the same
// time. What travels is a bundle that already carries everything a receiver
// needs to check it, and that bundle may sit on an intermediate node for a
// long time before moving on. That is why the protection is per block rather
// than per hop, and why a waypoint can verify a BIB without being able to read
// the payload.
//
// # What this package does
//
// It builds, reads and processes security blocks. Add attaches a BIB or a BCB
// to a bundle; Verify and Decrypt do the reverse.
//
//	integrity := bpsec.Integrity{
//	    Variant:     bpsec.HMACSHA512,
//	    Scope:       bpsec.ScopeAll,
//	    Source:      bp.IPN(2, 1),
//	    Key:         key,
//	    BlockNumber: 2,
//	}
//	bib, err := integrity.Add(bundle, bp.PayloadBlockNumber)
//
// The canonicalization both contexts depend on is exported as well: IPPT
// builds the Integrity-Protected Plaintext of RFC 9173 clause 3.7, and AAD
// builds the additional authenticated data of clause 4.7.2. They are the part
// of the standard most likely to disagree between two implementations, so they
// are callable on their own and pinned to the worked examples in
// RFC 9173 appendix A.
//
// # What it does not do
//
// It carries no security policy. RFC 9172 clause 7 leaves to each deployment
// which operations are required, which are optional, and what to do when one
// fails, and that decision needs to be a mission's rather than a library's.
// This package tells a caller whether a security operation succeeded; acting
// on the answer is the caller's job.
//
// It holds no keys. Both contexts take a symmetric key, either directly or
// wrapped inside the security block under a key encryption key the caller
// supplies. Where those keys come from is out of scope for RFC 9172, which
// says so in clause 6.
//
// It does not choose initialisation vectors. RFC 9173 clause 4.6 is blunt
// about the consequence of reusing an IV with the same key: a single reuse
// costs the integrity protection, not just the confidentiality. A library
// generating IVs from its own state would hide that decision. The caller
// passes one in.
//
// # Two contexts, and only two
//
// RFC 9173 defines BIB-HMAC-SHA2 and BCB-AES-GCM, and this package implements
// both and nothing else. A deployment may define its own context under
// RFC 9172 clause 9; a block naming an unknown context identifier decodes
// here, because the Abstract Security Block structure is common to all of
// them, but nothing will process it.
package bpsec
