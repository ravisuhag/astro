---
title: Packet Utilization Standard
description: "PICS proforma: what this package implements, clause by clause."
order: 210
---

## Conformance Statement for `pkg/pus` — ECSS-E-ST-70-41C

---

## A1.1 GENERAL INFORMATION

### A1.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 23/08/2026 |
| PICS Serial Number | ASTRO-PUS-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A1.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/pus |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | Every codec is parameterized by an explicit `MissionProfile`; there is no package-level default |
| Other Information | Go library implementing PUS-C secondary headers and four services. The TC and TM secondary headers implement `spp.SecondaryHeader`, so they compose with `pkg/spp` without changes to either package. The TM absolute time field encodes via `pkg/tcf` CUC. |

### A1.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/pus (Go package) |
| System Name(s) | Astro |

### A1.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | ECSS-E-ST-70-41C (Telemetry and telecommand packet utilization, 15 April 2016) |
| Have any exceptions been required? | Yes [X] No [ ] — see A1.5 |

---

## A1.2 PACKET STRUCTURE

| Feature | Reference | Status | Support |
|---|---|---|---|
| TM packet secondary header | 7.4.3.1, Figure 7-7 | M | Y |
| TM PUS version number = 2 | 7.4.3.1c | M | Y — other versions rejected on decode |
| Spacecraft time reference status | 7.4.3.1d, 7.4.3.1e | M | Y — 4 bits, zero when unsupported |
| TM message type ID | 7.4.3.1f | M | Y — service and subtype, 8 bits each |
| TM message type counter | 7.4.3.1g, 7.4.3.1h | M | Y — 16 bits, fixed by Figure 7-7 |
| TM destination ID | 7.4.3.1i | M | Y — 16 bits, fixed by Figure 7-7 |
| TM time field | 7.4.3.1j, 7.4.3.1k | M | Y — CUC, implicit or explicit P-field, or raw |
| TM spare field | 7.4.3.1l | O | Y — presence and width declared by the profile |
| TC packet secondary header | 7.4.4.1, Figure 7-9 | M | Y |
| TC PUS version number = 2 | 7.4.4.1c | M | Y |
| TC acknowledgement flags | 7.4.4.1d | M | Y — all four bits, positions as specified |
| TC message type ID | 7.4.4.1e | M | Y |
| TC source ID | 7.4.4.1f | M | Y — 16 bits, fixed by Figure 7-9 |
| TC spare field | 7.4.4.1g | O | Y — presence and width declared by the profile |
| Secondary header within the CCSDS 1-63 octet bound | CCSDS 133.0-B-2 | M | Y — `MissionProfile.Validate` enforces it |

---

## A1.3 TIME FIELD ENCODING

| Feature | Reference | Status | Support |
|---|---|---|---|
| Absolute time is PTC 9 | 7.4.3.1j, Table 7-10 | M | Y |
| PFC 0 — explicit format, including the P-field | Table 7-10 | O | Y — `TimeCUCExplicit` |
| PFC 3 to 18 — CUC, implicit P-field | Table 7-10 | O | Y — `TimeCUC`, coarse 1-4 and fine 0-3 octets |
| PFC 19 to 46 — CUC, wider fine time | Table 7-10 | O | N — see A1.5 |
| PFC 1, 2 — CDS format | Table 7-10 | O | N — see A1.5 |
| CCSDS 1958 epoch | 7.4.3.1j note 1 | M | Y |
| Agency-defined epoch | 7.4.3.1j note 1 | O | Y — selects CUC time code Level 2 |

---

## A1.4 SERVICES

| Service | Reference | Status | Support |
|---|---|---|---|
| ST[01] request verification | 8.1 | M | Y — all nine report subtypes: TM[1,1] to TM[1,8] and TM[1,10] |
| TM[1,1] successful acceptance | 8.1.2.1 | M | Y |
| TM[1,2] failed acceptance | 8.1.2.2 | M | Y — with failure notice |
| TM[1,3] successful start | 8.1.2.3 | O | Y |
| TM[1,4] failed start | 8.1.2.4 | O | Y |
| TM[1,5] successful progress | 8.1.2.5 | O | Y — with step ID |
| TM[1,6] failed progress | 8.1.2.6 | O | Y — with step ID and failure notice |
| TM[1,7] successful completion | 8.1.2.7 | O | Y |
| TM[1,8] failed completion | 8.1.2.8 | O | Y |
| TM[1,10] failed routing | 8.1.2.10 | O | Y — request ID and failure notice |
| Request ID structure | Figure 8-1 | M | Y — 32 bits, mirrors the CCSDS primary header |
| ST[03] housekeeping | 8.3 | O | P — five message subtypes; see A1.5 for the full list of excluded ones |
| TC[3,1] create report structure | 8.3.2.1 | O | Y — including super-commutated groups |
| TC[3,3] delete report structures | 8.3.2.3 | O | Y |
| TC[3,5] enable periodic generation | 8.3.2.5 | O | Y |
| TC[3,6] disable periodic generation | 8.3.2.6 | O | Y |
| TM[3,25] housekeeping parameter report | 8.3.2.25 | O | Y — framing only; values supplied by the caller |
| ST[05] event reporting | 8.5 | O | Y — all eight message subtypes |
| TM[5,1] to TM[5,4] event reports | 8.5.2.1 to 8.5.2.4 | O | Y — all four severities |
| TC[5,5] enable event reporting | 8.5.2.5 | O | Y |
| TC[5,6] disable event reporting | 8.5.2.6 | O | Y |
| TC[5,7] report disabled events | 8.5.2.7 | O | Y — empty body |
| TM[5,8] disabled events list report | 8.5.2.8 | O | Y |
| ST[17] test | 8.17 | O | Y |
| TC[17,1] / TM[17,2] are-you-alive | 8.17.2.1, 8.17.2.2 | O | Y |
| TC[17,3] / TM[17,4] on-board connection | 8.17.2.3, 8.17.2.4 | O | Y — APID width declared by `APIDBytes`, two octets by default |

