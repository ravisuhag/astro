# Consumer contract

What an implementation must do to consume `vectors/`, in enough detail
that it needs nothing else. If a question about these fixtures can only be
answered by reading some implementation's source, this document is
incomplete and that is a bug in it.

Covers `aos`, `bp`, `cfdp`, `cmac`, `cop`, `crc`, `epp`, `ldc`, `ltp`,
`ocsc`, `pn`, `pus`, `pxsc`, `sdls`, `sdnv`, `sle`, `spp`, `tcf`, `tcsc`,
`tmdl`, `tmsc`, `usdl`, `xtce`. What is not covered is in
[`COVERAGE.md`](COVERAGE.md).

## The authority rule

When an implementation and a vector disagree, the vector and the clause
it cites settle it — not any implementation, and not this document.

Where a vector carries a `clause`, the clause is the authority and the
vector is evidence of it. Where a vector carries
`"source": "unverified"`, there is no authority behind the value:
agreeing with it proves an implementation matches this corpus, not the
standard. Those are listed at the end and in
[`COVERAGE.md`](COVERAGE.md).

## Encoding conventions

**Octet strings** are lowercase hexadecimal, no separators, no `0x`
prefix, an even number of digits. Empty is `""`.

**Bit order** is most-significant-bit first within an octet, and octets
are transmitted left to right as written. This is what every CCSDS wire
diagram assumes and no vector departs from it. A field spanning a byte
boundary continues into the next octet's high bits.

**Scalar results** are octet strings too, at the value's natural width,
big-endian. A CRC-16 of 0x29b1 is `"29b1"`; a CRC-32 is eight hex
digits. There is no separate numeric-result form.

**Integers** are JSON numbers up to 2^53. Anything wider is a decimal
string, because a JSON number cannot hold it without loss — `pkg/sdnv`
has a 2^64-1 case. A consumer should accept both forms for any integer
field rather than switching on width.

**Field names** are `snake_case` and name the field the standard names,
not any implementation's identifier.

**A hex string of only decimal digits is ambiguous, and the field
dictionary is what resolves it.** `"112233"` is valid hex *and* a valid
wide-integer decimal string. Nothing in the vector distinguishes them, so
a consumer must decide from the field's declared type, not by inspecting
the value. A comparison that guesses numerically first will report an
octet field as unequal to itself. Consult the declared type first.

## `config` versus `fields`

`fields` are values that travel on the wire, or the inputs that produce
them.

`config` is channel-level agreement: frame length, whether an error
control field is present, a mission profile. It configures the channel,
not the frame, and both ends must already agree on it before a single
octet is exchanged. Keeping it separate means a consumer never has to
guess which values are transmitted and which are prior agreement.

A vector with no `config` needs no agreement beyond the standard.

## Match semantics

For `decode`, comparison covers **exactly the fields the vector lists**.
Fields your decoder exposes that the vector does not mention are
unconstrained — the vector states what it means to pin.

This is safe only because the field dictionaries below name everything a
decoder must expose per package. "Unlisted" is therefore always a vector
author's choice, never an unknown.

## The four kinds

| kind | asserts | required keys |
|---|---|---|
| `encode` | fields produce these octets | `name`, `note`, `fields`, `want` |
| `decode` | these octets produce these fields | `name`, `note`, `input`, `fields` |
| `reject` | this must fail, with this error | `name`, `error`, and exactly one of `input` or `fields` |
| `sequence` | a scripted run against a state machine | `name`, `note`, `steps` |

A `reject` with `input` is bad octets refused at decode. A `reject` with
`fields` is bad values refused at construction — an APID of 2048 does not
fit an 11-bit field in any language, so that rule is the standard's, not
Go's, and it belongs in the corpus.

`sequence` is defined by the schema. No file carries one yet. Time inside a sequence is a step (`{"call": "tick"}`),
never a real clock. An implementation that reads a real clock inside a
protocol layer cannot be tested by a sequence vector at all, so keeping
time injectable is a precondition for this form.

