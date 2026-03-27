# Astro

[![Go Reference](https://pkg.go.dev/badge/github.com/ravisuhag/astro.svg)](https://pkg.go.dev/github.com/ravisuhag/astro)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/ravisuhag/astro)](https://goreportcard.com/report/github.com/ravisuhag/astro)

Astro is an open-source Go library implementing [CCSDS](https://public.ccsds.org) and [ECSS](https://ecss.nl) space communication standards — the international protocols used by NASA, ESA, JAXA, and other space agencies for spacecraft communication and data systems.

## Installation

### Library

```bash
go get github.com/ravisuhag/astro
```

### CLI

```bash
go install github.com/ravisuhag/astro@latest
```

Requires Go 1.26 or later.

## CLI

The `astro` CLI provides commands for encoding, decoding, inspecting, and validating CCSDS data directly from the terminal.

```bash
# Encode a telemetry Space Packet
astro spp encode --apid 100 --type tm --data 68656c6c6f

# Inspect a packet with annotated hex dump
astro spp encode --apid 100 --type tm --data 68656c6c6f | astro spp inspect --input hex

# Validate a packet with CRC verification
astro spp encode --apid 100 --type tm --data a1b2c3d4 --crc | astro spp validate --input hex --crc
```

| Command | Description | Docs |
|---------|-------------|------|
| `astro spp` | Space Packet Protocol — encode, decode, inspect, validate, stream | [Reference](docs/cli/spp.md) |

## Library Usage

```go
import (
	"github.com/ravisuhag/astro/pkg/crc"
	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tmdl"
	"github.com/ravisuhag/astro/pkg/tmsc"
)

// Create and encode a telemetry packet
packet, _ := spp.NewTMPacket(123, []byte("temperature=22.5"))
encoded, _ := packet.Encode()

// Frame the packet with TM Data Link
frame, _ := tmdl.NewTMTransferFrame(0x1A, 1, encoded, nil, nil)
frameBytes, _ := frame.Encode()

// Wrap the frame as a Channel Access Data Unit (CADU)
cadu, _ := tmsc.NewCADU(frameBytes)
caduBytes, _ := cadu.Encode()

// Compute CRC for error detection
checksum := crc.CRC16CCITT(caduBytes)
```

## Protocols

| Protocol | Standard | Package | Docs |
|----------|----------|---------|------|
| **Space Packet and Transport** | | | |
| Space Packet Protocol | [CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2e2.pdf) | [`pkg/spp`](pkg/spp) | [Guide](docs/spp.md) \| [CLI](docs/cli/spp.md) \| [PICS](docs/pics/spp-pics.md) |
| Encapsulation Packet Protocol | [CCSDS 133.1-B-3](https://public.ccsds.org/Pubs/133x1b3e1.pdf) | | |
| CCSDS File Delivery Protocol | [CCSDS 727.0-B-5](https://public.ccsds.org/Pubs/727x0b5.pdf) | | |
| Licklider Transmission Protocol | [CCSDS 734.1-B-1](https://public.ccsds.org/Pubs/734x1b1.pdf) | | |
| Bundle Protocol | [CCSDS 734.2-B-1](https://public.ccsds.org/Pubs/734x2b1.pdf) | | |
| **Space Data Link** | | | |
| TM Space Data Link Protocol | [CCSDS 132.0-B-3](https://public.ccsds.org/Pubs/132x0b3.pdf) | [`pkg/tmdl`](pkg/tmdl) | [Guide](docs/tmdl.md) \| [PICS](docs/pics/tmdl-pics.md) |
| Proximity-1 Data Link Layer | [CCSDS 211.0-B-6](https://public.ccsds.org/Pubs/211x0b6e1.pdf) | | |
| TC Space Data Link Protocol | [CCSDS 232.0-B-4](https://public.ccsds.org/Pubs/232x0b4e1c1.pdf) | [`pkg/tcdl`](pkg/tcdl) | [Guide](docs/tcdl.md) \| [PICS](docs/pics/tcdl-pics.md) |
| Communications Operation Procedure-1 | [CCSDS 232.1-B-2](https://public.ccsds.org/Pubs/232x1b2e1.pdf) | [`pkg/cop`](pkg/cop) | [Guide](docs/cop.md) \| [PICS](docs/pics/cop-pics.md) |
| Space Data Link Security | [CCSDS 355.0-B-2](https://public.ccsds.org/Pubs/355x0b2.pdf) | | |
| AOS Space Data Link Protocol | [CCSDS 732.0-B-4](https://public.ccsds.org/Pubs/732x0b4.pdf) | | |
| Unified Space Data Link Protocol | [CCSDS 732.1-B-2](https://public.ccsds.org/Pubs/732x1b2.pdf) | | |
| **Synchronization and Channel Coding** | | | |
| TM Synchronization and Channel Coding | [CCSDS 131.0-B-5](https://public.ccsds.org/Pubs/131x0b5.pdf) | [`pkg/tmsc`](pkg/tmsc) | [Guide](docs/tmsc.md) \| [PICS](docs/pics/tmsc-pics.md) |
| Optical Communications Coding and Sync | [CCSDS 142.0-B-1](https://public.ccsds.org/Pubs/142x0b1.pdf) | | |
| Proximity-1 Coding and Sync | [CCSDS 211.2-B-3](https://public.ccsds.org/Pubs/211x2b3.pdf) | | |
| TC Synchronization and Channel Coding | [CCSDS 231.0-B-4](https://public.ccsds.org/Pubs/231x0b4e1.pdf) | [`pkg/tcsc`](pkg/tcsc) | [Guide](docs/tcsc.md) \| [PICS](docs/pics/tcsc-pics.md) |
| **Space Link Extension** | | | |
| SLE Return All Frames | [CCSDS 911.1-B-4](https://public.ccsds.org/Pubs/911x1b4.pdf) | | |
| SLE Return Channel Frames | [CCSDS 911.2-B-3](https://public.ccsds.org/Pubs/911x2b3.pdf) | | |
| SLE Forward CLTU | [CCSDS 912.1-B-4](https://public.ccsds.org/Pubs/912x1b4.pdf) | | |
| SLE Return Operational Control Fields | [CCSDS 913.1-B-2](https://public.ccsds.org/Pubs/913x1b2.pdf) | | |
| SLE Internet Protocol for Transfer Services | [CCSDS 914.0-B-2](https://public.ccsds.org/Pubs/914x0b2.pdf) | | |
| **Data Compression** | | | |
| Lossless Data Compression | [CCSDS 121.0-B-3](https://public.ccsds.org/Pubs/121x0b3.pdf) | | |
| Image Data Compression | [CCSDS 122.0-B-2](https://public.ccsds.org/Pubs/122x0b2e1.pdf) | | |
| Spectral Preprocessing Transform | [CCSDS 122.1-B-1](https://public.ccsds.org/Pubs/122x1b1e1.pdf) | | |
| Low-Complexity Lossless Image Compression | [CCSDS 123.0-B-2](https://public.ccsds.org/Pubs/123x0b2e2c3.pdf) | | |
| Robust Compression of Housekeeping Data | [CCSDS 124.0-B-1](https://public.ccsds.org/Pubs/124x0b1.pdf) | | |
| **Time** | | | |
| Time Code Formats | [CCSDS 301.0-B-4](https://public.ccsds.org/Pubs/301x0b4e1.pdf) | [`pkg/tcf`](pkg/tcf) | [Guide](docs/tcf.md) |
| **Packet Utilization** | | | |
| Packet Utilization Standard | [ECSS-E-ST-70-41C](https://ecss.nl/standard/ecss-e-st-70-41c-space-engineering-telemetry-and-telecommand-packet-utilization-15-april-2016/) | | |
| Test and Operations Procedure Language | [ECSS-E-ST-70-32C](https://ecss.nl/standard/ecss-e-st-70-32c-rev-1-test-and-operations-procedure-language/) | | |
| Space Data Links — Service Specification | [ECSS-E-ST-50-03C](https://ecss.nl/standard/ecss-e-st-50-03c-rev-1-space-data-links-telemetry-transfer-frame-protocol/) | | |
| **Mission Database** | | | |
| XML Telemetric and Command Exchange | [XTCE](https://www.omg.org/spec/XTCE/) / [CCSDS 660.1-G-2](https://public.ccsds.org/Pubs/660x1g2.pdf) | | |
| **Shared Utilities** | | | |
| CRC-16-CCITT | [CCSDS 130.0-G-3](https://public.ccsds.org/Pubs/130x0g3.pdf) | [`pkg/crc`](pkg/crc) | |

## Contributing

Contributions are welcome. Each protocol listed above without a package is open for implementation. To get started:

1. Read the relevant CCSDS Blue Book or ECSS standard (linked in the table above).
2. Look at [`pkg/spp`](pkg/spp), [`pkg/tmdl`](pkg/tmdl), [`pkg/tmsc`](pkg/tmsc), or [`pkg/crc`](pkg/crc) for the established patterns — struct design, encode/decode, validation, options, and tests.
3. Open an issue to discuss your approach before submitting a PR.

## License

This project is licensed under the [Apache 2.0 License](LICENSE).
