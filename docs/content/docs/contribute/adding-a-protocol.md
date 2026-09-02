---
title: Adding a protocol
short: New protocol
description: The conventions and the page set a new protocol package needs.
order: 2
---

Several standards are still open — see [the protocol index](/protocols#not-implemented-yet). Open an issue to discuss the approach before you start.

Read [contributing](/docs/contribute) first, especially the rule about never coding from memory.

## Code conventions

One package per standard, under `pkg/`, flat, standard library only.

Follow what the existing packages do:

| | |
|---|---|
| `Encode() ([]byte, error)` | on every wire type |
| `Decode...` | a package-level function, not a method |
| `Validate() error` | on every wire type, checking the value against the standard |
| `Humanize() string` | on every wire type, for the CLI's annotated dump |
| `errors.go` | sentinel errors, one per failure mode, with the clause in a comment |
| package doc comment | names the standard and its issue number |

A coding or compression package has no wire type with fields, so `Validate` and `Humanize` do not apply to it — `pkg/tmsc` and `pkg/ocsc` carry neither.

Look at [`pkg/spp`](https://github.com/ravisuhag/astro/tree/main/pkg/spp), [`pkg/tmdl`](https://github.com/ravisuhag/astro/tree/main/pkg/tmdl), or [`pkg/tmsc`](https://github.com/ravisuhag/astro/tree/main/pkg/tmsc) for the established patterns.

Every decoder needs a fuzz target. The property is that arbitrary bytes never panic and never allocate from an attacker-controlled length field. Copy `pkg/tcdl/fuzz_test.go` and wire it into the `fuzz-smoke` Makefile target.

## The docs

**These docs bridge the Blue Book and the code. They do not replace it.**

Blue Books are free public PDFs. Green Books already give the friendly overview. Retyping a field table here buys nothing, goes stale on the next issue, and competes for time with actually implementing protocols. Write the part nobody else can write: what this package does, what it refuses to do, and where the standard left a choice open.

### Required files

```
docs/content/protocols/<layer>/<pkg>.md   required
docs/content/conformance/<pkg>.md         required
```

One page per protocol, covering both what the standard is and how to call the
package.

Pick the layer your protocol sits at: `transport`, `data-link`, `coding`,
`ground`, `compression`, or `mission`. Conformance lives in its own top-level
section, not beside the API — its readers are doing assurance rather than
writing code.

### The protocol page

Frontmatter first — `title`, a `short` sidebar label, `description`, and an `order` that puts it in the right layer group. Then a header line:

```markdown
> **CCSDS 123.4-B-5** | [Blue Book](url) | [`pkg/foo`](url) | [`astro foo`](/cli/foo)
```

Then a paragraph or two on what the protocol is for, linking to [the stack](/docs/start/concepts) rather than re-explaining the layers. Then these sections:

**`## Scope`** — the most valuable section on the page. Four things:

- What is implemented.
- What is deliberately absent, and why.
- What is left to the caller, because the standard says mission-defined.
- What lives in a different package.

**`## Field map`** — for a standard with a wire format, a compact table, one row per wire field. A coding or compression standard has no header to map, so it walks through the chain instead — see [`ldc`](/protocols/compression/ldc) or [`ocsc`](/protocols/coding/ocsc) for that shape.

```markdown
| Field | Bits | Go | Notes |
|---|---|---|---|
| APID | 11 | `PrimaryHeader.APID` | 0-2047. `0x7FF` is idle. |
```

A table, not a walkthrough. If a reader wants the bit diagram they can open the PDF. What they cannot get anywhere else is the mapping to your struct.

**`## Gotchas`** — the rules that bite. Off-by-ones, fields that must agree with each other, things that fail silently, defaults that differ from a neighbouring protocol. Cite the clause. Name the error the package returns. Where there is a wire format, this section and Scope are why the page exists.

**`## Using the package`** — quick start, the types and options that matter, and an error table. Sits between Gotchas and Notes. Name the sections for what they do (`## Quick start`, `## Errors`) rather than nesting them all under one heading; the page's table of contents is the navigation.

**`## Notes`** — optional. Commentary on why the format looks the way it does. Say plainly that it is commentary, or cite the Green Book where it explains the choice. Do not present a plausible reconstruction as fact; people build spacecraft from this.

**`## Reference`** — the Blue Book, any Green Book, then one bullet linking the siblings:

```markdown
- [CLI](/cli/foo) | [Conformance](/conformance/foo) | [The stack](/docs/start/concepts)
```

Keep the part that explains the standard tight — a few screens. If it runs much longer you are probably restating the Blue Book. The API part can be as long as the package needs.

### The conformance page

Where the standard ships a PICS proforma, fill it in. Otherwise write a coverage matrix.

**Record what is not implemented.** An honest gap is worth more than a silent one, and it is the first thing a mission assurance reader looks for.

### Wiring it up

1. Add a row to `docs/content/protocols/<layer>/index.md` and to
   `docs/content/conformance/index.md`, and add the protocol to the layer's
   `meta.json` `pages` array.
2. If you added a CLI command, write `docs/content/cli/<cmd>.md` and add it to the `protocols` map in `cli/manual.go` — that file is embedded in the binary and served by `astro manual`. Give it a `## Subcommands` table with a row for every subcommand, one `## astro <cmd> <sub>` section each, and the closing **See also** line the other pages end with. Add it to `docs/content/cli/meta.json` too, and give it an `order` matching its place in that array.

## Checks

```bash
make test
make race
make fuzz-smoke
golangci-lint run
```

All four clean before the pull request.

## Commits

Conventional commits: `feat(sdls):`, `fix(tmdl):`, `docs:`, `refactor:`, `chore:`.

Write the body for someone reading `git log` in a year with no memory of the change. For a bug fix, say what was wrong, what the standard requires, and why the tests did not catch it.
