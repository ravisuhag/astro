---
title: TM Sync and Channel Coding
short: TMSC
description: CCSDS 131.0-B-5, sync markers, randomization, and Reed-Solomon on the downlink.
identifiers:
  - "CCSDS 131.0-B-5 * TM Sync and Channel Coding"
  - "pkg/tmsc * astro cadu"
order: 30
---

> **CCSDS 131.0-B-5** | [Blue Book](https://public.ccsds.org/Pubs/131x0b5.pdf) | [`pkg/tmsc`](https://github.com/ravisuhag/astro/tree/main/pkg/tmsc) | [`astro cadu`](/cli/cadu)

## Overview

This is the layer between a [TM frame](/protocols/data-link/tmdl) and the radio. It does three things: puts a known pattern in front of each frame so a receiver can find frame boundaries in a continuous bitstream, scrambles the bits so the receiver's clock recovery has transitions to lock onto, and adds Reed-Solomon parity so bit errors can be corrected without asking for a resend.

Everything here is undone at the other end. The data link layer never knows it happened.

## Scope

**Implemented.** The Attached Sync Marker and CADU wrap and unwrap, the 255-bit pseudo-randomizer, and Reed-Solomon RS(255,223) and RS(255,239) with interleaving, dual-basis conversion, and shortened codeblocks.

**Not here.** The long 131,071-bit randomizer added in Issue 5 (clause 10.4.1) for high-rate links. Also no convolutional or LDPC/Turbo codes. The RS codes are what `pkg/tmsc` covers.

**Also handles AOS.** [AOS frames](/protocols/data-link/aos) use this same sublayer. So do [USLP](/protocols/data-link/usdl) frames on a downlink.

## Field map

There are no header fields here. The sublayer wraps rather than annotates.

| Piece | Size | Go | Notes |
|---|---|---|---|
| Attached Sync Marker | 4 B | `DefaultASM()` | `0x1ACFFC1D`. Never randomized. |
| Transfer Frame | fixed | - | Whatever the data link handed down |
| Reed-Solomon parity | 32 or 16 B per codeword | `RSCodec` | RS(255,223) or RS(255,239) |

A CADU is the ASM plus the coded, randomized frame. `WrapCADU(frame, asm, randomize)` builds one; `UnwrapCADU` takes it apart.

| Code | Constructor | Parity | Corrects |
|---|---|---|---|
| RS(255,223) | `NewRS255_223()` | 32 symbols | up to 16 symbol errors |
| RS(255,239) | `NewRS255_239()` | 16 symbols | up to 8 symbol errors |

Interleave depths: 1, 2, 3, 4, 5, and 8. Depth 5 with RS(255,223) is common for deep space, 1275 bytes per interleaved block.

## Gotchas

**The ASM is never randomized.** Only the frame content is XORed. Randomizing the marker would defeat the point of having one, since the receiver has to find it before it can de-randomize anything.

**Astro implements the 255-bit randomizer, not the long one.** Clause 10.4.2, the legacy sequence: LFSR polynomial `x^8 + x^7 + x^5 + x^3 + 1`, seeded all ones, period 255 bits. Issue 5 added a 131,071-bit sequence (`x^17 + x^14 + 1`, clause 10.4.1) to avoid spectral spikes at high data rates. Which randomizer a channel uses is a managed parameter, check yours matches.

**`Randomize` is its own inverse.** XOR twice with the same sequence gives you back what you started with, so the same function serves transmit and receive. There is no `Derandomize`.

**CCSDS Reed-Solomon uses the dual basis.** Symbols are converted to the Berlekamp dual basis before encoding and back afterwards. A textbook RS(255,223) implementation will not interoperate. Astro handles the conversion, but this is why you cannot swap in another library.

**Virtual fill is a managed parameter, not a signaled one.** Shortened codeblocks (clauses 4.3.7 and 4.3.8) work by both ends agreeing that Q zero symbols sit at the front. Nothing in the stream says so. Configure encoder and decoder with the same Q or nothing decodes.

Q must be a multiple of the interleave depth, so each codeword shortens equally, and must leave at least one data symbol per codeword. Otherwise `ErrInvalidVirtualFill`.

**A "corrected" fill position means the block is bad.** Zero is zero in both bases, so the decoder just prepends Q zeros, decodes, and strips them. If error correction turns a fill position nonzero, the transmitter cannot have sent that codeword, and Astro rejects the block instead of returning plausible garbage.

**Interleaving is what makes burst errors survivable.** A fade that corrupts 30 consecutive bytes would blow past RS(255,223)'s 16-symbol limit inside one codeword. At depth 5 the same burst puts 6 bytes into each of 5 codewords, and every one of them is correctable.

## Quick start

```go
import "github.com/ravisuhag/astro/pkg/tmsc"

// Wrap a Transfer Frame into a CADU for transmission
cadu := tmsc.WrapCADU(encodedFrame, nil, true) // nil=default ASM, true=randomize

// Unwrap a received CADU back into a Transfer Frame
frameData, err := tmsc.UnwrapCADU(cadu, nil, true)

// Reed-Solomon error correction
rs := tmsc.NewRS255_223()
codeword, _ := rs.Encode(data)
corrected, nerrs, _ := rs.Decode(codeword)
```

## Architecture

The TMSC sublayer sits between the TM Data Link Protocol (`tmdl`) and the physical layer:

```
+-----------------------------------------+
|  TM Space Data Link Protocol (tmdl)     |
|  Packs data into Transfer Frames        |
+-----------------------------------------+
|  TM Sync & Channel Coding (tmsc)        |  <-- This package
|  ASM, pseudo-randomization, FEC         |
+-----------------------------------------+
|  Physical Layer (RF/Optical link)       |
+-----------------------------------------+
```

## Attached sync marker (ASM)

The ASM is a known 4-byte bit pattern prepended to each Transfer Frame. The receiver uses it to find frame boundaries in the continuous bitstream.

```go
// Get the standard CCSDS ASM (0x1ACFFC1D)
asm := tmsc.DefaultASM() // Returns []byte{0x1A, 0xCF, 0xFC, 0x1D}
```

The ASM was carefully chosen for its autocorrelation properties. It can be detected reliably even in the presence of noise. A fresh copy is returned each call to prevent accidental mutation.

## CADU wrapping and unwrapping

A **Channel Access Data Unit (CADU)** is the combination of ASM + Transfer Frame data. This is the unit that is actually transmitted over the physical link.

```
+--------+------------------------+
|  ASM   |    Transfer Frame      |
| (4B)   |                        |
+--------+------------------------+
|<------------ CADU ------------->|
```

### Wrapping (send path)

```go
// Wrap with default ASM and pseudo-randomization
cadu := tmsc.WrapCADU(encodedFrame, nil, true)

// Wrap with default ASM, no randomization
cadu := tmsc.WrapCADU(encodedFrame, nil, false)

// Wrap with custom ASM
customASM := []byte{0xDE, 0xAD, 0xBE, 0xEF}
cadu := tmsc.WrapCADU(encodedFrame, customASM, true)
```

### Unwrapping (receive path)

```go
// Unwrap with default ASM and de-randomization
frameData, err := tmsc.UnwrapCADU(cadu, nil, true)
if errors.Is(err, tmsc.ErrSyncMarkerMismatch) {
    // ASM not found at expected position
}

// Unwrap without de-randomization
frameData, err := tmsc.UnwrapCADU(cadu, nil, false)
```

## Pseudo-Randomization

CCSDS pseudo-randomization ensures good signal properties by preventing long runs of identical bits that can confuse clock recovery. The Transfer Frame bytes are XORed with a pseudo-random noise (PN) sequence.

```go
// Randomize data (XOR with PN sequence)
randomized := tmsc.Randomize(data)

// De-randomize (same operation, XOR is self-inverse)
original := tmsc.Randomize(randomized)
```

The PN sequence is generated by an 8-bit LFSR with polynomial `h(x) = x^8 + x^7 + x^5 + x^3 + 1`, initialized to all 1s (`0xFF`).

```go
// Generate PN sequence of arbitrary length
pnSeq := tmsc.GeneratePNSequence(1024)
```

**Important:** The ASM is never randomized. Only the Transfer Frame content is XORed.

## Reed-Solomon error correction

The package provides CCSDS Reed-Solomon codes over GF(2^8) with primitive polynomial `0x187` and first consecutive root (FCR) 112.

### Available codes

| Code | Parity Symbols | Error Correction | Use Case |
|---|---|---|---|
| RS(255,223) | 32 | Up to 16 errors | Standard CCSDS coding |
| RS(255,239) | 16 | Up to 8 errors | Lower overhead alternative |

### Basic encoding and decoding

```go
// Create a codec
rs := tmsc.NewRS255_223() // or tmsc.NewRS255_239()

// Encode: data (223 bytes) -> codeword (255 bytes)
data := make([]byte, rs.DataLen()) // 223 bytes for RS(255,223)
codeword, err := rs.Encode(data)

// Decode: corrects errors in-place, returns corrected data
corrected, numErrors, err := rs.Decode(codeword)
if errors.Is(err, tmsc.ErrUncorrectable) {
    // Too many errors to correct
}
fmt.Printf("Corrected %d symbol errors\n", numErrors)
```

### Interleaved encoding and decoding

CCSDS supports symbol interleaving to spread burst errors across multiple codewords, improving resilience against clustered bit errors.

Valid interleave depths: 1, 2, 3, 4, 5, 8.

```go
rs := tmsc.NewRS255_223()
depth := 4

// Input must be exactly depth * DataLen() bytes
data := make([]byte, depth*rs.DataLen()) // 4 * 223 = 892 bytes
interleaved, err := rs.EncodeInterleaved(data, depth)
// Output: depth * 255 = 1020 bytes

// Decode interleaved data
corrected, totalErrors, err := rs.DecodeInterleaved(interleaved, depth)
```

**How interleaving works:**
1. Input data is de-interleaved into `depth` separate blocks.
2. Each block is independently RS-encoded to a 255-byte codeword.
3. The codewords are interleaved byte-by-byte into the output.

On decode, the reverse is performed. This means a burst error affecting consecutive bytes in the interleaved stream is distributed across multiple codewords, where each codeword sees only a few symbol errors.

### Codec properties

```go
rs := tmsc.NewRS255_223()
rs.NRoots()  // 32 (number of parity symbols
rs.DataLen() // 223) data bytes per codeword
```

## Full pipeline example

### Send path (spacecraft to ground)

```go
import (
    "github.com/ravisuhag/astro/pkg/tmdl"
    "github.com/ravisuhag/astro/pkg/tmsc"
)

// 1. Get a Transfer Frame from the TM Data Link layer
frame, _ := pc.GetNextFrame()
encoded, _ := frame.Encode()

// 2. (Optional) Apply Reed-Solomon encoding
rs := tmsc.NewRS255_223()
// Pad encoded frame to RS data length if needed, or use interleaving

// 3. Wrap as CADU, randomize and prepend ASM
cadu := tmsc.WrapCADU(encoded, nil, true)

// 4. Transmit CADU over the physical link
transmit(cadu)
```

### Receive path (ground station)

```go
// 1. Receive CADU from physical link
cadu := receive()

// 2. Unwrap CADU, strip ASM and de-randomize
frameData, err := tmsc.UnwrapCADU(cadu, nil, true)
if err != nil { /* handle sync errors */ }

// 3. (Optional) Apply Reed-Solomon decoding/correction

// 4. Decode the Transfer Frame
frame, err := tmdl.DecodeTransferFrame(frameData)
if err != nil { /* handle CRC errors */ }
```

## Errors

All errors are exported package-level variables, suitable for use with `errors.Is`:

| Error | Meaning |
|---|---|
| `ErrDataTooShort` | CADU too short to contain the ASM |
| `ErrSyncMarkerMismatch` | CADU does not start with the expected ASM |
| `ErrInvalidDataLength` | Data length does not match RS code parameters |
| `ErrInvalidInterleaveDepth` | Unsupported interleaving depth (must be 1, 2, 3, 4, 5, or 8) |
| `ErrUncorrectable` | Errors exceed RS correction capability |

## Notes

Commentary, not sourced from the standard.

**Why `0x1ACFFC1D`?** It was picked for its autocorrelation. Slide the pattern against itself at any offset other than zero and it matches poorly, so a correlator finds the true frame boundary and not a near-miss, even when a good fraction of the bits arrive wrong.

**Why randomize at all?** A receiver recovers its clock from bit transitions. A long run of identical bits gives it nothing, and it drifts. Scrambling with a known sequence guarantees transitions without changing what is being sent.

**Why Reed-Solomon rather than a retransmission scheme?** The downlink is one-way and the round trip is minutes to hours. Correcting errors where they land is the only option that finishes in time.

**Why symbol-oriented coding?** RS works on bytes, not bits, so a byte with eight wrong bits costs the same as a byte with one. Space channel errors arrive in bursts, which is exactly the shape RS is good at.

## Reference

- [CCSDS 131.0-B-5](https://public.ccsds.org/Pubs/131x0b5.pdf), TM Synchronization and Channel Coding (Blue Book)
- [CCSDS 130.1-G-3](https://public.ccsds.org/Pubs/130x1g3.pdf), TM Synchronization and Channel Coding Summary (Green Book)
- [CLI](/cli/cadu) | [Conformance](/conformance/tmsc) | [The stack](/docs/start/concepts)
