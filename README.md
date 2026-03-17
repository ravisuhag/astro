# Astro

[![Go Reference](https://pkg.go.dev/badge/github.com/ravisuhag/astro.svg)](https://pkg.go.dev/github.com/ravisuhag/astro)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/ravisuhag/astro)](https://goreportcard.com/report/github.com/ravisuhag/astro)

Astro is an open-source Go library implementing CCSDS (Consultative Committee for Space Data Systems) protocols — the international standards used by NASA, ESA, JAXA, and other space agencies for spacecraft communication and data systems.

## Installation

```bash
go get github.com/ravisuhag/astro
```

Requires Go 1.23 or later.

## Packages

| Package | Protocol | Standard | Docs |
|---------|----------|----------|------|
| [`pkg/spp`](pkg/spp) | Space Packet Protocol | [CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2e2.pdf) | [Guide](docs/spp.md) |
| [`pkg/tmdl`](pkg/tmdl) | TM Space Data Link Protocol | [CCSDS 132.0-B-3](https://public.ccsds.org/Pubs/132x0b3.pdf) | [Guide](docs/tmdl.md) |

## Usage

```go
// Create and encode a telemetry packet
packet, _ := spp.NewTMPacket(123, []byte("temperature=22.5"))
encoded, _ := packet.Encode()

// Decode a received packet
decoded, _ := spp.Decode(encoded)

// Create a TM Transfer Frame
frame, _ := tmdl.NewTMTransferFrame(0x1A, 1, payload, nil, nil)
encoded = frame.Encode()
```

See the [SPP Guide](docs/spp.md) and [TMDL Guide](docs/tmdl.md) for detailed examples.

## Roadmap

Implementation of CCSDS protocols:

| Protocol | Standard | Status |
|----------|----------|:------:|
| Lossless Data Compression | [CCSDS 121.0-B-3](https://public.ccsds.org/Pubs/121x0b3.pdf) | |
| Image Data Compression | [CCSDS 122.0-B-2](https://public.ccsds.org/Pubs/122x0b2e1.pdf) | |
| Spectral Preprocessing Transform | [CCSDS 122.1-B-1](https://public.ccsds.org/Pubs/122x1b1e1.pdf) | |
| Low-Complexity Lossless Image Compression | [CCSDS 123.0-B-2](https://public.ccsds.org/Pubs/123x0b2e2c3.pdf) | |
| Robust Compression of Housekeeping Data | [CCSDS 124.0-B-1](https://public.ccsds.org/Pubs/124x0b1.pdf) | |
| TM Synchronization and Channel Coding | [CCSDS 131.0-B-5](https://public.ccsds.org/Pubs/131x0b5.pdf) | |
| Flexible Advanced Coding and Modulation | [CCSDS 131.2-B-2](https://public.ccsds.org/Pubs/131x2b2.pdf) | |
| Space Link Protocols over DVB-S2 | [CCSDS 131.3-B-2](https://public.ccsds.org/Pubs/131x3b2e1.pdf) | |
| TM Space Data Link Protocol | [CCSDS 132.0-B-3](https://public.ccsds.org/Pubs/132x0b3.pdf) | ✅ |
| Space Packet Protocol | [CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2e2.pdf) | ✅ |
| Encapsulation Packet Protocol | [CCSDS 133.1-B-3](https://public.ccsds.org/Pubs/133x1b3e1.pdf) | |
| Optical Communications Physical Layer | [CCSDS 141.0-B-1](https://public.ccsds.org/Pubs/141x0b1.pdf) | |
| Optical Communications Coding and Sync | [CCSDS 142.0-B-1](https://public.ccsds.org/Pubs/142x0b1.pdf) | |
| Proximity-1 Data Link Layer | [CCSDS 211.0-B-6](https://public.ccsds.org/Pubs/211x0b6e1.pdf) | |
| Proximity-1 Physical Layer | [CCSDS 211.1-B-4](https://public.ccsds.org/Pubs/211x1b4e1.pdf) | |
| Proximity-1 Coding and Sync | [CCSDS 211.2-B-3](https://public.ccsds.org/Pubs/211x2b3.pdf) | |
| TC Synchronization and Channel Coding | [CCSDS 231.0-B-4](https://public.ccsds.org/Pubs/231x0b4e1.pdf) | |
| TC Space Data Link Protocol | [CCSDS 232.0-B-4](https://public.ccsds.org/Pubs/232x0b4e1c1.pdf) | |
| Time Code Formats | [CCSDS 301.0-B-4](https://public.ccsds.org/Pubs/301x0b4e1.pdf) | |

## License

This project is licensed under the [Apache 2.0 License](LICENSE).
