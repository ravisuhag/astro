---
title: NDM Combined Instantiation
short: NDM
description: CCSDS 505.0-B-3 clause 4.11, several navigation messages in one XML file.
identifiers:
  - "CCSDS 505.0-B-3 * XML Specification for Navigation Data Messages"
  - "pkg/ndm"
order: 37
---

> **CCSDS 505.0-B-3** | [Blue Book](https://public.ccsds.org/Pubs/505x0b3e2.pdf) | [`pkg/ndm`](https://github.com/ravisuhag/astro/tree/main/pkg/ndm)

## Overview

The other navigation packages each read one standard's messages:
[`pkg/odm`](/protocols/mission/odm) the orbit messages,
[`pkg/adm`](/protocols/mission/adm) the attitude messages,
[`pkg/tdm`](/protocols/mission/tdm) tracking data and
[`pkg/cdm`](/protocols/mission/cdm) conjunctions.

Clause 4.11 of the XML specification lets one file carry any number of those,
of any types, in any order, wrapped in an `<ndm>` root. That file is a
**combined instantiation**, and this package is it.

Clause 4.11.2 gives the reasons an operator would want one:

- a constellation's ephemerides, all in one message;
- an attitude message beside the orbit state it depends on;
- an ephemeris together with the tracking data used to determine it.

```
<ndm>                          the root: namespaces and schema, no version
  <COMMENT>...</COMMENT>
  <opm id="..." version="...">    a constituent: id and version, nothing else
    <header>...</header>
    <body>...</body>
  </opm>
  <apm id="..." version="...">
    ...
  </apm>
</ndm>
```

## It is not a concatenation

Every rule in clause 4.11 is about attributes, and they move in opposite
directions.

| | `<ndm>` root | Constituent tag |
|---|---|---|
| `xmlns:xsi`, `xmlns:ndm` | yes (4.11.4) | no (4.11.5) |
| `xsi:noNamespaceSchemaLocation` | yes (4.11.4) | no (4.11.5) |
| `id`, `version` | **no** (4.11.4) | yes (4.11.5) |

The root gets the namespace and schema attributes; the `id` and `version` a
message would carry as a standalone document stay on the message. The root has
neither, because it is not a message and has no version of its own.

So joining several files end to end does not produce a combined instantiation.
It leaves each message's namespace and schema attributes exactly where clause
4.11.5 forbids them, and several XML declarations in one file. `Encode` writes
the attributes where they belong, and `DecodeCombined` refuses a root that
carries an `id` or a `version` — a file with one was written against the
single-message rules, so nothing about its constituents can be trusted.

## A file names one schema, and the standards name different ones

A combined instantiation carries one `xsi:noNamespaceSchemaLocation` for the
whole file. Each navigation standard names its own master schema:
CCSDS 502.0-B-3 gives `3.0` and CCSDS 504.0-B-2 gives `4.0`. A file mixing
their messages can only name one.

The documents show the difficulty rather than settling it. Figure 7-3 of
CCSDS 504.0-B-2 writes `ndmxml-4.0.0-master-4.0.xsd` over a file of ADM
messages; figure G-12 of the same document writes `ndmxml-3.0.0-master-3.0.xsd`
over another. This package carries whatever the file had, and defaults a new
one to the schema its **first** message names — so a file of orbit messages
gets the ODM's and a file of attitude messages the ADM's.

## A constituent is a whole message

Each one is handed to its own package's decoder, so it faces exactly the rules
it would face alone: the keyword tables, the block structure, the row widths,
the cross-field conditions. A file whose OPM would be refused on its own is
refused here.

There is one decoder per message type, not one for single files and another for
combined ones. That costs a serialise and a parse per constituent and buys the
guarantee that a message means the same thing in either place.

## Using the package

```go
if ndm.IsCombined(data) {
    combined, err := ndm.DecodeCombined(data)
    for _, m := range combined.Messages {
        fmt.Println(ndm.Kind(m))          // "opm", "aem", "tdm", ...
        switch message := m.(type) {
        case *odm.OPM:
            fmt.Println(message.Data.StateVector.X)
        case *adm.APM:
            fmt.Println(message.Quaternion.QC)
        }
    }
}
```

Building one is the same shape in reverse:

```go
combined := &ndm.Combined{
    Comments: []string{"Orbit and attitude for pass planning"},
    Messages: []ndm.Message{orbit, attitude},
}
file, err := combined.Encode()
```

`IsCombined` reads the root element and nothing else, so a caller handed a
navigation file that could be either can pick a decoder before parsing it.

### Errors

| Error | Means |
|---|---|
| `ErrUnknownMessageType` | A constituent whose element name is not one of the nine messages here — an `<rdm>`, for instance. |
| `ErrNoMessage` | A nil constituent, or an empty file with no schema to fall back on. |

## The key-value form has no equivalent

Aggregation is an XML feature. Clause 5.2.2 of CCSDS 504.0-B-2 says a sequence
of ACMs "may be aggregated into a single Navigation Data Message (NDM) XML
file", and neither standard defines a way to do it in the `keyword = value`
notation. There is nothing here for the key-value form because there is nothing
in the standards for it.

## An empty file is allowed

Clause 4.11.8 says a combined instantiation "**should** consist of at least one
constituent message" — a should, not a shall. An `<ndm>` holding only comments
is odd but well formed, and refusing it would refuse a file the standard
permits.

## Reference

- [CCSDS 505.0-B-3, XML Specification for Navigation Data Messages](https://public.ccsds.org/Pubs/505x0b3e2.pdf)
- [CCSDS 502.0-B-3 clause 8.12](https://public.ccsds.org/Pubs/502x0b3e1.pdf) and [CCSDS 504.0-B-2 clause 7.8](https://ccsds.org/Pubs/504x0b2.pdf), which repeat the rules for their own messages
- [Conformance](/conformance/ndm) | [Orbit Data Messages](/protocols/mission/odm) | [Attitude Data Messages](/protocols/mission/adm)