## Error vocabulary

A `reject` names one of these. The names say what a conforming
implementation must refuse, not how it phrases the refusal — map them to
your own error type.

| name | meaning |
|---|---|
| `truncated` | the octets end before a field the layout requires |
| `length_mismatch` | a length field disagrees with the octets present, or with the length the channel agreed |
| `crc_mismatch` | a check sequence over the data does not match it |
| `header_check_failed` | a code protecting a header does not verify |
| `field_out_of_range` | a value does not fit the bits the standard gives its field |
| `reserved_value` | a field holds a value the standard reserves |
| `unsupported_version` | a version or protocol identifier this implementation does not carry |
| `trailing_data` | octets remain after the structure the length field described |
| `buffer_too_small` | a caller-supplied output buffer is too short |

The list is fixed. Extending it requires every consumer to agree on what
the new name means, so it is a deliberate change rather than an
incidental one.

### Two names for check failures, and why

`crc_mismatch` covers check sequences over data: a CRC-based FECF, a
packet error control field, a CFDP checksum.

`header_check_failed` covers codes that protect a header: the
Reed-Solomon frame header error control of AOS and USLP, the BCH of a
CLTU codeblock. These are not CRCs, and a corpus that called them one
would lie about what failed. The split is by what the field protects,
which keeps both names honest.

## Capabilities

A vector may declare `requires`, naming behaviour not every
implementation has:

| capability | meaning |
|---|---|
| `encode_into` | encoding into a caller-supplied buffer, returning the octets written |

A consumer skips vectors whose `requires` it cannot satisfy, and should
report the skip count rather than passing silently. An implementation
whose API always allocates has no capabilities and skips every
`encode_into` vector. No such vector is committed yet; the shape is
reserved so they can be added without a schema change.

## Field dictionaries

Every field a vector may use, per package, with its type. `hex` means an
octet string, `uint` a number or decimal string, `bool` a JSON boolean.

### `aos` — CCSDS 732.0-B-4

`vectors/aos/frame.json`, `vectors/aos/mpdu.json`

| field | type | standard's field |
|---|---|---|
| `tfvn` | uint | Transfer Frame Version Number, 2 bits, always 1 |
| `scid` | uint | Spacecraft Identifier, 8 bits |
| `vcid` | uint | Virtual Channel Identifier, 6 bits |
| `vc_frame_count` | uint | VC Frame Count, 24 bits |
| `replay_flag` | bool | Replay Flag, 1 bit |
| `vcfc_usage_flag` | bool | VC Frame Count Usage Flag, 1 bit |
| `vc_frame_count_cycle` | uint | VC Frame Count Cycle, 4 bits |
| `fhec` | hex | Frame Header Error Control, 2 octets |
| `data` | hex | Transfer Frame Data Field |
| `fhp` | uint | M_PDU First Header Pointer, 11 bits |

Config: `has_fecf` (bool), `has_fhec` (bool), `has_ocf` (bool),
`insert_zone_len` (uint), `ocf` (hex).

A decoder must expose every primary header field above plus `data`, and
`fhec` when the channel carries it.

### `spp` — CCSDS 133.0-B-2

`vectors/spp/header.json`, `vectors/spp/packet.json`

| field | type | standard's field |
|---|---|---|
| `version` | uint | Packet Version Number, 3 bits, always 0 |
| `packet_type` | uint | Packet Type, 1 bit: 0 = TM, 1 = TC |
| `secondary_header_flag` | uint | Secondary Header Flag, 1 bit |
| `apid` | uint | Application Process Identifier, 11 bits |
| `sequence_flags` | uint | Sequence Flags, 2 bits: 0 cont, 1 first, 2 last, 3 unsegmented |
| `sequence_count` | uint | Packet Sequence Count, 14 bits |
| `packet_length` | uint | Packet Data Length, 16 bits, **data field octets minus one** |
| `data` | hex | packet data field contents |

