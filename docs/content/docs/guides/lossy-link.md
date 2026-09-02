---
title: Handle a lossy link
short: Lossy link
description: What happens when frames are dropped and corrupted, and how the protocols cope.
order: 3
---

Real links lose frames and corrupt bits. CCSDS has four separate mechanisms for that, working at different layers, and this shows all four doing their job at once.

The complete program is [`examples/lossylink`](https://github.com/ravisuhag/astro/tree/main/examples/lossylink). Run it:

```bash
go run ./examples/lossylink/
```

Output varies run to run — the link drops and corrupts at random.

## The four mechanisms

| Layer | Mechanism | Catches |
|---|---|---|
| [Coding](/protocols/coding/tmsc) | Reed-Solomon | Bit errors, up to 16 bad bytes per codeword |
| [Frame](/protocols/data-link/tmdl) | CRC-16 | Anything RS could not fix |
| [Frame](/protocols/data-link/tmdl) | MC and VC counters | Frames that never arrived |
| [Frame](/protocols/data-link/tmdl) | First Header Pointer | Where to start reading again after a gap |

They are layered on purpose. RS fixes what it can. CRC rejects what RS could not. Counters notice what is missing. The FHP gets the packet stream back in sync.

## The chain

Smaller frames than [the downlink guide](/docs/guides/downlink), to make packets span more often:

```
Frame (128 B) -> RS encode (128->160 B) -> randomize -> ASM -> CADU (259 B)
```

and backwards on the ground.

## What the link does to it

```
Spacecraft: generating 20 telemetry packets...
  Sent: 20 packets (2233 bytes) in 19 frames
  Each frame: 128 bytes -> RS(255 bytes) -> CADU(259 bytes)

RF Link statistics:
  Delivered intact:  16 frames
  Dropped (lost):    1 frames
  Corrupted:         2 frames
```

One frame vanished. Two arrived with bit errors.

## What the ground makes of it

```
Ground Station: processing received CADUs...

  [RS OK] Corrected 1 symbol errors
  [GAP] MC counter gap: 1 frame(s) lost
  [RS OK] Corrected 2 symbol errors

Frame reception summary:
  Good frames accepted:  18 / 19 transmitted
  RS corrections:        3 symbols across all frames
  RS failures:           0 (uncorrectable, >16 errors)
  CRC rejects:           0
  MC frame gaps:         1 (frames lost in transit)
```

Both corrupted frames were **repaired**, not discarded. Reed-Solomon fixed three bad symbols across them and handed up clean frames. The CRC never had to reject anything.

The dropped frame is a different problem. Nothing can fix a frame that did not arrive — but the Master Channel Frame Count jumps, so the receiver knows exactly that one is gone.

## Getting the packet stream back

```
Packet recovery (FHP-based resync after gaps):
  Recovered packet: APID=100 Seq=0 Size=10 bytes
  ...
  Recovered packet: APID=100 Seq=8 Size=115 bytes
  Recovered packet: APID=100 Seq=11 Size=35 bytes
  ...
```

Sequence 8, then 11. Packets 9 and 10 were inside the frame that vanished.

The receiver does not stall on them. It throws away the partial packet it was accumulating, waits for the next frame with a usable First Header Pointer, and starts reading from that offset. That is the whole resync mechanism.

```
=== Results ===
  Packets sent:       20
  Packets recovered:  18 (90%)
  Packets lost:       2 (spanned a dropped/corrupted frame)
  CRC failures:       0 (partial packet from lost frame)
```

One lost frame cost exactly the packets that lived in it. Everything after resynchronized cleanly.

## Why each layer is needed

**Why not just retransmit?** The round trip is minutes to hours. Asking again does not finish in time. Everything on a downlink is forward error correction and graceful degradation.

**Why RS *and* a CRC?** RS corrects up to 16 bad symbols per codeword. Past that it can fail loudly, or — worse — "correct" into wrong data. The frame CRC is the independent check that catches both cases.

**Why two counters?** The Master Channel count says the spacecraft lost a frame. The Virtual Channel count says *this* channel lost one. An MC gap with no VC gap means the missing frame belonged to another channel, and your stream is intact.

**Why does the FHP matter so much?** Frames are fixed length, packets are not, so packets span. After losing a frame the receiver holds a fragment of a packet it can never complete. Without the FHP it would have no idea where the next real packet header starts, and the stream would be dead. With it, recovery costs exactly the packets that were in the lost frame.

## Tuning it

**Interleaving depth** spreads a burst error across codewords. Depth 5 with RS(255,223) means a 30-byte burst becomes 6 bytes in each of five codewords — all correctable, where one codeword alone would have failed.

**RS(255,223) or RS(255,239).** The first corrects 16 symbols and costs 32 bytes of parity per codeword. The second corrects 8 and costs 16. Deep space takes the stronger code; a good LEO link often does not need it.

**Frame length** trades overhead against loss granularity. Shorter frames lose less when one goes missing, but pay the header and parity cost more often.

## Next

- [TMSC protocol page](/protocols/coding/tmsc) — Reed-Solomon, interleaving, virtual fill
- [TM protocol page](/protocols/data-link/tmdl) — the FHP and the frame counters
- [Build an uplink](/docs/guides/uplink) — where loss *is* recoverable, by asking again
