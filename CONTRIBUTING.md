# Contributing

Astro implements CCSDS and ECSS standards in Go. The standards are the
specification; this repository is only ever an implementation of them.

## The one rule

**Never code a constant, field layout, or algorithm from memory.**

Download the standard, read the clause, cite it in a comment next to the
code. Blue Books are free at [public.ccsds.org](https://public.ccsds.org/Publications/BlueBooks.aspx)
and ECSS standards at [ecss.nl](https://ecss.nl).

This is not pedantry. Three real defects in this repository were the same
shape: an implementation that was self-consistent and still wrong. A
round-trip test cannot catch that, because it only proves the code agrees
with itself, not with the standard.

## Before you open a pull request

```bash
make check      # build, vet, lint, test, vectors
make race       # if you touched anything concurrent
make fuzz-smoke # if you touched a decoder
```

## Full guide

For testing conventions (assert what goes on the wire, cite the standard's
vectors, fuzz every decoder), repository structure, and how to add a new
protocol, see [docs/content/docs/contribute/](docs/content/docs/contribute/index.md).