Config: `error_control` (bool). The error control field is **not** defined
by CCSDS 133.0-B-2 — it is a mission or PUS-style extension carried inside
the data field, wire-compatible because the standard leaves the data field
content to the mission. When present it is the last two octets of the data
field and counts toward `packet_length`, and the CRC-16-CCITT covers the
primary header and everything in the data field before the CRC itself.

### `epp` — CCSDS 133.1-B-3

`vectors/epp/header.json`

| field | type | standard's field |
|---|---|---|
| `pvn` | uint | Packet Version Number, 3 bits, always `'111'` (7) |
| `protocol_id` | uint | Encapsulation Protocol ID, 3 bits |
| `length_of_length` | uint | Length of Length, 2 bits |
| `user_defined` | uint | User Defined Field, 4 bits, 4- and 8-octet headers only |
| `extended_protocol_id` | uint | Protocol ID Extension, 4 bits, 4- and 8-octet headers only |
| `ccsds_defined` | uint | CCSDS Defined Field, 16 bits, 8-octet header only |
| `packet_length` | uint | total octets in the whole encapsulation packet |

The header size is a pure function of Length of Length (table 4-1): `'00'`
gives 1 octet, `'01'` gives 2, `'10'` gives 4, `'11'` gives 8. A 1-octet
header omits the length field entirely and is therefore legal only for
idle packets.

Protocol ID `'110'` (6) means the real protocol is named by the Protocol ID
Extension field. That is **distinct** from `'111'` (7), mission-specific
data. Both have the high bit set and are easy to confuse.

### `tmdl` — CCSDS 132.0-B-3

`vectors/tmdl/header.json`

| field | type | standard's field |
|---|---|---|
| `version_number` | uint | Transfer Frame Version Number, 2 bits, always 0 |
| `spacecraft_id` | uint | Spacecraft Identifier, 10 bits |
| `virtual_channel_id` | uint | Virtual Channel Identifier, 3 bits |
| `ocf_flag` | bool | Operational Control Field Flag, bit 15 |
| `mc_frame_count` | uint | Master Channel Frame Count, octet 2 |
| `vc_frame_count` | uint | Virtual Channel Frame Count, octet 3 |
| `fsh_flag` | bool | Frame Secondary Header Flag, bit 32 |
| `sync_flag` | bool | Synchronization Flag, bit 33 |
| `packet_order_flag` | bool | Packet Order Flag, bit 34 |
| `segment_length_id` | uint | Segment Length Identifier, 2 bits |
| `first_header_pointer` | uint | First Header Pointer, 11 bits |

**Two semantic rules constrain octet 4**, and an implementation must
enforce both. With the sync flag clear, the packet order flag must be zero
(clause 4.1.2.7.4) and the segment length identifier must be `'11'`
(clause 4.1.2.7.5). Both are pinned as reject vectors.

First header pointer `0x7ff` means no packet starts in the frame; `0x7fe`
means only idle data.

### `usdl` — CCSDS 732.1-B-3

`vectors/usdl/frame.json`

| field | type | standard's field |
|---|---|---|
| `scid` | uint | Spacecraft Identifier, 16 bits |
| `source_or_dest` | uint | 1 bit: 0 = SCID is source, 1 = destination |
| `vcid` | uint | Virtual Channel Identifier, 6 bits |
| `mapid` | uint | Multiplexer Access Point Identifier, 4 bits |
| `truncated` / `end_of_fph` | bool | End of Frame Primary Header flag |
| `frame_length` | uint | total frame octets minus one, non-truncated only |
| `ocf_flag` | bool | Operational Control Field present |
| `vcf_count_len` | uint | VCF Count field length in octets, 3 bits |
| `vcf_count` | uint | Virtual Channel Frame Count, 0-56 bits |
| `construction_rule` | uint | TFDZ Construction Rule, 3 bits |
| `upid` | uint | USLP Protocol Identifier, 5 bits |
| `pointer` | uint | FHP (rule `'000'`) or LVOP (rules `'001'`/`'010'`), 16 bits |
| `has_pointer` | bool | whether the construction rule carries a pointer |
| `ocf` | hex | Operational Control Field, 4 octets |
| `data` | hex | Transfer Frame Data Zone |
| `sequence` / `length` | string / uint | OID fill generator selector |

