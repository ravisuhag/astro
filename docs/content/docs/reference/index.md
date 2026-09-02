---
title: Reference
description: The vocabulary, how correctness is checked, the measured throughput, and what happens when the octets are hostile.
order: 0
---

Four things that cut across every protocol rather than belonging to one. Each
answers a question you ask about a library before you commit to it.

- **[Glossary](/docs/reference/glossary)**, what the words mean. CCSDS runs on acronyms, and 23 of the ones used most in these docs are never spelled out anywhere else.
- **[Verification](/docs/reference/verification)**, whether it is correct. Which claims rest on a published test vector, which rest on a reading of the standard, and what has never been tested against another implementation.
- **[Performance](/docs/reference/performance)**, whether it is fast enough. Measured rather than estimated, so you can tell whether a link rate is achievable before you build for it.
- **[Security](/docs/reference/security)**, whether it is safe. The threat model for a library whose input arrives from a radio, and the resource limits each decoder enforces.

The [conformance statements](/conformance) are the other half of the correctness
picture: this section is the method, those are the clause-by-clause results.
