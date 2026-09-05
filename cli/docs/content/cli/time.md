---
title: astro time
short: TIME
description: test fixture for the manual command, not the real docs page.
order: 140
---

# astro time (test fixture)

This file exists only so `TestRunCLI_Manual` has something real to read:
`cli.New(embed.FS{})` in the ordinary test harness gives `manual` an empty
filesystem, and `printManual` always fails at `ReadFile` against it. A small
testdata copy lets the command actually run end to end.
