# XTCE 1.2 COVERAGE MATRIX

## Element coverage for `pkg/xtce` — XTCE 1.2 (OMG), CCSDS 660.1-G-2

---

## Why a matrix and not a PICS

Every other package in this repository carries a PICS proforma, because CCSDS
Blue Books publish one to fill in. XTCE has none to fill in: it is an OMG
schema, and CCSDS 660.1-G-2 is a Green Book — a guide, not a Recommended
Standard. So this file substitutes a coverage matrix, listing the schema's
elements against what the package does with each. It lives in `docs/pics/` so
the documentation triad stays uniform.

## What the statuses mean

| Status | Meaning |
|---|---|
| **Supported** | Modeled as Go types, decoded, and reachable through the API. |
| **Opaque** | Parsed as far as the XML allows and kept as raw bytes, so a caller can handle it and a later version can model it without changing what Load accepts. |
| **Ignored** | Decoded past without error. The element does not appear in the model. A file using it still loads. |
| **Unsupported** | Same as ignored on the wire, but named here because something depends on it — a parameter whose type is an unsupported kind will fail Validate with an unresolved reference. |

Nothing in this matrix causes Load to fail. The loader rejects only malformed
XML, a non-SpaceSystem root, and documents past the size or depth limits.

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
| Namespace `http://www.omg.org/spec/XTCE/20180204` | Supported | Both as a default namespace and with a prefix; tested both ways. |

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
| `RelativeTimeParameterType` | Unsupported | A duration rather than an instant. |
| `ArrayParameterType` | Unsupported | Needs dimension handling the model does not have. |
| `AggregateParameterType` | Unsupported | A struct of members; needs member resolution. |
| `UnitSet`, `Unit` | Supported | Carried, not converted. |
| `ValidRange` | Ignored | |
| `ToString` | Ignored | |
| `@baseType` | Opaque | The attribute is decoded; type inheritance is not followed. |

## Table A-4: Data encodings

| Element | Status | Notes |
|---|---|---|
| `IntegerDataEncoding` | Supported | Encoding, size, bit and byte order, default calibrator. |
| `FloatDataEncoding` | Supported | Same. |
| `StringDataEncoding` | Supported | Fixed size supported; `Variable` is opaque. |
| `BinaryDataEncoding` | Supported | Fixed size supported; dynamic size is opaque. |
| `@bitOrder`, `@byteOrder` | Supported | Defaults applied through accessors. |
| `ErrorDetectCorrect` | Ignored | Checksums and CRCs described in the database. This repository's own CRCs are in `pkg/crc`. |
| `FromBinaryTransformAlgorithm` | Ignored | An algorithm, and algorithms are out of scope. |
| `ToBinaryTransformAlgorithm` | Ignored | Same. |

## Table A-5: Calibrators

| Element | Status | Notes |
|---|---|---|
| `DefaultCalibrator` | Supported | On integer and float encodings. |
| `PolynomialCalibrator` | Supported | All terms, with coefficient and exponent. |
| `SplineCalibrator` | Supported | All points, plus order and the extrapolate flag. |
| `MathOperationCalibrator` | Opaque | Kept as raw XML, and `Calibrator.Kind()` reports it, so a caller is not told the type has no calibrator when it has one this package cannot evaluate. |
| `ContextCalibratorList` | Ignored | Calibration that depends on another parameter's value. |

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
| `LocationInContainerInBits` | Supported | Fixed values; the reference location is carried. Dynamic and lookup forms are opaque. |
| `RepeatEntry` | Supported | Count and offset, fixed forms. |
| `IncludeCondition` | Opaque | Raw XML. Evaluating match criteria belongs to an extraction engine. |
| `TimeAssociation` | Ignored | |
| `BaseContainer` | Supported | The reference is resolved and checked for cycles. |
| `BaseContainer/RestrictionCriteria` | Opaque | Raw XML. Which container a packet matches is an extraction decision. |
| `DefaultRateInStream`, `RateInStreamSet` | Ignored | |
| `BinaryEncoding` on a container | Ignored | |

## Table A-7: Commands

| Element | Status | Notes |
|---|---|---|
| `MetaCommandSet` | Supported | |
| `MetaCommand` | Supported | Skeleton only: name, abstract flag, descriptions. |
| `BaseMetaCommand` | Supported | The reference is decoded but not resolved or cycle-checked. |
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
