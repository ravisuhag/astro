---
title: astro pxsc
short: PXSC
description: Proximity-1 coding, PLTUs, sync, convolutional code.
order: 100
---

Proximity-1 coding and synchronisation ([CCSDS 211.2-B-3](https://public.ccsds.org/Pubs/211x2b3.pdf)).

A PLTU is the Proximity-1 equivalent of a CADU: the sync marker `0xFAF320`, the transfer frame, then a CRC-32 over the frame.

## Subcommands

| Command | Description |
|---------|-------------|
| `astro pxsc wrap` | Wrap a transfer frame as a PLTU |
| `astro pxsc unwrap` | Extract the frame, verifying the CRC-32 |
| `astro pxsc sync` | Scan a byte stream for PLTUs |
| `astro pxsc encode` | Apply the convolutional code |
| `astro pxsc decode` | Decode it with Viterbi |

---

## astro pxsc wrap

Prepend the sync marker and append the CRC-32, turning a transfer frame into a PLTU.

```
astro pxsc wrap [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `hex` | Output format: `text`, `hex`, or `bin` |

**Examples**

```bash
astro pxdl encode --scid 42 --port 1 --data 0102 | astro pxsc wrap --input hex
```

---

## astro pxsc unwrap

Strip the marker, verify the CRC-32, and return the frame.

A CRC mismatch is an **error**. The frame is corrupt, and passing it up would put bad data into the layer above.

```
astro pxsc unwrap [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `hex` | Output format: `text`, `hex`, or `bin` |
| `--max-frame` | `2048` | Largest frame to accept, in octets |

**Examples**

```bash
astro pxsc unwrap --input hex < pltu.hex | astro pxdl decode --input hex
```

---

## astro pxsc sync

Search a raw stream for the sync marker and extract the frames whose CRC-32 checks out.

Unlike a CADU stream, a PLTU carries no fixed length, so the synchroniser tries candidate lengths and lets the CRC decide. A frame is reported only when its checksum agrees, which is what stops a false marker producing a bogus frame.

```
astro pxsc sync [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text`, `hex`, or `json` |

---

## astro pxsc encode

Apply the rate-1/2 convolutional code of clause 3.3: constraint length 7, generators 171 and 133 in octal. Every input bit becomes two output bits.

```
astro pxsc encode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `hex` | Output format: `text`, `hex`, or `bin` |
| `--flush` | `true` | Append the zero tail the decoder needs |

### The tail

A Viterbi decoder does not commit a bit until it has seen enough of what follows to be sure of it. This one looks back 35 bits. **Without a tail, the last 35 bits of a stream never come out and a round trip silently loses its final few octets.**

So `encode` appends five zero octets by default, which is the smallest whole number covering 35 bits, and a round trip through `encode | decode` comes out exact. Pass `--flush=false` when you are appending more data yourself.

Symbols from elsewhere may still be short by a few octets at the end, which is the decoder working correctly rather than a fault.

---

## astro pxsc decode

Decode code symbols with a Viterbi decoder, recovering the original octets. The decoder corrects errors, which is the point of the code: a symbol stream with a few bits flipped still decodes to the right octets.

```
astro pxsc decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `hex` | Output format: `text`, `hex`, or `bin` |

**Examples**

```bash
# Exact round trip
astro pxsc encode --input hex < frame.hex | astro pxsc decode --input hex

# The full transmit chain
astro pxdl encode --scid 42 --port 1 --data 0102 |
  astro pxsc wrap --input hex |
  astro pxsc encode --input hex
```

The decoder corrects errors, which is the point of the code: a symbol stream with a few bits flipped still decodes to the right octets.

---

**See also**: [the protocol page](/protocols/coding/pxsc) for the standard and the Go API, and the [conformance statement](/conformance/pxsc) for what is and is not implemented.
