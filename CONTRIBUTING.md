# Contributing to Astro

Astro implements CCSDS and ECSS space communication standards in Go. That
shapes almost everything below: the standards are the specification, and this
repository is only ever an implementation of them.

## The one rule that matters

**Never code a constant, field layout, or algorithm from memory.**

Download the standard, read the clause, and cite it in a comment next to the
code. Every CCSDS Blue Book used here is a free public PDF at
[public.ccsds.org](https://public.ccsds.org/Publications/BlueBooks.aspx), and
the ECSS standards are free at [ecss.nl](https://ecss.nl).

This is not pedantry. Three defects found in this repository were all the same
shape: an implementation that was self-consistent and wrong.

- The PN randomizer read the polynomial exponents as bit indices, producing a
  perfectly good maximal-length sequence that was not the CCSDS one. Every
  randomized frame was unreadable by a conforming receiver.
- The Transfer Frame Secondary Header length field was off by one. Encoder and
  decoder agreed, so every round trip passed.
- The packet service had two First Header Pointer codes swapped, on both the
  send and receive paths at once.

None of the three could be caught by a round-trip test, because a round trip
only proves the code agrees with itself. Which leads directly to:

## Testing

**Assert what goes on the wire, not that it comes back.** A round-trip test is
worth having, but it is never sufficient on its own. Where the standard
publishes a vector, transcribe it and cite the table:

```go
// CCSDS 131.0-B specifies h(x) = x^8 + x^7 + x^5 + x^3 + 1 with the register
// preset to all ones. CCSDS 142.0-B-1 §3.5.2.1 publishes the first 40 digits.
want := []byte{0xFF, 0x48, 0x0E, 0xC0, 0x9A}
```

Where it publishes none, derive the expected octets by hand from the normative
text and show the derivation in a comment. Say so plainly if no vector exists —
several of the PICS documents here do exactly that.

Every decoder gets a fuzz target. The pattern is `pkg/tcdl/fuzz_test.go`; wire
new targets into the `fuzz-smoke` Makefile target. The property is that
arbitrary bytes never panic and never allocate from an attacker-controlled
length field.

Before opening a pull request:

```
make test      # go test ./...
make race      # go test -race ./...
make fuzz-smoke
golangci-lint run
```

All four must be clean.

## Structure

```
pkg/       one package per standard, flat, stdlib only
internal/  shared implementation with no API commitment
cli/       cobra commands
docs/guides/   one guide per package, prose
docs/pics/     one conformance statement per package
```

**`pkg/` takes no dependencies outside the standard library.** The CLI may.
This is deliberate: a mission integrating one protocol should not inherit a
dependency tree.

New packages follow the conventions the existing ones do — `Encode() ([]byte,
error)`, a package-level `Decode...`, `Validate() error`, `Humanize() string`,
sentinel errors in `errors.go`, and a package doc comment naming the standard
and its issue.

## Documentation

A new protocol package lands with three things, or it is not finished:

1. `docs/guides/<pkg>.md` — what the protocol is for and how to use this
   package, in prose. Explain the *why*, not just the API.
2. `docs/pics/<pkg>-pics.md` — a conformance statement. Where the standard
   ships a PICS proforma, fill that in. Otherwise write a coverage matrix.
   **Record what is not implemented**, and why; an honest gap is worth more
   than a silent one.
3. A row in the README protocol table.

## Commits

Conventional commits: `feat(sdls):`, `fix(tmdl):`, `docs:`, `refactor:`,
`chore:`.

Write the body for someone reading `git log` in a year with no memory of the
change. For a bug fix, say what was wrong, what the standard requires, and why
the tests did not catch it. The subject line says what changed; the body says
why it was wrong.

## Reporting a conformance defect

If you find a place where this code disagrees with a standard, that is the most
valuable issue you can file. Please include the standard and issue number, the
clause, what the code does, and what the clause requires. A worked example with
octets is ideal.

## Scope

In scope: CCSDS and ECSS space communication standards — data link, coding and
synchronization, packets, file delivery, compression, mission databases, ground
segment.

Out of scope: mission-specific extensions, hardware drivers, ground station
control, and anything requiring a non-stdlib dependency in `pkg/`.

`plans/` and `ROADMAP.md` are local working notes and are not tracked.
