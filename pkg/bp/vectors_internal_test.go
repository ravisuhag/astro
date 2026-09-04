package bp

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
)

// The wire vectors for this package live in vectors/bp/bundle.json.
//
// Most of them are published rather than derived. RFC 9173 appendix A prints
// four worked example bundles beside their hex, so the primary block, the
// payload block, the Bundle Age block and a whole bundle are all checked
// against octets a different working group wrote. That is outside
// corroboration, which almost nothing else in this corpus has.
//
// Which structure a vector's octets hold comes from its config, not from the
// octets: a bare CBOR array does not announce whether it is a primary block,
// a canonical block or an endpoint ID. That is the same reasoning the contract
// records for LV and TLV in pkg/pus.
//
// Not covered here: fragmentation and reassembly need a sequence of calls
// across several bundles, which no vector form expresses. They are covered by
// fragment_internal_test.go.
func TestBundleVectors(t *testing.T) {
	vectors.RunFile(t, "bp/bundle.json", vectors.Impl{
		EncodeFn: encodeVector,
		DecodeFn: decodeVector,
	})
}

func structureOf(config vectors.Fields) (string, error) {
	return config.Str("structure")
}

func encodeVector(f, config vectors.Fields) ([]byte, error) {
	structure, err := structureOf(config)
	if err != nil {
		return nil, err
	}

	switch structure {
	case "eid":
		e, err := eidFromFields(f)
		if err != nil {
			return nil, err
		}
		return appendEID(nil, e)

	case "primary_block":
		p, err := primaryFromFields(f)
		if err != nil {
			return nil, err
		}
		return appendPrimaryBlock(nil, p)

	case "canonical_block":
		b, err := blockFromFields(f)
		if err != nil {
			return nil, err
		}
		return appendCanonicalBlock(nil, b)

	case "bundle":
		return bundleFromFields(f)

	case "status_report":
		r, err := statusReportFromFields(f)
		if err != nil {
			return nil, err
		}
		return r.Encode()
	}
	return nil, errUnknownVectorStructure
}

// decodeVector runs the shipped decoders, so a reject vector proves the real
// code refuses the input rather than a test-only reimplementation of it.
func decodeVector(input []byte, config vectors.Fields) (vectors.Fields, error) {
	structure, err := structureOf(config)
	if err != nil {
		return nil, err
	}

	switch structure {
	case "eid":
		_, err := newDecoder(input).eid()
		return vectors.Fields{}, err
	case "primary_block":
		_, err := newDecoder(input).primaryBlock()
		return vectors.Fields{}, err
	case "canonical_block":
		_, err := newDecoder(input).canonicalBlock()
		return vectors.Fields{}, err
	case "bundle":
		_, err := Decode(input)
		return vectors.Fields{}, err
	case "status_report":
		_, err := DecodeStatusReport(input)
		return vectors.Fields{}, err
	}
	return nil, errUnknownVectorStructure
}

func eidFromFields(f vectors.Fields) (EID, error) {
	scheme, err := f.Uint("scheme")
	if err != nil {
		return EID{}, err
	}
	if SchemeCode(scheme) == SchemeDTN {
		ssp, err := f.Str("ssp")
		if err != nil {
			return EID{}, err
		}
		return DTN(ssp), nil
	}

	allocator, err := f.Uint("allocator")
	if err != nil {
		return EID{}, err
	}
	node, err := f.Uint("node")
	if err != nil {
		return EID{}, err
	}
	service, err := f.Uint("service")
	if err != nil {
		return EID{}, err
	}
	return IPNWithAllocator(allocator, node, service), nil
}

