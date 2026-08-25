# Astro Protocol Conformance Audit

> **Resolution (2026-08-25):** Every finding below has been fixed in the working tree — code, tests, PICS, and guides. All 21 critical and all major findings are closed; golden wire vectors were added across the codecs. Three UNCERTAIN items were settled against the spec PDFs: the EPP length field is total-including-header (EPP-F4), the Proximity-1 source/destination bit was genuinely inverted (PXDL-6, fixed), and the RHC bit-extraction order was genuinely reversed (RHC-F1, fixed). Two audit assumptions were corrected during fixing: the USLP 32-bit FECF is the Proximity-1 CRC-32 (not CRC-32C), and USLP does reserve VCID 63 for idle frames. `go build ./...`, `go vet ./...`, and `go test -count=1 ./...` are green. Nothing is committed.

**Date:** 2026-08-23
**Scope:** All 25 protocol packages under `pkg/`, audited against their governing CCSDS / ECSS / OMG standards.
**Method:** Every non-test source file was read in full. Claims in each package's PICS and guide were checked against the code. Where possible, behavior was verified against the actual spec text (fetched PDFs), published test vectors, and independent implementations. Round-trip consistency was explicitly not accepted as evidence of conformance.
**Severity scale:**
- **CRITICAL** — produces or accepts non-conformant wire data; breaks interop with a conformant peer.
- **MAJOR** — mandatory behavior missing or wrong.
- **MINOR** — optional feature missing, over/under-strict behavior, or edge defect.
- **DOC** — PICS/guide claim not backed by the code.

---

## Executive summary

| Package | Standard | Verdict | Critical | Major |
|---|---|---|---|---|
| `spp` | CCSDS 133.0-B-2 | Largely conformant | 0 | 2 |
| `epp` | CCSDS 133.1-B-3 | **Not conformant — header rewrite needed** | 4 | 1 |
| `cfdp` | CCSDS 727.0-B-5 | Wire format conformant; procedures weak | 0 | 5 |
| `ltp` | CCSDS 734.1-B-1 | Codecs conformant; session machines break vs real peers | 0 | 5 |
| `bp` | CCSDS 734.2-B-1 | BPv6 codec conformant; CBHE (mandated) missing | 0 | 1 |
| `sdnv` | RFC 6256 / 5050 | Conformant | 0 | 0 |
| `tmdl` | CCSDS 132.0-B-3 / ECSS-E-ST-50-03C | Codec conformant; fill and idle-frame defects | 1 | 3 |
| `tcdl` | CCSDS 232.0-B-4 | Partial — AD/BD frames fine, BC layer missing | 0 | 4 |
| `cop` | CCSDS 232.1-B-2 | **Not conformant as COP-1** | 0 | 13 |
| `tcsc` | CCSDS 231.0-B-4 | **Not conformant — BCH parity wrong on every codeblock** | 2 | 2 |
| `aos` | CCSDS 732.0-B-4 | Conditionally conformant — FHP constants swapped | 1 | 2 |
| `usdl` | CCSDS 732.1-B-2 | **Not conformant — header codecs need redesign** | 5 | 4 |
| `tmsc` | CCSDS 131.0-B-5 | Conformant (dual-basis RS independently verified) | 0 | 0 |
| `sdl` | (shared helpers) | Conformant for what it does | 0 | 0 |
| `ocsc` | CCSDS 142.0-B-1 | Conformant subset, honestly declared | 0 | 6 (declared/scoped) |
| `pxdl` | CCSDS 211.0-B-6 | Conformant codec; COP-P/MAC out of scope (declared) | 0 | 2 (declared) |
| `pxsc` | CCSDS 211.2-B-3 | **Coding fails — reciprocal convolutional code** | 2 | 0 |
| `tcf` | CCSDS 301.0-B-4 | Conditionally conformant — leap seconds ignored | 0 | 2 |
| `crc` | CCSDS FECF/PEC | Conformant (verified check values) | 0 | 0 |
| `sle` | CCSDS 911.x / 912.1 / 913.1 | **Not interoperable — 4 BER encoding defects** | 4 | 4 |
| `ldc` | CCSDS 121.0-B-3 | Conformant (byte-identical vs official vectors) | 0 | 0 |
| `rhc` | CCSDS 124.0-B-1 | Likely critical bit-order defect | 1* | 0 |
| `pus` | ECSS-E-ST-70-41C | Conformant subset | 0 | 0 |
| `xtce` | OMG XTCE 1.2 | Conforming subset; one critical parse defect | 1 | 3 |
| `sdls` | CCSDS 355.0-B-2 | Conformant with the claimed profile | 0 | 0 |

\* flagged UNCERTAIN — no official test vectors exist for CCSDS 124.0; verdict rests on spec-text reading plus an independent implementation.

**Repo-wide total:** 21 critical, ~60 major findings.

### The headline problems

1. **Round-trip tests hid every critical bug.** All 21 critical findings are symmetric encode/decode mistakes: the package agrees with itself and disagrees with the standard. The packages with external test vectors (`ldc`, `tmsc`, `sdls`, `crc`, `ocsc`, internal `pn`) are exactly the ones with zero critical findings. Packages tested only by round-trips (`epp`, `usdl`, `tcsc`, `pxsc`, `sle`) hold the worst defects.
2. **Five packages cannot interoperate with conformant equipment today:** `epp`, `usdl`, `tcsc`, `pxsc` (coding layer), `sle`. `cop` cannot run a real command link (its FARM can never be unlocked by a spec-compliant Unlock).
3. **PICS documents materially overstate conformance** for `epp`, `usdl`, `tcsc`, `cop`, `tcdl`, `tmdl` (ECSS "0 gaps" claim), `sle` (partially), and several guides teach the buggy behavior as if it were the spec (`tmdl`, `aos`, `usdl`, `tcsc`, `cop`).
4. **Protocol procedures lag the codecs.** Frame/PDU byte layouts are mostly right; the state machines (COP-1 FOP/FARM, CFDP transaction handling, LTP session close-out) are simplified to the point of deadlocking or stalling against a third-party peer.

### Priority fix list (highest interop value first)

1. `tcsc`: complement BCH parity bits and zero the filler bit (`bch.go:62-64`) — 2-line fix, restores the entire TC uplink chain.
2. `pxsc`: bit-reverse the convolutional masks (use 0x4F/0x6D with the current shift direction) in `convolutional.go` — the Viterbi table fixes itself.
3. `aos`: swap `FHPNoPacketStart`/`FHPAllIdle` constants (`frame.go:46-48`).
4. `epp`: rewrite octet 0 as PVN(3)+PID(3)+LoL(2) and derive header size from the 2-bit LoL alone.
5. `tmdl` / `aos`: fill partial packet-zone frames with real SPP idle packets, not raw 0xFF/0xFE.
6. `sle`: encode SII attribute identifiers as OIDs; fix PEER-ABORT primitive form; remove the double-SEQUENCE wraps in SYNC-NOTIFY and FCLTU CltuLastProcessed/CltuLastOk.
7. `usdl`: redesign both header codecs (4-bit MAP ID, 4-octet truncated header, full non-truncated header fields, 1-octet TFDF header, correct construction rules).
8. `cop`: fix BC-frame identification and Unlock/Set V(R) decoding; add the FARM negative window; then grow FOP-1 (T1 timer, transmission limit, directives, S2–S5).
9. `xtce`: accept hex/octal/binary `FixedValue` forms.
10. `rhc`: resolve the bit-extraction ordering question against an independent implementation.
11. Add golden wire vectors from the Blue Books or reference implementations to every package that lacks them — this is the structural fix that prevents the whole class of bug.

