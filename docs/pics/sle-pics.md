# PICS PROFORMA FOR SPACE LINK EXTENSION TRANSFER SERVICES

## Conformance Statement for `pkg/sle` — CCSDS 913.1-B-2, 911.1-B-5, 911.2-B-4, 911.5-B-4, 912.1-B-5

---

## A2.1 GENERAL INFORMATION

### A2.1.1 Identification of PICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 23/08/2026 |
| PICS Serial Number | ASTRO-SLE-PICS-001 |
| System Conformance Statement Cross-Reference | This document |

### A2.1.2 Identification of Implementation Under Test (IUT)

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/sle |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library implementing the ISP1 transport and the four SLE transfer services. The **user role is implemented in full**; the **provider role is partial** and is meant for testing and prototyping, not for running a ground station. The package owns no goroutines, no timers and no sockets beyond the TML reader and writer: the caller drives every machine and supplies the clock. |

### A2.1.3 Identification of Supplier

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub — github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/sle (Go package) |
| System Name(s) | Astro |

### A2.1.4 Identification of Specification

| Field | Value |
|---|---|
| Specification | CCSDS 913.1-B-2 (ISP1), 911.1-B-5 (RAF), 911.2-B-4 (RCF), 911.5-B-4 (ROCF), 912.1-B-5 (FCLTU) |
| Have any exceptions been required? | Yes [X] No [ ] |

NOTE — Non-supported and partly supported capabilities are identified in
section A2.3 with explanations. The provider role is partial by design and is
marked so on every row it touches.

---

## A2.2 REQUIREMENTS LIST

### Table A-1: Transport Mapping Layer (CCSDS 913.1-B-2 §3.3)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SLE-1 | TML message framing | 3.3.2 | M | Yes | `Message` with an eight-octet header. `DecodeMessage`, `ReadMessage`, `WriteMessage`. |
| SLE-2 | Context message | 3.3.2.2.4 | M | Yes | `ContextMessage`, a fixed 12-octet body carrying 'ISP1', version 1, the heartbeat interval and the dead factor. |
| SLE-3 | Heartbeat message | 3.3.2.2.5 | M | Yes | `HeartbeatMessage()`. An empty body is required and enforced. |
| SLE-4 | SLE PDU message | 3.3.2.2.3 | M | Yes | `MessageSLEPDU` carrying a BER-encoded PDU. |
| SLE-5 | Heartbeat timing | 3.3.3 | M | Partial | `Association.HeartbeatDue`, `PeerDead` and `NextHeartbeat` report when a heartbeat is owed and when the peer has gone silent. The library runs no timer: the caller acts on the hint. |
| SLE-6 | Message size limit | — | O | Yes | `DefaultMaxMessageSize` (16 MiB), overridable per read. Not a spec requirement; a bound on hostile length fields. |

### Table A-2: BER encoding (ITU-T X.690, as SLE uses it)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SLE-7 | Definite-length form | X.690 8.1.3 | M | Yes | Short and long forms, encoding and decoding. |
| SLE-8 | Indefinite-length form | X.690 8.1.3.6 | O | No | `ErrIndefiniteLength`. SLE PDUs do not need it; accepting it would complicate every decoder for no gain. |
| SLE-9 | INTEGER, OCTET STRING, VisibleString, NULL, SEQUENCE | X.690 8 | M | Yes | `AppendInteger`, `AppendOctetString`, `AppendVisibleString`, `AppendNull`, `AppendSequence`. |
| SLE-10 | Context-specific tags | X.690 8.1.2 | M | Yes | Multi-octet tag numbers supported, which SLE needs for tags 100 to 104. |

### Table A-3: Credentials and authentication (CCSDS 913.1-B-2 §3.1.2)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SLE-11 | ISP1 credentials | 3.1.2 | M | Yes | `Credentials` with time, random number and a SHA-256 digest. |
| SLE-12 | SHA-256 digest | 3.1.2.3 | M | Yes | 32 octets. `DigestSizeSHA256`. Issue 2 replaced SHA-1; this implements Issue 2 only. |
| SLE-13 | Credential time window | 3.1.2.2.1 | M | Yes | `AcceptableDelay` on `AssociationConfig`. Zero disables the check. |
| SLE-14 | Unauthenticated associations | 3.1.2 | O | Yes | Leaving `Password` empty omits credentials. |
| SLE-15 | Credentials on service PDUs | 3.1.2 | M | Yes | `Association.MakeCredentials` and `CheckPeerCredentials`; the service machines stamp every outgoing PDU. |

