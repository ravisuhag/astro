package sle_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/sle"
)

// buildParameter encodes one parameter alternative the way a provider would:
// a context tag holding SEQUENCE { parameterName, parameterValue }.
func buildParameter(tag uint32, name sle.ParameterName, value int64) []byte {
	inner := sle.AppendInteger(nil, int64(name))
	inner = sle.AppendInteger(inner, value)
	return sle.AppendElement(nil, sle.ClassContext, true, tag, inner)
}

// buildStructuredParameter is the same but with an opaque value, standing in
// for the sets and nested CHOICEs the schema uses.
func buildStructuredParameter(tag uint32, name sle.ParameterName, value []byte) []byte {
	inner := sle.AppendInteger(nil, int64(name))
	inner = sle.AppendElement(inner, sle.ClassContext, false, 0, value)
	return sle.AppendElement(nil, sle.ClassContext, true, tag, inner)
}

// Each service's simple parameters decode to a name and a number.
func TestDecodeServiceParameterSimpleValues(t *testing.T) {
	for _, tc := range []struct {
		service sle.ServiceKind
		tag     uint32
		name    sle.ParameterName
		value   int64
	}{
		{sle.ServiceRAF, 0, sle.ParamBufferSize, 100},
		{sle.ServiceRAF, 7, sle.ParamMinReportingCycle, 60},
		{sle.ServiceRCF, 4, sle.ParamReportingCycle, 30},
		{sle.ServiceROCF, 9, sle.ParamRequestedControlWordType, 1},
		{sle.ServiceROCF, 13, sle.ParamMinReportingCycle, 600},
		{sle.ServiceFCLTU, 7, sle.ParamMaximumSlduLength, 4096},
		{sle.ServiceFCLTU, 19, sle.ParamMinReportingCycle, 1},
	} {
		content := buildParameter(tc.tag, tc.name, tc.value)

		parameter, err := sle.DecodeServiceParameter(content, tc.service)
		if err != nil {
			t.Fatalf("%s tag [%d]: %v", tc.service, tc.tag, err)
		}
		if parameter.Name != tc.name {
			t.Errorf("%s tag [%d] named %s, want %s", tc.service, tc.tag, parameter.Name, tc.name)
		}
		if !parameter.HasValue {
			t.Errorf("%s tag [%d] gave no value", tc.service, tc.tag)
		}
		if parameter.Value != tc.value {
			t.Errorf("%s tag [%d] = %d, want %d", tc.service, tc.tag, parameter.Value, tc.value)
		}
		if parameter.Service != tc.service {
			t.Errorf("parameter reports service %s, want %s", parameter.Service, tc.service)
		}
	}
}

// This is why the service has to be given: one identical tag means four
// different parameters. Decoding a RAF PDU against the FCLTU set would report
// the wrong parameter with a plausible value, which no error would catch.
func TestTagMeansDifferentParametersPerService(t *testing.T) {
	want := map[sle.ServiceKind]sle.ParameterName{
		sle.ServiceRAF:   sle.ParamRequestedFrameQuality,
		sle.ServiceRCF:   sle.ParamReportingCycle,
		sle.ServiceROCF:  sle.ParamPermittedControlWordTypeSet,
		sle.ServiceFCLTU: sle.ParamDeliveryMode,
	}

	seen := make(map[sle.ParameterName]bool)
	for service, name := range want {
		// Build with the name that service expects at tag [4].
		var content []byte
		if service == sle.ServiceROCF {
			content = buildStructuredParameter(4, name, []byte{0x01})
		} else {
			content = buildParameter(4, name, 1)
		}

		parameter, err := sle.DecodeServiceParameter(content, service)
		if err != nil {
			t.Fatalf("%s tag [4]: %v", service, err)
		}
		if parameter.Name != name {
			t.Errorf("%s tag [4] named %s, want %s", service, parameter.Name, name)
		}
		seen[parameter.Name] = true
	}

	if len(seen) != 4 {
		t.Errorf("tag [4] resolved to %d distinct parameters, want 4. The tables are not distinct",
			len(seen))
	}
}

