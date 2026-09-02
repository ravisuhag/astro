# Astro

[![Go Reference](https://pkg.go.dev/badge/github.com/ravisuhag/astro.svg)](https://pkg.go.dev/github.com/ravisuhag/astro)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Astro is a Go library and CLI implementing [CCSDS](https://public.ccsds.org) and [ECSS](https://ecss.nl) space communication standards, the protocols NASA, ESA, JAXA and other agencies use to talk to spacecraft.

22 standards, from channel coding up to mission operations. `pkg/` takes no dependencies outside the Go standard library.

**[Documentation](https://astro-docs.vercel.app)** | [Protocols](https://astro-docs.vercel.app/protocols) | [CLI](https://astro-docs.vercel.app/cli) | [Conformance](https://astro-docs.vercel.app/conformance)

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
frame, _ := tmdl.NewTMTransferFrame(42, 0, encoded, nil, nil)
frameBytes, _ := frame.Encode()

// Sync marker attached, ready for the radio.
cadu := tmsc.WrapCADU(frameBytes, tmsc.DefaultASM(), true)
```

That is the chain by hand. [`pkg/stack`](https://pkg.go.dev/github.com/ravisuhag/astro/pkg/stack) builds both ends of it from one configuration value, so the spacecraft and the ground station cannot drift apart.

Start with the [Go quickstart](https://astro-docs.vercel.app/docs/start/quickstart-go), then [build a downlink](https://astro-docs.vercel.app/docs/guides/downlink).

## CLI

```bash
# Encode a telemetry Space Packet, then look at what you built
astro spp encode --apid 100 --type tm --data 68656c6c6f | astro spp inspect --input hex

# Encode with a CRC and check it
astro spp encode --apid 100 --type tm --data a1b2c3d4 --crc | astro spp validate --input hex --crc
```

22 commands, one per protocol: `spp` `epp` `tm` `tc` `aos` `usdl` `pxdl` `cadu` `cltu` `pxsc` `ocsc` `sdls` `cop` `time` `xtce` `pus` `ldc` `rhc` `cfdp` `ltp` `bp` `sle`.

Every one takes `encode`, `decode`, `inspect` and friends, reads stdin and writes stdout. Run `astro manual` for the reference built into the binary, or see the [CLI docs](https://astro-docs.vercel.app/cli).

## Protocols

Every standard, its package, and its conformance statement: the [protocol index](https://astro-docs.vercel.app/protocols).

Packets [SPP](https://astro-docs.vercel.app/protocols/transport/spp) | [EPP](https://astro-docs.vercel.app/protocols/transport/epp), data link [TM](https://astro-docs.vercel.app/protocols/data-link/tmdl) | [TC](https://astro-docs.vercel.app/protocols/data-link/tcdl) | [AOS](https://astro-docs.vercel.app/protocols/data-link/aos) | [USLP](https://astro-docs.vercel.app/protocols/data-link/usdl) | [Proximity-1](https://astro-docs.vercel.app/protocols/data-link/pxdl), coding [TMSC](https://astro-docs.vercel.app/protocols/coding/tmsc) | [TCSC](https://astro-docs.vercel.app/protocols/coding/tcsc) | [PXSC](https://astro-docs.vercel.app/protocols/coding/pxsc) | [OCSC](https://astro-docs.vercel.app/protocols/coding/ocsc) (reliability [COP-1](https://astro-docs.vercel.app/protocols/data-link/cop)) security [SDLS](https://astro-docs.vercel.app/protocols/data-link/sdls), files and DTN [CFDP](https://astro-docs.vercel.app/protocols/transport/cfdp) | [LTP](https://astro-docs.vercel.app/protocols/transport/ltp) | [BP](https://astro-docs.vercel.app/protocols/transport/bp) (ground [SLE](https://astro-docs.vercel.app/protocols/ground/sle)) compression [LDC](https://astro-docs.vercel.app/protocols/compression/ldc) | [RHC](https://astro-docs.vercel.app/protocols/compression/rhc), mission [time codes](https://astro-docs.vercel.app/protocols/mission/tcf) | [PUS](https://astro-docs.vercel.app/protocols/mission/pus) | [XTCE](https://astro-docs.vercel.app/protocols/mission/xtce).

Each package ships a [conformance statement](https://astro-docs.vercel.app/conformance) saying what it implements clause by clause, and what it does not.

## Examples

```bash
go run ./examples/downlink/    # telemetry, spacecraft to ground
go run ./examples/uplink/      # commands, with COP-1 reliable delivery
go run ./examples/lossylink/   # the same downlink under frame loss
go run ./examples/composed/    # both ends from one configuration
```

Each has a walkthrough under [building things](https://astro-docs.vercel.app/docs/guides/downlink).

## Contributing

Several standards are still [unimplemented and open](https://astro-docs.vercel.app/protocols#not-implemented-yet).

Read [contributing](https://astro-docs.vercel.app/docs/contribute) (especially the rule about never coding a constant or field layout from memory) and [adding a protocol](https://astro-docs.vercel.app/docs/contribute/adding-a-protocol). Open an issue to discuss your approach first.

## License

[Apache 2.0](LICENSE).
