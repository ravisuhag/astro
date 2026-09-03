# Wire test vectors

Machine-readable test vectors for the CCSDS and ECSS standards. Each file
pins the exact octets a protocol puts on the wire, together with the
derivation from the standard that makes those octets authoritative.

These files are the reference. An implementation is correct here when it
agrees with them; when an implementation and a vector disagree, the vector
and the clause it cites settle it.

They are plain JSON data, so any implementation in any language can run
them.

See [`CONTRACT.md`](CONTRACT.md) for the full consumer contract —
conventions, field dictionaries per package, and what is deliberately not
covered.

## Why octets, not round trips

A round trip proves an implementation agrees with itself. It proves
nothing about the standard.

An encoder and a decoder can share a misreading of a clause and pass every
round-trip test between them, while a conforming peer misparses every
frame they produce. Symmetric errors are invisible from the inside. Only a
pinned octet string, derived from the clause rather than from the code,
catches them.

That is what each vector here is: the bytes the standard requires, and the
arithmetic showing why.

## Layout

```
vectors/
├── README.md          this file
├── CONTRACT.md        normative consumer contract
├── COVERAGE.md        what is covered, what is not, what is unverified
├── schema.json        JSON Schema for a vector file
├── crc/               one directory per package
│   └── crc16.json
├── spp/
│   ├── header.json
│   └── packet.json
├── cop/
│   ├── clcw.json      encode, decode and reject
│   └── farm1.json     sequence vectors against a state machine
└── ldc/
    └── corpus/        published CCSDS files, referenced not transcribed
```

One file per concern, named after the structure it covers.

## A vector file

```json
{
  "schema_version": 1,
  "standard": "CCSDS 732.0-B-4",
  "package": "aos",
  "source": "hand-computed from the field layouts",
  "encode": [
    {
      "name": "frame-with-fecf",
      "clause": "4.1",
      "note": "byte 0 = 01<<6 | 0xAB>>2 = 0x6a; byte 1 = (0xAB&0x3)<<6 | 42 = 0xea.",
      "fields": { "scid": 171, "vcid": 42, "data": "deadbeef" },
      "want": "6aea...9e2c"
    }
  ]
}
```

## The four kinds

**`encode`** — a set of field values produces exactly these octets. Needs
`fields` and `want`.

**`decode`** — these octets produce these field values. Needs `input` and
`fields`. Comparison covers exactly the fields listed; anything unlisted
is unconstrained. The field dictionary in `CONTRACT.md` names everything a
decoder must expose, so "unlisted" is always a choice, never an unknown.

**`reject`** — something must fail, with a named error. Exactly one of
`input` (octets refused at decode) or `fields` (values refused at
construction). An APID of 2048 does not fit an 11-bit field in any
language, so construction rules belong here too.

**`sequence`** — a scripted run against a state machine: a starting
state, then steps that each name a call and assert the octets it emits,
the state it leaves behind, or the error it must raise. The only kind
that can pin ordering. Time is an explicit step, never a real clock.
`cop/farm1.json` is the worked example.

A vector that is exactly invertible appears in both `encode` and `decode`
rather than getting a third kind.

## Conventions

Byte strings are lowercase hex, no separators, no `0x`.

A scalar result is hex at its natural width, big-endian. A CRC-16 of
`0x29b1` is `"29b1"`; a CRC-32 is eight hex digits. One representation for
every `want`, and it matches the order the value takes on the wire.

Integers wider than 2^53 are decimal strings, because a JSON number cannot
hold them without loss. Anything 32 bits or under is a plain number.

Field names are `snake_case` and name the standard's field, not any
implementation's identifier.

`config` is separate from `fields`. Channel-level agreement — a frame
length, whether an error control field is present, a mission profile —
configures the channel, not the frame, and both ends hold it before any
octet is exchanged. Keeping it separate means a consumer never has to
guess which values travel on the wire.

## The derivation rule

**Every `want` comes from the standard, never from running an
implementation.**

Each vector carries its derivation in a required `note`: the field
arithmetic, the polynomial, the clause that fixes the value. A note that
is missing, or that stops mid-sentence, makes the file invalid.

Writing the derivation down is what makes the vector evidence rather than
a record. A value copied from an implementation's output only proves the
implementation is self-consistent, which is the thing octets are supposed
to rule out.

Deriving requires the clause in front of you. A derivation written from
memory of what a standard probably says is a fabrication with a citation
attached, and it is worse than no vector, because the next implementation
will trust it.

Where a value genuinely cannot be traced to a clause or a published
corpus, it carries `"source": "unverified"` and no `clause`. That marks it
honestly: an implementation agreeing with it has matched this corpus, not
the standard. `COVERAGE.md` lists them.

## Validating the corpus

[`schema.json`](schema.json) describes the file format, and any JSON
Schema validator checks a file against it:

```
check-jsonschema --schemafile schema.json */*.json
```

Four rules the schema cannot express, which a consumer's own loader
should enforce:

- vector names are unique across every kind within a file;
- every path in `corpus` resolves to a file that exists;
- a `reject` names an error from the vocabulary and carries exactly one
  of `input` or `fields`;
- `buffer_too_small` appears only on a vector that declares the
  `encode_into` capability.

[`CONTRACT.md`](CONTRACT.md) states each of these in full.

## Adding a vector

1. Find the clause that defines the layout. Open it.
2. Derive the octets by hand. Write the arithmetic into `note`.
3. Add the entry and cite the clause.
4. Validate the file.

If steps 1 and 2 are not possible, mark it unverified rather than guessing.