Config: `fec_size` (uint) — 0 or 16.

**The primary header is variable length.** Read the End of Frame Primary
Header flag first: set means a 4-octet truncated header with no frame
length, flags, OCF, insert zone or FECF. Clear means 7 octets plus a VCF
count field whose width the header itself declares. A decoder that reads
the frame length before the EOFPH flag will misparse every truncated frame.

Construction rules `'000'`, `'001'` and `'010'` carry the 16-bit pointer
(clause 4.1.4.2.4.1); others do not, and the data zone starts immediately
after the TFDF header octet.

### `crc`

`vectors/crc/crc16.json` — CRC-16-CCITT, generator 0x1021, preset all
ones, MSB first, no reflection, no final inversion.

`vectors/crc/crc32.json` — Proximity-1 CRC-32, generator 0x00a00805,
**zero** preset, MSB first, no reflection, no final inversion.

| field | type | meaning |
|---|---|---|
| `data` | hex | the octets covered |

`want` is the check value, big-endian, 2 octets for CRC-16 and 4 for
CRC-32.

### `sdnv` — RFC 5050 clause 4.1

`vectors/sdnv/sdnv.json`

| field | type | meaning |
|---|---|---|
| `value` | uint | the number encoded; a decimal string past 2^53 |
| `consumed` | uint | octets a decoder used, on `decode` only |

`consumed` matters because SDNVs are self-delimiting: a decoder reads one
value from a longer buffer and reports how much it took. The
`stops-at-the-first-octet-without-continuation` vector pins that.

Maximum encoding is 10 octets for a 64-bit value. Longer than that is
`length_mismatch`, not `trailing_data`.

### `pn` — CCSDS 131.0-B-5, 231.0-B-4, 132.0-B-3

`vectors/pn/sequences.json`

| field | type | meaning |
|---|---|---|
| `sequence` | string | `tm`, `tc`, or `oid` |
| `length` | uint | octets of sequence to generate |

Three distinct generators, all preset to all ones, and they are not
interchangeable:

- `tm` — h(x) = x^8 + x^7 + x^5 + x^3 + 1
- `tc` — h(x) = x^8 + x^6 + x^4 + x^3 + x^2 + x + 1
- `oid` — the 32-cell generator for Only Idle Data frames

Both `tm` and `tc` open with `ff` because both registers preset to all
ones; they diverge at the second octet. Astro once shipped TC using the
TM taps, and every round-trip test passed, because the randomizer is XOR
and therefore its own inverse. **A round trip cannot validate this
package.** Only these published octets can.

### `cmac` — RFC 4493, NIST SP 800-38B

`vectors/cmac/aes.json`

| field | type | meaning |
|---|---|---|
| `key` | hex | AES key, 16 or 32 octets |
| `message` | hex | message to authenticate, may be empty |

`want` is the full 16-octet tag. CCSDS 355.0-B-2 clause E2a requires a
256-bit key, so the AES-256 cases are the operative ones and the AES-128
cases are the more widely published cross-check.

Test keys only. Never real keys.

### `tcsc` — CCSDS 231.0-B-4

`vectors/tcsc/bch.json`

| field | type | meaning |
|---|---|---|
| `info` | hex | the 7-octet BCH information field |
| `sequence` | string | `start` or `tail`, for the CLTU framing constants |

BCH(63,56) shortened: 56 information bits, 7 parity bits, 1 filler bit,
8 octets total. **Parity is the complement of the LFSR remainder**, which
is why an all-zero information field gives parity octet `0xfe` and not
`0x00`. Generator g(x) = x^7 + x^6 + x^2 + 1. That complement is the
detail an implementation is most likely to drop, so it is the first
vector in the file.

