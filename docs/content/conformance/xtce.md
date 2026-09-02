---
title: XTCE
description: "PICS proforma: what this package implements, clause by clause."
order: 220
---

## Element coverage for `pkg/xtce` — XTCE 1.2 (OMG), CCSDS 660.1-G-2

---

## Why a matrix and not a PICS

Every other package in this repository carries a PICS proforma, because CCSDS
Blue Books publish one to fill in. XTCE has none to fill in: it is an OMG
schema, and CCSDS 660.1-G-2 is a Green Book — a guide, not a Recommended
Standard. So this file substitutes a coverage matrix, listing the schema's
elements against what the package does with each. It lives beside the protocol page so
the documentation triad stays uniform.

## What the statuses mean

| Status | Meaning |
|---|---|
| **Supported** | Modeled as Go types, decoded, and reachable through the API. |
| **Opaque** | Parsed as far as the XML allows and kept as raw bytes, so a caller can handle it and a later version can model it without changing what Load accepts. |
| **Ignored** | Decoded past without error. The element does not appear in the model. A file using it still loads. |
| **Unsupported** | Same as ignored on the wire, but named here because something depends on it — a parameter whose type is an unsupported kind will fail Validate with an unresolved reference. |

The loader rejects malformed XML, a non-SpaceSystem root, documents past the
size or depth limits — and a document whose values cannot be read as their
schema types, such as a `FixedValue` that is not a number (`ErrInvalidValue`).
Element *coverage* never fails Load: every status in the tables below loads.
Values inside a covered element still have to parse.

---

## Table A-1: Document structure

| Element | Status | Notes |
|---|---|---|
| `SpaceSystem` | Supported | Root and recursive. Parent links are built during Load so references can resolve upwards. |
| `SpaceSystem/@name` | Supported | Required by the schema. Load rejects a root without one. |
| `SpaceSystem/@shortDescription` | Supported | |
| `SpaceSystem/@operationalStatus` | Supported | Carried, not interpreted. |
| `LongDescription` | Supported | |
| `Header` | Supported | Version, date, classification, validation status. |
| `Header/AuthorSet`, `NoteSet`, `HistorySet` | Ignored | |
| `AliasSet` | Ignored | Alternate names for a thing. Nothing in this package resolves by alias. |
| `AncillaryDataSet` | Ignored | Mission-defined key-value data. |
| `TelemetryMetaData` | Supported | ParameterTypeSet, ParameterSet, ContainerSet. |
| `CommandMetaData` | Supported | ParameterTypeSet, ParameterSet, MetaCommandSet. |
| `ServiceSet` | Ignored | |
| Namespace `http://www.omg.org/spec/XTCE/20180204` | Supported | Both as a default namespace and with a prefix; tested both ways. Child elements are matched by namespace too, so a `TelemetryMetaData` from another vocabulary is ignored rather than decoded as XTCE's. |

## Table A-2: Parameters

| Element | Status | Notes |
|---|---|---|
| `ParameterSet` | Supported | Telemetry and command sides share one name space, matching the schema's `parameterNameKey`. |
| `Parameter` | Supported | Name, `parameterTypeRef`, initial value, descriptions. |
| `ParameterRef` | Ignored | The schema allows a reference in place of a definition. |
| `ParameterProperties` | Ignored | Data source, persistence, read-only. |

## Table A-3: Parameter types