### Table A-4: Association operations (common PDUs module)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SLE-16 | BIND invocation and return | 911.1-B-5 3.2 | M | Yes | `BindInvocation`, `BindReturn`, `Association.Bind`, `HandleBindInvocation`, `HandleBindReturn`. |
| SLE-17 | UNBIND invocation and return | 3.3 | M | Yes | `UnbindInvocation`, `UnbindReturn`, with the 'end' and 'suspend' reasons. |
| SLE-18 | PEER-ABORT | 3.11 | M | Yes | `PeerAbort` with the full diagnostic set. Sent on any PDU the state forbids. |
| SLE-19 | Service instance identifier | 3.2.2 | M | Yes | `ServiceInstanceIdentifier`, a sequence of attribute pairs. |
| SLE-20 | Version negotiation | 3.2.2 | M | Partial | The version number is carried and checked on BIND. The package implements version 5 semantics only; it does not fall back to an older version's PDU set. |

### Table A-5: Service state machine (911.1-B-5 §4.2, and the same in the other three)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SLE-21 | State 1 'unbound' | 4.2.1 | M | Yes | `ServiceUnbound`. |
| SLE-22 | State 2 'ready' | 4.2.1 | M | Yes | `ServiceReady`. |
| SLE-23 | State 3 'active' | 4.2.1 | M | Yes | `ServiceActive`. |
| SLE-24 | BIND valid only in state 1 | 3.2.1.6 | M | Yes | `ErrAlreadyBound` otherwise. |
| SLE-25 | UNBIND valid only in state 2 | 3.3.1.5 | M | Yes | `ErrAlreadyStarted` when still active. |
| SLE-26 | START valid only in state 2 | 3.4.1.7 | M | Yes | `ErrNotBound` or `ErrAlreadyStarted` otherwise. |
| SLE-27 | STOP valid only in state 3 | 3.5.1.3 | M | Yes | `ErrNotStarted` otherwise. |
| SLE-28 | Data transfer valid only in state 3 | 3.6.1.3 | M | Yes | Both halves refuse; an inbound transfer buffer outside state 3 draws a PEER-ABORT. |
| SLE-29 | Negative STOP return leaves state 3 | table 4-1 row 10 | M | Yes | Tested. |
| SLE-30 | Unexpected PDU → PEER-ABORT 'protocol error' | table 4-1 | M | Yes | `ErrUnexpectedPDU`, with the abort queued for sending. |
| SLE-31 | Return \<n\> timer | table 4-1 note 11 | M | Partial | Not run by the library. `ServiceUser.Outstanding()` reports which invocations are waiting so the caller can time them. |
| SLE-32 | Provider-initiated BIND | table 4-1 row 1 | O | No | Only the user initiates. A provider-initiated association is a ground-station arrangement this package does not model. |

### Table A-6: Return All Frames (CCSDS 911.1-B-5)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SLE-33 | RAF-START | 3.4 | M | Yes | `RAFStartInvocation`, `RAFStartReturn`, with both diagnostic alternatives and the conditional time range. |
| SLE-34 | RAF-STOP | 3.5 | M | Yes | `StopInvocation` and `Acknowledgement`, shared across services. |
| SLE-35 | RAF-TRANSFER-DATA | 3.6 | M | Yes | `RAFTransferDataInvocation`: earth receive time, antenna id, data link continuity, delivered frame quality, private annotation, frame. |
| SLE-36 | Transfer buffer | 3.1.9 | M | Yes | `RAFTransferBuffer`, a SEQUENCE OF frames and notifications. |
| SLE-37 | RAF-SYNC-NOTIFY | 3.7 | M | Yes | `SyncNotifyInvocation`, all four alternatives. Shared with RCF and ROCF, which define it identically. |
| SLE-38 | RAF-STATUS-REPORT | 3.8 | M | Yes | `RAFStatusReportInvocation`, both frame counters and all four lock statuses. |
| SLE-39 | RAF-SCHEDULE-STATUS-REPORT | 3.9 | M | Yes | `ScheduleStatusReportInvocation` and its return, with the 2-to-600 second cycle enforced. |
| SLE-40 | RAF-GET-PARAMETER | 3.10 | M | No | The tag decodes as an envelope; the `RafGetParameter` CHOICE is not built. A user cannot query provider parameters. |
| SLE-41 | Requested frame quality | 3.4.2 | M | Yes | `RequestedFrameQuality`: good only, erred only, all. |

