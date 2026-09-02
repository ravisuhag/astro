package sle

import "fmt"

// The per-service GET-PARAMETER parameter sets.
//
// GET-PARAMETER's invocation is the same in all four services, and
// getparameter.go handles it. The return's positive alternative is not: each
// service defines its own CHOICE of configuration parameters, and the same
// context tag means a different parameter in each. Tag [4] is
// requestedFrameQuality in RAF, reportingCycle in RCF,
// permittedControlWordTypeSet in ROCF and deliveryMode in FCLTU.
//
// So a decoder needs to be told which service the PDU came from. That is not
// a limitation of this package but of the format: the association carries the
// service, and the octets do not.
//
// Every alternative has the same inner shape,
//
//	SEQUENCE { parameterName ParameterName, parameterValue <varies> }
//
// so one decoder reads the SEQUENCE and the tag says which parameter it is.
// The value is handed back as an integer where the schema makes it a single
// integer, and as raw BER where it is structured: a set of GVCIDs, the
// online/offline CHOICE of a latency limit. Those need the caller's own
// decoding against the service ASN.1, and inventing Go types for each would
// commit this package to shapes it cannot test against real vectors.
//
// Sources, each the current issue with its errata:
//
//   - CCSDS 911.1-B-5 annex A, RafGetParameter and the shared ParameterName
//   - CCSDS 911.2-B-4 annex A, RcfGetParameter
//   - CCSDS 911.5-B-4 annex A, RocfGetParameter
//   - CCSDS 912.1-B-5 annex A, CltuGetParameter

// ParameterName is the schema's ParameterName: the integer that names a
// configuration parameter.
//
// The values are not contiguous and not ordered (acquisitionSequenceLength
// is 201 while apidList is 2) because the enumeration grew across issues and
// each service's additions were given their own range. They are transcribed
// from the INTEGER definition in CCSDS 911.1-B-5 annex A.
type ParameterName int32

// The parameter names, from CCSDS 911.1-B-5 annex A.
const (
	ParamAcquisitionSequenceLength ParameterName = 201
	ParamAPIDList                  ParameterName = 2
	ParamBitLockRequired           ParameterName = 3
	ParamBlockingTimeoutPeriod     ParameterName = 0
	ParamBlockingUsage             ParameterName = 1
	ParamBufferSize                ParameterName = 4
	ParamClcwGlobalVcID            ParameterName = 202
	ParamClcwPhysicalChannel       ParameterName = 203
	ParamCopCntrFramesRepetition   ParameterName = 300
	ParamDeliveryMode              ParameterName = 6
	ParamDirectiveInvocation       ParameterName = 7
	ParamDirectiveInvocationOnline ParameterName = 108

	ParamExpectedDirectiveIdentification       ParameterName = 8
	ParamExpectedEventInvocationIdentification ParameterName = 9
	ParamExpectedSlduIdentification            ParameterName = 10

	ParamFopSlidingWindow    ParameterName = 11
	ParamFopState            ParameterName = 12
	ParamLatencyLimit        ParameterName = 15
	ParamMapList             ParameterName = 16
	ParamMapMuxControl       ParameterName = 17
	ParamMapMuxScheme        ParameterName = 18
	ParamMaximumFrameLength  ParameterName = 19
	ParamMaximumPacketLength ParameterName = 20
	ParamMaximumSlduLength   ParameterName = 21
	ParamMinimumDelayTime    ParameterName = 204
	ParamMinReportingCycle   ParameterName = 301
	ParamModulationFrequency ParameterName = 22
	ParamModulationIndex     ParameterName = 23
	ParamNotificationMode    ParameterName = 205

	ParamPermittedControlWordTypeSet ParameterName = 101
	ParamPermittedFrameQuality       ParameterName = 302
	ParamPermittedGvcidSet           ParameterName = 24
	ParamPermittedTcVcidSet          ParameterName = 102
	ParamPermittedTransmissionMode   ParameterName = 107
	ParamPermittedUpdateModeSet      ParameterName = 103

	ParamPlop1IdleSequenceLength ParameterName = 206
	ParamPlopInEffect            ParameterName = 25
	ParamProtocolAbortMode       ParameterName = 207
	ParamReportingCycle          ParameterName = 26

	ParamRequestedControlWordType ParameterName = 104
	ParamRequestedFrameQuality    ParameterName = 27
	ParamRequestedGvcid           ParameterName = 28
	ParamRequestedTcVcid          ParameterName = 105
	ParamRequestedUpdateMode      ParameterName = 106

	ParamReturnTimeoutPeriod ParameterName = 29
	ParamRfAvailable         ParameterName = 30
	ParamRfAvailableRequired ParameterName = 31
	ParamSegmentHeader       ParameterName = 32

	ParamSequCntrFramesRepetition ParameterName = 303
	ParamSubcarrierToBitRateRatio ParameterName = 34
	ParamThrowEventOperation      ParameterName = 304
	ParamTimeoutType              ParameterName = 35
	ParamTimerInitial             ParameterName = 36
	ParamTransmissionLimit        ParameterName = 37

	ParamTransmitterFrameSequenceNumber ParameterName = 38
	ParamVcMuxControl                   ParameterName = 39
	ParamVcMuxScheme                    ParameterName = 40
	ParamVirtualChannel                 ParameterName = 41
)

