package csts

import (
	"github.com/ravisuhag/astro/internal/ber"

	"strings"
)

// Diagnostic says why an operation returned a negative result
// (annex F3.3, and clause 3.2.1.7 for the accompanying text).
//
// The four defined alternatives cover what any operation can go wrong with.
// A procedure that needs to say something else extends the CHOICE through
// alternative [100], which carries an EMBEDDED PDV whose syntax the procedure
// names — so this package reports that one as raw octets rather than guessing
// at a syntax it was not given.
type Diagnostic struct {
	Kind DiagnosticKind
	// Text is the AdditionalText that accompanies every alternative but the
	// extension.
	Text string
	// Appellations names the parameters a conflictingValues diagnostic is
	// about, or the single parameter of an invalidParameterValue.
	//
	// Annex F3.3 notes that an appellation is not formally agreed between the
	// two parties, so it is for logging rather than for acting on.
	Appellations []string
	// Extension is the encoded EMBEDDED PDV of a diagnosticExtension.
	Extension []byte
}

// DiagnosticKind is which alternative of the Diagnostic CHOICE arrived.
type DiagnosticKind int

const (
	// DiagnosticInvalidParameterValue names one parameter whose value the
	// performer would not take.
	DiagnosticInvalidParameterValue DiagnosticKind = iota
	// DiagnosticConflictingValues names parameters that are each acceptable
	// alone and not acceptable together.
	DiagnosticConflictingValues
	// DiagnosticOtherReason is free text.
	DiagnosticOtherReason
	// DiagnosticUnsupportedOption is an option the performer does not have.
	DiagnosticUnsupportedOption
	// DiagnosticExtension is a procedure's own diagnostic, carried as an
	// EMBEDDED PDV this package does not interpret.
	DiagnosticExtension
)

// The context tags of the Diagnostic CHOICE in annex F3.3.
const (
	tagDiagInvalidParameterValue uint32 = 0
	tagDiagConflictingValues     uint32 = 1
	tagDiagOtherReason           uint32 = 2
	tagDiagUnsupportedOption     uint32 = 3
	tagDiagExtension             uint32 = 100
)

func (k DiagnosticKind) String() string {
	switch k {
	case DiagnosticInvalidParameterValue:
		return "invalid parameter value"
	case DiagnosticConflictingValues:
		return "conflicting values"
	case DiagnosticOtherReason:
		return "other reason"
	case DiagnosticUnsupportedOption:
		return "unsupported option"
	case DiagnosticExtension:
		return "procedure-specific diagnostic"
	}
	return "unknown"
}

// String renders a diagnostic as one readable line.
func (d Diagnostic) String() string {
	var sb strings.Builder
	sb.WriteString(d.Kind.String())
	if d.Text != "" {
		sb.WriteString(": ")
		sb.WriteString(d.Text)
	}
	if len(d.Appellations) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(d.Appellations, ", "))
		sb.WriteString(")")
	}
	return sb.String()
}

// Appellation length limits, from the SIZE constraint in annex F3.3.
const (
	MinAppellationLength = 1
	MaxAppellationLength = 128
)

func appendDiagnostic(dst []byte, d Diagnostic) ([]byte, error) {
	switch d.Kind {
	case DiagnosticInvalidParameterValue:
		if len(d.Appellations) != 1 {
			// The alternative is a SEQUENCE of one text and one appellation.
			return nil, ErrMalformedDiagnostic
		}
		if err := checkAppellation(d.Appellations[0]); err != nil {
			return nil, err
		}
		content := ber.AppendVisibleString(nil, d.Text)
		content = ber.AppendVisibleString(content, d.Appellations[0])
		return ber.AppendElement(dst, ber.ClassContext, true, tagDiagInvalidParameterValue, content), nil

	case DiagnosticConflictingValues:
		content := ber.AppendVisibleString(nil, d.Text)
		var list []byte
		for _, appellation := range d.Appellations {
			if err := checkAppellation(appellation); err != nil {
				return nil, err
			}
			list = ber.AppendVisibleString(list, appellation)
		}
		content = ber.AppendSequence(content, list)
		return ber.AppendElement(dst, ber.ClassContext, true, tagDiagConflictingValues, content), nil

	case DiagnosticOtherReason:
		return ber.AppendElement(dst, ber.ClassContext, false, tagDiagOtherReason, []byte(d.Text)), nil

	case DiagnosticUnsupportedOption:
		return ber.AppendElement(dst, ber.ClassContext, false, tagDiagUnsupportedOption, []byte(d.Text)), nil

	case DiagnosticExtension:
		return ber.AppendElement(dst, ber.ClassContext, true, tagDiagExtension, d.Extension), nil
	}
	return nil, ErrMalformedDiagnostic
}

func decodeDiagnostic(e *ber.Element) (Diagnostic, error) {
	switch {
	case e.IsContext(tagDiagInvalidParameterValue):
		d := ber.NewDecoder(e.Bytes)
		text, err := d.Next()
		if err != nil {
			return Diagnostic{}, err
		}
		appellation, err := d.Next()
		if err != nil {
			return Diagnostic{}, err
		}
		return Diagnostic{
			Kind:         DiagnosticInvalidParameterValue,
			Text:         text.String(),
			Appellations: []string{appellation.String()},
		}, nil

	case e.IsContext(tagDiagConflictingValues):
		d := ber.NewDecoder(e.Bytes)
		text, err := d.Next()
		if err != nil {
			return Diagnostic{}, err
		}
		list, err := d.Next()
		if err != nil {
			return Diagnostic{}, err
		}
		out := Diagnostic{Kind: DiagnosticConflictingValues, Text: text.String()}
		inner := ber.NewDecoder(list.Bytes)
		for !inner.Empty() {
			appellation, err := inner.Next()
			if err != nil {
				return Diagnostic{}, err
			}
			out.Appellations = append(out.Appellations, appellation.String())
		}
		return out, nil

	case e.IsContext(tagDiagOtherReason):
		return Diagnostic{Kind: DiagnosticOtherReason, Text: e.String()}, nil

	case e.IsContext(tagDiagUnsupportedOption):
		return Diagnostic{Kind: DiagnosticUnsupportedOption, Text: e.String()}, nil

	case e.IsContext(tagDiagExtension):
		return Diagnostic{Kind: DiagnosticExtension, Extension: e.Copy()}, nil
	}
	return Diagnostic{}, ErrMalformedDiagnostic
}

// checkAppellation enforces the SIZE constraint of annex F3.3.
func checkAppellation(s string) error {
	if len(s) < MinAppellationLength || len(s) > MaxAppellationLength {
		return ErrAppellationLength
	}
	return nil
}