---

## 1. spp — Space Packet Protocol (CCSDS 133.0-B-2)

**Verdict: largely conformant.** Primary header bit layout, length-minus-one encoding, APID/type/version ranges, and min/max sizes all verified correct.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| SPP-F1 | MAJOR | 4.1.4.2.1.4 | `packet.go:51-104, 220-335` | Idle-packet rules not enforced: APID 0x7FF with a secondary header encodes/decodes without error. `IsIdle()` exists but `Validate()` never uses it. |
| SPP-F2 | MINOR | 4.1.4.2.2 | `header.go:117-126` | Invented 63-octet secondary-header cap. The Blue Book has no such limit (that is TM's FSH limit); conformant packets are rejected. |
| SPP-F3 | DOC | 4.1.4 | `docs/pics/spp-pics.md:83` | PICS presents the CRC "Error Control" field as part of the Blue Book. It is a PUS/mission extension; wire-compatible, but mislabeled. |
| SPP-F4 | MAJOR | 4.1.3.5.3 | `packet.go:165-194` | `Encode()` never re-validates: mutating `UserData` after construction emits a length field inconsistent with the data field. |
| SPP-F5 | MINOR | 4.1.4.2.2 | `packet.go:178-182` | `Encode()` doesn't verify `SecondaryHeader.Encode()` returns `Size()` bytes. |
| SPP-F6 | MINOR | 4.1.3.4.3.2 | `service.go:50-59` | `SendPacket()` silently overwrites `WithSequenceCount()` values and mutates the caller's packet. |
| SPP-F7 | MINOR | 4.1.3.4.2 | package-wide | No segmentation/reassembly (optional), and the PICS doesn't declare its absence. |
| SPP-F8 | DOC/UNCERTAIN | 3.4.3.3 | `service.go:151-157` | Octet-string indication drops the decoded secondary header without an indicator. |
| SPP-F9 | Note | 4.1 | `packet.go:238` | Decode ignores trailing bytes (acceptable, should be documented). |
| SPP-F10 | Note | — | tests | Almost entirely round-trip tests; no golden wire vectors (layout verified correct by inspection). |

## 2. epp — Encapsulation Packet Protocol (CCSDS 133.1-B-3)

**Verdict: NOT CONFORMANT.** The header layer is structurally wrong and needs a rewrite. Nothing it emits parses at a conformant peer (first byte 0x70–0x7F instead of 0xE0–0xFF) and it rejects every real encapsulation packet.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| EPP-F1 | CRITICAL | 4.1.2.2, 4.1.2.4 | `header.go:60,139,182-184` | Octet 0 encoded as PVN(4 bits)=0111 \| PID(3) \| LoL(1). Spec: PVN(3)='111' \| PID(3) \| LoL(2). Every packet non-conformant; 1-octet idle must be 0xE0, code emits 0x70. |
| EPP-F2 | CRITICAL | 4.1.2.4 | `header.go:98-131, 274-294` | Header size derived from PID + 1-bit LoL, inventing a "Format 4". Spec: size is a pure function of the 2-bit LoL ('00'→1, '01'→2, '10'→4, '11'→8). |
| EPP-F3 | CRITICAL | 4.1.2.5/4.1.2.6 | `header.go:92-93, 149-168` | User Defined Field and Protocol ID Extension modeled as 8-bit mutually exclusive fields. Spec: two 4-bit fields sharing octet 1 of every 4- and 8-octet header. |
| EPP-F4 | CRITICAL/UNCERTAIN | 4.1.2.7 | `packet.go:118-137` | Packet Length field stored as total length, not total−1. Verify the clause, then fix. |
| EPP-F5 | MAJOR | 4.1.2.3/4.1.3 | `packet.go:97-109, 229-235` | Idle packets hard-restricted to the 1-octet form; multi-octet idle fill packets (needed for frame fill) can't be built or accepted. |
| EPP-F6 | MINOR | SANA registry | `header.go:63-69, 296-310` | PID '001' is LTP, code names it "Reserved"; PID 6 "User-Defined" doubtful. |
| EPP-F7 | MINOR | 4.1.2.3.2 | `packet.go:46-52` | No guard that PID '111' requires LoL ≥ '10'. |
| EPP-F8 | DOC | Annex A | `docs/pics/epp-pics.md:53-146` | Nearly every "Yes" in the PICS is unbacked; rewrite after the header rework. |
| EPP-F9 | DOC | — | `epp-pics.md:39` | Wrong edition date (Oct 2009 is B-2; B-3 is May 2020). |
| EPP-F10 | Note | — | `header_test.go:265-301` | The one raw-byte test hard-codes the wrong layout, cementing the bug. |
| EPP-F11 | Note | — | `service.go:18-27` | Default `MaxPacketLength` 65535 silently rejects valid 32-bit-length packets. |

## 3. cfdp — CCSDS File Delivery Protocol (CCSDS 727.0-B-5)

**Verdict: PDU codecs bit-correct (headers, all directives, checksums, CRC placement — verified against tables 5-1..5-18 with bit-level tests). No CRITICAL findings. The procedures layer fails the awkward orderings the standard is about.**

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| F1 | MAJOR | 4.6.1 | `receiver.go:270-301` | File Data before Metadata is discarded but marked received; after metadata retransmission the transaction "completes" with silently lost content (zero-filled under null checksum). |
| F2 | MAJOR | 4.6.4 | `receiver.go:269-301,394` | Completion only evaluated from `handleEOF`/`ResendFinished`; the last NAK-recovered segment never triggers Finished — Class 2 close-out stalls. |
| F3 | MAJOR | 4.11 | `receiver.go:314-342` | EOF (cancel) doesn't terminate the receive; the receiver NAKs for the rest of a cancelled transaction instead of ACK + Finished(cancel). |
| F4 | MAJOR | 4.8, table 4-1 | package-wide | No fault-handler machinery (cancel/suspend/ignore/abandon); override TLVs decode but are never applied. |
| F5 | MAJOR | 4.6.3.3 | `receiver.go:399-461` | Class 1 with closure can't close out incomplete transfers — the check-limit path is unreachable by any caller hook. |
| F6 | MINOR | 4.2.1 | `receiver.go:280-311` | Overlapping re-segmented retransmissions are folded into the checksum twice. |
| F7 | MINOR/UNCERTAIN | 4.11.2 | `sender.go:243-262` | Cancelled sender's EOF carries full file size/checksum rather than progress. |
| F8 | MINOR | 5.1 | `sender.go:354-365`, `receiver.go:187-229` | Inbound PDUs not filtered by source entity/sequence number; foreign PDUs applied to the wrong transaction. |
| F9 | MINOR | 4.11 | receiver (absent) | No receiver-side `Cancel()`; PICS "Cancel — Y" overstates. |
| F10 | MINOR | table 5-5 (0x6) | `receiver.go:176-184,280` | Data beyond declared file size undetected when it precedes EOF; reported as clean delivery. |
| F11 | MINOR | 4.2.2 | `receiver.go:249-263,410-424` | Unsupported checksum type: file written with no checksum, then Finished reports "discarded" while the file is retained. |
| F12 | MINOR | table 5-9 note | `sender.go:198-205` | Closure bit copied from config in acknowledged mode (spec: transmit '0'). |
| F13 | MINOR | table 5-5 | package-wide | Positive-ACK/keep-alive/NAK/check-limit faults can never be raised. |
| F14 | DOC | table 5-5 | `cfdp-pics.md:72` | "All fifteen" condition codes — the table defines fourteen. |
| F15 | DOC | — | `cfdp-pics.md:115,117`, guide | "Class 2 — Y" and "Cancel — Y" overstated given F2/F3/F9; guide's `ResendFinished()` advice is a no-op in the stated scenario. |

## 4. ltp / bp / sdnv (CCSDS 734.1-B-1, 734.2-B-1, RFC 6256)

**Verdicts:** `sdnv` conformant (only doc fixes). `ltp` wire codecs conformant; session machines would not survive a conformant RFC 5326 peer — two outright deadlocks. `bp` is a correct RFC 5050 BPv6 codec, but CBHE — mandated by the CCSDS profile and claimed by the docs — is absent, and CBHE bundles are rejected on decode.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| SDNV-1 | DOC | RFC 5050 §4.1 | `sdnv.go:10` | Package doc's worked example bytes are wrong (not even a valid SDNV). |
| SDNV-2 | DOC | — | `docs/guides/ltp.md:226` | Same wrong example repeated in the guide. |
| SDNV-3 | MINOR | RFC 6256 §3.2 | `sdnv.go:78,101` | 11-octet non-canonical encoding of a small value rejected as `ErrOverflow` (misleading name). |
| LTP-1 | MAJOR | RFC 5326 §6.13/6.14 | `session.go:280-282,399-411` | RA for the final report never sent — `StateClosed` short-circuits before draining pending RAs; a real peer retransmits that report forever. |
| LTP-2 | MAJOR | §6.9 | `session.go:324` | Retransmission cycles don't end with a checkpoint for interior gaps — sender wedges in `StateWaitingReport`. |
| LTP-3 | MAJOR | §3.2.1/§6.9 | `session.go:258-264` | Report-prompted checkpoints never carry the prompting report serial (always 0). |
| LTP-4 | MAJOR | §6.17 | `session.go:388-389` | Sender never ACKs a Cancel-from-Receiver (no CAR queued); receiver's cancel timer retransmits forever. |
| LTP-5 | MAJOR | §6.13 | `receiver.go:193-196` | RS upper bound wrong when EORP hasn't arrived (contiguous prefix instead of checkpoint end) — can deadlock the session. |
| LTP-6 | MINOR | §6.7/§6.13 | `session.go:259-261`, `receiver.go:186-230` | Fresh serial for every CP retransmission; no CP dedup, new RS per CP. |
| LTP-7 | MINOR | §3.2.2 | `content.go:124-144` | Claims not validated as sorted/non-overlapping; zero-claim reports accepted/emitted. |
| LTP-8 | MINOR | §6.11/§6.16 | `receiver.go:232-234` | Receiver closes before its RS is acked; all-green sessions never leave `StateActive`. |
| LTP-9 | DOC | PICS A1.4 | `ltp-pics.md:92,95,68` | RA/CAR/report-serial rows overstated given LTP-1/3/4. |
| LTP-10 | DOC | 734.1-B-1 | `ltp-pics.md` | PICS claims CCSDS profile conformance but cites only RFC 5326; no profile-aware behavior anywhere. |
| BP-1 | MAJOR | 734.2-B-1 §3.2 / RFC 6260 | `primary.go:146-162`, `eid.go:151-178` | CBHE not implemented; decode of a CBHE bundle fails. Cannot interoperate with CCSDS/ION peers using ipn EIDs. |
| BP-2 | DOC | RFC 6260 | `bp-pics.md:24`, `guides/bp.md:82-84` | Docs call string interning "CBHE" — false. |
| BP-3 | MINOR | RFC 5050 §4.2 | `primary.go:126-137` | Anonymous-source constraints, contradictory flags, reserved priority 3 unvalidated. |
| BP-4 | MINOR | annex C | `canonical.go:245-298`, `bundle.go:151-157` | ECOS C2 b)/c) rules enforced only by the construction helper, not validate/decode; ordinal 255 unpoliced. |
| BP-5 | MINOR | §5.8 | `fragment.go:60-75` | Post-payload blocks go to the first fragment, not the last. |
| BP-6 | MINOR | §4.1 | `bundle.go:212-227` | Trailing garbage after a bundle silently ignored; no consumed-length return. |
| BP-7 | DOC | §5.8 | `bp-pics.md:106` | PICS row restates the implementation, not the clause. |

