# Communications Operation Procedure-1

> CCSDS 232.1-B-2 — Communications Operation Procedure-1

## Overview

The Communications Operation Procedure-1 (COP-1) is the **reliability protocol** for telecommand delivery in space missions. It ensures that TC Transfer Frames arrive at the spacecraft correctly, in order, and without gaps — even over a lossy uplink.

COP-1 is a **sliding window protocol**, conceptually similar to TCP's reliability mechanism but designed for the unique constraints of space communication: long propagation delays (seconds to hours), simplex or near-simplex links, and the absolute necessity that commands arrive correctly (a corrupted command could damage or destroy a spacecraft).

### Where COP-1 Fits

```
+-----------------------------------------+
|  TC Space Data Link Protocol (TCDL)     |
|  Builds TC Transfer Frames              |
+-----------------------------------------+
|  COP-1                                  |  <-- This protocol
|  Reliable delivery: sequencing,         |
|  acknowledgment, retransmission         |
+-----------------------------------------+
|  TC Sync & Channel Coding               |
|  CLTU construction, BCH encoding        |
+-----------------------------------------+
|  Physical Layer (RF uplink)             |
+-----------------------------------------+
```

COP-1 sits between the TC Data Link Protocol and the sync layer. It does not modify frame content — it manages the **sequence numbering**, **acknowledgment**, and **retransmission** of TC frames.

### Key Characteristics

- **Sliding window**: A configurable window (typically 10) limits how many unacknowledged frames can be outstanding.
- **Split architecture**: FOP-1 runs on the ground, FARM-1 runs on the spacecraft. They communicate via the CLCW embedded in the TM return link.
- **Two frame types**: Type-A frames are sequence-controlled (reliable). Type-B frames bypass sequencing (best-effort, for emergencies).
- **Lockout recovery**: If the protocol detects an unrecoverable sequence error, it enters lockout. Recovery requires an explicit ground command.

## Protocol Architecture

COP-1 has three components that work together across the space link:

```
Ground Station                              Spacecraft
+--------------------+                      +--------------------+
|  FOP-1             |   TC Uplink          |  FARM-1            |
|  - Assigns N(S)    | ──────────────────>  |  - Checks N(S)     |
|  - Manages window  |  TC Transfer Frames  |  - Accepts/rejects |
|  - Retransmits     |                      |  - Updates V(R)    |
|                    |   TM Return Link     |                    |
|  - Processes CLCW  | <──────────────────  |  - Generates CLCW  |
|  - Acknowledges    |  CLCW in TM OCF      |                    |
+--------------------+                      +--------------------+
```

### The Feedback Loop

1. **Ground sends** a TC frame with sequence number N(S).
2. **Spacecraft validates** N(S) against its expected value V(R).
3. **Spacecraft reports** its state via a CLCW in the next TM frame's OCF field.
4. **Ground processes** the CLCW to learn what was acknowledged.
5. **Ground retransmits** if the CLCW indicates a problem.

This feedback loop operates continuously. The CLCW arrives on every TM frame (typically many per second), so the ground gets frequent status updates even though the round-trip delay may be significant.

## FOP-1 (Flight Operations Procedure)

FOP-1 runs on the **ground side**. It is responsible for:

- Assigning sequence numbers V(S) to outgoing Type-A frames
- Tracking which frames have been acknowledged
- Retransmitting frames when requested by the spacecraft
- Detecting lockout conditions

### State Machine

| State | Name | Description |
|-------|------|-------------|
| S1 | Active | Normal operation. Accepting and transmitting frames. |
| S6 | Initial | Not started or lockout detected. Must be re-initialized. |

### Key Variables

| Variable | Name | Description |
|----------|------|-------------|
| V(S) | Transmitter Frame Sequence Number | Next sequence number to assign. Increments with each Type-A frame. |
| N(N)R | Receiver Frame Sequence Number (from CLCW) | Last acknowledged sequence number. All frames with N(S) < N(N)R are confirmed received. |
| FW | Window Width | Maximum number of unacknowledged frames allowed. |

### Sliding Window

The sliding window controls how many frames can be "in flight" — sent but not yet acknowledged:

```
    N(N)R                V(S)
      |                    |
      v                    v
  ... [ack'd] [ack'd] [sent] [sent] [sent] [not yet sent] ...
               |<---- window (FW) ---->|
```

- Frames before N(N)R have been acknowledged.
- Frames between N(N)R and V(S) have been sent but await acknowledgment.
- If V(S) - N(N)R >= FW, the window is full — no more frames can be sent until an acknowledgment arrives.

## FARM-1 (Frame Acceptance and Reporting Mechanism)

FARM-1 runs on the **spacecraft side**. It is responsible for:

- Validating incoming frame sequence numbers
- Accepting or rejecting frames based on sequence state
- Reporting its state via the CLCW

### State Machine

| State | Name | Description |
|-------|------|-------------|
| S1 | Open | Normal operation. Accepting in-sequence frames. |
| S2 | Wait | Buffer full or resource constraint. |
| S3 | Lockout | Unrecoverable sequence error detected. Requires ground intervention. |

### Key Variables

