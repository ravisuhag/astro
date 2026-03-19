# TM Synchronization and Channel Coding

> CCSDS 131.0-B-4 — TM Synchronization and Channel Coding

## Overview

The TM Synchronization and Channel Coding sublayer is the bridge between the TM Data Link Protocol and the physical layer. It provides three critical functions: **frame synchronization** (how the receiver finds frame boundaries in a continuous bitstream), **pseudo-randomization** (ensuring good signal properties), and **forward error correction** (detecting and correcting bit errors without retransmission).

This sublayer is the reason spacecraft telemetry can be received reliably across millions of kilometers of space with signal power measured in femtowatts.

### Where TMSC Fits

```
+-----------------------------------------+
|  TM Space Data Link Protocol (TMDL)     |
|  Produces fixed-length Transfer Frames  |
+-----------------------------------------+
|  TM Sync & Channel Coding (TMSC)        |  <-- This sublayer
|  ASM, randomization, Reed-Solomon FEC   |
+-----------------------------------------+
|  Physical Layer (RF/Optical link)       |
+-----------------------------------------+
```

The sublayer receives Transfer Frames from the Data Link Protocol, applies error correction coding, pseudo-randomizes the data, prepends a sync marker, and hands the result to the physical layer for modulation and transmission.

### Key Characteristics

- **Transparent to upper layers**: Everything this sublayer does is reversed at the receiver. The Data Link Protocol neither knows nor cares about randomization or FEC.
- **Self-synchronizing**: The Attached Sync Marker allows the receiver to find frame boundaries without any out-of-band signaling.
- **Forward error correction**: Reed-Solomon coding corrects bit errors at the receiver without any retransmission — critical for one-way TM links.
- **Deterministic**: No protocol state, no handshaking, no negotiation. Given the same input, always produces the same output.

## Attached Sync Marker (ASM)

The Attached Sync Marker is a fixed 32-bit pattern prepended to each Transfer Frame:

```
0x1ACFFC1D
0001 1010 1100 1111 1111 1100 0001 1101
```

### Purpose

In a continuous bitstream arriving at the receiver, there are no gaps or delimiters between frames. The receiver must find where each frame starts. The ASM provides this synchronization:

1. The receiver correlates the incoming bitstream against the known ASM pattern.
2. When a match is found (above a confidence threshold), a frame boundary is declared.
3. Since frames are fixed-length, subsequent frame boundaries are predicted.
4. The ASM of the next frame confirms lock.

### Why This Particular Pattern?

The ASM was chosen for its **autocorrelation properties**. When you slide this pattern across itself, the correlation peak is sharp and the sidelobes are low. This means:
- A match is unambiguous — the pattern is unlikely to appear by chance in random data.
- Even with some bit errors in the ASM itself, the correlation peak is still detectable.
- The pattern works well with common modulation schemes.

### The ASM Is Never Randomized

When pseudo-randomization is applied, the ASM is excluded. The receiver needs to find the ASM in the raw bitstream before it can de-randomize anything else — randomizing the ASM would create a chicken-and-egg problem.

## Channel Access Data Unit (CADU)

The CADU is the complete unit handed to the physical layer:

```
+---------+---------------------------+
|   ASM   |     Transfer Frame        |
|  (4B)   | (possibly randomized,     |
|         |  possibly RS-encoded)     |
+---------+---------------------------+
|<-------------- CADU --------------->|
```

The CADU has a fixed length on any given physical channel: 4 bytes (ASM) + frame length.

## Pseudo-Randomization

### The Problem

Digital communication systems need frequent bit transitions (changes between 0 and 1) for clock recovery. If the data happens to contain long runs of identical bits (all zeros, for example), the receiver's clock can drift, causing bit slip errors. Some modulation schemes also have DC balance requirements.

### The Solution

The CCSDS standard defines a **pseudo-random noise (PN) sequence** that is XORed with the Transfer Frame data. This spreads the spectral energy evenly regardless of the actual data content, ensuring adequate bit transitions.

### The PN Generator

The PN sequence is produced by an 8-bit Linear Feedback Shift Register (LFSR) with:

- **Polynomial**: h(x) = x^8 + x^7 + x^5 + x^3 + 1
- **Initial state**: All 1s (0xFF)
- **Output**: One bit per clock cycle from the MSB

```
     +--[XOR]<--[XOR]<------[XOR]<-----------+
     |   ^        ^           ^               |
     v   |        |           |               |
   [b7]-[b6]-[b5]-[b4]-[b3]-[b2]-[b1]-[b0]   |
     |                                         |
     +---> output bit                          |
     +----> feedback -------------------------+
```

The sequence has a period of 255 bits. It produces the same output every time (it is deterministic, not random), so both transmitter and receiver generate identical sequences.

### Self-Inverse Property

XOR has a critical property: applying it twice with the same value restores the original. This means the same `Randomize()` function is used for both randomization (on transmit) and de-randomization (on receive).

## Reed-Solomon Error Correction

### The Problem

The space communication channel introduces bit errors — cosmic rays, thermal noise, signal attenuation over vast distances. Unlike terrestrial networks, there is no retransmission for TM. Errors must be corrected at the receiver using only the received data.

### The Solution