## 5. tmdl — TM Space Data Link (CCSDS 132.0-B-3 / ECSS-E-ST-50-03C)

**Verdict: core frame codec substantially conformant** (header layout, FHP sentinels, FSH length semantics, FECF all verified; the four previously-closed ECSS gaps are genuinely closed). The "0 gaps" ECSS claim does not survive review.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| TMDL-01 | CRITICAL | 4.2.2/4.1.4; ECSS 5.4.3.3b/5.4.3.4g | `service.go:164`, `frame.go:370-377` | VCP pads partial frames with raw 0xFF instead of SPP idle packets. A conformant receiver parses fill as a packet header and loses sync. In-repo receive survives only via a nonstandard `isIdleFill` heuristic. |
| TMDL-02 | MAJOR | 4.1.2.7.4 | `frame.go:121-123` | Decoder rejects frames with SyncFlag=1 and FHP≠0x7FF; spec leaves FHP undefined there — conformant frames rejected. |
| TMDL-03 | MAJOR | 4.1.2.5 | `channel.go:150`, `physical.go:113` | Inserted idle frames always carry MC/VC counts of 0 (never pass through `FrameCounter`) — broken MC sequence at any conformant receiver. |
| TMDL-04 | MAJOR | 4.1.1/2.1.3 | `service.go:135-144`, `frame.go:252-298` | Fixed frame length never enforced by the codec; default VCP path emits variable-length frames; decode never checks length. |
| TMDL-05 | MINOR | 3.4 | `service.go:422-471` | VCA SDU size not validated against capacity; padding delivered to foreign receivers. |
| TMDL-06 | MINOR | 4.1.5 | `service.go:216-218,433-436` | `HasOCF` emits an all-zero OCF (reads as an all-zero CLCW); no OCF supplier hook. |
| TMDL-07 | MINOR | — | `physical.go:108-113` | Idle-frame SCID from nondeterministic map iteration with multiple MCs. |
| TMDL-08 | DOC | 4.1.2.7.6 | `guides/tmdl.md:193-196,272-276,331-334` | Guide has the FHP sentinels **inverted** (the pre-fix bug), contradicting the now-correct code. |
| TMDL-09 | DOC | 4.1.3.2.2.3 | `guides/tmdl.md:109,218-220` | Guide documents the old (wrong) FSH length semantics. |
| TMDL-10 | DOC | 232.0-B-4 §4.1.2.2 | `guides/tmdl.md:138` | Claims TC's TFVN is '01'; it is '00' (AOS is '01'). |
| TMDL-11 | DOC | ECSS 5.4.3.3b/5.4.3.4g | `ecss-50-03c-conformance.md` | Rows marked *conforms* citing 0xFF fill as an "idle packet" — should be *gap* (see TMDL-01). |
| TMDL-12 | DOC | ECSS 5.2.5c/5.2.6c/5.1c | same doc | Counter and frame-length-constancy rows overstated ("conforms" → "configurable" at best). |
| TMDL-13 | DOC | — | `tmdl-pics.md` TM-56/88/73 | Describes a removed mechanism, cites a nonexistent error, contradicts optional-FECF support. |
| TMDL-14 | DOC/UNCERTAIN | 3.5-3.10 | `tmdl-pics.md` TM-25-33,44-49 | MC_FSH/MC_OCF/MCF "Yes" with no service implementations. |
| TMDL-15 | DOC | — | ECSS doc | Many cited line numbers don't match the tree despite the doc's freshness claim. |
| TMDL-16 | DOC | — | `service_test.go:214-349` | Test comments still state swapped FHP semantics. |

