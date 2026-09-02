---
title: COP-1
short: COP
description: "PICS proforma: what this package implements, clause by clause."
order: 110
---

## Conformance Statement for `pkg/cop`, CCSDS 232.1-B-2

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 25/08/2026 |
| PICS Serial Number | ASTRO-COP-PICS-002 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/cop |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS COP-1 reliable frame delivery. Three components: FOP-1 (ground-side state machine S1-S6 with directives, T1 timer, transmission limit, suspend/resume, BC/BD paths), FARM-1 (spacecraft-side frame acceptance with positive/negative windows, buffer-driven Wait state, BC control command decoding), and CLCW (status reporting via TM return link). Thread-safe implementations with mutex protection. The T1 timer is caller-driven (no wall clock): configure with `SetT1Initial` and advance with `Tick`. |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub, github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/cop (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 232.1-B-2 (Communications Operation Procedure-1, Blue Book, Issue 2, October 2019) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE, The FOP-1 event/action behavior follows the state tables of the standard as closely as practical for a library API; simplifications are declared per item below and in the exceptions list.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: FOP-1 State Machine (section 5.1)

| Item | Description | Reference | Status | Support | Notes |
|---|---|---|---|---|---|
| COP-1 | S1, Active | 5.1, table 5-1 | M | Yes | `FOPActive`. Frames accepted, assigned V(S), transmitted within the sliding window. |
| COP-2 | S2, Retransmit without Wait | 5.1 | M | Yes | `FOPRetransmitWithoutWait`. Entered on a retransmit-requesting CLCW or T1 expiry; unacknowledged frames re-queued for transmission. |
| COP-3 | S3, Retransmit with Wait | 5.1 | M | Yes | `FOPRetransmitWithWait`. Entered on CLCW with Retransmit=1 and Wait=1; retransmissions held until the Wait flag clears. |
| COP-4 | S4, Initialising without BC Frame | 5.1 | M | Yes | `FOPInitialisingWithoutBC`. Entered by Initiate AD with CLCW check; completes on a clean CLCW, adopting its Report Value as V(S)/NN(R). |
| COP-5 | S5, Initialising with BC Frame | 5.1 | M | Yes | `FOPInitialisingWithBC`. Entered by Initiate AD with Unlock or Set V(R); the BC frame is served by `GetNextFrame()` and retransmitted on T1 expiry until a confirming CLCW arrives. |
| COP-6 | S6, Initial | 5.1 | M | Yes | `FOPInitial`. Start state, and target of every Alert. CLCWs are ignored in S6. |
| COP-7 | Suspend States SS 0-4 | 5.1.9 | M | Yes | `SuspendState()` returns SS (0 = not suspended, 1-4 = suspended from S1-S4). Entered on T1 expiry with the transmission limit reached and timeout type TT1. |
| COP-8 | Alert with Reason Codes | 5.1.10 | M | Yes | Every Alert purges all queues, stops T1, and moves to S6. `LastAlert()` reports the reason: `AlertLimit`, `AlertLockout`, `AlertSynch`, `AlertNNR`, `AlertCLCW`, `AlertT1`, `AlertTerminate`. |

### Table A-2: FOP-1 Directives (section 5.2)

| Item | Description | Reference | Status | Support | Notes |
|---|---|---|---|---|---|
| COP-9 | Initiate AD Service (without CLCW check) | 5.2 (E23) | M | Yes | `InitiateADWithoutCLCW()`; `Initialize(vs)` combines it with Set V(S). |
| COP-10 | Initiate AD Service with CLCW check | 5.2 (E24) | M | Yes | `InitiateADWithCLCWCheck()`, waits in S4 for a clean CLCW (Lockout=0, Wait=0, Retransmit=0) under T1. |
| COP-11 | Initiate AD Service with Unlock | 5.2 (E25) | M | Yes | `InitiateADWithUnlock(bcFrame)`, transmits the encoded BC Unlock frame (built with `tcdl.NewUnlockFrame`), retransmits on T1 expiry, completes when the CLCW Lockout flag clears. |
| COP-12 | Initiate AD Service with Set V(R) | 5.2 (E27) | M | Yes | `InitiateADWithSetVR(vr, bcFrame)`, completes when a CLCW with Lockout=0 and Report Value == vr arrives; V(S) and NN(R) are then set to vr. |
| COP-13 | Terminate AD Service | 5.2 (E29) | M | Yes | `TerminateAD()`, purges all queues, stops T1, records `AlertTerminate`, moves to S6. |
| COP-14 | Resume AD Service | 5.2 (E30-E33) | M | Yes | `ResumeAD()`, restores S1-S4 from SS 1-4 and restarts T1. Returns `ErrFOPNotSuspended` when SS = 0. |
| COP-15 | Set V(S) | 5.2 (E35) | M | Yes | `SetVS(vs)`, only in S6 with SS = 0; sets V(S) and NN(R). |
| COP-16 | Set FOP Sliding Window | 5.2 (E36) | M | Yes | `SetSlidingWindow(w)`, validates 1..255, rejects 0 with `ErrFOPInvalidWindow`. `NewFOP` clamps a zero width to 1. |
| COP-17 | Set T1 Initial | 5.2 (E37) | M | Yes | `SetT1Initial(ticks)`, caller-defined tick units; 0 disables the timer. |
| COP-18 | Set Transmission Limit | 5.2 (E38) | M | Yes | `SetTransmissionLimit(n)`, validates 1..255. Default 255. |
| COP-19 | Set Timeout Type | 5.2 (E39) | M | Yes | `SetTimeoutType(tt)`, TT0 (Alert on expiry at limit) or TT1 (suspend). Default TT0. |

### Table A-3: FOP-1 Timer and Transmission Limit

| Item | Description | Reference | Status | Support | Notes |
|---|---|---|---|---|---|
| COP-20 | T1 Timer | 5.2 (E16-E18) | M | Yes | Caller-driven clock: `Tick(n)` advances it; `TimerRunning()` reports state. Started on frame queueing and initiate directives, restarted when new frames are acknowledged, stopped when everything is acknowledged. NOTE: the library holds no wall clock by design; the mission supplies the tick source. |
| COP-21 | Transmission Limit / Transmission Count | 5.2 | M | Yes | `txCount` starts at 1, increments on every retransmission initiation, resets when new frames are acknowledged. When T1 expires (or a retransmit is requested) with the count at the limit and no progress: Alert(LIMIT) under TT0, suspend under TT1. |
| COP-22 | Timer expiry actions | 5.2 (E16-E18) | M | Yes | Below the limit: re-queue unacknowledged AD frames (or re-serve the BC frame in S5), increment the count, restart T1. At the limit: TT0 -> `AlertLimit` (S1-S3) / `AlertT1` (S4-S5); TT1 -> suspend with SS 1-4. |

### Table A-4: FOP-1 CLCW Processing (E1-E14)

| Item | Description | Reference | Status | Support | Notes |
|---|---|---|---|---|---|
| COP-23 | Lockout detection | E14 | M | Yes | CLCW Lockout=1 in S1-S4 -> Alert(LOCKOUT), purge, S6. In S5 the flag is expected (the Unlock has not landed yet) and is ignored. |
| COP-24 | N(R) validity check | E13 | M | Yes | N(R) must lie in NN(R)..V(S) (mod 256). Violations -> Alert(NNR) and `ErrFOPInvalidNR`. |
| COP-25 | Acknowledgment | E1/E5 | M | Yes | Frames with N(S) < N(R) removed from the sent and wait queues; NN(R) updated; transmission count reset and T1 restarted while frames remain outstanding, stopped when all are acknowledged. |
| COP-26 | Retransmit flag handling | E8-E12 | M | Yes | Retransmit=1 with Wait=0: unacknowledged frames re-queued (no duplicates), count incremented, S2. Retransmit=1 with Wait=1: retransmissions withheld, S3, released when Wait clears. Retransmit=1 with nothing outstanding: Alert(SYNCH). |
| COP-27 | Wait flag handling | E2/E6/E10 | M | Yes | Wait=1 without Retransmit, or Wait=1 with nothing outstanding, is an invalid CLCW -> Alert(CLCW). Wait=1 with Retransmit=1 -> hold retransmissions (S3). |

### Table A-5: FOP-1 Transmit Paths

| Item | Description | Reference | Status | Support | Notes |
|---|---|---|---|---|---|
| COP-28 | AD frame transmission | 5.1 | M | Yes | `TransmitFrame()` in S1-S3, assigns V(S), enforces the sliding window (`ErrFOPWindowFull`), starts T1. `ErrFOPNotActive` elsewhere. |
| COP-29 | BC frame transmission | 5.1 | M | Yes | BC frames carried by the Initiate-with-Unlock / Set V(R) directives; served first by `GetNextFrame()` (N(S)=0) and retransmitted under T1. |
| COP-30 | BD frame transmission | 5.1 | M | Yes | `TransmitBDFrame()`, expedited frames bypass sequence control, served ahead of AD frames, allowed in any state. |

### Table A-6: FARM-1 (section 6)

| Item | Description | Reference | Status | Support | Notes |
|---|---|---|---|---|---|
| COP-31 | S1, Open | 6.1 | M | Yes | `FARMOpen`. |
| COP-32 | S2, Wait | 6.1 (E2/E10) | M | Yes | `FARMWait`. Entered when an in-sequence frame arrives with no buffer free (frame discarded, Wait and Retransmit flags set); left when `ReleaseBuffer()` frees one. Buffer accounting configured with `SetBuffers(n)`; disabled by default. |
| COP-33 | S3, Lockout | 6.1 (E5) | M | Yes | `FARMLockout`. Entered when N(S) falls outside both windows. The Retransmit flag is left untouched on entry. Cleared only by a BC Unlock. |
| COP-34 | Positive/negative window | 6.1.5 (E1-E5) | M | Yes | W is even (clamped/rounded by `NewFARM`); PW = NW = W/2. N(S)=V(R): accept (E1) or Wait-discard (E2). V(R) < N(S) <= V(R)+PW-1: discard, set Retransmit (E3). V(R)-NW <= N(S) < V(R): duplicate, discarded silently, no flags, no lockout (E4). Otherwise: Lockout (E5). |
| COP-35 | Type-BC identification | 232.0-B-4 4.1.2.3 | M | Yes | BC = Bypass=1 AND Control Command=1. Bypass=0 with CC=1 is rejected as an invalid frame type. Type-BD (Bypass=1, CC=0) is always accepted. |
| COP-36 | Unlock control command | 4.1.3.3 / E7 | M | Yes | Data field `0x00`: clears Lockout, Wait, and Retransmit; V(R) is NOT modified. |
| COP-37 | Set V(R) control command | 4.1.3.3 / E8 | M | Yes | Data field `0x82 0x00 <V(R)>`: sets V(R) from the directive payload (not from the frame sequence number) and clears Retransmit. In Lockout it only increments the FARM-B counter. Malformed contents are discarded (E9, `ErrInvalidControlCommand`). |
| COP-38 | FARM-B counter | 6.1.7 | M | Yes | Incremented (mod 4) for EVERY accepted Type-B frame, BD data frames and BC control commands alike. |
| COP-39 | CLCW generation | 6.2 | M | Yes | `GenerateCLCW()` reports Lockout/Wait/Retransmit flags, FARM-B counter, and V(R). |

### Table A-7: CLCW Format (section 4.2)

| Item | Description | Reference | Status | Support | Notes |
|---|---|---|---|---|---|
| COP-40 | CLCW field layout | 4.2 | M | Yes | `CLCW` struct, 4 bytes, all 12 fields (Control Word Type, Version, Status, COP in Effect, VCID, No RF Available, No Bit Lock, Lockout, Wait, Retransmit, FARM-B, Report Value) with bit-exact `Encode()`/`Decode()` and validation. |

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Not Supported |
|---|---|---|---|
| Mandatory (M) | 40 | 40 | 0 |
| **Total** | **40** | **40** | **0** |

### Declared Simplifications

The event/action tables of CCSDS 232.1-B-2 are followed as closely as practical for a library API. Known deviations:

| Area | Deviation |
|---|---|
| T1 timer | Caller-driven (`Tick`), not wall-clock; a T1 initial of 0 disables it. The standard assumes a real timer; the mission must supply the tick source. |
| S5 confirmation | The BC frame is confirmed by observing the CLCW (Lockout cleared, or Report Value matching the pinned V(R)) rather than by tracking the FARM-B counter delta. |
| Wait_Queue | Higher-layer flow control is a bounded queue (`TransmitFrame` + window check) rather than the standard's one-FDU Wait_Queue with Accept/Reject signals. |
| S4 dirty CLCWs | In S4, CLCWs with Wait or Retransmit set are ignored (initialisation keeps waiting under T1) rather than raising an immediate alert. |

### Key Implementations

| Area | Items | Implementation |
|---|---|---|
| FOP-1 states | COP-1-8 | S1-S6, SS 0-4, alerts with reason codes, purge-on-alert. |
| FOP-1 directives | COP-9-19 | All 11 directives with parameter validation. |
| Timer/limit | COP-20-22 | Caller-driven T1, transmission limit/count, TT0/TT1. |
| CLCW processing | COP-23-27 | Lockout, N(R) validity, ack, retransmit/wait flag handling. |
| Transmit paths | COP-28-30 | AD sliding window, BC via initiate directives, BD expedited. |
| FARM-1 | COP-31-39 | Positive/negative windows, buffer-driven Wait, spec-compliant BC decoding, FARM-B on all Type-B frames. |
| CLCW | COP-40 | Bit-exact codec. |
