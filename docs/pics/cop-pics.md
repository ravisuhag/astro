# PICS PROFORMA FOR COMMUNICATIONS OPERATION PROCEDURE-1

## Conformance Statement for `pkg/cop` — CCSDS 232.1-B-2

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 19/03/2026 |
| PICS Serial Number | ASTRO-COP-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/cop |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing CCSDS COP-1 reliable frame delivery. Three components: FOP-1 (ground-side frame transmission with sliding window), FARM-1 (spacecraft-side frame acceptance with window validation), and CLCW (status reporting via TM return link). Thread-safe implementations with mutex protection. |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/cop (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 232.1-B-2 (Communications Operation Procedure-1, Blue Book, Issue 2, October 2019) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE — Non-supported optional capabilities are identified in section A2.2 with explanations.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: COP-1 Components

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| COP-1 | FOP-1 (Flight Operations Procedure) | 4 | M | Yes | `FOP` struct implements FOP-1 ground-side logic: state machine (Active/Initial), V(S) management, sliding window with configurable width, sent queue with retransmission support, CLCW processing. Thread-safe via mutex. |
| COP-2 | FARM-1 (Frame Acceptance and Reporting Mechanism) | 5 | M | Yes | `FARM` struct implements FARM-1 spacecraft-side logic: state machine (Open/Wait/Lockout), V(R) tracking, window-based frame acceptance, Type-B unconditional acceptance, control command processing, CLCW generation. Thread-safe via mutex. |
| COP-3 | CLCW (Communications Link Control Word) | 4.2 | M | Yes | `CLCW` struct — 4 bytes (32 bits). Full encode/decode with all fields: ControlWordType, Version, StatusField, COPInEffect, VirtualChannelID, NoRFAvailableFlag, NoBitLockFlag, LockoutFlag, WaitFlag, RetransmitFlag, FARMBCounter, ReportValue. |

### Table A-2: FOP-1 State Machine

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| COP-4 | S1 — Active State | 4.3 | M | Yes | `FOPActive` state. FOP accepts frames for transmission, assigns V(S), manages sliding window. |
| COP-5 | S6 — Initial State | 4.3 | M | Yes | `FOPInitial` state. FOP is not started or has detected lockout. Must be re-initialized via `Initialize()`. |
| COP-6 | Initialize Directive | 4.4 | M | Yes | `FOP.Initialize(initialVS)` sets V(S), clears queues, transitions to Active state. |
| COP-7 | Transfer Frame Acceptance | 4.5 | M | Yes | `FOP.TransmitFrame(encodedFrame)` assigns V(S), increments V(S), adds to sent queue. Returns `ErrFOPWindowFull` if window exhausted. Returns `ErrFOPLockout` if not in Active state. |
| COP-8 | Sliding Window Management | 4.6 | M | Yes | Window check: `(V(S) - N(N)R) & 0xFF >= windowWidth`. Outstanding frame count tracked. Window width configurable at construction. |

### Table A-3: FOP-1 Variables

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| COP-9 | V(S) — Transmitter Frame Sequence Number | 4.3.1 | M | 0-255 | Yes | `FOP.vs` — 8-bit counter. Assigned to each outgoing Type-A frame. Incremented after each transmission. Accessible via `FOP.VS()`. |
| COP-10 | N(N)R — Last Acknowledged Sequence Number | 4.3.2 | M | 0-255 | Yes | `FOP.nnr` — updated from CLCW Report Value during `ProcessCLCW()`. All frames with N(S) < N(N)R are acknowledged. |
| COP-11 | FW — Window Width | 4.3.3 | M | 1-255 | Yes | `FOP.windowWidth` — configurable at construction via `NewFOP()`. Must match FARM-1 window width. |

### Table A-4: FOP-1 CLCW Processing

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| COP-12 | CLCW Processing | 4.7 | M | Yes | `FOP.ProcessCLCW(clcw)` processes received CLCW: updates N(N)R from ReportValue, acknowledges frames with N(S) < V(R) using modular arithmetic, re-queues unacknowledged frames for retransmission when RetransmitFlag is set, enters Initial state on Lockout. |
| COP-13 | Frame Acknowledgment | 4.7.1 | M | Yes | Sent queue pruned: frames with `(V(R) - N(S)) & 0xFF` in range (0, 128] are acknowledged and removed. |
| COP-14 | Retransmission | 4.7.2 | M | Yes | When CLCW RetransmitFlag is set and sent queue is non-empty, all unacknowledged frames are re-queued in the wait queue for retransmission via `GetNextFrame()`. |
| COP-15 | Lockout Detection | 4.7.3 | M | Yes | When CLCW LockoutFlag is set, FOP transitions to Initial state and returns `ErrFOPLockout`. |

### Table A-5: FARM-1 State Machine

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| COP-16 | S1 — Open State | 5.3 | M | Yes | `FARMOpen` state. FARM accepts in-sequence frames, rejects out-of-sequence. |
| COP-17 | S2 — Wait State | 5.3 | M | Yes | `FARMWait` state. FARM wait flag tracked in `FARM.wait`. |
| COP-18 | S3 — Lockout State | 5.3 | M | Yes | `FARMLockout` state. All Type-A data frames rejected. Returns `ErrFARMLockout`. Requires ground unlock via control command. |

### Table A-6: FARM-1 Frame Processing

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| COP-19 | Type-A Frame — In Sequence | 5.4.1 | M | Yes | N(S) == V(R): accepted, V(R) incremented, retransmit flag cleared. Returns `(true, nil)`. |
| COP-20 | Type-A Frame — Within Window | 5.4.2 | M | Yes | N(S) within (V(R), V(R)+W): rejected, retransmit flag set. Returns `(false, ErrFARMReject)`. Window check uses modular arithmetic: `diff := (N(S) - V(R)) & 0xFF; diff > 0 && diff < W`. |
| COP-21 | Type-A Frame — Outside Window | 5.4.3 | M | Yes | N(S) outside window: rejected, lockout entered, retransmit cleared. Returns `(false, ErrFARMLockout)`. |
| COP-22 | Type-B Frame | 5.5 | M | Yes | Bypass frames always accepted. FARM-B counter incremented for Type-B data frames (not control). Returns `(true, nil)`. |
| COP-23 | Control Command | 5.6 | M | Yes | `processControlCommand()`: clears lockout, resets to Open state, sets V(R) to frame sequence number, clears wait and retransmit flags. Used for unlock and Set-V(R) directives. |

### Table A-7: FARM-1 Variables

| Item | Description | Reference | Status | Values Allowed | Support | Notes |
|------|-------------|-----------|--------|----------------|---------|-------|
| COP-24 | V(R) — Receiver Frame Sequence Number | 5.3.1 | M | 0-255 | Yes | `FARM.vr` — 8-bit counter. Next expected frame sequence number. Incremented on in-sequence acceptance. Accessible via `FARM.VR()`. |
| COP-25 | W — Window Width | 5.3.2 | M | 1-255 | Yes | `FARM.windowWidth` — configurable at construction via `NewFARM()`. |
| COP-26 | FARM-B Counter | 5.5.1 | M | 0-3 | Yes | `FARM.farmBCounter` — 2-bit counter. Incremented for each Type-B data frame accepted. Wraps at 4. Reported in CLCW. |

### Table A-8: CLCW Fields

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| COP-27 | Control Word Type | 4.2.1 | M | Yes | `CLCW.ControlWordType` — 1 bit. Always 0 for CLCW. Validated in `Validate()`. |
| COP-28 | CLCW Version Number | 4.2.2 | M | Yes | `CLCW.Version` — 2 bits. Always 00. Validated in `Validate()`. |
| COP-29 | Status Field | 4.2.3 | M | Yes | `CLCW.StatusField` — 3 bits. Mission-specific status. |
| COP-30 | COP in Effect | 4.2.4 | M | Yes | `CLCW.COPInEffect` — 2 bits. Set to 1 (COP-1) by `FARM.GenerateCLCW()`. |
| COP-31 | Virtual Channel Identifier | 4.2.5 | M | Yes | `CLCW.VirtualChannelID` — 6 bits. Set from FARM's VCID by `GenerateCLCW()`. |
| COP-32 | No RF Available Flag | 4.2.6 | M | Yes | `CLCW.NoRFAvailableFlag` — 1 bit. |
| COP-33 | No Bit Lock Flag | 4.2.7 | M | Yes | `CLCW.NoBitLockFlag` — 1 bit. |
| COP-34 | Lockout Flag | 4.2.8 | M | Yes | `CLCW.LockoutFlag` — 1 bit. Set when FARM is in Lockout state. |
| COP-35 | Wait Flag | 4.2.9 | M | Yes | `CLCW.WaitFlag` — 1 bit. Set when FARM is in Wait state. |
| COP-36 | Retransmit Flag | 4.2.10 | M | Yes | `CLCW.RetransmitFlag` — 1 bit. Set when FARM requests retransmission. |
| COP-37 | FARM-B Counter | 4.2.11 | M | Yes | `CLCW.FARMBCounter` — 2 bits. Type-B frame acceptance counter. |
| COP-38 | Report Value | 4.2.12 | M | Yes | `CLCW.ReportValue` — 8 bits. V(R): next expected frame sequence number. |

### Table A-9: CLCW Encoding

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| COP-39 | CLCW Encode | 4.2 | M | Yes | `CLCW.Encode()` packs all fields into 4-byte big-endian representation per CCSDS bit layout. Validates before encoding. |
| COP-40 | CLCW Decode | 4.2 | M | Yes | `CLCW.Decode(data)` parses 4-byte slice into CLCW struct. Validates after decoding. Returns `ErrDataTooShort` if input < 4 bytes. |
| COP-41 | CLCW Generation by FARM | 5.7 | M | Yes | `FARM.GenerateCLCW()` produces a `*CLCW` reflecting current FARM-1 state: ControlWordType=0, Version=0, COPInEffect=1, VCID from FARM, all flags from FARM state, ReportValue=V(R). |

### Table A-10: Optional Capabilities

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| COP-42 | FOP-1 Timer-Based Retransmission | 4.8 | O | No | Timer-based retransmission not implemented. Retransmission triggered by CLCW RetransmitFlag only. |
| COP-43 | FOP-1 Multiple VCs | 4.9 | O | Yes | Multiple FOP-1 instances can be created for different VCIDs. Each instance is independent. |
| COP-44 | FARM-1 Sliding Window Width Configuration | 5.8 | O | Yes | Window width configurable at construction via `NewFARM(vcid, windowWidth)`. |

---

## A2.3 CONFORMANCE SUMMARY

### Overall Statistics

| Category | Total Items | Supported | Not Supported |
|----------|-------------|-----------|---------------|
| Mandatory (M) | 41 | 41 | 0 |
| Optional (O) | 3 | 2 | 1 |
| **Total** | **44** | **43** | **1** |

### Non-Conformances (Mandatory Items Not Supported)

None. All 41 mandatory items are fully supported.

### Non-Supported Optional Items

| Item | Description | Reason |
|------|-------------|--------|
| COP-42 | FOP-1 Timer-Based Retransmission | No timer-based retransmission. Retransmission is triggered by CLCW RetransmitFlag processing only. Timer support can be added at the application layer. |

### Fully Supported Mandatory Items

All 41 mandatory items (COP-1 through COP-41) are supported. Key implementations:

| Area | Items | Implementation |
|------|-------|----------------|
| COP-1 Components | COP-1–3 | `FOP`, `FARM`, `CLCW` structs with full encode/decode. |
| FOP-1 State Machine | COP-4–8 | Active/Initial states, Initialize directive, frame acceptance with window check. |
| FOP-1 Variables | COP-9–11 | V(S), N(N)R, FW — all tracked and enforced. |
| FOP-1 CLCW Processing | COP-12–15 | Acknowledgment, retransmission, lockout detection. |
| FARM-1 State Machine | COP-16–18 | Open/Wait/Lockout states with transitions. |
| FARM-1 Frame Processing | COP-19–23 | In-sequence acceptance, window rejection, lockout, Type-B bypass, control commands. |
| FARM-1 Variables | COP-24–26 | V(R), W, FARM-B counter. |
| CLCW | COP-27–41 | All 12 fields, encode/decode, FARM-1 generation. |