// minReportingCycle carries the highest tag in every service, and a different
// one in each, because it was added in a later issue and took the next free
// number. Reading the alternatives in listing order would map it wrongly.
func TestMinReportingCycleTagsDifferPerService(t *testing.T) {
	tags := map[sle.ServiceKind]uint32{
		sle.ServiceRAF:   7,
		sle.ServiceRCF:   7,
		sle.ServiceROCF:  13,
		sle.ServiceFCLTU: 19,
	}

	for service, tag := range tags {
		content := buildParameter(tag, sle.ParamMinReportingCycle, 42)

		parameter, err := sle.DecodeServiceParameter(content, service)
		if err != nil {
			t.Fatalf("%s tag [%d]: %v", service, tag, err)
		}
		if parameter.Name != sle.ParamMinReportingCycle {
			t.Errorf("%s tag [%d] named %s, want minReportingCycle",
				service, tag, parameter.Name)
		}
	}

	// And the tag one service uses is not the same parameter in another.
	content := buildParameter(13, sle.ParamMinReportingCycle, 42)
	if _, err := sle.DecodeServiceParameter(content, sle.ServiceRAF); err == nil {
		t.Error("RAF accepted tag [13], which it does not define")
	}
}

// A structured value is handed back as raw BER rather than guessed at.
func TestDecodeServiceParameterStructuredValues(t *testing.T) {
	for _, tc := range []struct {
		service sle.ServiceKind
		tag     uint32
		name    sle.ParameterName
	}{
		{sle.ServiceRAF, 2, sle.ParamLatencyLimit},          // CHOICE online/offline
		{sle.ServiceRAF, 6, sle.ParamPermittedFrameQuality}, // SET OF
		{sle.ServiceRCF, 3, sle.ParamPermittedGvcidSet},     // GvcIdSet
		{sle.ServiceROCF, 10, sle.ParamRequestedTcVcid},     // CHOICE
		{sle.ServiceFCLTU, 2, sle.ParamClcwGlobalVcID},      // ClcwGvcId
	} {
		content := buildStructuredParameter(tc.tag, tc.name, []byte{0xAA, 0xBB})

		parameter, err := sle.DecodeServiceParameter(content, tc.service)
		if err != nil {
			t.Fatalf("%s tag [%d]: %v", tc.service, tc.tag, err)
		}
		if parameter.Name != tc.name {
			t.Errorf("%s tag [%d] named %s, want %s", tc.service, tc.tag, parameter.Name, tc.name)
		}
		if parameter.HasValue {
			t.Errorf("%s tag [%d] reported an integer value for a structured parameter",
				tc.service, tc.tag)
		}
		if len(parameter.Raw) != 2 || parameter.Raw[0] != 0xAA {
			t.Errorf("%s tag [%d] raw = %x, want aabb", tc.service, tc.tag, parameter.Raw)
		}
	}
}

// The schema constrains parameterName to match the alternative. A provider
// that sends one and means the other is reporting a defect, and trusting the
// tag would hide it.
func TestDecodeServiceParameterRejectsMismatchedName(t *testing.T) {
	// RAF tag [0] must carry bufferSize; give it deliveryMode instead.
	content := buildParameter(0, sle.ParamDeliveryMode, 1)

	_, err := sle.DecodeServiceParameter(content, sle.ServiceRAF)
	if !errors.Is(err, sle.ErrInvalidTag) {
		t.Errorf("err = %v, want ErrInvalidTag for a name that disagrees with its tag", err)
	}
}

// A tag the service does not define is reported rather than read as whichever
// parameter happens to sit nearby.
func TestDecodeServiceParameterRejectsUnknownTag(t *testing.T) {
	content := buildParameter(30, sle.ParamBufferSize, 1)

	for _, service := range []sle.ServiceKind{
		sle.ServiceRAF, sle.ServiceRCF, sle.ServiceROCF, sle.ServiceFCLTU,
	} {
		if _, err := sle.DecodeServiceParameter(content, service); !errors.Is(err, sle.ErrInvalidTag) {
			t.Errorf("%s accepted tag [30]: %v", service, err)
		}
	}
}