## 6. tcdl / cop / tcsc — the telecommand chain

### tcdl (CCSDS 232.0-B-4) — **PARTIAL**

Frame codec wire-conformant for AD/BD data frames (header layout, 1024 cap, segment header, FECF all verified).

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| TCDL-1 | MAJOR | 4.1.3.3 | absent | BC control-command contents (Unlock `0x00`, Set V(R) `0x82 0x00 <V(R)>`) can be neither built nor parsed anywhere. |
| TCDL-2 | MAJOR | 4.1.2.3 | `frame.go:206-211` | Bypass=0 + CC=1 (invalid type) never rejected. |
| TCDL-3 | MAJOR | 4.1.2.7 | `service.go:135-137,228-230` | Type-B frames get incrementing N(S) when a FrameCounter is set; spec requires all-zeros. |
| TCDL-4 | MAJOR | 4.3.2 | `service.go:147-190` | Packet extraction returns the whole data field as one packet; `PacketSizer` is checked but never called. Multi-packet frames from a compliant sender break this receiver. |
| TCDL-5 | MINOR | 4.2.2 | `service.go:92-127` | No blocking on send (optional). |
| TCDL-6 | MINOR | 4.3.2 | `service.go:174-187` | Reassembly ignores MAP ID continuity/sequence gaps; `ErrIncompleteSegment` declared, never used. |
| TCDL-7 | MINOR/UNCERTAIN | 4.1.2.7.2 | `frame.go:334-339` | Trailing octets beyond frame length silently ignored. |
| TCDL-8 | DOC | — | `tcdl-pics.md:79,107,149-155` | "All 48 mandatory items" claim omits BC contents and advertises PacketSizer behavior the code never exercises. |

### cop (CCSDS 232.1-B-2) — **NON-CONFORMANT as COP-1**

CLCW bit layout is fully conformant. FOP-1/FARM-1 are drastic reductions.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| COP-1 | MAJOR | §5.1, table 5-1 | `fop.go:8-11` | FOP states S2–S5 missing (only Active and Initial exist). |
| COP-2 | MAJOR | §5.2 | absent | 10 mandatory directives missing (Initiate AD with CLCW check/Unlock/Set V(R), Terminate, Resume, Set V(S), sliding window, T1, transmission limit, timeout type). |
| COP-3 | MAJOR | §5.2 E16 | absent | Timer T1 does not exist — a lost CLCW stalls the machine forever. |
| COP-4 | MAJOR | §5.2 | absent | No Transmission_Limit/Count — unlimited retransmissions, no Alert(LIMIT). |
| COP-5 | MAJOR | E7-E12 | `fop.go:114-157` | CLCW Wait flag ignored entirely — keeps transmitting into a waiting FARM. |
| COP-6 | MAJOR | E3/E13-E14 | `fop.go:124-125` | No N(R) validity check; corrupted CLCW silently desynchronizes. |
| COP-7 | MAJOR | §5.1.9-10 | absent | No Suspend/Resume, no Alert notifications with reason codes. |
| COP-8 | MAJOR | §5.1 | absent | No BC/BD paths in FOP. |
| COP-9 | MAJOR | §6.1, E4 | `farm.go:84-96` | FARM negative window missing: a duplicate of the just-accepted frame triggers **lockout** instead of silent discard. |
| COP-10 | MAJOR | 232.0-B-4 §4.1.2.3 | `farm.go:58-68` | BC frames misidentified (executes commands for Bypass=0+CC=1, an invalid type; real BC frames pass as plain Type-B). A spec-compliant Unlock can never unlock this FARM. |
| COP-11 | MAJOR | 4.1.3.3, E7/E8 | `farm.go:102-111` | Unlock/Set V(R) contents never decoded; V(R) taken from the frame sequence number; Unlock wrongly rewrites V(R). |
| COP-12 | MAJOR | §6.1.7 | `farm.go:59-61` | FARM-B counter not incremented for BC frames. |
| COP-13 | MAJOR | E2/E10 | `farm.go:27` | Wait state never entered; CLCW Wait permanently 0. |
| COP-14 | MINOR | §5.2 | `fop.go:45-52,118-121` | windowWidth 0 accepted; queues not purged on lockout. |
| COP-15 | MINOR/UNCERTAIN | E5 | `farm.go:95` | Lockout entry clears the retransmit flag. |
| COP-16 | DOC | — | `cop-pics.md:138,150-156` | T1 classified "Optional"; "all 41 mandatory items" rests on a list omitting most of the state machine. |
| COP-17 | DOC | — | `guides/cop.md:78-79,143-144` | Guide presents the simplifications as the CCSDS design. |

### tcsc (CCSDS 231.0-B-4) — **NON-CONFORMANT on the wire**