### `tmsc` — CCSDS 131.0-B-5

`vectors/tmsc/cadu.json`

| field | type | meaning |
|---|---|---|
| `marker` | bool | select the sync marker on its own |
| `frame` | hex | transfer frame octets |

Config: `randomize` (bool).

A CADU is the 4-octet attached sync marker `1acffc1d` followed by the
optionally pseudo-randomized frame. **The marker is never randomized** —
a receiver must find it before it can derandomize anything. The
randomizer XORs against the TM sequence in `vectors/pn/sequences.json`,
and because XOR is its own inverse a round trip proves nothing here.

### `pxsc` — CCSDS 211.2-B-3, code per CCSDS 131.0

`vectors/pxsc/convolutional.json`

| field | type | meaning |
|---|---|---|
| `data` | hex | input octets |

Rate-1/2, constraint length 7. These vectors pin the *convention*
deployed receivers use: newest bit into the register's least significant
bit, taps `0x4f` and `0x6d`, **G2 output inverted**. A reciprocal
mirror-image encoder decodes its own output and nobody else's — it emits
`86b9` for input `0x80` instead of `ba49`. Both values are given so the
distinction is checkable.

### `sdls` — CCSDS 355.0-B-2

`vectors/sdls/protected-frame.json`

| field | type | meaning |
|---|---|---|
| `key` | hex | AES key, 16 or 32 octets — test keys only |
| `frame_header` | hex | the carrier frame's primary header |
| `plaintext` | hex | the data field to protect |
| `spi` | uint | Security Parameter Index |
| `iv` | hex | initialization vector, GCM/GMAC |
| `seq` | hex | sequence number, CMAC |

Config: `mode` (`authentication`, `encryption`,
`authenticated_encryption`), `auth_algorithm` (`gmac`, `cmac`),
`iv_len`, `seq_len`, `mac_len`, and on decode also `key` and
`frame_header`.

**The associated data ordering is the thing to get right.** Clause
4.2.3.2.2.3 builds it as the frame header, then the SPI, then the IV
field **as zeros** — clause 4.2.2.6.2 h) excludes the IV itself from the
authenticated data. The `gcm-frame-header-altered` reject vector proves
the header really enters the AAD.

Each value is reproducible two independent ways: through an SDLS
implementation, and from first principles with any standard AES-GCM or
CMAC by building the authenticated data in the clause 4.2.3 order.
Agreement between the two is what makes these vectors evidence.

### `tcf` — CCSDS 301.0-B-4

`vectors/tcf/timecode.json`

| field | type | meaning |
|---|---|---|
| `extension`, `time_code_id`, `detail`, `ext_detail` | bool/uint | P-field |
| `code` | string | `cuc` or `cds` |
| `coarse_time`, `coarse_bytes`, `fine_time`, `fine_bytes` | uint | CUC T-field |
| `day`, `day_bytes`, `milliseconds`, `submilliseconds`, `subms_bytes` | uint | CDS T-field |

**Scope limit, deliberate.** These vectors pin the P-field and T-field
*packing* from explicit field values. They do **not** go through the
`time.Time` conversion path, because Level 1 CUC adds the TAI-UTC offset
in effect at the given instant, and a vector derived that way would pin
a particular leap-second table rather than the standard's layout. Treat
leap-second handling as outside the corpus.

The extension flag and the octet count cannot disagree: a set flag means
a second P-field octet follows. Bit 0 of that second octet is reserved
for a third octet the standard never defines, so `0x7f` is the largest
legal extension detail and `0xff` must be refused.

### `ocsc` — CCSDS 142.0-B-1

`vectors/ocsc/asm.json` — the sync marker only.