func primaryFromFields(f vectors.Fields) (*PrimaryBlock, error) {
	var (
		p   PrimaryBlock
		err error
	)
	read := func(name string, into *uint64) {
		if err != nil {
			return
		}
		*into, err = f.Uint(name)
	}

	var flags, crcType, destNode, destSvc, srcNode, srcSvc, repNode, repSvc, created, sequence uint64
	read("flags", &flags)
	read("crc_type", &crcType)
	read("destination_node", &destNode)
	read("destination_service", &destSvc)
	read("source_node", &srcNode)
	read("source_service", &srcSvc)
	read("report_to_node", &repNode)
	read("report_to_service", &repSvc)
	read("creation_time", &created)
	read("sequence_number", &sequence)
	read("lifetime", &p.Lifetime)
	if err != nil {
		return nil, err
	}

	p.Flags = BundleControlFlags(flags)
	p.CRCType = CRCType(crcType)
	p.Destination = IPN(destNode, destSvc)
	p.Source = IPN(srcNode, srcSvc)
	p.ReportTo = IPN(repNode, repSvc)
	p.Timestamp = CreationTimestamp{Time: DTNTime(created), Sequence: sequence}
	return &p, nil
}

func blockFromFields(f vectors.Fields) (*CanonicalBlock, error) {
	blockType, err := f.Uint("block_type")
	if err != nil {
		return nil, err
	}
	number, err := f.Uint("block_number")
	if err != nil {
		return nil, err
	}
	flags, err := f.Uint("flags")
	if err != nil {
		return nil, err
	}
	crcType, err := f.Uint("crc_type")
	if err != nil {
		return nil, err
	}

	var block *CanonicalBlock
	if BlockType(blockType) == BlockTypeBundleAge {
		age, err := f.Uint("age_milliseconds")
		if err != nil {
			return nil, err
		}
		if block, err = NewBundleAgeBlock(number, age); err != nil {
			return nil, err
		}
	} else {
		data, err := f.Hex("data")
		if err != nil {
			return nil, err
		}
		block = &CanonicalBlock{Type: BlockType(blockType), Number: number, Data: data}
	}

	block.Flags = BlockControlFlags(flags)
	block.CRCType = CRCType(crcType)
	return block, nil
}

// bundleFromFields assembles the bundle without going through Encode.
//
// Encode would refuse this one: its creation time is unknown and it carries no
// Bundle Age block, which clause 4.4.2 requires of a bundle being created.
// RFC 9173's published example does not satisfy that rule — see the note on
// Bundle.Validate — and the point of the vector is the octets.
func bundleFromFields(f vectors.Fields) ([]byte, error) {
	p, err := primaryFromFields(f)
	if err != nil {
		return nil, err
	}
	payload, err := f.Hex("payload")
	if err != nil {
		return nil, err
	}

	out := appendIndefiniteArrayHeader(nil)
	if out, err = appendPrimaryBlock(out, p); err != nil {
		return nil, err
	}
	if out, err = appendCanonicalBlock(out, NewPayloadBlock(payload)); err != nil {
		return nil, err
	}
	return appendBreak(out), nil
}

func statusReportFromFields(f vectors.Fields) (*StatusReport, error) {
	var (
		r   StatusReport
		err error
	)
	item := func(name string, into *StatusItem) {
		if err != nil {
			return
		}
		var asserted bool
		if asserted, err = f.Bool(name); err == nil {
			into.Asserted = asserted
		}
	}
	item("received", &r.Received)
	item("forwarded", &r.Forwarded)
	item("delivered", &r.Delivered)
	item("deleted", &r.Deleted)
	if err != nil {
		return nil, err
	}

	reason, err := f.Uint("reason")
	if err != nil {
		return nil, err
	}
	r.Reason = StatusReportReason(reason)

	node, err := f.Uint("subject_source_node")
	if err != nil {
		return nil, err
	}
	service, err := f.Uint("subject_source_service")
	if err != nil {
		return nil, err
	}
	r.SubjectSource = IPN(node, service)

	created, err := f.Uint("subject_creation_time")
	if err != nil {
		return nil, err
	}
	sequence, err := f.Uint("subject_sequence_number")
	if err != nil {
		return nil, err
	}
	r.SubjectTimestamp = CreationTimestamp{Time: DTNTime(created), Sequence: sequence}
	return &r, nil
}
