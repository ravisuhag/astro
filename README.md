# Astro

[![Go Reference](https://pkg.go.dev/badge/github.com/ravisuhag/astro.svg)](https://pkg.go.dev/github.com/ravisuhag/astro)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/ravisuhag/astro)](https://goreportcard.com/report/github.com/ravisuhag/astro)

Astro is an open-source Go library and CLI implementing [CCSDS](https://public.ccsds.org) and [ECSS](https://ecss.nl) space communication standards — the protocols NASA, ESA, JAXA and other agencies use for spacecraft communication and data systems.

22 standards across packets, data link, coding and synchronization, ground transfer, compression, time, packet utilization, and mission databases. No dependencies outside the Go standard library.

**[Documentation](https://astro-docs.vercel.app)** · [Protocols](https://astro-docs.vercel.app/protocols) · [CLI](https://astro-docs.vercel.app/cli)

## Install

Requires Go 1.26 or later.

```bash
go get github.com/ravisuhag/astro          # library
go install github.com/ravisuhag/astro@latest   # CLI
```

## Library

```go
import (
	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tmdl"
	"github.com/ravisuhag/astro/pkg/tmsc"
)

// A telemetry packet from application 100.
packet, _ := spp.NewTMPacket(100, []byte("temperature=22.5"))
encoded, _ := packet.Encode()

// Framed for the downlink: spacecraft 42, virtual channel 0.
frame, _ := tmdl.NewTMTransferFrame(42, 0, encoded, nil, nil)
frameBytes, _ := frame.Encode()

// Sync marker attached, ready for the radio.
cadu := tmsc.WrapCADU(frameBytes, tmsc.DefaultASM(), true)
```

See the [Go quickstart](https://astro-docs.vercel.app/docs/start/quickstart-go) to go further, or [build a downlink](https://astro-docs.vercel.app/docs/guides/downlink) for the full chain with services and virtual channels.

## CLI

```bash
# Encode a telemetry Space Packet
astro spp encode --apid 100 --type tm --data 68656c6c6f

# Inspect one with an annotated hex dump
astro spp encode --apid 100 --type tm --data 68656c6c6f | astro spp inspect --input hex

# Verify a CRC
astro spp encode --apid 100 --type tm --data a1b2c3d4 --crc | astro spp validate --input hex --crc
```

Commands: `spp`, `epp`, `tm`, `tc`, `aos`, `usdl`, `cadu`, `cltu`, `time`. Run `astro manual` for the built-in reference, or see the [CLI docs](https://astro-docs.vercel.app/cli).

## Protocols

The full table — every standard, its package, and its conformance statement — is in the [protocol index](https://astro-docs.vercel.app/protocols).

Briefly: [SPP](https://astro-docs.vercel.app/protocols/transport/spp) and [EPP](https://astro-docs.vercel.app/protocols/transport/epp) for packets; [TM](https://astro-docs.vercel.app/protocols/data-link/tmdl), [TC](https://astro-docs.vercel.app/protocols/data-link/tcdl), [AOS](https://astro-docs.vercel.app/protocols/data-link/aos), [USLP](https://astro-docs.vercel.app/protocols/data-link/usdl) and [Proximity-1](https://astro-docs.vercel.app/protocols/data-link/pxdl) for data link; [TMSC](https://astro-docs.vercel.app/protocols/coding/tmsc), [TCSC](https://astro-docs.vercel.app/protocols/coding/tcsc), [PXSC](https://astro-docs.vercel.app/protocols/coding/pxsc) and [OCSC](https://astro-docs.vercel.app/protocols/coding/ocsc) for coding; [COP-1](https://astro-docs.vercel.app/protocols/data-link/cop) for reliable commanding and [SDLS](https://astro-docs.vercel.app/protocols/data-link/sdls) for security; [CFDP](https://astro-docs.vercel.app/protocols/transport/cfdp), [LTP](https://astro-docs.vercel.app/protocols/transport/ltp) and [BP](https://astro-docs.vercel.app/protocols/transport/bp) for files and delay-tolerant networking; [SLE](https://astro-docs.vercel.app/protocols/ground/sle) for ground-to-ground; [LDC](https://astro-docs.vercel.app/protocols/compression/ldc) and [RHC](https://astro-docs.vercel.app/protocols/compression/rhc) for compression; [time codes](https://astro-docs.vercel.app/protocols/mission/tcf), [PUS](https://astro-docs.vercel.app/protocols/mission/pus), and [XTCE](https://astro-docs.vercel.app/protocols/mission/xtce).

## Examples

Runnable programs in [`examples/`](examples), each with a [walkthrough](https://astro-docs.vercel.app/docs/guides/downlink):

```bash
go run ./examples/downlink/    # telemetry, spacecraft to ground
go run ./examples/uplink/      # commands, with COP-1 reliable delivery
go run ./examples/lossylink/   # the same downlink under frame loss
```

## Contributing

Contributions are welcome. Several standards are still unimplemented and open.

Read [CONTRIBUTING.md](CONTRIBUTING.md) — especially the rule about never coding a constant or field layout from memory — and [adding a protocol](https://astro-docs.vercel.app/docs/contribute/adding-a-protocol) for the conventions and required docs. Open an issue to discuss your approach before submitting a pull request.

## License

[Apache 2.0](LICENSE).