// parameterNames maps each value to the name the schema spells it with.
var parameterNames = map[ParameterName]string{
	ParamAcquisitionSequenceLength:             "acquisitionSequenceLength",
	ParamAPIDList:                              "apidList",
	ParamBitLockRequired:                       "bitLockRequired",
	ParamBlockingTimeoutPeriod:                 "blockingTimeoutPeriod",
	ParamBlockingUsage:                         "blockingUsage",
	ParamBufferSize:                            "bufferSize",
	ParamClcwGlobalVcID:                        "clcwGlobalVcId",
	ParamClcwPhysicalChannel:                   "clcwPhysicalChannel",
	ParamCopCntrFramesRepetition:               "copCntrFramesRepetition",
	ParamDeliveryMode:                          "deliveryMode",
	ParamDirectiveInvocation:                   "directiveInvocation",
	ParamDirectiveInvocationOnline:             "directiveInvocationOnline",
	ParamExpectedDirectiveIdentification:       "expectedDirectiveIdentification",
	ParamExpectedEventInvocationIdentification: "expectedEventInvocationIdentification",
	ParamExpectedSlduIdentification:            "expectedSlduIdentification",
	ParamFopSlidingWindow:                      "fopSlidingWindow",
	ParamFopState:                              "fopState",
	ParamLatencyLimit:                          "latencyLimit",
	ParamMapList:                               "mapList",
	ParamMapMuxControl:                         "mapMuxControl",
	ParamMapMuxScheme:                          "mapMuxScheme",
	ParamMaximumFrameLength:                    "maximumFrameLength",
	ParamMaximumPacketLength:                   "maximumPacketLength",
	ParamMaximumSlduLength:                     "maximumSlduLength",
	ParamMinimumDelayTime:                      "minimumDelayTime",
	ParamMinReportingCycle:                     "minReportingCycle",
	ParamModulationFrequency:                   "modulationFrequency",
	ParamModulationIndex:                       "modulationIndex",
	ParamNotificationMode:                      "notificationMode",
	ParamPermittedControlWordTypeSet:           "permittedControlWordTypeSet",
	ParamPermittedFrameQuality:                 "permittedFrameQuality",
	ParamPermittedGvcidSet:                     "permittedGvcidSet",
	ParamPermittedTcVcidSet:                    "permittedTcVcidSet",
	ParamPermittedTransmissionMode:             "permittedTransmissionMode",
	ParamPermittedUpdateModeSet:                "permittedUpdateModeSet",
	ParamPlop1IdleSequenceLength:               "plop1IdleSequenceLength",
	ParamPlopInEffect:                          "plopInEffect",
	ParamProtocolAbortMode:                     "protocolAbortMode",
	ParamReportingCycle:                        "reportingCycle",
	ParamRequestedControlWordType:              "requestedControlWordType",
	ParamRequestedFrameQuality:                 "requestedFrameQuality",
	ParamRequestedGvcid:                        "requestedGvcid",
	ParamRequestedTcVcid:                       "requestedTcVcid",
	ParamRequestedUpdateMode:                   "requestedUpdateMode",
	ParamReturnTimeoutPeriod:                   "returnTimeoutPeriod",
	ParamRfAvailable:                           "rfAvailable",
	ParamRfAvailableRequired:                   "rfAvailableRequired",
	ParamSegmentHeader:                         "segmentHeader",
	ParamSequCntrFramesRepetition:              "sequCntrFramesRepetition",
	ParamSubcarrierToBitRateRatio:              "subcarrierToBitRateRatio",
	ParamThrowEventOperation:                   "throwEventOperation",
	ParamTimeoutType:                           "timeoutType",
	ParamTimerInitial:                          "timerInitial",
	ParamTransmissionLimit:                     "transmissionLimit",
	ParamTransmitterFrameSequenceNumber:        "transmitterFrameSequenceNumber",
	ParamVcMuxControl:                          "vcMuxControl",
	ParamVcMuxScheme:                           "vcMuxScheme",
	ParamVirtualChannel:                        "virtualChannel",
}

