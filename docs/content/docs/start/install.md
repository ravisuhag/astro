---
title: Install
short: Install
description: Add the library to a Go project, or install the command line tool.
order: 1
---

Astro needs **Go 1.26 or later**. The `pkg/` libraries have no dependencies outside the standard library, so importing one pulls in nothing else. The CLI adds Cobra for its command tree.

## Library

```bash
go get github.com/ravisuhag/astro
```

Then import the packages you need:

```go
import (
    "github.com/ravisuhag/astro/pkg/spp"
    "github.com/ravisuhag/astro/pkg/tmdl"
)
```

Each protocol lives in its own package under `pkg/`. Nothing pulls in anything else you did not ask for.

## Command line

```bash
go install github.com/ravisuhag/astro@latest
```

That puts an `astro` binary in your `GOBIN`. Check it works:

```bash
astro spp encode --apid 100 --type tm --data 68656c6c6f
```

## From source

```bash
git clone https://github.com/ravisuhag/astro
cd astro
make test
```

The Makefile has the gates the project runs in CI: `make test`, `make race`, `make cover`, `make fuzz-smoke`, and `make lint`.

## Next

- [Go quickstart](/docs/start/quickstart-go), build and parse a packet
- [CLI quickstart](/docs/start/quickstart-cli), do the same from a terminal
- [The stack](/docs/start/concepts), how the layers fit together