| Element | Status | Notes |
|---|---|---|
| `IntegerParameterType` | Supported | Size, signedness, encoding, calibrator. |
| `FloatParameterType` | Supported | Size, encoding, calibrator. |
| `EnumeratedParameterType` | Supported | Full enumeration list including `maxValue` ranges. |
| `StringParameterType` | Supported | Encoding and restriction pattern. |
| `BinaryParameterType` | Supported | |
| `BooleanParameterType` | Supported | Including the one and zero string values. |
| `AbsoluteTimeParameterType` | Supported | Encoding, units, scale, offset, reference epoch. Its encoding nests one level deeper than the others'. |
| `RelativeTimeParameterType` | Opaque | The name is decoded so references resolve; the contents stay raw. `TypeKind()` reports "relative time (not modeled)", and `Layout` refuses a parameter of this type. |
| `ArrayParameterType` | Opaque | Same treatment; the name and `arrayTypeRef` are decoded, the dimension list stays raw. |
| `AggregateParameterType` | Opaque | Same treatment; the member list stays raw. |
| `UnitSet`, `Unit` | Supported | Carried, not converted. `power` defaults to 1 through `PowerOrDefault()`. |
| `ValidRange` | Ignored | |
| `ToString` | Ignored | |
| `@baseType` | Opaque | The attribute is decoded; type inheritance is not followed. |

## Table A-4: Data encodings

| Element | Status | Notes |
|---|---|---|
| `IntegerDataEncoding` | Supported | Encoding, size, bit and byte order, default calibrator. |
| `FloatDataEncoding` | Supported | Same. |
| `StringDataEncoding` | Supported | Fixed size in `Layout`. `TerminationChar` resolves through `ResolveLayout`, with the terminator counted toward the field because it occupies packet space; a delimited string starting off an octet boundary is refused, since searching over octets would be meaningless. `LeadingSize` is refused by name: the width of the size field is an attribute of an element kept raw, so there is no way to know how far to skip. `Variable` is opaque. |
| `BinaryDataEncoding` | Supported | Fixed size in `Layout`; a `DynamicValue` size resolves through `ResolveLayout`. A negative resolved width is refused rather than read as zero. |
| `@changeThreshold` | Supported | Carried on both numeric encodings, as a pointer so absent — meaning any change is significant — stays distinguishable from zero. |
| `@bitOrder`, `@byteOrder` | Supported | Defaults applied through accessors. `Validate` checks that `encoding`, `bitOrder` and `byteOrder` are legal enumeration members (`ErrInvalidEncoding`). |
| `ErrorDetectCorrect` | Ignored | Checksums and CRCs described in the database. This repository's own CRCs are in `pkg/crc`. |
| `FromBinaryTransformAlgorithm` | Ignored | An algorithm, and algorithms are out of scope. |
| `ToBinaryTransformAlgorithm` | Ignored | Same. |

## Table A-5: Calibrators

| Element | Status | Notes |
|---|---|---|
| `DefaultCalibrator` | Supported | On integer and float encodings. |
| `PolynomialCalibrator` | Supported | All terms, with coefficient and exponent. |
| `SplineCalibrator` | Supported | All points, plus order and the extrapolate flag. |
| `MathOperationCalibrator` | Supported | The postfix expression is modeled in document order and evaluated. 47 of the 49 operators in `MathOperatorsType` are implemented; `~` and `div` are refused because the schema's definitions of them contradict themselves. `ParameterInstanceRefOperand` needs a value source, supplied through `ApplyWith`. |
| `ContextCalibratorList` | Opaque | Calibration that depends on another parameter's value, kept as raw XML. `DataEncoding.HasContextCalibrators()` marks its presence, so a consumer knows the default curve alone may be wrong for a given packet. |

## Table A-6: Containers

