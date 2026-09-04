package csts_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/csts"
)

// The vectors for this package live in vectors/csts/.
//
// They are derived rather than published, and the corpus note says so.
// CCSDS 921.1-B-2 prints no worked example and no octets: it is an abstract
// specification with an ASN.1 annex, so there is nothing to transcribe. Each
// vector's note carries the derivation from annex F instead, octet by octet,
// so a reader can check it against the module rather than against this
// package.
func TestCSTSVectors(t *testing.T) {
	vectors.RunFile(t, "csts/framework.json", vectors.Impl{
		EncodeFn: encodeVector,
		DecodeFn: decodeVector,
	})
}

func encodeVector(f, _ vectors.Fields) ([]byte, error) {
	pdu, err := pduFromFields(f)
	if err != nil {
		return nil, err
	}
	return pdu.Encode()
}

func decodeVector(input []byte, _ vectors.Fields) (vectors.Fields, error) {
	pdu, err := csts.Decode(input)
	if err != nil {
		return nil, err
	}
	return pduFields(pdu), nil
}

// pduFromFields builds a PDU from a vector's fields. Only the operations a
// vector needs are built; the rest are covered by the package's own tests.
func pduFromFields(f vectors.Fields) (*csts.PDU, error) {
	operation, err := f.Uint("operation")
	if err != nil {
		return nil, err
	}
	pdu := &csts.PDU{Type: csts.OperationType(operation)}

	switch pdu.Type {
	case csts.OpPeerAbortInvocation:
		diagnostic, err := f.Uint("peer_abort_diagnostic")
		if err != nil {
			return nil, err
		}
		pdu.PeerAbort = &csts.PeerAbortInvocation{
			Diagnostic: csts.PeerAbortDiagnostic(diagnostic),
		}
		return pdu, nil

	case csts.OpUnbindInvocation:
		header, err := invocationHeaderFromFields(f)
		if err != nil {
			return nil, err
		}
		pdu.Unbind = &csts.UnbindInvocation{Header: header}
		return pdu, nil

	case csts.OpStartInvocation:
		header, err := invocationHeaderFromFields(f)
		if err != nil {
			return nil, err
		}
		pdu.Start = &csts.StartInvocation{Header: header}
		return pdu, nil

	case csts.OpUnbindReturn:
		header, err := returnHeaderFromFields(f)
		if err != nil {
			return nil, err
		}
		pdu.UnbindReturn = &csts.UnbindReturn{Header: header}
		return pdu, nil

	case csts.OpStartReturn:
		header, err := returnHeaderFromFields(f)
		if err != nil {
			return nil, err
		}
		pdu.Return = &csts.StandardReturn{Header: header}
		return pdu, nil
	}
	return nil, errUnknownVectorOperation
}

func invocationHeaderFromFields(f vectors.Fields) (csts.InvocationHeader, error) {
	invokeID, err := f.Uint("invoke_id")
	if err != nil {
		return csts.InvocationHeader{}, err
	}
	procedure, err := procedureFromFields(f)
	if err != nil {
		return csts.InvocationHeader{}, err
	}
	return csts.InvocationHeader{
		InvokeID:  uint32(invokeID),
		Procedure: procedure,
	}, nil
}

func returnHeaderFromFields(f vectors.Fields) (csts.ReturnHeader, error) {
	invokeID, err := f.Uint("invoke_id")
	if err != nil {
		return csts.ReturnHeader{}, err
	}
	positive, err := f.Bool("positive")
	if err != nil {
		return csts.ReturnHeader{}, err
	}
	h := csts.ReturnHeader{InvokeID: uint32(invokeID), Positive: positive}
	if !positive {
		text, err := f.Str("diagnostic_text")
		if err != nil {
			return h, err
		}
		h.Diagnostic = csts.Diagnostic{Kind: csts.DiagnosticOtherReason, Text: text}
	}
	return h, nil
}

func procedureFromFields(f vectors.Fields) (csts.ProcedureName, error) {
	role, err := f.Str("procedure_role")
	if err != nil {
		return csts.ProcedureName{}, err
	}
	typeName, err := f.Str("procedure_type")
	if err != nil {
		return csts.ProcedureName{}, err
	}

	p := csts.ProcedureName{}
	switch typeName {
	case "associationControl":
		p.Type = csts.OIDAssociationControl
	case "cyclicReport":
		p.Type = csts.OIDCyclicReport
	case "bufferedDataDelivery":
		p.Type = csts.OIDBufferedDataDelivery
	default:
		return p, errUnknownVectorOperation
	}

	switch role {
	case "prime":
		p.Role = csts.RolePrime
	case "association control":
		p.Role = csts.RoleAssociationControl
	case "secondary":
		p.Role = csts.RoleSecondary
		instance, err := f.Uint("procedure_instance")
		if err != nil {
			return p, err
		}
		p.Instance = uint32(instance)
	default:
		return p, errUnknownVectorOperation
	}
	return p, nil
}

// pduFields reports what a decode vector may compare.
func pduFields(p *csts.PDU) vectors.Fields {
	f := vectors.Fields{
		"operation":      uint64(p.Type),
		"operation_name": p.Type.String(),
	}

	if header, ok := p.Header(); ok {
		f["invoke_id"] = uint64(header.InvokeID)
		f["procedure_role"] = header.Procedure.Role.String()
		f["procedure_oid"] = header.Procedure.Type.String()
		f["procedure_name"] = csts.ProcedureTypeName(header.Procedure.Type)
		f["credentials_used"] = header.InvokerCredentials.Used
		if header.Procedure.Role == csts.RoleSecondary {
			f["procedure_instance"] = uint64(header.Procedure.Instance)
		}
	}
	if header, ok := p.ReturnHeader(); ok {
		f["invoke_id"] = uint64(header.InvokeID)
		f["positive"] = header.Positive
		f["credentials_used"] = header.PerformerCredentials.Used
		if !header.Positive {
			f["diagnostic"] = header.Diagnostic.String()
		}
	}
	if p.PeerAbort != nil {
		f["peer_abort_diagnostic"] = uint64(p.PeerAbort.Diagnostic)
		f["peer_abort_name"] = p.PeerAbort.Diagnostic.String()
		f["peer_abort_origin"] = p.PeerAbort.Diagnostic.Origin().String()
	}
	return f
}

var errUnknownVectorOperation = errors.New("csts: vector names an operation this runner does not build")
