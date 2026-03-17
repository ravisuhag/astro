# Astro

[![Go Reference](https://pkg.go.dev/badge/github.com/ravisuhag/astro.svg)](https://pkg.go.dev/github.com/ravisuhag/astro)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/ravisuhag/astro)](https://goreportcard.com/report/github.com/ravisuhag/astro)

Astro is an open-source Go library implementing [CCSDS](https://public.ccsds.org) (Consultative Committee for Space Data Systems) standards — the international protocols used by NASA, ESA, JAXA, and other space agencies for spacecraft communication and data systems.

## Installation

```bash
go get github.com/ravisuhag/astro
```

Requires Go 1.23 or later.

## Usage

```go
import (
	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tmdl"
)

// Create and encode a telemetry packet
packet, _ := spp.NewTMPacket(123, []byte("temperature=22.5"))
encoded, _ := packet.Encode()

// Decode a received packet
decoded, _ := spp.Decode(encoded)

// Create and encode a TM Transfer Frame
frame, _ := tmdl.NewTMTransferFrame(0x1A, 1, encoded, nil, nil)
frameBytes, _ := frame.Encode()

// Decode a received frame
decoded, _ := tmdl.DecodeTMTransferFrame(frameBytes)
```

## Protocols

#### Data Compression

| Protocol | Standard | Package | Docs |
|----------|----------|---------|------|
| Lossless Data Compression | [CCSDS 121.0-B-3](https://public.ccsds.org/Pubs/121x0b3.pdf) | | |
| Image Data Compression | [CCSDS 122.0-B-2](https://public.ccsds.org/Pubs/122x0b2e1.pdf) | | |
| Spectral Preprocessing Transform | [CCSDS 122.1-B-1](https://public.ccsds.org/Pubs/122x1b1e1.pdf) | | |
| Low-Complexity Lossless Image Compression | [CCSDS 123.0-B-2](https://public.ccsds.org/Pubs/123x0b2e2c3.pdf) | | |
| Robust Compression of Housekeeping Data | [CCSDS 124.0-B-1](https://public.ccsds.org/Pubs/124x0b1.pdf) | | |

#### Synchronization and Channel Coding

| Protocol | Standard | Package | Docs |
|----------|----------|---------|------|
| TM Synchronization and Channel Coding | [CCSDS 131.0-B-5](https://public.ccsds.org/Pubs/131x0b5.pdf) | | |
| TC Synchronization and Channel Coding | [CCSDS 231.0-B-4](https://public.ccsds.org/Pubs/231x0b4e1.pdf) | | |
| Optical Communications Coding and Sync | [CCSDS 142.0-B-1](https://public.ccsds.org/Pubs/142x0b1.pdf) | | |
| Proximity-1 Coding and Sync | [CCSDS 211.2-B-3](https://public.ccsds.org/Pubs/211x2b3.pdf) | | |

#### Space Data Link

| Protocol | Standard | Package | Docs |
|----------|----------|---------|------|
| TM Space Data Link Protocol | [CCSDS 132.0-B-3](https://public.ccsds.org/Pubs/132x0b3.pdf) | [`pkg/tmdl`](pkg/tmdl) | [Guide](docs/tmdl.md) \| [PICS](docs/pics/tmdl-pics.md) |
| TC Space Data Link Protocol | [CCSDS 232.0-B-4](https://public.ccsds.org/Pubs/232x0b4e1c1.pdf) | | |
| AOS Space Data Link Protocol | [CCSDS 732.0-B-4](https://public.ccsds.org/Pubs/732x0b4.pdf) | | |
| Unified Space Data Link Protocol | [CCSDS 732.1-B-2](https://public.ccsds.org/Pubs/732x1b2.pdf) | | |
| Proximity-1 Data Link Layer | [CCSDS 211.0-B-6](https://public.ccsds.org/Pubs/211x0b6e1.pdf) | | |
| Communications Operation Procedure-1 | [CCSDS 232.1-B-2](https://public.ccsds.org/Pubs/232x1b2e1.pdf) | | |
| Space Data Link Security | [CCSDS 355.0-B-2](https://public.ccsds.org/Pubs/355x0b2.pdf) | | |

#### Space Packet and Transport

| Protocol | Standard | Package | Docs |
|----------|----------|---------|------|
| Space Packet Protocol | [CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2e2.pdf) | [`pkg/spp`](pkg/spp) | [Guide](docs/spp.md) \| [PICS](docs/pics/spp-pics.md) |
| Encapsulation Packet Protocol | [CCSDS 133.1-B-3](https://public.ccsds.org/Pubs/133x1b3e1.pdf) | | |
| CCSDS File Delivery Protocol | [CCSDS 727.0-B-5](https://public.ccsds.org/Pubs/727x0b5.pdf) | | |

#### Time

| Protocol | Standard | Package | Docs |
|----------|----------|---------|------|
| Time Code Formats | [CCSDS 301.0-B-4](https://public.ccsds.org/Pubs/301x0b4e1.pdf) | | |

## Contributing

Contributions are welcome. Each protocol listed above without a package is open for implementation. To get started:

1. Read the relevant CCSDS Blue Book (linked in the table above).
2. Look at [`pkg/spp`](pkg/spp) or [`pkg/tmdl`](pkg/tmdl) for the established patterns — struct design, encode/decode, validation, options, and tests.
3. Open an issue to discuss your approach before submitting a PR.

## License

This project is licensed under the [Apache 2.0 License](LICENSE).