| Element | Status | Notes |
|---|---|---|
| `ContainerSet` | Supported | |
| `SequenceContainer` | Supported | Name, abstract flag, idle pattern. |
| `EntryList` | Supported | **Ordered.** Decoded by a hand-written unmarshaller because entry order is packet order and `encoding/xml` cannot preserve it across separate fields. |
| `ParameterRefEntry` | Supported | |
| `ContainerRefEntry` | Supported | |
| `ParameterSegmentRefEntry` | Opaque | Kept in the ordered list as `EntryOther`, so it still occupies its position. Its reference is not validated. |
| `ContainerSegmentRefEntry` | Opaque | Same. |
| `StreamSegmentEntry` | Opaque | Same. |
| `IndirectParameterRefEntry` | Opaque | Same. |
| `ArrayParameterRefEntry` | Opaque | Same. |
| `LocationInContainerInBits` | Supported | Fixed values in every `FixedIntegerValueType` spelling — decimal, `0x`, `0o`, `0b`. The reference location is carried, defaulting through `ReferenceLocationOrDefault()`. `DynamicValue` resolves through `ResolveLayout`; `DiscreteLookupList` is opaque. `containerEnd` resolves against the packet length for the container being read, and is refused for a spliced inner container whose end is not yet known. `nextEntry` is refused: it positions the *following* entry, and treating it as `previousEntry` would silently misplace the field. |
| `RepeatEntry` | Supported | Fixed counts in every `FixedIntegerValueType` spelling, and `DynamicValue` counts through `ResolveLayout`. `Offset` is refused: the gap between repetitions is not modeled, and packing them without it would place them wrongly. |
| `IncludeCondition` | Opaque | Raw XML. `Layout` places the entry regardless; a caller that needs the condition can parse it. |
| `TimeAssociation` | Ignored | |
| `BaseContainer` | Supported | The reference is resolved and checked for cycles. |
| `BaseContainer/RestrictionCriteria` | Yes | `Comparison`, `ComparisonList` and `BooleanExpression` are modeled and evaluated by `Match`, which resolves a candidate container against the packet when its shape depends on the contents. |
| `RestrictionCriteria/BooleanExpression` | Supported | `Condition`, `ANDedConditions` and `ORedConditions`, nested to any depth, evaluated by `Match`. |
| `Condition` | Supported | Both forms of right-hand side: a `Value` literal, or a second `ParameterInstanceRef` so two fields can be compared against each other. |
| `Condition/@useCalibratedValue` | Yes | Defaults to true, per `ParameterInstanceRefType`. |
| `Condition/@instance` | Parsed | A value from another packet; `Match` reports a non-zero instance on either side rather than guessing. |
| `RestrictionCriteria/CustomAlgorithm` | Opaque | Raw XML. By definition outside the file. |
| `RestrictionCriteria/NextContainer` | Parsed | Deciding it needs the stream rather than one packet, so `Match` does not evaluate it. |
| `Comparison/@useCalibratedValue` | Yes | Defaults to true, so a comparison is against the engineering value. |
| `Comparison/@instance` | Parsed | A value from another packet; `Match` reports a non-zero instance rather than guessing. |
| `DynamicValue` | Supported | `ParameterInstanceRef` plus an optional `LinearAdjustment`. Resolved by `ResolveLayout`, which decodes each field as it places it so a later one can be sized or positioned by an earlier one's value. A forward reference is refused rather than guessed. A non-zero `instance` reads another packet and is refused. |
| `LinearAdjustment` | Supported | Slope and intercept. An absent slope is one, not zero: the schema states no default, and zero would discard the parameter. |
| `DefaultRateInStream`, `RateInStreamSet` | Ignored | |
| `BinaryEncoding` on a container | Ignored | |

## Table A-7: Commands

| Element | Status | Notes |
|---|---|---|
| `MetaCommandSet` | Supported | All three member kinds are kept: `MetaCommand`, `MetaCommandRef`, `BlockMetaCommand`. |
| `MetaCommand` | Supported | Skeleton only: name, abstract flag, descriptions. |
| `MetaCommandRef` | Supported | A command included by reference. The reference is kept but not resolved. |
| `BlockMetaCommand` | Opaque | The name and descriptions are decoded; the `MetaCommandStepList` stays raw. |
| `BaseMetaCommand` | Supported | The reference and the `ArgumentAssignmentList` — the name/value pairs that narrow the base command — are decoded. The reference is not resolved or cycle-checked. |
| `ArgumentList`, `Argument` | Supported | Names and `argumentTypeRef`. |
| `ArgumentTypeSet` and its types | Unsupported | The argument-side mirror of the parameter types. |
| `CommandContainer` | Unsupported | The uplink bit layout. |
| `TransmissionConstraintList` | Unsupported | When a command may be sent. |
| `VerifierSet` | Unsupported | How to tell a command worked. |
| `DefaultSignificance`, `ContextSignificanceList` | Unsupported | How dangerous a command is. |
| `Interlock` | Unsupported | |
| `ParameterToSetList`, `ParametersToSuspendAlarmsOnSet` | Unsupported | |

