# Astro

[![Go Reference](https://pkg.go.dev/badge/github.com/ravisuhag/astro.svg)](https://pkg.go.dev/github.com/ravisuhag/astro)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Astro is a Go library and CLI implementing [CCSDS](https://public.ccsds.org) and [ECSS](https://ecss.nl) space communication standards, the protocols NASA, ESA, JAXA and other agencies use to talk to spacecraft.

29 standards, from channel coding up to mission operations. `pkg/` takes no dependencies outside the Go standard library.

**[Documentation](https://astro.ravisuhag.com)** | [Protocols](https://astro.ravisuhag.com/protocols) | [CLI](https://astro.ravisuhag.com/cli) | [Conformance](https://astro.ravisuhag.com/conformance)

## Install

Requires Go 1.26 or later.

```bash
go get github.com/ravisuhag/astro                # library
go install github.com/ravisuhag/astro@latest     # CLI
```

## Library

```go
// A telemetry packet from application 100.
packet, _ := spp.NewTMPacket(100, []byte("temperature=22.5"))
encoded, _ := packet.Encode()

// Framed for the downlink: spacecraft 42, virtual channel 0.
frame, _ := tmdl.NewTransferFrame(42, 0, encoded, nil, nil)
frameBytes, _ := frame.Encode()

// Sync marker attached, ready for the radio.
cadu := tmsc.WrapCADU(frameBytes, tmsc.DefaultASM(), true)
```

That is the chain by hand. [`pkg/stack`](https://pkg.go.dev/github.com/ravisuhag/astro/pkg/stack) builds both ends of it from one configuration value, so the spacecraft and the ground station cannot drift apart.

Start with the [Go quickstart](https://astro.ravisuhag.com/docs/start/quickstart-go), then [build a downlink](https://astro.ravisuhag.com/docs/guides/downlink).

## CLI

```bash
# Encode a telemetry Space Packet, then look at what you built
astro spp encode --apid 100 --type tm --data 68656c6c6f | astro spp inspect --input hex

# Encode with a CRC and check it
astro spp encode --apid 100 --type tm --data a1b2c3d4 --crc | astro spp validate --input hex --crc
```

22 commands, one per protocol: `spp` `epp` `tm` `tc` `aos` `usdl` `pxdl` `cadu` `cltu` `pxsc` `ocsc` `sdls` `cop` `time` `xtce` `pus` `ldc` `rhc` `cfdp` `ltp` `bp` `sle`.

Every one takes `encode`, `decode`, `inspect` and friends, reads stdin and writes stdout. Run `astro manual` for the reference built into the binary, or see the [CLI docs](https://astro.ravisuhag.com/cli).

## Protocols

Every standard, its package, and its conformance statement: the [protocol index](https://astro.ravisuhag.com/protocols).

Packets [SPP](https://astro.ravisuhag.com/protocols/transport/spp) | [EPP](https://astro.ravisuhag.com/protocols/transport/epp), data link [TM](https://astro.ravisuhag.com/protocols/data-link/tmdl) | [TC](https://astro.ravisuhag.com/protocols/data-link/tcdl) | [AOS](https://astro.ravisuhag.com/protocols/data-link/aos) | [USLP](https://astro.ravisuhag.com/protocols/data-link/usdl) | [Proximity-1](https://astro.ravisuhag.com/protocols/data-link/pxdl), coding [TMSC](https://astro.ravisuhag.com/protocols/coding/tmsc) | [TCSC](https://astro.ravisuhag.com/protocols/coding/tcsc) | [PXSC](https://astro.ravisuhag.com/protocols/coding/pxsc) | [OCSC](https://astro.ravisuhag.com/protocols/coding/ocsc) (reliability [COP-1](https://astro.ravisuhag.com/protocols/data-link/cop)) security [SDLS](https://astro.ravisuhag.com/protocols/data-link/sdls), files and DTN [CFDP](https://astro.ravisuhag.com/protocols/transport/cfdp) | [LTP](https://astro.ravisuhag.com/protocols/transport/ltp) | [BP](https://astro.ravisuhag.com/protocols/transport/bp) | [BPSec](https://astro.ravisuhag.com/protocols/transport/bpsec) (ground [SLE](https://astro.ravisuhag.com/protocols/ground/sle) | [CSTS](https://astro.ravisuhag.com/protocols/ground/csts)) compression [LDC](https://astro.ravisuhag.com/protocols/compression/ldc) | [RHC](https://astro.ravisuhag.com/protocols/compression/rhc), mission [time codes](https://astro.ravisuhag.com/protocols/mission/tcf) | [PUS](https://astro.ravisuhag.com/protocols/mission/pus) | [XTCE](https://astro.ravisuhag.com/protocols/mission/xtce) | [ODM](https://astro.ravisuhag.com/protocols/mission/odm) | [TDM](https://astro.ravisuhag.com/protocols/mission/tdm) | [ADM](https://astro.ravisuhag.com/protocols/mission/adm) | [CDM](https://astro.ravisuhag.com/protocols/mission/cdm) | [NDM](https://astro.ravisuhag.com/protocols/mission/ndm).

Each package ships a [conformance statement](https://astro.ravisuhag.com/conformance) saying what it implements clause by clause, and what it does not.

## Examples

Fifteen runnable programs, each with a walkthrough under [guides](https://astro.ravisuhag.com/docs/guides).

```bash
# The core chain
go run ./examples/downlink/          # telemetry, spacecraft to ground
go run ./examples/uplink/            # commands, with COP-1 reliable delivery
go run ./examples/lossylink/         # the same downlink under frame loss
go run ./examples/duplex/            # both directions, with the CLCW riding home
go run ./examples/composed/          # both ends from one configuration

# What the bytes mean
go run ./examples/pus/               # five ECSS services working together
go run ./examples/xtce/              # decoding from a mission database
go run ./examples/timecorrelation/   # a drifting clock turned into real time

# Moving more than a packet
go run ./examples/cfdp/              # a file, with the hole in it filled
go run ./examples/dtn/               # store and forward over BP and LTP

# High rate and protected
go run ./examples/aos/               # packets, a bitstream, and opaque blocks
go run ./examples/sdls/              # AES-GCM and AES-CMAC, plus three attacks
go run ./examples/compression/       # Rice coding and POCKET+

# Ground segment
go run ./examples/sle/               # an SLE session over a real TCP connection
go run ./examples/capture/           # writes a capture to practise debugging on
```

## Contributing

Every standard astro has scoped now has a package, so a new protocol starts with a case for why it belongs. The [scope note](https://astro.ravisuhag.com/protocols#out-of-scope) says what astro deliberately leaves out.

Read [contributing](https://astro.ravisuhag.com/docs/contribute) (especially the rule about never coding a constant or field layout from memory) and [adding a protocol](https://astro.ravisuhag.com/docs/contribute/adding-a-protocol). Open an issue to discuss your approach first.

## License

[Apache 2.0](LICENSE).
