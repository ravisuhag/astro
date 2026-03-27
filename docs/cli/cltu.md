# astro cltu

Command Link Transmission Unit operations — wrap, unwrap, and inspect CLTUs ([CCSDS 231.0-B-4](https://public.ccsds.org/Pubs/231x0b4e1.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro cltu wrap` | Wrap a TC frame into a CLTU (BCH encode, add start/tail sequences) |
| `astro cltu unwrap` | Validate sequences, BCH decode, extract TC frame |
| `astro cltu inspect` | Annotated CLTU breakdown with codeblock details |

---

## astro cltu wrap

Pad, BCH(63,56)-encode, and add start/tail sequences to produce a CLTU from TC Transfer Frame data.

```
astro cltu wrap [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |
| `--randomize` | `false` | Apply CCSDS pseudo-randomization |

**Examples**

```bash
# Wrap a TC frame
astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro cltu wrap --input hex

# Wrap with randomization
astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro cltu wrap --input hex --randomize
```

---

## astro cltu unwrap

Validate start/tail sequences, BCH-decode each codeblock (correcting up to 1 bit error per block), and optionally de-randomize to extract TC Transfer Frame data.

```
astro cltu unwrap [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `hex` | Output format: `text`, `json`, or `hex` |
| `--derandomize` | `false` | Apply CCSDS de-randomization |

**Examples**

```bash
# Unwrap a CLTU
astro cltu unwrap --input hex cltu.hex

# Unwrap with de-randomization
cat cltu.hex | astro cltu unwrap --input hex --derandomize
```

---

## astro cltu inspect

Display an annotated breakdown of a CLTU showing start/tail sequence validation, individual codeblock info and parity bytes, and a full hex dump.

```
astro cltu inspect [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |

**Examples**

```bash
astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro cltu wrap --input hex | astro cltu inspect --input hex
```

**Sample Output**

```
CLTU Inspector
────────────────────────────────────────────────────────────
Start Sequence (2 bytes): eb90 [VALID]
Tail Sequence (8 bytes): c5c5c5c5c5c5c579 [VALID]
────────────────────────────────────────────────────────────
Codeblocks: 2 (8 bytes each = 7 info + 1 parity)
  Block 1: info=001a040b000102 parity=46
  Block 2: info=0304059b155555 parity=02
────────────────────────────────────────────────────────────
Raw CLTU (26 bytes)
  0000  eb 90 00 1a 04 0b 00 01  02 46 03 04 05 9b 15 55  |.........F.....U|
  0010  55 02 c5 c5 c5 c5 c5 c5  c5 79                    |U........y|
```

---

## Piping

```bash
# Full TC chain: encode frame → wrap CLTU → inspect
astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro cltu wrap --input hex | astro cltu inspect --input hex

# Wrap → Unwrap round-trip
astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro cltu wrap --input hex | astro cltu unwrap --input hex

# Wrap with randomize → Unwrap with derandomize
astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro cltu wrap --input hex --randomize | astro cltu unwrap --input hex --derandomize
```