Reed-Solomon (RS) codes add redundant **parity symbols** to each data block. The receiver uses these parity symbols to detect and correct errors. The CCSDS standard defines two RS codes:

| Code | Data Symbols | Parity Symbols | Total | Max Correctable Errors |
|------|-------------|---------------|-------|----------------------|
| RS(255,223) | 223 | 32 | 255 | 16 symbol errors |
| RS(255,239) | 239 | 16 | 255 | 8 symbol errors |

Each "symbol" is one byte (8 bits). RS(255,223) can correct up to 16 corrupted bytes in a 255-byte codeword — that is 6.3% of the data, or equivalently, any burst of up to 128 corrupted bits within a codeword.

### Galois Field Arithmetic

RS codes operate over the Galois Field GF(2^8) — a finite field with 256 elements where addition is XOR and multiplication is defined modulo an irreducible polynomial.

The CCSDS field uses:
- **Primitive polynomial**: x^8 + x^7 + x^2 + x + 1 (0x187)
- **First consecutive root (FCR)**: 112 (α^112)

All RS operations (encoding, syndrome computation, error location, error correction) use GF(2^8) arithmetic.

### Encoding

Systematic encoding appends parity symbols to the data:

```
Input:  [223 data bytes]
Output: [223 data bytes][32 parity bytes] = 255-byte codeword
```

The data is preserved unchanged — the parity bytes are simply appended. This means even without RS decoding, the first 223 bytes of the codeword are the original data (though possibly corrupted).

### Decoding

The decoder performs four steps:

1. **Syndrome computation**: Evaluate the received polynomial at the roots of the generator polynomial. If all syndromes are zero, no errors were introduced.

2. **Berlekamp-Massey algorithm**: Compute the error-locator polynomial σ(x) from the syndromes. The degree of σ gives the number of errors.

3. **Chien search**: Find the roots of σ(x) by exhaustive evaluation over all 255 field elements. Each root corresponds to an error position in the codeword.

4. **Forney algorithm**: Compute the error magnitude at each position using the error-evaluator polynomial Ω(x) and the formal derivative of σ(x).

### Symbol Interleaving

Burst errors (caused by signal fading, interference, or other transient phenomena) can corrupt many consecutive bytes. If all corrupted bytes fall within a single RS codeword, they may exceed the correction capability.

**Interleaving** distributes the symbols from multiple codewords across the transmitted data, so a burst error is spread across multiple codewords:

```
Without interleaving (depth=1):
[-----CW1-----][-----CW2-----][-----CW3-----]
     ^^^^^^ burst hits all 6 bytes in CW1

With interleaving (depth=3):
[c1 c2 c3 c1 c2 c3 c1 c2 c3 c1 c2 c3 ...]
     ^^^^^^ burst distributes: 2 bytes to each CW
```

CCSDS supports interleave depths of 1, 2, 3, 4, 5, and 8. Depth 5 with RS(255,223) is common for deep-space missions, providing 5 × 255 = 1275 bytes per interleaved block.

## Processing Order

### Transmit Path

```
Transfer Frame
      |
      v
[RS Encode] ──> Adds parity symbols (optional)
      |
      v
[Pseudo-Randomize] ──> XOR with PN sequence (optional)
      |
      v
[Prepend ASM] ──> Attach sync marker
      |
      v
    CADU ──> To physical layer
```

### Receive Path

```
    CADU ──> From physical layer
      |
      v
[Find ASM] ──> Locate frame boundary
      |
      v
[Strip ASM] ──> Remove sync marker
      |
      v
[De-Randomize] ──> XOR with PN sequence (optional)
      |
      v
[RS Decode] ──> Correct errors (optional)
      |
      v
Transfer Frame ──> To TM Data Link Protocol
```

## Design Rationale

**Why randomize if RS can correct errors?** Randomization and RS solve different problems. Randomization prevents clock slip (a receiver synchronization issue). RS corrects bit errors (a channel noise issue). Without randomization, even a perfectly noise-free channel can lose data if the receiver's clock drifts on a long run of zeros.

**Why a fixed ASM instead of a length-delimited protocol?** Deep-space receivers operate at extremely low signal-to-noise ratios. A correlation-based sync marker detection is far more robust than parsing length fields from noisy data. The fixed ASM works even when individual bits are unreliable.

**Why RS(255,223) specifically?** The 255-symbol codeword length is the maximum for GF(2^8). The 223/255 rate (87.5% efficiency) provides a good balance between error correction capability and bandwidth overhead. The 16-symbol correction capability handles the typical error rates seen on well-designed deep-space links.

**Why interleaving depths of 1-5 and 8?** These depths were chosen to align with common frame lengths. Depth 5 with RS(255,223) produces a 1275-byte block, which maps well to typical TM frame sizes. Depth 8 provides maximum burst error protection for particularly challenging link conditions.

## Reference

- [CCSDS 131.0-B-5](https://public.ccsds.org/Pubs/131x0b5.pdf) — TM Synchronization and Channel Coding (Blue Book)
- [CCSDS 130.1-G-3](https://public.ccsds.org/Pubs/130x1g3.pdf) — TM Synchronization and Channel Coding Summary (Green Book)
- [CCSDS 131.0-O-3](https://public.ccsds.org/Pubs/131x0o3.pdf) — TM Synchronization and Channel Coding (Orange Book - Experimental)