// String returns the schema's spelling of the name.
func (p ParameterName) String() string {
	if name, ok := parameterNames[p]; ok {
		return name
	}
	return fmt.Sprintf("unknown parameter name %d", int32(p))
}

// parameterShape says how a parameter's value is carried.
type parameterShape struct {
	// name is the parameterName the schema constrains this alternative to.
	name ParameterName
	// simple reports whether parameterValue is a single INTEGER. Where it is
	// not (a set, a nested CHOICE) the value comes back as raw BER.
	simple bool
}

// The per-service tag tables.
//
// The tags are the CHOICE alternatives, and they are not in the order the
// schema lists the alternatives: minReportingCycle was added in a later issue
// and took the next free tag, so it appears alphabetically among the others
// while carrying the highest number. RAF puts it at [7], RCF at [7], ROCF at
// [13], FCLTU at [19]. Reading them in listing order would map most of a
// service's parameters to the wrong name.
var (
	// rafParameters is CCSDS 911.1-B-5 annex A, RafGetParameter.
	rafParameters = map[uint32]parameterShape{
		0: {ParamBufferSize, true},
		1: {ParamDeliveryMode, true},
		2: {ParamLatencyLimit, false}, // CHOICE of online IntPosShort or offline NULL
		3: {ParamReportingCycle, true},
		4: {ParamRequestedFrameQuality, true},
		5: {ParamReturnTimeoutPeriod, true},
		6: {ParamPermittedFrameQuality, false}, // SET OF RequestedFrameQuality
		7: {ParamMinReportingCycle, true},
	}

	// rcfParameters is CCSDS 911.2-B-4 annex A, RcfGetParameter.
	rcfParameters = map[uint32]parameterShape{
		0: {ParamBufferSize, true},
		1: {ParamDeliveryMode, true},
		2: {ParamLatencyLimit, false},
		3: {ParamPermittedGvcidSet, false}, // GvcIdSet
		4: {ParamReportingCycle, true},
		5: {ParamRequestedGvcid, false}, // GvcId, a SEQUENCE
		6: {ParamReturnTimeoutPeriod, true},
		7: {ParamMinReportingCycle, true},
	}

	// rocfParameters is CCSDS 911.5-B-4 annex A, RocfGetParameter.
	rocfParameters = map[uint32]parameterShape{
		0:  {ParamBufferSize, true},
		1:  {ParamDeliveryMode, true},
		2:  {ParamLatencyLimit, false},
		3:  {ParamPermittedGvcidSet, false},
		4:  {ParamPermittedControlWordTypeSet, false}, // SEQUENCE OF
		5:  {ParamPermittedTcVcidSet, false},          // TcVcidSet
		6:  {ParamPermittedUpdateModeSet, false},      // SEQUENCE OF
		7:  {ParamReportingCycle, true},
		8:  {ParamRequestedGvcid, false},
		9:  {ParamRequestedControlWordType, true},
		10: {ParamRequestedTcVcid, false}, // RequestedTcVcid, a CHOICE
		11: {ParamRequestedUpdateMode, true},
		12: {ParamReturnTimeoutPeriod, true},
		13: {ParamMinReportingCycle, true},
	}

	// cltuParameters is CCSDS 912.1-B-5 annex A, CltuGetParameter.
	cltuParameters = map[uint32]parameterShape{
		0:  {ParamAcquisitionSequenceLength, true},
		1:  {ParamBitLockRequired, true},
		2:  {ParamClcwGlobalVcID, false}, // ClcwGvcId, a CHOICE
		3:  {ParamClcwPhysicalChannel, false},
		4:  {ParamDeliveryMode, true},
		5:  {ParamExpectedSlduIdentification, true},
		6:  {ParamExpectedEventInvocationIdentification, true},
		7:  {ParamMaximumSlduLength, true},
		8:  {ParamMinimumDelayTime, true},
		9:  {ParamModulationFrequency, true},
		10: {ParamModulationIndex, true},
		11: {ParamNotificationMode, true},
		12: {ParamPlop1IdleSequenceLength, true},
		13: {ParamPlopInEffect, true},
		14: {ParamProtocolAbortMode, true},
		15: {ParamReportingCycle, true},
		16: {ParamReturnTimeoutPeriod, true},
		17: {ParamRfAvailableRequired, true},
		18: {ParamSubcarrierToBitRateRatio, true},
		19: {ParamMinReportingCycle, true},
	}
)

// serviceParameters returns the tag table for one service.
func serviceParameters(service ServiceKind) (map[uint32]parameterShape, error) {
	switch service {
	case ServiceRAF:
		return rafParameters, nil
	case ServiceRCF:
		return rcfParameters, nil
	case ServiceROCF:
		return rocfParameters, nil
	case ServiceFCLTU:
		return cltuParameters, nil
	default:
		return nil, fmt.Errorf("%w: service %d has no parameter set", ErrInvalidTag, service)
	}
}

