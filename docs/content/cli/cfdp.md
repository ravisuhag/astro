---
title: astro cfdp
short: CFDP
description: CCSDS File Delivery Protocol, decode PDUs.
order: 190
---

Decode CFDP Protocol Data Units ([CCSDS 727.0-B-5](https://public.ccsds.org/Pubs/727x0b5.pdf)).

## Subcommands

| Command | Description |
|---------|-------------|
| `astro cfdp decode` | Decode a PDU: the fixed header, then its directive or file data |

---

## astro cfdp decode

Decode the fixed header, then the file directive or file data the PDU carries.

A file directive names itself in the first octet of its data field ([table 5-4](https://public.ccsds.org/Pubs/727x0b5.pdf)), so the body is decoded properly rather than shown as octets. EOF, Finished, ACK, Metadata, NAK and Prompt are all read down to their fields.

File data is **not** interpreted (it is your file, and the PDU says nothing about its contents) so it comes back as octets with a note saying so.

The CRC is verified when the header says one is present, and a mismatch is an error. Clause 4.1.2 requires the receiver to discard such a PDU, so decoding it into something plausible would be wrong.

```
astro cfdp decode [file] [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | `hex` | Input format: `hex` or `bin` |
| `--format` | `text` | Output format: `text` or `json` |

**Examples**

```bash
astro cfdp decode --input hex < pdu.hex
astro cfdp decode --input bin pdu.bin --format json
```

## Part 2: the User Operations

Section 6 puts proxy, directory, remote status, suspend and resume operations in a **Message to User TLV** in an ordinary transaction's metadata, not in PDUs of their own. So they surface when you decode a Metadata PDU, and `decode` names each one and reads its content.

A Message to User is a protocol message only if it opens with the four ASCII characters `cfdp`. Anything else is an application message and is left alone, which is what that identifier exists for.

```
astro cfdp decode --input hex < metadata.hex
```

```
CFDP File Directive PDU: Metadata
  ...
User operations (2 message(s)):
  Proxy Put Request
  Beneficiary ....... 3
  Source file ....... /remote/a.dat
  Destination file .. /local/a.dat
  Originating Transaction ID
  Originating transaction: entity 1, sequence 100
```

## Limits

The message **formats** of Part 2 are implemented. The user **behaviour** around them is not, and the standard makes that the CFDP user's job: which primitive to call on receipt, and how to queue concurrent suspension orders, which clause 6.5.4.1.2 says outright is "an implementation matter".

---

**See also**: [the protocol page](/protocols/transport/cfdp) for the standard and the Go API, and the [conformance statement](/conformance/cfdp) for what is and is not implemented.