Most of this package works on **bit strings**, not octet strings:
bit-level lengths, termination digits, sequence indicators. The vector
format cannot express those and they stay as Go tests. The marker is the
same `1acffc1d` as TM, and a Go test cross-checks the two vector files
against each other so a drift in either is caught here rather than by a
receiver.

### `cop` — CCSDS 232.0-B-4

`vectors/cop/clcw.json`

| field | type | standard's field |
|---|---|---|
| `control_word_type`, `version` | uint | always 0 |
| `status_field` | uint | mission-specific status, 3 bits |
| `cop_in_effect` | uint | 2 bits, `01` = COP-1 |
| `virtual_channel_id` | uint | 6 bits |
| `no_rf_available_flag`, `no_bit_lock_flag`, `lockout_flag`, `wait_flag`, `retransmit_flag` | bool | octet 2, bits 7 down to 3 |
| `farm_b_counter` | uint | 2 bits |
| `report_value` | uint | V(R), octet 3 |

The five status flags are **independent bits**, not an enumeration. Each
has its own vector, and `lockout-and-retransmit-together` pins that they
combine.

Note `all-zero-octets-decode-as-a-valid-clcw`: four zero octets are a
structurally valid CLCW reporting V(R) = 0. That is why an implementation
must refuse to invent an operational control field when a channel
declares one but no source is supplied — a receiver cannot tell invented
zeros from a real report.

**Two range rules are not yet asserted.** An out-of-range VCID or FARM-B
counter must be refused rather than truncated, because a truncated value
produces a control word reporting on a different virtual channel than
intended. [`COVERAGE.md`](COVERAGE.md) carries the vectors that would pin
this.

### `cfdp` — CCSDS 727.0-B-5

`vectors/cfdp/wire.json`

| field | type | meaning |
|---|---|---|
| `file` | hex | file content, for a checksum vector |
| `value` | hex | LV or TLV value |
| `type` | uint | TLV type octet |
| `consumed` | uint | octets a decoder used |

Config: `kind` (`lv` or `tlv`).

The annex F modular checksum sums big-endian 32-bit words with the final
partial word **zero-padded on the right**. Padding left would give a
different sum. This is one of very few genuinely published CCSDS test
vectors.

**LV and TLV are not self-describing.** Both open with an octet a decoder
could read as either a length or a type, so nothing in the octets says
which encoding is present. Which decoder to use is prior agreement and
lives in `config`. A consumer that guesses from the octets will read a
TLV as an LV.

### `ltp` — CCSDS 734.1-B-1 / RFC 5326

`vectors/ltp/header.json`

| field | type | meaning |
|---|---|---|
| `segment_type` | uint | 4 bits, low nibble of the control octet |
| `engine_id`, `session_number` | uint | session ID, each SDNV-encoded |
| `consumed` | uint | header octets used |

**The header is variable length** because the session ID is SDNV-encoded,
so the extension-count octet is not at a fixed offset. The
`engine-id-crossing-the-sdnv-boundary` vector is the one that catches a
decoder assuming otherwise: engine ID 128 is the first value needing two
SDNV octets, which shifts everything after it.

### `pus` — ECSS-E-ST-70-41C

`vectors/pus/tc-header.json`

| field | type | standard's field |
|---|---|---|
| `ack_flags` | uint | acknowledgement flags, 4 bits |
| `service`, `subtype` | uint | message type ID, one octet each |
| `source_id` | uint | 16 bits, big-endian |

Config: `tc_spare_bytes` (uint).

**The wire format is not self-describing.** The spare width is a
per-application-process declaration, so a decoder must already hold the
mission profile. Read `tc-with-spare-inverse` under a zero-spare profile
and one octet is left unconsumed. PUS version is 2 for PUS-C, and the
field layout changed between versions, so a version mismatch must be
refused rather than misparsed.

### `sle` — ITU-T X.690 and CCSDS 913.1-B-2

`vectors/sle/ber.json`