### Table A-7: Return Channel Frames (CCSDS 911.2-B-4)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SLE-42 | RCF-START with GVCID | 3.4 | M | Yes | `RCFStartInvocation` carrying `RequestedGVCID`. No frame quality: RCF delivers only good frames. |
| SLE-43 | GvcId SEQUENCE | annex A | M | Yes | `GVCID` with the master-channel alternative. Ranges checked per frame version. |
| SLE-44 | Transfer frame version numbers | annex A | M | Yes | TM 0, AOS 1, USLP 12. The USLP value is the four-bit wire field `'1100'`, not the "version 4" the protocol is named for. |
| SLE-45 | RCF-TRANSFER-DATA | 3.6 | M | Yes | `RCFTransferDataInvocation`. No delivered-frame-quality field. |
| SLE-46 | RCF transfer buffer | 3.1.9 | M | Yes | `RCFTransferBuffer`. |
| SLE-47 | RCF-STATUS-REPORT | 3.8 | M | Yes | `RCFStatusReportInvocation`, one frame counter rather than RAF's two. |
| SLE-48 | RCF-GET-PARAMETER | 3.10 | M | No | As SLE-40. |

### Table A-8: Return Operational Control Fields (CCSDS 911.5-B-4)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SLE-49 | ROCF-START | 3.4 | M | Yes | `ROCFStartInvocation`: GVCID, control word type, update mode. |
| SLE-50 | ControlWordType CHOICE | annex A | M | Yes | `ControlWordType`: all control words, CLCW (optionally from one TC virtual channel), or not CLCW. |
| SLE-51 | Update mode | 3.4.2 | M | Yes | `UpdateContinuous` and `UpdateChangeBased`. |
| SLE-52 | ROCF-TRANSFER-DATA | 3.6 | M | Yes | `ROCFTransferDataInvocation` carrying the four-octet control field. `pkg/cop` decodes a CLCW from it. |
| SLE-53 | ROCF transfer buffer | 3.1.9 | M | Yes | `ROCFTransferBuffer`. |
| SLE-54 | ROCF-STATUS-REPORT | 3.8 | M | Yes | `ROCFStatusReportInvocation`: frames processed and OCFs delivered, counted separately. |
| SLE-55 | ROCF-GET-PARAMETER | 3.10 | M | No | As SLE-40. |

### Table A-9: Forward CLTU (CCSDS 912.1-B-5)

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SLE-56 | CLTU-START | 3.4 | M | Yes | `FCLTUStartInvocation` with the first CLTU identification; `FCLTUStartReturn` whose positive result carries the radiation window. |
| SLE-57 | CLTU-TRANSFER-DATA | 3.6 | M | Yes | `FCLTUTransferDataInvocation`: CLTU id, earliest and latest transmission time, delay, radiation notification request, the CLTU. |
| SLE-58 | CLTU identification sequence | 3.6.2.5 | M | Yes | `FCLTUUser` keeps the count and advances it only on acceptance. `FCLTUProvider` enforces the rule and quotes the expected number in a refusal. |
| SLE-59 | Buffer available reporting | 3.6.2 | M | Yes | `CltuBufferAvailable` on every TRANSFER-DATA return. The library reports the figure the caller supplies; it manages no buffer of its own. |
| SLE-60 | CLTU-ASYNC-NOTIFY | 3.7 | M | Yes | `FCLTUAsyncNotifyInvocation`, all nine notification alternatives, with `CltuLastProcessed` and `CltuLastOk`. |
| SLE-61 | CLTU-THROW-EVENT | 3.9 | M | Yes | `FCLTUThrowEventInvocation` and its return. The event identifier and qualifier are carried through unread: their meaning is in the service agreement. |
| SLE-62 | CLTU-STATUS-REPORT | 3.8 | M | Yes | `FCLTUStatusReportInvocation`: CLTUs received, processed and radiated, plus buffer and uplink status. |
| SLE-63 | FCLTU production status | annex A | M | Yes | `FCLTUProductionStatus`, four values. Deliberately a separate Go type from the return services' three-value `ProductionStatus`: the numbers disagree. |
| SLE-64 | CltuStatus values | annex A | M | Yes | `CltuStatus`: 0, 1, 2, 4, 5. Value 3 is FSP's 'acknowledged' and is rejected. |
| SLE-65 | CLTU-GET-PARAMETER | 3.10 | M | No | As SLE-40. |

