package sle

import "fmt"

// GET-PARAMETER, per §3.10 of CCSDS 911.1-B-5, 911.2-B-4, 911.5-B-4 and
// 912.1-B-5.
//
// The operation asks the provider for one configuration value. Its
// invocation has the same three fields in all four services — credentials,
// invoke identifier, and the parameter name INTEGER — so one Go type covers
// them. The return's positive alternative differs: each service defines its
// own parameter CHOICE, a large enumeration of provider configuration.
//
// This package carries that CHOICE opaquely. The invocation and both return
// alternatives encode and decode in full; the chosen parameter itself is
// surfaced as its raw BER element, for the caller to interpret against the
// service's ASN.1. A provider with nothing to say answers negatively with
// 'unknown parameter', which the specs define for exactly this.

// GetParameterDiagnostic is the specific alternative of the
// DiagnosticRafGet / DiagnosticRcfGet / DiagnosticRocfGet /
// DiagnosticCltuGetParameter CHOICE. All four define the same single value.
type GetParameterDiagnostic int

// GetParameterUnknown is 'unknownParameter (0)': the provider does not have
// the named parameter.
const GetParameterUnknown GetParameterDiagnostic = 0

// String names the diagnostic.
func (g GetParameterDiagnostic) String() string {
	if g == GetParameterUnknown {
		return "unknown parameter"
	}
	return fmt.Sprintf("diagnostic(%d)", int(g))
}

// GetParameterInvocation is the GET-PARAMETER invocation every service
// shares: RafGetParameterInvocation and its three siblings are the same
// three fields.
type GetParameterInvocation struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// Parameter is the service's ParameterName value, from annex A of the
	// service specification. The names differ per service, so this package
	// does not enumerate them.
	Parameter int
}

// Encode serializes the GET-PARAMETER invocation's content.
func (g *GetParameterInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, g.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(g.InvokeId))
	return AppendInteger(content, int64(g.Parameter)), nil
}

// DecodeGetParameterInvocation parses a GET-PARAMETER invocation's content.
func DecodeGetParameterInvocation(data []byte) (*GetParameterInvocation, error) {
	d := NewDecoder(data)
	g := &GetParameterInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if g.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}
	if g.InvokeId, err = decodeInvokeId(d); err != nil {
		return nil, err
	}

	parameter, err := decodeInt(d)
	if err != nil {
		return nil, err
	}
	g.Parameter = int(parameter)
	return g, nil
}

// Humanize returns a human-readable summary.
func (g *GetParameterInvocation) Humanize() string {
	return fmt.Sprintf("SLE GET-PARAMETER Invocation\n  Invoke ID ... %d\n  Parameter ... %d",
		g.InvokeId, g.Parameter)
}

// GetParameterReturn is the GET-PARAMETER return every service shares in
// shape:
//
//	result CHOICE
//	{ positiveResult [0] <service>GetParameter
//	, negativeResult [1] Diagnostic<service>Get
//	}
//
// The positive alternative's parameter CHOICE is service-specific and large,
// so it travels here as raw BER rather than a per-parameter Go type.
type GetParameterReturn struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// Positive reports whether the provider had the parameter.
	Positive bool
	// Parameter is the chosen alternative of the service's parameter CHOICE,
	// still encoded: one complete BER element. Set when Positive.
	Parameter []byte
	// CommonDiagnostic is set when a refusal used the common alternative.
	CommonDiagnostic Diagnostics
	// SpecificDiagnostic is set when it used the specific one, whose only
	// value is 'unknown parameter'.
	SpecificDiagnostic GetParameterDiagnostic
	// UsedCommon says which alternative a refusal took.
	UsedCommon bool
}

// Encode serializes the GET-PARAMETER return's content.
func (g *GetParameterReturn) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, g.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(g.InvokeId))

	if g.Positive {
		if len(g.Parameter) == 0 {
			return nil, ErrDataTooShort
		}
		// A tag on a CHOICE is explicit even in an implicit-tags module, so
		// [0] wraps the chosen alternative's own element.
		return AppendElement(content, ClassContext, true, 0, g.Parameter), nil
	}
	diagnostic := appendCommonOrSpecific(g.UsedCommon, g.CommonDiagnostic, int64(g.SpecificDiagnostic))
	return AppendElement(content, ClassContext, true, 1, diagnostic), nil
}

// DecodeGetParameterReturn parses a GET-PARAMETER return's content.
func DecodeGetParameterReturn(data []byte) (*GetParameterReturn, error) {
	d := NewDecoder(data)
	g := &GetParameterReturn{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if g.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}
	if g.InvokeId, err = decodeInvokeId(d); err != nil {
		return nil, err
	}

	result, err := d.Next()
	if err != nil {
		return nil, err
	}
	switch {
	case result.IsContext(0):
		g.Positive = true
		g.Parameter = result.Copy()
	case result.IsContext(1):
		usedCommon, v, err := decodeCommonOrSpecific(result)
		if err != nil {
			return nil, err
		}
		g.UsedCommon = usedCommon
		if usedCommon {
			g.CommonDiagnostic = Diagnostics(v)
		} else {
			g.SpecificDiagnostic = GetParameterDiagnostic(v)
		}
	default:
		return nil, ErrInvalidTag
	}
	return g, nil
}

// Humanize returns a human-readable summary.
func (g *GetParameterReturn) Humanize() string {
	if g.Positive {
		return fmt.Sprintf("SLE GET-PARAMETER Return\n  Invoke ID ... %d\n  Result ...... %d octets of parameter",
			g.InvokeId, len(g.Parameter))
	}
	reason := g.SpecificDiagnostic.String()
	if g.UsedCommon {
		reason = g.CommonDiagnostic.String()
	}
	return fmt.Sprintf("SLE GET-PARAMETER Return\n  Invoke ID ... %d\n  Result ...... refused: %s",
		g.InvokeId, reason)
}