| field | type | meaning |
|---|---|---|
| `kind` | string | `integer`, `octet_string`, `null`, `sequence`, `length`, `tml` |
| `value` | uint / hex | per kind; INTEGER values may be negative |
| `content` | hex | SEQUENCE content, already encoded |
| `message_type`, `body` | uint / hex | TML message |

These pin the BER primitives and the ISP1 framing that everything else in
SLE sits on. A wrong INTEGER length or length-form boundary corrupts every
operation above it.

The pairs matter: 127 fits one content octet but **128 needs a leading
zero** or the 0x80 would read as −128; −1 is one octet but 255 is two.
−128 fits one octet and −129 needs two, because two's-complement reaches
−128 in eight bits.

A TML **context** message body is fixed at 12 octets (clause 3.3.2.2.4),
opening with `ISP1`; a **heartbeat** carries no body at all. Both are reject vectors.

The operation encodings are absent: they need their own vectors once the
GET-PARAMETER structured values are typed (see ROADMAP).

### `bp` — CCSDS 734.2-B-1 / RFC 5050

`vectors/bp/admin-record.json` — deliberately narrow.

| field | type | meaning |
|---|---|---|
| `record_type` | uint | high nibble of octet 0 |
| `flags` | uint | low nibble of octet 0 |

Most of BP's encoding is SDNVs, pinned more thoroughly in
`vectors/sdnv/sdnv.json`; duplicating them here would add no coverage and
a second place to drift. Endpoint identifiers are URI strings, not octet
fields. No decode vectors: the real decoder needs a complete record body
whose layout depends on the type, which is beyond what this file scopes.

## Deliberate absences

Things a consumer will notice are missing, and why.

**The state machines** — FOP-1 and FARM-1 in `cop`, the CFDP transaction
and LTP session engines, SLE's association machine, and `stack`'s
send/flush ordering. A single input/output pair cannot express a
sequence. The `sequence` form the schema defines is not yet populated.

**`ldc`** — CCSDS 121.0 has a published test corpus of 107 files, and
re-encoding it as JSON would add nothing and risk transcription error.
It is referenced by path from a vector file's `corpus` array instead.

**`xtce` parse expectations** — the input XML is shared and
language-neutral, but what a parser should produce from it is a tree, not
an octet string, and no vector kind expresses that. Only load-or-refuse
behaviour is checkable from the corpus.

**`sdl`** — internal channel machinery with no wire format of its own.
Nothing to pin.

**Behavioural tests** — API shape, aliasing, concurrency safety, fuzz
targets. Not portable, and a consumer writes its own.

## `schema_version` policy

Current version: 1.

Additive changes do **not** bump it: a new optional key, a new
capability name, a new package directory. A consumer that ignores keys it
does not know keeps working.

Anything that changes the meaning of an existing fixture **does** bump
it, and this document records the migration. Renaming a field, changing
hex to some other encoding, or redefining an error name are all version
bumps.

The loader rejects a file whose `schema_version` it does not recognise,
rather than guessing.

## Vectors marked `unverified`

These carry no clause authority. Agreeing with them proves an
implementation matches this corpus, not the standard. Each vector's
`note` states exactly what is and is not established.

| file | vector | what is unconfirmed |
|---|---|---|
| `aos/frame.json` | `fhec-of-primary-header` | The RS(10,6) arithmetic is reproducible (GF(2^4), field polynomial x^4+x+1, generator roots α^6..α^9). What is **not** established is that information symbols `[6,10,14,10,4,3]` are the ones CCSDS 732.0-B-4 specifies: they are header nibbles 0-3 and 10-11, which skips the 24-bit VC frame count in octets 2-4. Clause 4.1.2 settles it. |
| `aos/frame.json` | `frame-with-fhec-and-fecf` | Frame layout and FECF are confirmed; the FHEC value carried inside is subject to the row above. |
| `aos/frame.json` | `frame-with-fhec-inverse` | Same. Pins that FHEC octets survive decode and the data field starts after them. |

Three of 46 vectors. This number should fall, not rise.