### Table A-10: Delivery modes

| Item | Description | Reference | Status | Support | Notes |
|------|-------------|-----------|--------|---------|-------|
| SLE-66 | Return timely online | 911.1-B-5 1.2.2 | M | Partial | The mode is carried and its predicates reported. Discarding is the caller's: the machines hold one PDU and never queue. |
| SLE-67 | Return complete online | 1.2.2 | M | Partial | As above; backpressure is the caller's. |
| SLE-68 | Return offline | 1.2.2 | O | Partial | Modeled as configuration. `AllowsPastStartTime` and `AllowsPeriodicStatusReport` change what the user machine will ask for. No store is read; the caller supplies the data. |
| SLE-69 | Forward online | 912.1-B-5 1.2 | M | Partial | As the return online modes. |
| SLE-70 | Forward offline | 1.2 | O | Partial | Enum value and predicates only. |
| SLE-71 | Data discarded notification | table 4-1 row 14 | M | Partial | The notification is encodable and decodable. Deciding to discard is the caller's, because the buffer is. |

---

## A2.3 EXCEPTIONS AND LIMITATIONS

### Non-Supported Items

| Item | Description | Reason |
|------|-------------|--------|
| SLE-8 | Indefinite-length BER | No SLE PDU requires it. Supporting it would add a termination-scan path to every decoder for no gain and some risk. |
| SLE-32 | Provider-initiated BIND | Only the user initiates an association. A provider-initiated one is a ground-station arrangement outside what a library consumer needs. |
| SLE-40, 48, 55, 65 | GET-PARAMETER for all four services | The operation's envelope decodes, but the per-service parameter CHOICEs are not built. Each is a large enumeration of provider configuration, none of it needed to move data. |

### Partly Supported Items

| Item | Description | What is missing |
|------|-------------|-----------------|
| SLE-5, SLE-31 | Heartbeat and return timers | The library runs no clock. It reports when a heartbeat is due, when a peer looks dead and which invocations are outstanding; the caller's loop acts. This is deliberate — see the guide's "No goroutines, no timers". |
| SLE-20 | Version negotiation | Version 5 PDU semantics only. The number is carried and checked, but there is no fallback to an earlier version's PDU set. |
| SLE-66 to SLE-71 | Delivery modes | Modeled as configuration and predicates, not as a buffering engine. All buffering is caller-side. |
| The provider role, throughout | `ServiceProvider` and the four per-service providers | They answer a user, hold the three states correctly and let the caller push data. They do not manage multiple associations, size or release transfer buffers, run production, or enforce a service agreement. Use them to test a user or to prototype; do not run a ground station on them. |

### Fully Supported Mandatory Items

The user role is complete for all four services: BIND, UNBIND, PEER-ABORT,
START, STOP, SCHEDULE-STATUS-REPORT, and each service's data operations —
TRANSFER-DATA and SYNC-NOTIFY and STATUS-REPORT for the return services,
TRANSFER-DATA and THROW-EVENT and ASYNC-NOTIFY and STATUS-REPORT for FCLTU.
GET-PARAMETER is the one gap, and it queries configuration rather than moving
data.

| Area | Items | Implementation |
|------|-------|----------------|
| Transport | SLE-1–6 | `tml.go` — framing, context, heartbeat. |
| Encoding | SLE-7–10 | `ber.go` — the definite-length subset SLE uses. |
| Authentication | SLE-11–15 | `credentials.go`, `assoc.go`. |
| Association | SLE-16–20 | `bind.go`, `assoc.go`. |
| State machine | SLE-21–31 | `service.go`, shared by all four services. |
| RAF | SLE-33–41 | `raf.go`. |
| RCF | SLE-42–48 | `rcf.go`, `common.go` for the GVCID. |
| ROCF | SLE-49–55 | `rocf.go`. |
| FCLTU | SLE-56–65 | `fcltu.go`. |
| Delivery modes | SLE-66–71 | `delivery.go`. |