CLTU framing, fill, and the randomizer (pinned to the published vector) are right.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| TCSC-1 | CRITICAL | §3.3 | `bch.go:62-64` | BCH parity bits not complemented. Every codeblock's 8th octet wrong in both directions; zero interop with real equipment. Fix: `parity = ^sr & 0x7F`. |
| TCSC-2 | CRITICAL | §3.3.2 | `bch.go:63-64` | Filler bit set to complement of last parity bit; spec fixes it at '0'. All-zeros info must encode to last octet 0xFE; code produces 0x01. Test pins the wrong value. |
| TCSC-3 | MAJOR (clause UNCERTAIN) | randomizer section | `tcsc.go:69-84` | Randomization applied before fill insertion; 0x55 fill goes out un-randomized. Fix: pad first, then randomize. |
| TCSC-4 | MAJOR | §8 | absent | PLOP-1/PLOP-2, acquisition and idle sequences not implemented; PICS omits the section while claiming full mandatory conformance. |
| TCSC-5 | MINOR | §3 | `bch.go:73-135` | Only SEC decoding; TED mode unavailable (3-bit errors can silently miscorrect). |
| TCSC-6 | MINOR | §6 | `tcsc.go:136-150` | `UnwrapCLTU` demands an exact tail match; spec receiver terminates on first failed codeblock and tolerates tail bit errors. |
| TCSC-7 | DOC | §3.3.2 | `tcsc-pics.md:69`, `bch.go:9`, guide | PICS/comments/guide present the wrong filler rule as the CCSDS rule; guide's "tail = BCH encoding of all-ones" claim is false. |
| TCSC-8 | DOC/UNCERTAIN | — | `tcsc-pics.md:52-90` | Clause references don't match 231.0-B-4 numbering. |
| TCSC-9 | DOC | — | `tcsc-pics.md:71` | "Detects up to 3 bit errors" untrue in the implemented SEC mode. |

## 7. aos / usdl (CCSDS 732.0-B-4, 732.1-B-2)

### aos — **conditionally conformant**

Header layout, FECF, B_PDU pointers, insert zone, OCF all verified correct.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| AOS-1 | CRITICAL | 4.1.4.2.3.4-5 | `frame.go:46-48` | M_PDU FHP special values swapped (`FHPNoPacketStart=0x7FE`, `FHPAllIdle=0x7FF`; spec is the reverse). Every continuation frame reads as "idle only" to a conformant receiver. One-line fix. |
| AOS-2 | MAJOR | 4.1.4.2.2 | `service.go:135`, `frame.go:484-491` | Packet-zone fill on Flush is raw 0xFE, not an idle packet; misparsed by other implementations. |
| AOS-3 | MAJOR | 4.1.4.3.3 | `service.go:362-366` | B_PDU Flush clamps the pointer to "all valid" for partials ≥2048 octets — idle fill delivered as user data. |
| AOS-4 | MINOR | 4.1.2.7 | absent | Frame Header Error Control (RS(10,6)) not implemented (optional). |
| AOS-5 | MINOR | 4.1.2.6.5 | `frame.go:123-155` | Signaling-field spares not validated on decode. |
| AOS-6 | MINOR | 4.1.2.5 | `frame.go:493-517` | OID frames always VCFC=0; no counter per VC 63; fill byte hardcoded. |
| AOS-7 | MINOR | 3.3.4 | `service.go:447-485` | VCA accepts short SDUs and pads; receiver trims via out-of-band knowledge. |
| AOS-8 | DOC | Annex A | `aos-pics.md:40,44-58` | "No exceptions" while AOS-1/2/3 deviate; FHEC row silently omitted. |
| AOS-9 | DOC | 4.1.4.2.3 | `guides/aos.md:110` | Guide documents the swapped FHP values. |

### usdl — **NOT CONFORMANT** (header codecs need redesign)

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| USLP-1 | CRITICAL | 4.1.2.6 | `frame.go:49,97-148` | MAP ID implemented as 6 bits; spec is 4. Shifts every header bit after it. |
| USLP-2 | CRITICAL | 4.1.2.2-7 | `frame.go:54-103` | Truncated header encoded as 5 octets; spec is exactly 4. |
| USLP-3 | CRITICAL | 4.1.2.9-14 | `frame.go:44-131` | Non-truncated header fixed at 7 octets; missing bypass/seq-control flag, protocol-command flag, spares, OCF flag, VCF count length, and the variable VCF count. |
| USLP-4 | CRITICAL | 4.1.4.2 | `frame.go:180-233` | TFDF header is 5 octets with an invented 16-bit "Sequence Number"; spec is 1 octet + conditional 16-bit pointer. |
| USLP-5 | CRITICAL | 4.1.4.2.2 | `frame.go:17-22,563-581`, `service.go:373-486` | Construction-rule values misassigned (code 1/2/7 vs spec '001' start / '010' continuing / '011' octet stream / '111' complete SDUs). |
| USLP-6 | MAJOR | 4.1.2.13-14 | `service.go:46-53`, `channel.go:66-87` | Frame counting/gap detection reads the invented TFDFH field instead of the VCF Count. |
| USLP-7 | MAJOR | 4.1.2.12 | `frame.go:519-561` | OCF presence not signaled/parsed via the header flag; sliced off by out-of-band knowledge. |
| USLP-8 | MAJOR | 2.2.1, 3.2.4 | `service.go:232-516` | No MAP-level demultiplexing — two MAPs on one VC corrupt each other; PICS claims MAP multiplexing. |
| USLP-9 | MAJOR | 4.1.2.7-8 | `service.go:141-489` | All fixed-length traffic emitted as truncated frames; spec restricts truncated frames to a special variant. |
| USLP-10 | MINOR/UNCERTAIN | 4.1.6 | `crc.go:33-46` | CRC-32C variant unverified against §4.1.6.3; no known-answer test. |
| USLP-11 | MINOR | 4.1.2.8 | `frame.go:466-542` | Decoded frame length never cross-checked against buffer length. |
| USLP-12 | MINOR | 4.1.4.2 | service Send paths | UPID never set (0 = reserved); idle frames hardcode VCID 63 (USLP reserves no idle VCID). |
| USLP-13 | DOC | Annex A | `usdl-pics.md:40,52-59,84` | PICS claims conformance for the nonconformant layout, including a §4.1.4.2.5 field that doesn't exist in the spec. |
| USLP-14 | DOC | 4.1.2/4.1.4 | `guides/usdl.md:33-109` | Guide teaches the nonconformant format as fact. |

## 8. tmsc / sdl (CCSDS 131.0-B-5, shared helpers)

**Verdict: CONFORMANT — zero critical, zero major.** The RS codec is a genuine dual-basis implementation (field poly 0x187, roots β^(112+j) with β=α^11, Berlekamp dual-basis transform) verified against an independent from-scratch re-implementation and libfec-compatible golden vectors. Randomizer pinned to the published sequence. The classic interop trap (conventional-basis RS) is **not** present.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| TMSC-F1 | MINOR | 4.3.6/4.4.4 | `rs.go:70-74,293-300` | No virtual-fill/shortened-codeblock support; only functional gap likely to bite real missions. |
| TMSC-F2 | MINOR | §3 | absent | Convolutional code not implemented (honestly declared in PICS). |
| TMSC-F3 | MINOR | §5-7 | absent | Turbo/LDPC/concatenated absent (declared); scheme-specific ASMs correspondingly absent. |
| TMSC-F4 | DOC | §4.2 | `guides/tmsc.md:135-136`, `tmsc-pics.md:76` | Docs say roots are α^(112+i) and never mention the dual basis — the one property that matters most; the code gets it right. |
| TMSC-F5 | DOC | TOC/Annex A | `tmsc-pics.md:50-103` | PICS clause references don't match the Blue Book numbering; "M" markings misstate mission-optional schemes. |
| TMSC-F6 | DOC | — | PICS+comments vs guide | Version skew: B-4 cited in code/PICS, B-5 in the guide. |
| TMSC-F7 | DOC | §4.1 | `guides/tmsc.md:127` | "128-bit burst" correction claim overstates (121 bits unaligned). |
| TMSC-F8 | MINOR | — | `rs.go:280-282` | Forney silently skips σ′=0 positions but still returns success. |
| TMSC-F9 | MINOR | — | `rs.go:152-157` | No post-correction syndrome recheck. |
| SDL-F1 | MINOR/UNCERTAIN | 732.1-B-2 §4.1.2 | `gap.go:18-24` | Counter constraint tops out at uint32 and the comment asserts USLP's count is 16-bit; USLP allows up to 56 bits (managed). |
| SDL-F2 | MINOR | 732.0-B-4 §4.1.2.5 | `gap.go` | AOS VC Frame Count Cycle ignored in gap arithmetic (edge case). |
| SDL-F3 | MINOR | — | `gap.go:27-37` | `GapCounter` is the one stateful type without a mutex, against package convention. |
| SDL-F4 | DOC | — | `gap.go:5-8` | Doc states USLP width as a fixed fact; it's a managed parameter. |