Decoders enforce exact body lengths on fixed-size messages, in line with the
PUS acceptance checks: octets beyond the structure a message type declares
are rejected with `ErrTrailingBytes`, not ignored. Messages that
end in a variable-length field the receiving end interprets — the ST[01]
failure data, the ST[05] auxiliary data, the TM[3,25] parameter values — carry
those trailing octets verbatim by design.

---

## A1.5 EXCEPTIONS AND UNSUPPORTED FEATURES

| Feature | Reference | Support | Rationale |
|---|---|---|---|
| CDS time format, PFC 1 and 2 | Table 7-10 | N | `pkg/tcf` implements CDS, but the PUS time field currently wires only CUC. A follow-up. |
| CUC fine time beyond 3 octets, PFC 19 to 46 | Table 7-10 | N | `pkg/tcf` caps fine time at 3 octets. |
| No time field at all — `TimeNone` | 7.4.3.1j | Extension | Not a Table 7-10 option: the standard makes the TM time field mandatory. `TimeNone` exists for ground tooling and tests; a flight profile declares a real format. |
| Housekeeping parameter sampling | 8.3 | N by design | Only the flight software knows what a parameter means. This package frames the values; the caller supplies them. |
| ST[03] diagnostic subtypes: TC[3,2], TC[3,4], TC[3,7], TC[3,8], TC[3,11], TM[3,12], TM[3,26], TC[3,28], TC[3,30], TC[3,32], TC[3,34], TM[3,36] | 8.3.2 | N | The diagnostic twin of every housekeeping message. Structurally identical to the housekeeping side; a mechanical follow-up. |
| ST[03] structure reporting: TC[3,9], TM[3,10] | 8.3.2.9, 8.3.2.10 | N | Report-back of stored structure definitions. A follow-up. |
| ST[03] one-shot, append, and interval modification: TC[3,27], TC[3,29], TC[3,31] | 8.3.2 | N | A follow-up. |
| ST[03] periodic generation properties: TC[3,33], TM[3,35] | 8.3.2 | N | A follow-up. |
| ST[03] parameter functional reporting, subtypes [3,37] to [3,44] | 8.3.2 | N | The whole functional-reporting capability is excluded. A follow-up. |
| Packet error control field | 7.4.3.2d to f, 7.4.4.2d | N | Checksumming is declared per mission; `pkg/spp` offers the CRC-16 alternative via `WithErrorControl`. The ISO 16-bit checksum alternative the standard also allows is not implemented anywhere in this stack, so a mission declaring it cannot use these packages unmodified. |
| User data spare and padding word size | 7.4.3.2b, 7.4.4.2b | N | Padding of the user data field to the mission word size is left to the caller. For the secondary headers, a declared `WordSizeBytes` makes `MissionProfile.Validate` check word alignment; zero leaves it unchecked. |
| ST[02], ST[04], ST[06], ST[08], ST[09], ST[11] to ST[16], ST[18] to ST[23] | clauses 6 and 8 | N | Deliberately out of scope for this first pass. Each is a follow-up. |
| On-board scheduling semantics | 6.11, 6.22 | N | Out of scope. |

---

## A1.6 MISSION TAILORING SURFACE

Widths this implementation exposes through `MissionProfile`, each because the
standard leaves it to the mission:

| Profile field | Reference |
|---|---|
| `TCSpareBytes` | 7.4.4.1g |
| `TMSpareBytes` | 7.4.3.1l |
| `TimeFormat`, `CUCCoarseBytes`, `CUCFineBytes`, `CUCEpoch`, `TimeRawBytes` | 7.4.3.1j, Table 7-10 |
| `StepIDBytes` | Figures 8-5, 8-6 (step ID is enumerated, no stated width) |
| `FailureCodeBytes` | Figure 8-2 and siblings (failure code is enumerated) |
| `EventDefinitionIDBytes` | Figure 8-59 |
| `HousekeepingStructureIDBytes`, `ParameterIDBytes`, `CollectionIntervalBytes`, `CountBytes` | Figure 8-21 |
| `APIDBytes` | 8.17.2.3, 8.17.2.4 (the ST[17] APID is enumerated, no stated width; zero selects the common 2-octet width) |
| `WordSizeBytes` | 7.4.3.1l, 7.4.4.1g (when non-zero, `Validate` requires both secondary header sizes to be whole multiples of it) |

Widths the standard fixes, and this implementation therefore treats as
constants rather than profile fields: TC source ID, TM message type counter,
and TM destination ID, all 16 bits (Figures 7-7 and 7-9).
