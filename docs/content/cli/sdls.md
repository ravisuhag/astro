---
title: astro sdls
short: SDLS
description: Space Data Link Security, protect, verify, and read frame data fields.
order: 120
---

Protect a frame data field, check and recover one, or read its Security Header ([CCSDS 355.0-B-2](https://public.ccsds.org/Pubs/355x0b2.pdf)).

## Keys come from a file

`apply` and `process` need the Security Association's key. They read it from the file `--key-file` names, and there is no flag that takes the key itself. A key on the command line lands in shell history and in the process table, where anyone on the machine can read it.

The file holds the 32-octet AES-256 key, either as raw octets or as 64 hex digits. If other users can read the file, the command warns you. `chmod 600` it.

`inspect` needs no key at all. It is usually what you want when a protected frame is not behaving.

## Counters are yours to keep

A long-running sender keeps its IV and sequence number counters in memory. A command runs once and forgets. So `apply` asks you for the last value sent under the key, and prints the new one to stderr. Save that value for the next run.

This matters most for AES-GCM. Sending the same IV twice under one key breaks the key. That is why `--last-iv` has no default. For a key that has never sent anything, pass all zeros.

`process` does not check for replays. That needs the last value accepted, and a single run has none. A receiver that must reject replays should keep an SA alive in the library.

## Subcommands

| Command | Description |
|---|---|
| `astro sdls apply` | Protect a frame data field |
| `astro sdls process` | Check and recover a protected data field |
| `astro sdls inspect` | Decode a Security Header |

---

## astro sdls apply

Apply security to one Transfer Frame Data Field. The output is the Security Header, the protected data, and the Security Trailer, ready to place in the carrier frame.

The defaults are the clause E1 baseline: AES-GCM authenticated encryption, a 12-octet IV and a 16-octet MAC.

```
astro sdls apply [file] [flags]
```

**Flags**

| Flag | Default | Description |
|---|---|---|
| `--key-file` | | File holding the key, raw or hex (required) |
| `--spi` | | Security Parameter Index (required; 0 and 65535 are reserved) |
| `--last-iv` | | Last IV sent under this key, in hex (required when `--iv` is above 0) |
| `--last-seq` | | Last sequence number sent under this key, in hex (required when `--seq` is above 0) |
| `--mode` | `aead` | `aead` (AES-GCM authenticated encryption) or `auth` (authentication only) |
| `--auth-alg` | `gmac` | With `--mode auth`: `gmac` or `cmac` |
| `--iv` | `12` | Initialisation vector length in octets (`0` for `cmac`) |
| `--seq` | `0` | Anti-replay sequence number length in octets |
| `--pad` | `0` | Pad length field width in octets |
| `--mac` | `16` | Message authentication code length in octets |
| `--frame-header` | | Frame header octets in hex, authenticated but not encrypted |
| `--auth-mask` | | Authentication bit mask in hex (default: authenticate every header octet) |
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `hex` | Output format: `hex`, `bin`, or `text` |

**The frame header and the mask.** SDLS authenticates the carrier frame's header too. Pass it with `--frame-header` so the MAC covers it. Some header fields change after security is applied, like the TM Master Channel Frame Count. Those must be left out of the MAC with `--auth-mask`. The library's `BaselineAuthMaskTM` and its siblings build the right mask for each frame type.

**Examples**

```bash
# The clause E1 baseline, the first frame under a new key
astro sdls apply --key-file sa7.key --spi 7 \
  --last-iv 000000000000000000000000 < data.hex

# The clause E2 telecommand baseline: AES-CMAC with a 4-octet sequence number
astro sdls apply --key-file tc.key --spi 9 --mode auth --auth-alg cmac \
  --iv 0 --seq 4 --last-seq 00000041 < command.hex
```

---

## astro sdls process

Check and recover one protected data field. It checks the SPI, verifies the MAC, decrypts, and writes the recovered Transfer Frame Data Field. If anything fails, it writes nothing but the error.

It takes the same Security Association flags as `apply`, without `--last-iv` and `--last-seq`. The `--frame-header` and `--auth-mask` must match what the sender used.

```
astro sdls process [file] [flags]
```

**Example**

```bash
# Recover what apply protected
astro sdls process --key-file sa7.key --spi 7 < protected.hex
```

---

## astro sdls inspect

Decode the Security Header at the front of a protected frame's data field: the Security Parameter Index, and whichever of the initialisation vector, sequence number and pad length the Security Association carries.

The field widths are **per Security Association**, not per frame, and nothing in the header states them, so they are flags. Getting them wrong shifts everything after the SPI, which is why the SPI is reported separately: it is the one field whose position is fixed.

The protected data is reported as a length, not decrypted, and a MAC is shown but not verified. Both need keys.

```
astro sdls inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|---|---|---|
| `--iv` | `0` | Initialisation vector length in octets |
| `--seq` | `0` | Anti-replay sequence number length in octets |
| `--pad` | `0` | Pad length field width in octets |
| `--mac` | `0` | Message authentication code length in octets |
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
# A confidentiality SA: a 12-octet IV and a 16-octet MAC
astro sdls inspect --input hex --iv 12 --mac 16 < frame-data.hex

# An authentication-only SA: a sequence number, no IV
astro sdls inspect --input hex --seq 4 --mac 16 < frame-data.hex
```

---

**See also**: [the protocol page](/protocols/data-link/sdls) for the standard and the Go API, and the [conformance statement](/conformance/sdls) for what is and is not implemented.