## 9. ocsc — Optical Coding & Sync (CCSDS 142.0-B-1)

**Verdict: conformant subset, honestly labeled.** All implemented stages (§3.2–§3.7: ASM, slicer, randomizer verified against the spec's published vector, optical CRC-32 0x20044009, termination) checked against the fetched spec text. No critical findings; the majors are declared scope (SCPPM encoder onward absent) plus two real receive-side gaps.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| OCSC-01..04 | MAJOR (declared) | §3.8-3.13 | `ocsc.go:19-22` | SCPPM encoder, channel interleaver, CSM, repeat/slot mapping/guard slots absent — declared in PICS A1.5; output ends at the SCPPM encoder input block. |
| OCSC-05 | MAJOR | §3.14.2 | `chain.go:82-110` | Quality indicator per-block, not per-frame; frames straddling a bad block returned indistinguishable from good ones. |
| OCSC-06 | MAJOR | §3.15 | package-wide | Sequence Indicator entirely absent and undeclared. |
| OCSC-07 | MINOR | §3.2 NOTE | `chain.go:41-67` | Every `Condition` call treated as transmission closure (mid-stream fill inserted). |
| OCSC-08 | MINOR | §3.14.1 | `chain.go:119-159` | ASM re-hunted at every bit offset; frame data containing the ASM pattern yields spurious frames. |
| OCSC-09 | MINOR | §5.2 | `ocsc.go:123-131` | 65536-octet frame-length bound not enforced. |
| OCSC-10 | DOC | Annex A | `ocsc-pics.md:44-86` | PICS omits HPE-1, HPE-14, §3.15, and §5.2 managed-parameter rows. |
| OCSC-11 | DOC | HPE-11 | `ocsc-pics.md:83` | Repeat marked "O"; official status is "1+". |
| OCSC-12 | DOC | §4 | PICS+guide | §4 misdescribed as "AOS profile" (it is beacon signaling, AOS **or USLP**). |
| OCSC-13 | DOC | — | `chain.go:79-81` | Wrong clause citation (§2 vs §3.14.2). |

## 10. pxdl / pxsc — Proximity-1 (CCSDS 211.0-B-6, 211.2-B-3)

### pxdl — **conformant codec, declared scope**

Header packing, length arithmetic, segmentation, SPDU shapes all verified.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| PXDL-1 | MAJOR (declared) | §7 | absent | COP-P (FOP-P/FARM-P) not implemented; PLCW is carry-only. Declared in PICS. |
| PXDL-2 | MAJOR (declared) | §5-6 | absent | MAC/hailing/session not implemented. Declared. |
| PXDL-3 | MINOR | §4 | absent | No MIB managed-parameter representation; not a PICS exception row. |
| PXDL-4 | MINOR | table 3-1 | `frame.go:164-186` | Reserved DFC ID '10' can be emitted on the wire. |
| PXDL-5 | MINOR | 3.2.4.1 | `frame.go:337-367` | Empty SPDU run accepted in a P-frame constructor. |
| PXDL-6 | UNCERTAIN (CRITICAL if wrong) | 3.2.2.9.2 | `frame.go:116-121` | Source/Destination bit polarity ('0'=destination) could not be independently confirmed; if inverted it misroutes every frame. Verify against the spec. |
| PXDL-7 | UNCERTAIN | fig. 3-5 | `spdu.go:78-117` | PLCW field order verified only against the repo's own docs. |
| PXDL-8 | MINOR (declared) | annex B | `spdu.go:129-195` | Directive/status payloads carried opaque. Declared "P". |

### pxsc — **FAIL on channel coding; PASS on framing**

PLTU, annex-C CRC-32 (poly derivation, zero preset, no inversion, ASM exclusion — all correct), idle sequences, synchronizer conform.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| PXSC-1 | CRITICAL | 3.4.3.1 | `convolutional.go:25-31,65-74` | Convolutional encoder realizes the **reciprocal** of the CCSDS 171/133 code (register orientation reversed vs masks). Confirmed by differential test vs the libfec convention: input 0x80 → repo 86B9 vs reference BA49. A compliant receiver cannot decode this output. Fix: bit-reverse the masks (0x4F/0x6D) with the current shift direction. |
| PXSC-2 | CRITICAL | 3.4.3 | `viterbi.go:52-67` | New Viterbi decoder builds its table by mirroring the wrong encoder — inherits PXSC-1; decodes itself, fails a conformant transmitter. Fix falls out of PXSC-1; add an independent known-answer vector. |
| PXSC-3 | DOC | — | `pltu.go:14-19` | Package doc still says no Viterbi decoder exists; contradicted by `viterbi.go` in the same package. |
| PXSC-4 | DOC | — | `pxsc-pics.md:24,90,93,104` | PICS stale vs working tree (Viterbi/soft decisions marked absent). |
| PXSC-5 | DOC | — | `guides/pxsc.md:158-170` | Guide stale ("only the encoder is here"). |
| PXSC-6 | MINOR (declared) | 3.4.4-5 | absent | LDPC, CSM, randomizer not implemented (declared). |
| PXSC-7 | MINOR | §3.6 | `sync.go:57-112` | Octet-aligned, CRC-brute-force synchronizer; spec receiver is bit-stream + length-field driven. |
| PXSC-8 | MINOR | 3.4.3.3 | `viterbi.go:43` | No external BER/known-answer benchmark; everything self-referential. |

## 11. tcf / crc — Time Codes and CRC

### tcf (CCSDS 301.0-B-4) — **conditionally conformant**

Wire-level bit layouts for CUC, CDS, CCS, ASCII all correct.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| TCF-1 | MAJOR | 1.6.2/3.2.1 | `tcf.go:27,32`, `cuc.go:82-87,216-217` | Leap seconds silently ignored: CUC "TAI" is computed on leap-second-free UTC arithmetic (~37 s off vs real TAI systems). `TAIUTCOffset` defined but never used. Fix or document. |
| TCF-7 | MAJOR | 3.3.2 | `cds.go:187-195` | Reserved sub-ms code '11' decoded as if '00' — wrong T-field length and wrong time, silently. |
| TCF-2..4 | DOC | 3.2-3.3 | `guides/tcf.md:140-211,359` | Guide contradicts code on TAI handling; both worked examples (CUC and CDS) are numerically wrong including a wrong P-field. |
| TCF-5 | MINOR | 3.2.2 | `cuc.go:39-40,175-180,241` | Fine octets capped at 6; spec allows 10. Spec-valid codes rejected. |
| TCF-6 | MINOR | 3.2.2 | `pfield.go:52-57` | Second P-field octet's further-extension bit masked off and ignored — silent misparse. |
| TCF-8 | MINOR | 3.3.1 | `cds.go:256-271` | Submilliseconds never range-checked (65 ms of "microseconds" accepted). |
| TCF-9 | MINOR | 3.3.1/3.3.4 | `cds.go:88-97` | Leap-second-day behavior undocumented; 23:59:60 unrepresentable. |
| TCF-10 | MINOR | 3.4.1 | `ccs.go:343-345` | Non-BCD nibbles accepted silently. |
| TCF-11 | MINOR | 3.4.1 | `ccs.go:219-228,261` | Second=60 validates but `Time()` silently normalizes to the next minute. |
| TCF-12 | MINOR | 3.4.1 | `ccs.go:251-282` | No year≤9999 or calendar cross-checks (Feb 31 encodes). |
| TCF-13 | MINOR | 3.5 | `ascii.go:93-194` | ASCII decode far laxer than the fixed-width ISO 8601 subset; out-of-range values silently normalized. |
| TCF-14 | MINOR | 2.2 | all decoders | No implicit-P-field (T-field-only) API — the common case in SPP secondary headers. |
| TCF-15 | MINOR | — | `cuc.go:278`, `cds.go:277` | Epoch compared with `!=` on `time.Time` (monotonic/location traps). |
| TCF-16 | MINOR | 3.2.3 | `cuc.go:88-105` | Fractional truncation (not rounding) undocumented. |
| TCF-17 | MINOR | — | `cuc.go:217`, `cds.go:241` | `time.Duration` overflow (~292 years) returns garbage for large day/coarse counts. |
| TCF-18 | DOC | 3.2.2 | `guides/tcf.md:334` | Size table understates max CUC size. |

### crc — **conformant**

CRC-16-CCITT bit-exact (0x29B1 check value verified); CRC-32C matches the standard variant. Two doc-level nits (Green Book cited as authority; CRC-32C clause reference unverified — add a USLP §4.1.6.3 known-answer test).

## 12. sle — Space Link Extension (911.1/911.2/911.5/912.1/913.1)

**Verdict: structurally faithful, NOT interoperable until four encoding defects are fixed.** TML framing, context/heartbeat, ISP1 credentials (SHA-256, constant-time), state machines, and most PDU layouts match the ASN.1. All tests are self-round-trip; no independent wire vectors.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| I-1 | CRITICAL | SII module | `bind.go:272-361`, `ber.go` | Service-instance attribute identifiers encoded as VisibleString instead of OBJECT IDENTIFIER (the BER codec has no OID type). Every BIND rejected by a real provider. |
| I-2 | CRITICAL | annex A2 [104] | `bind.go:566-581`, `service.go:196-199` | PEER-ABORT emitted as constructed [104] wrapping a full INTEGER TLV; must be primitive [104] with bare diagnostic octets. Wrong both directions. |
| I-3 | CRITICAL | Notification CHOICE | `common.go:504,541-546` | Loss-of-frame-sync SYNC-NOTIFY double-wrapped ([0]{SEQUENCE{...}} instead of [0]{...}) — breaks every RAF/RCF/ROCF transfer buffer carrying it. |
| FCLTU-1 | CRITICAL | CltuLastProcessed/CltuLastOk | `fcltu.go:979-1067` | Same double-wrap class: every ASYNC-NOTIFY and FCLTU STATUS-REPORT after a processed CLTU is mis-encoded. |
| I-4 | MAJOR | X.690 8.1.3.6 | `ber.go:299-303` | Indefinite-length BER rejected on receive; real providers emit it (the code's own comment concedes this). |
| I-5 | MAJOR | 913.1-B-2 §3.4 | absent | ISP1 peer-abort urgent-data transport mapping entirely missing. |
| I-6 | MAJOR | §3.1 | `assoc.go:565` | `CheckPeerCredentials` never called by any service machine — incoming service PDUs never authenticated; PICS SLE-15 overstates. |
| RAF/RCF/ROCF/FCLTU-GET | MAJOR | §3.10 each | all four services | GET-PARAMETER is a stub, and receiving one triggers PEER-ABORT — a lawful peer kills the association. |
| I-7..I-10 | MINOR | various | `assoc.go`, `credentials.go:138` | Auth level not configurable; context ranges unvalidated; duplicate invoke IDs never detected; digest length 20–32 accepted instead of {20,32}. |
| RCF-3/ROCF-3 | MINOR | VcId (0..63) | `common.go:378`, `rocf.go:100-104` | VC IDs accept 0–255. |
| FCLTU-3 | MINOR | 3.1.6/3.6.2.5.1 | `fcltu.go:1478-1525` | CLTU id advances only on return — pipelining (normal uplink mode) impossible. |
| FCLTU-4 | MINOR | §3.9 | `fcltu.go:1534-1555` | Event-invocation ids left entirely to the caller. |
| RAF-3 | MINOR | table 4-1 | `raf.go:808-814` | STATUS-REPORT accepted in state 1. |
| D-1..D-5 | DOC | — | PICS + `doc.go` + guide | Credential-check claim (I-6); PEER-ABORT/SYNC-NOTIFY/ASYNC-NOTIFY marked "Yes" while mis-encoded; SII presented as strings; stale "not in this package yet"; SHA-1 receive claim contradicted by code. |

## 13. ldc / rhc — Compression (CCSDS 121.0-B-3, 124.0-B-1)

### ldc — **CONFORMANT**

Encoder and decoder validated byte-identically against the official CCSDS test vector set (64 run; the 8 vendored-but-unrun n25–n32 vectors were independently run during this audit and also pass). All findings are polish:

| ID | Severity | Finding |
|---|---|---|
| LDC-F1 | MINOR | `vectors_test.go:108` loop stops at n=24; extend to 32. |
| LDC-F2 | DOC | PICS vector count (says 64 vendored; 72 are). |
| LDC-F3 | MINOR | Unbounded `Decompress` can't skip >7 bits of word fill for B>1 output words (fails safe). |
| LDC-F4 | MINOR | uint64 overflow in second-extension limit arithmetic at n=31/32 (unreachable in practice). |
| LDC-F5 | DOC | Signed+bypass refusal is the conservative table 7-1 reading; note the interpretation in the PICS. |

### rhc — **likely CRITICAL bit-order defect (UNCERTAIN)**

Mask update, COUNT/RLE coding, and vector structure match 124.0-B-1 equation-for-equation against the fetched spec text.

| ID | Severity | Clause | Location | Finding |
|---|---|---|---|---|
| RHC-F1 | CRITICAL (UNCERTAIN) | §5.2.4 eq 11 | `vector.go:173-181`, `compressor.go:503-554`, `decompressor.go:189-404` | Bit-extraction output order appears reversed vs the standard (spec emits last selected position first; code emits forward order). The independent VisionSpace PocketPlus implementation reverses at every BE site. Streams would silently mis-reconstruct on a conformant decoder. No official vectors exist to arbitrate; cross-validate, then fix or document. |
| RHC-F2 | DOC | §5.2.4 | `rhc-pics.md:106` | Eq-11 support claimed without qualification. |
| RHC-F3 | MINOR | — | `compressor.go:146-150` | Mislabeled error sentinel for negative intervals. |
| RHC-F4 | MINOR | §2.1/2.2 | `decompressor.go:379-410` | Hostile vector can satisfy the robustness gate via its self-declared V_t; document, or add a strict mode. |
| RHC-F5 | DOC | Annex A | `rhc-pics.md:130-156` | Accurate statement that the decompressor is derived and no vectors exist — kept for context. |

## 14. pus — Packet Utilization Standard (ECSS-E-ST-70-41C)

**Verdict: conformant subset — no critical or major findings.** Both secondary headers, ack-flag bit positions, ST[01] request ID, ST[03]/[05]/[17] layouts, and the CUC time field all verified; excluded services are properly declared.

| ID | Severity | Finding |
|---|---|---|
| PUS-001 | MINOR | TM[1,10] (failed routing) missing; `Validate` rejects subtype > 8. |
| PUS-002 | MINOR | TC[5,7]/TM[5,8] (disabled-events list) missing. |
| PUS-003 | DOC | PICS ST[03] exception rows don't enumerate all missing subtypes. |
| PUS-004 | MINOR/UNCERTAIN | ST[17,3]/[17,4] APID hardcoded at 2 octets instead of profile-tailorable. |
| PUS-005 | MINOR | Word-boundary rule left to caller-supplied spare bytes with no validation. |
| PUS-006 | DOC | `TimeNone` extension not listed as a PICS exception. |
| PUS-007 | DOC/UNCERTAIN | Internally inconsistent clause citations. |
| PUS-008 | MINOR | Zero-width ID fields + hostile count → unbounded allocation (memory exhaustion); fix before ingesting untrusted telemetry. |
| PUS-009 | DOC | ISO checksum alternative unsupported by the stack; PICS rationale silent. |
| PUS-010 | DOC | Three PICS self-inconsistencies (PFC 19–46 P vs N; ST[05] "Y" vs missing subtypes; ST[01] wording). |
| PUS-011 | MINOR/UNCERTAIN | Trailing octets ignored on fixed-size bodies (undocumented leniency). |

## 15. xtce — XML Telemetric and Command Exchange (OMG XTCE 1.2)

**Verdict: conforming as a documented subset (parser/validator only), with one critical parse defect.** Namespace handling, EntryList ordering, encoding defaults, reference resolution, and validation all verified empirically.

| ID | Severity | Finding |
|---|---|---|
| X-01 | CRITICAL | Hex/octal/binary `FixedValue` (legal `FixedIntegerValueType` union forms) reject the **entire document** — fields decoded as `*int64`. Verified empirically. |
| X-02 | MINOR | The X-01 failure is misreported as `ErrNotSpaceSystem`. |
| X-03 | MAJOR | `BlockMetaCommand`/`MetaCommandRef` vanish silently; not in the coverage matrix. |
| X-04 | MAJOR | `BaseMetaCommand` argument assignments dropped silently; matrix says "Supported". |
| X-05 | MAJOR | Array/Aggregate/RelativeTime parameter types dropped (documented; Validate catches references, but definitions leave no trace). |
| X-06 | MAJOR | Context calibrators dropped with no marker — consumers compute wrong engineering values with the default curve. |
| X-07..X-18 | MINOR | changeThreshold dropped; sizeInBits/boolean/unit-power/referenceLocation defaults not applied via accessors; lax name-reference and namespace checking; encoding enums unvalidated; documented ignores (alarms, streams, commanding, segment entries). |
| X-19..X-21 | DOC | Matrix "nothing causes Load to fail" claim false given X-01; MetaCommandSet/BaseMetaCommand rows overstate; defaults table gaps. |

## 16. sdls — Space Data Link Security (CCSDS 355.0-B-2)

**Verdict: conformant with the claimed profile — no critical or major findings.** The §E2 AES-CMAC baseline (recent commit) verified against the fetched spec text: 256-bit key, 32-bit SN, 128-bit MAC, 6-octet header, no encryption — all correct; CMAC validated against the full RFC 4493 + NIST SP 800-38B AES-256 vector sets; header layout, AAD scope, auth mask semantics, anti-replay ordering all verified.

| ID | Severity | Finding |
|---|---|---|
| SDLS-1 | DOC | Guide still says AES-CMAC is "deliberately absent" and points at a removed TODO — contradicts code and PICS. |
| SDLS-2 | MINOR | Nil `AuthMask` authenticates every header octet, violating mandatory exclusions (TM MCFC, AOS FHEC, insert zone) — ship per-frame-type baseline mask constructors. |
| SDLS-3 | MINOR+DOC | SA lookup gets only the SPI; GVCID binding left to the caller while the PICS marks §4.2.4.3 fully supported. |
| SDLS-4 | MINOR/DOC | No published frame-level SDLS interop vectors in tests (crypto primitives have them; frame layer is self-referential). |
| SDLS-5 | MINOR | MAC truncation floor of 12 octets applied even to CMAC where the spec permits shorter. |
| SDLS-6 | DOC | Comment mislabels GMAC as an annex baseline. |
| SDLS-7 | MINOR/UNCERTAIN | Anti-replay window off-by-one under one reading of "expected value" (common interpretation matches the code). |
| SDLS-8 | MINOR (declared) | Encryption-only service type unimplemented — properly declared. |

---

## Cross-cutting recommendations

1. **Golden vectors everywhere.** Every package that ships an encoder should pin at least one externally sourced known-answer vector (Blue Book examples, SANA/agency test data, libfec/jSLE/CryptoLib/ION/VisionSpace traces). This one habit would have caught all 21 critical findings before release.
2. **Regenerate the PICS documents after fixes.** Six PICS files currently claim conformance the code does not have; two guides teach the bugs as spec behavior. Treat PICS rows as claims to be tested, not documentation to be written last.
3. **Idle/fill discipline.** Three packages (tmdl, aos, epp) mishandle idle packets or fill in ways that break other receivers. Implement SPP idle-packet fill once and share it.
4. **Procedures need adversarial peers.** The state-machine gaps (cop, cfdp, ltp) are invisible to loopback tests by construction. Add tests that drive each machine with spec-derived event sequences a real peer would produce (duplicate frames, cancels, out-of-order arrivals, timer expiries).
5. **Two open verification questions** worth settling against the spec PDFs: PXDL-6 (source/destination bit polarity) and EPP-F4 (length-minus-one semantics) — each is a one-line fix if wrong.