func TestDecodeServiceParameterRejectsRubbish(t *testing.T) {
	if _, err := sle.DecodeServiceParameter(nil, sle.ServiceRAF); err == nil {
		t.Error("an empty parameter was accepted")
	}
	if _, err := sle.DecodeServiceParameter([]byte{0x30, 0x00}, sle.ServiceRAF); err == nil {
		t.Error("a universal SEQUENCE tag was accepted where a context tag is required")
	}
}

// The whole path: a GET-PARAMETER return round-trips and its parameter
// decodes against the right service.
func TestGetParameterReturnDecodeParameter(t *testing.T) {
	original := &sle.GetParameterReturn{
		InvokeId:  7,
		Positive:  true,
		Parameter: buildParameter(0, sle.ParamBufferSize, 250),
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	decoded, err := sle.DecodeGetParameterReturn(encoded)
	if err != nil {
		t.Fatalf("DecodeGetParameterReturn: %v", err)
	}

	parameter, ok, err := decoded.DecodeParameter(sle.ServiceRAF)
	if err != nil {
		t.Fatalf("DecodeParameter: %v", err)
	}
	if !ok {
		t.Fatal("a positive return reported no parameter")
	}
	if parameter.Name != sle.ParamBufferSize {
		t.Errorf("named %s, want bufferSize", parameter.Name)
	}
	if parameter.Value != 250 {
		t.Errorf("value = %d, want 250", parameter.Value)
	}
}

// A negative return has no parameter, which is a normal answer rather than an
// error: the specs define 'unknown parameter' for exactly the case where the
// provider does not have the one asked for.
func TestGetParameterReturnNegativeHasNoParameter(t *testing.T) {
	negative := &sle.GetParameterReturn{
		InvokeId:           7,
		Positive:           false,
		SpecificDiagnostic: sle.GetParameterUnknown,
	}

	parameter, ok, err := negative.DecodeParameter(sle.ServiceRAF)
	if err != nil {
		t.Fatalf("DecodeParameter on a negative return: %v", err)
	}
	if ok || parameter != nil {
		t.Error("a negative return produced a parameter")
	}
}

// Every name in the enumeration renders as the schema spells it, and an
// unknown one says so rather than printing a bare number.
func TestParameterNameString(t *testing.T) {
	for name, want := range map[sle.ParameterName]string{
		sle.ParamBlockingTimeoutPeriod:     "blockingTimeoutPeriod",
		sle.ParamBufferSize:                "bufferSize",
		sle.ParamMinReportingCycle:         "minReportingCycle",
		sle.ParamAcquisitionSequenceLength: "acquisitionSequenceLength",
		sle.ParamThrowEventOperation:       "throwEventOperation",
	} {
		if got := name.String(); got != want {
			t.Errorf("ParameterName(%d).String() = %q, want %q", int32(name), got, want)
		}
	}

	if got := sle.ParameterName(9999).String(); got == "" {
		t.Error("an unknown parameter name rendered as empty")
	}
}

// A service with no parameter set is refused rather than silently matching
// one of the four.
func TestDecodeServiceParameterUnknownService(t *testing.T) {
	content := buildParameter(0, sle.ParamBufferSize, 1)

	if _, err := sle.DecodeServiceParameter(content, sle.ServiceKind(99)); err == nil {
		t.Error("an unknown service was accepted")
	}
}

// Humanize says which service and parameter, and distinguishes a decoded
// value from raw octets.
func TestServiceParameterHumanize(t *testing.T) {
	simple, err := sle.DecodeServiceParameter(
		buildParameter(0, sle.ParamBufferSize, 100), sle.ServiceRAF)
	if err != nil {
		t.Fatalf("DecodeServiceParameter: %v", err)
	}
	if got := simple.Humanize(); got == "" {
		t.Error("Humanize returned nothing for a simple value")
	}

	structured, err := sle.DecodeServiceParameter(
		buildStructuredParameter(2, sle.ParamLatencyLimit, []byte{1, 2, 3}), sle.ServiceRAF)
	if err != nil {
		t.Fatalf("DecodeServiceParameter: %v", err)
	}
	if got := structured.Humanize(); got == "" {
		t.Error("Humanize returned nothing for a structured value")
	}
}