| Variable | Name | Description |
|----------|------|-------------|
| V(R) | Receiver Frame Sequence Number | Next expected sequence number. Incremented when an in-sequence frame arrives. |
| W | Window Width | Positive sliding window width. Must match FOP-1's window. |

### Frame Acceptance Rules

When a Type-A data frame arrives with sequence number N(S):

```
                V(R)            V(R) + W
                 |                 |
    ... [past] [ | in-sequence | within window | outside ] ...
                 |      ^           ^                ^
                 |   Accept     Reject+Retransmit  Lockout
```

| Condition | Action |
|-----------|--------|
| N(S) == V(R) | **Accept**. Increment V(R). Clear retransmit flag. |
| V(R) < N(S) < V(R) + W | **Reject**. Set retransmit flag. Frame is within the window but out of order — likely a frame was lost. |
| N(S) outside window | **Lockout**. Reject frame. Enter lockout state. Something is seriously wrong — ground must send an unlock command to recover. |

Type-B frames **always** bypass this check and are accepted unconditionally.

## CLCW (Communications Link Control Word)

The CLCW is a 4-byte (32-bit) status word that FARM-1 generates and inserts into the TM return link's Operational Control Field (OCF). It is the spacecraft's way of telling the ground "here is my current state."

### Structure

```
Byte 0: [CWType:1][Version:2][Status:3][COP:2]
Byte 1: [VCID:6][Reserved:2]
Byte 2: [NoRF:1][NoBitLock:1][Lockout:1][Wait:1][Retransmit:1][FARMB:2][spare:1]
Byte 3: [Report Value V(R):8]
```

### Critical Fields

| Field | Description |
|-------|-------------|
| **Report Value** | V(R) — the next sequence number FARM expects. All frames with N(S) < V(R) are implicitly acknowledged. |
| **Lockout Flag** | FARM is in lockout. Ground must send an unlock control command. |
| **Retransmit Flag** | FARM received an out-of-sequence frame within the window. Ground should retransmit from the reported V(R). |
| **Wait Flag** | Spacecraft cannot accept more frames (buffer full). |

### The CLCW as an Implicit ACK

The CLCW does not explicitly list which frames were received. Instead, V(R) serves as a **cumulative acknowledgment**: "I have received everything up to (but not including) frame V(R)."

This is efficient: a single 8-bit value acknowledges an entire sliding window's worth of frames. And because the CLCW rides on every TM frame, the ground gets this status update frequently without any dedicated uplink bandwidth.

## Lockout and Recovery

Lockout is COP-1's safety mechanism. When FARM-1 receives a frame whose sequence number is completely outside the expected window, something has gone seriously wrong — the ground and spacecraft sequence counters have diverged beyond recovery.

**Causes:**
- Ground system restarted and lost track of V(S)
- Severe uplink noise corrupted many frames
- Configuration error (wrong VC, wrong spacecraft)

**Recovery:**
1. Ground detects lockout via the Lockout flag in the CLCW.
2. Ground sends a **control command** (Type-A frame with ControlCommandFlag=1) containing the desired new V(R) value.
3. FARM-1 processes the control command: clears lockout, resets to Open state, sets V(R) to the specified value.
4. Ground re-initializes FOP-1 with matching V(S).
5. Normal operation resumes.

If the normal unlock mechanism cannot be used (COP-1 itself is broken), the ground can send the unlock as a Type-B frame, which bypasses all sequence checking.

## Design Rationale

**Why a sliding window?** A single-frame stop-and-wait protocol would waste the link during the round-trip propagation delay (which can be seconds for GEO, minutes for lunar, hours for deep space). The sliding window allows multiple frames to be in flight simultaneously, utilizing the full uplink bandwidth.

**Why split FOP/FARM?** The ground side (FOP) has plentiful computing resources and can manage complex retransmission logic. The spacecraft side (FARM) needs to be simple and radiation-hardened — it only needs to check one sequence number and generate a 4-byte status word. This asymmetry mirrors the asymmetry of space system resources.

**Why lockout instead of automatic recovery?** In space operations, automated recovery from severe protocol errors is dangerous. A corrupted command that reaches the spacecraft could cause physical damage. Lockout forces a human operator to assess the situation and explicitly restart the protocol, providing a safety checkpoint.

**Why no negative acknowledgment (NAK)?** The retransmit flag in the CLCW serves as an implicit NAK. Combined with V(R) telling the ground exactly where to restart, explicit per-frame NAKs are unnecessary and would complicate the spacecraft-side implementation.

**Why does the CLCW ride on TM frames?** Using the existing TM downlink for feedback avoids dedicating uplink bandwidth to acknowledgments. Since TM frames are transmitted continuously (even as idle frames), the CLCW arrives at a predictable rate. This piggyback approach is simple and bandwidth-efficient.

## Reference

- [CCSDS 232.1-B-2](https://public.ccsds.org/Pubs/232x1b2.pdf) — Communications Operation Procedure-1 (Blue Book)
- [CCSDS 232.0-B-4](https://public.ccsds.org/Pubs/232x0b4.pdf) — TC Space Data Link Protocol (Blue Book)
- [CCSDS 230.1-G-2](https://public.ccsds.org/Pubs/230x1g2.pdf) — TC Synchronization and Channel Coding Summary (Green Book)
