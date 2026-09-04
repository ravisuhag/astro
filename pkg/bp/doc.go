// Package bp implements the Bundle Protocol version 7, RFC 9171.
//
// Bundle Protocol is the store-and-forward layer of Delay-Tolerant Networking.
// Where TCP assumes an end-to-end path that exists right now, BP assumes the
// opposite: links come and go, round trips take minutes or hours, and a node
// holds data until the next contact rather than giving up. That is what makes
// it the networking layer for deep space, and increasingly for lunar relays.
//
// # This is version 7, not an upgrade of version 6
//
// Version 6 (RFC 5050) is a different wire format, not an earlier revision of
// this one. It encodes with SDNV; version 7 encodes with CBOR. The endpoint
// naming, the time base and the block structure all changed. No code and no
// bytes carry over, and nothing here reads a version 6 bundle.
//
// # What this package does
//
// It encodes, decodes and validates bundles. It does not move them. There is
// no convergence layer, no routing, no contact graph and no daemon — those
// need timers, sockets and a network, and astro packages hand that to the
// caller. Compose with pkg/ltp when a bundle needs a transport underneath.
//
// Bundle Protocol Security (RFC 9172) is not here either. It is a separate
// standard and has its own package, pkg/bpsec, which adds integrity and
// confidentiality blocks to a bundle this package built.
//
//	primary := &bp.PrimaryBlock{
//	    CRCType:     bp.CRC32C,
//	    Destination: bp.IPN(1, 2),
//	    Source:      bp.IPN(2, 1),
//	    ReportTo:    bp.IPN(2, 1),
//	    Timestamp:   bp.CreationTimestamp{Time: now, Sequence: 1},
//	    Lifetime:    3600000,
//	}
//	bundle, _ := bp.NewBundle(primary, payload)
//	wire, _ := bundle.Encode()
//
// # Two things the standard makes easy to get wrong
//
// A bundle is a CBOR indefinite-length array. The CDDL grammar in RFC 9171
// appendix B writes it as though it were definite, and the appendix says the
// prose of clause 4.1 wins wherever the two disagree. An implementation that
// trusted the grammar would emit bundles no conforming node accepts, while
// reading its own output back without complaint.
//
// The block-type-specific field of an extension block is a byte string whose
// contents are themselves CBOR — two layers, not one. The constructors and
// accessors here peel both, so callers never see the seam.
//
// # Strictness
//
// Clause 4.1 requires the deterministic CBOR encoding of RFC 8949, and this
// package holds both sides to it: arguments are written in the shortest form
// that fits, and anything longer is refused on the way in. Clause 4.1 does
// permit an implementation to accept sloppy input and repair it. This one does
// not, because quietly accepting a malformed bundle is how two implementations
// come to disagree about what they are exchanging.
//
// Bundle.Validate is stricter than Decode, deliberately. See its documentation
// for why, and for the published example bundle that sits between them.
package bp
