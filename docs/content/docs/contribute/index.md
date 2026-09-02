---
title: Contributing
description: How this project works, and the one rule that matters most.
order: 1
---

Astro implements CCSDS and ECSS standards in Go. That shapes everything below: the standards are the specification, and this repository is only ever an implementation of them.

## The one rule

**Never code a constant, field layout, or algorithm from memory.**

Download the standard, read the clause, cite it in a comment next to the code. Blue Books are free at [public.ccsds.org](https://public.ccsds.org/Publications/BlueBooks.aspx) and ECSS standards at [ecss.nl](https://ecss.nl).

This is not pedantry. Three defects found in this repository were the same shape: an implementation that was self-consistent and wrong.

- The PN randomizer read the polynomial exponents as bit indices. It produced a perfectly good maximal-length sequence that was not the CCSDS one, so every randomized frame was unreadable by conforming equipment.
- The Transfer Frame Secondary Header length field was off by one. Encoder and decoder agreed, so every round trip passed.
- The packet service had two First Header Pointer codes swapped, on the send and receive paths at once.

None of the three could be caught by a round-trip test, because a round trip only proves the code agrees with itself.

## Testing

**Assert what goes on the wire, not that it comes back.** Where the standard publishes a vector, transcribe it and cite the table:

```go
// CCSDS 131.0-B specifies h(x) = x^8 + x^7 + x^5 + x^3 + 1 with the register
// preset to all ones. CCSDS 142.0-B-1 clause 3.5.2.1 publishes the first 40 digits.
want := []byte{0xFF, 0x48, 0x0E, 0xC0, 0x9A}
```

Where it publishes none, derive the octets by hand from the normative text and show the derivation in a comment. Say so plainly if no vector exists.

Every decoder gets a fuzz target. Copy the pattern in `pkg/tcdl/fuzz_test.go`.

Before opening a pull request, all four must be clean:

```bash
make test
make race
make fuzz-smoke
golangci-lint run
```

## Structure

```
pkg/                       one package per standard, flat, stdlib only
internal/                  shared code with no API commitment
cli/                       cobra commands
docs/content/docs/         narrative docs: start, guides, contribute
docs/content/protocols/    one folder per layer, one page per standard
docs/content/cli/          one page per command, embedded in the binary
docs/content/conformance/  one page per standard
```

`pkg/` takes no dependencies outside the standard library. The CLI may. A mission integrating one protocol should not inherit a dependency tree.

## Reporting a conformance defect

This is the most valuable issue you can file. Include the standard and issue number, the clause, what the code does, and what the clause requires. A worked example with octets is ideal.

## Adding a protocol

Several standards are still open. See [adding a protocol](/docs/contribute/adding-a-protocol) for the required page set and the conventions to follow, and the [protocol index](/protocols#not-implemented-yet) for what is unclaimed.