**A warning about the command side.** Everything that makes a command *safe to
send* — verifiers, constraints, significance — is in the unsupported list. This
package can tell you a command exists and what arguments it takes. It cannot
tell you whether sending it is allowed or what it will do. Do not build an
uplink path on this model.

## Table A-8: Alarms

| Element | Status | Notes |
|---|---|---|
| `DefaultAlarm` | Ignored | On numeric, enumerated and other types. |
| `ContextAlarmList` | Ignored | |
| `StaticAlarmRanges`, `ChangePerSecondAlarmRanges` | Ignored | |
| `AlarmConditions` | Ignored | |

Alarms are ignored rather than kept opaque. Half-modeled alarm limits are worse
than none: a caller who found some of them present might reasonably assume all
of them were.

## Table A-9: Streams, messages, algorithms, services

| Element | Status | Notes |
|---|---|---|
| `StreamSet` and every stream type | Ignored | Fixed and variable frame streams. This repository implements frames directly, in `pkg/tmdl`, `pkg/tcdl`, `pkg/aos` and `pkg/usdl`. |
| `MessageSet`, `Message` | Ignored | |
| `AlgorithmSet` and every algorithm type | Ignored | Out of scope by decision, not by effort. |
| `ServiceSet` | Ignored | |

## Table A-10: References and names

| Feature | Status | Notes |
|---|---|---|
| Absolute references (`/A/B/C`) | Supported | |
| Relative references (`../B/C`, `./C`) | Supported | |
| Bare names | Supported | Searched in the referencing system, then each ancestor to the root. |
| `NameReferenceType` pattern | Supported | Malformed references are rejected with `ErrInvalidReference`. |
| Alias-based resolution | Unsupported | `AliasSet` is ignored, so nothing resolves by alias. |

## Table A-11: Validation

| Check | Status | Notes |
|---|---|---|
| XSD schema conformance | Unsupported | No validator in the standard library and no dependencies taken. Use `xmllint` for this. |
| `parameterTypeRef` resolves | Supported | Spanning both metadata sides. |
| `parameterRef` in entries resolves | Supported | |
| `containerRef` in entries resolves | Supported | |
| `BaseContainer` resolves | Supported | |
| Container inheritance is acyclic | Supported | Linear graph colouring. Identity is by pointer, so two systems may each have a `Common`. |
| Duplicate names within a SpaceSystem | Supported | Parameters, types, containers, commands, and sibling systems. |
| Encoding enumerations are legal | Supported | `encoding`, `bitOrder` and `byteOrder` on every parameter type's data encoding are checked against the schema's members, including the arbitrary byte-list order. |
| `argumentTypeRef` resolves | Unsupported | Argument types are not modeled. |
| `metaCommandRef` resolves | Unsupported | |

---

## Limits

| Limit | Value | Why |
|---|---|---|
| `MaxDocumentSize` | 64 MiB | XTCE sets no ceiling. Real databases run to a few megabytes. |
| `MaxDepth` | 100 levels | `SpaceSystem` is recursive, so deep nesting would recurse during decoding. The check is a token scan that runs **before** decoding. |

## Growing the coverage

This matrix is the contract for scope. Anything moving out of Unsupported or
Opaque lands with its model structs, a fixture exercising it, and a change to
its row here — in the same commit. A status that drifts from the code is worse
than a gap that is written down.
