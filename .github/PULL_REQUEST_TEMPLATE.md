## What this changes

## Which clause of which standard backs this change?

Astro implements CCSDS/ECSS standards, not conventions remembered from
elsewhere. Name the document and clause (e.g. "CCSDS 133.0-B-2, clause
4.1.3") for any constant, field layout, or algorithm this PR adds or
changes. If nothing normative is involved (docs, CI, refactor with no
behavior change), say so instead.

## Testing

- [ ] `make test` passes
- [ ] `make race` passes (if concurrency-adjacent)
- [ ] `make fuzz-smoke` passes (if a decoder changed)
- [ ] `golangci-lint run` reports `0 issues.`
- [ ] New or changed wire-level behavior has a test asserting the actual
      octets (a standard's published vector, or a hand-derived one with the
      derivation shown), not just a round trip