// ServiceParameter is one configuration parameter from a GET-PARAMETER
// return.
type ServiceParameter struct {
	// Service is the transfer service whose parameter set this was read
	// against, because the same tag means different things in each.
	Service ServiceKind

	// Tag is the CHOICE alternative the provider chose.
	Tag uint32

	// Name is the parameterName the alternative carried. The schema
	// constrains it to match the alternative, and Decode checks that it
	// does: a provider that disagrees with itself is reporting something
	// this package should not paper over.
	Name ParameterName

	// Value is the parameter's value when the schema makes it a single
	// integer, which most are. Valid only when HasValue is set.
	Value    int64
	HasValue bool

	// Raw is the parameterValue element's content when the value is
	// structured. A set of GVCIDs, the online/offline CHOICE of a latency
	// limit. Decoding it further needs the service's own ASN.1, and this
	// package does not model those shapes rather than guess at them.
	Raw []byte
}

// Humanize returns a human-readable summary.
func (p *ServiceParameter) Humanize() string {
	if p.HasValue {
		return fmt.Sprintf("  %s.%s = %d", p.Service, p.Name, p.Value)
	}
	return fmt.Sprintf("  %s.%s = %d octets of BER, not decoded further",
		p.Service, p.Name, len(p.Raw))
}

// DecodeServiceParameter reads the positive result of a GET-PARAMETER return
// against one service's parameter set.
//
// content is the parameter CHOICE as GetParameterReturn carries it: the
// alternative's tag and content, which is where getparameter.go stops.
//
// service is required and cannot be inferred. The same context tag names a
// different parameter in each service, so decoding a RAF PDU against the
// FCLTU set would report the wrong parameter with a plausible value.
func DecodeServiceParameter(content []byte, service ServiceKind) (*ServiceParameter, error) {
	table, err := serviceParameters(service)
	if err != nil {
		return nil, err
	}

	decoder := NewDecoder(content)
	alternative, err := decoder.Next()
	if err != nil {
		return nil, fmt.Errorf("reading the parameter alternative: %w", err)
	}
	if alternative.Class != ClassContext {
		return nil, fmt.Errorf("%w: the parameter alternative is not a context tag", ErrInvalidTag)
	}

	shape, ok := table[alternative.Tag]
	if !ok {
		return nil, fmt.Errorf("%w: %s has no parameter at tag [%d]",
			ErrInvalidTag, service, alternative.Tag)
	}

	parameter := &ServiceParameter{
		Service: service,
		Tag:     alternative.Tag,
		Name:    shape.name,
	}

	// Every alternative is SEQUENCE { parameterName, parameterValue }.
	inner := NewDecoder(alternative.Bytes)

	nameElement, err := inner.Next()
	if err != nil {
		return nil, fmt.Errorf("reading parameterName of %s: %w", shape.name, err)
	}
	reported, err := nameElement.Int64()
	if err != nil {
		return nil, fmt.Errorf("reading parameterName of %s: %w", shape.name, err)
	}
	// The schema constrains the name to the alternative. A provider that
	// sends one and means the other is reporting a defect, and silently
	// trusting the tag would hide it.
	if ParameterName(reported) != shape.name {
		return nil, fmt.Errorf(
			"%w: %s tag [%d] should carry parameterName %s (%d) but carries %d",
			ErrInvalidTag, service, alternative.Tag, shape.name, int32(shape.name), reported)
	}

	valueElement, err := inner.Next()
	if err != nil {
		return nil, fmt.Errorf("reading parameterValue of %s: %w", shape.name, err)
	}

	if shape.simple {
		value, err := valueElement.Int64()
		if err != nil {
			return nil, fmt.Errorf("reading parameterValue of %s: %w", shape.name, err)
		}
		parameter.Value = value
		parameter.HasValue = true
		return parameter, nil
	}

	parameter.Raw = valueElement.Bytes
	return parameter, nil
}

// DecodeParameter decodes this return's positive result against a service's
// parameter set.
//
// It reports false when the return is negative, a provider answering
// 'unknown parameter', which the specs define for exactly the case where it
// does not have the one asked for.
func (g *GetParameterReturn) DecodeParameter(service ServiceKind) (*ServiceParameter, bool, error) {
	if !g.Positive || len(g.Parameter) == 0 {
		return nil, false, nil
	}

	parameter, err := DecodeServiceParameter(g.Parameter, service)
	if err != nil {
		return nil, false, err
	}
	return parameter, true, nil
}
