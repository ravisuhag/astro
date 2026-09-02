---
title: Debug a real capture
short: Debug a capture
description: Someone hands you a binary file off an antenna. Work out what is in it.
order: 15
---

Every other guide builds a link and checks it works. This one is the job you actually get: a file off a recorder, no documentation, and a question about what is in it.

Nothing in a capture says what its parameters are. Frame length, error control, randomization, which virtual channels exist: all of it is mission configuration that never travels on the wire. So reading someone else's recording is a sequence of guesses, each one checked against what the file does.

## Get a capture to work on

```bash
go run ./examples/capture/
```

```
wrote examples/capture/capture.bin
  4037 octets
  15 CADUs, 1 frame(s) dropped, 1 corrupted
```

That is [`examples/capture`](https://github.com/ravisuhag/astro/tree/main/examples/capture), which writes a file that looks like a real recording: noise before the signal locks, two virtual channels multiplexed together, real Space Packets, one frame that never arrived, and one with bit errors in it.

The generator knows all the parameters. Everything below works them out from the file.

## 1. Find the frame boundaries

Start at the beginning:

```bash
xxd examples/capture/capture.bin | head -3
```

```
00000000: 58a9 6330 a177 0c62 1709 6bea 602f ff61  X.c0.w.b..k.`/.a
00000010: b1b9 e6be 497d 1b92 bac0 967a 8195 0ac4  ....I}.....z....
00000020: d0e9 28d5 8f45 accd fa92 986d 5f4c 5d50  ..(..E.....m_L]P
```

Rubbish, which is normal. A recorder writes whatever the demodulator produces and that starts before there is anything to demodulate.

What you are looking for is the Attached Sync Marker, `0x1ACFFC1D`. Do not hunt for it by eye, because it will not be on a tidy 16-octet boundary. `astro cadu sync` is the tool:

```bash
astro cadu sync --input bin --frame-len 1115 examples/capture/capture.bin
```

```
--- CADU #1 (offset 137, 1115 bytes) ---
  ASM: 1acffc1d
  Frame: 1111 bytes
--- CADU #2 (offset 1437, 1115 bytes) ---
  ASM: 1acffc1d
  Frame: 1111 bytes
```

1115 was a guess, and it is wrong. Two CADUs from a 4000-octet file, at offsets 137 and 1437, which is 1300 apart rather than 1115. Irregular spacing means the frame length is wrong.

1300 is a hint though: it is five times 260. Try that:

```bash
astro cadu sync --input bin --frame-len 260 examples/capture/capture.bin
```

```
--- CADU #1 (offset 137, 260 bytes) ---
--- CADU #2 (offset 397, 260 bytes) ---
--- CADU #3 (offset 657, 260 bytes) ---
...
```

Fifteen CADUs, evenly spaced 260 apart. That is the frame length: 260 octets of CADU, so 4 of sync marker and 256 of frame.

**Even spacing is the test.** A wrong frame length finds only the markers that happen to fall where it looks. The right one finds them all, at a constant stride.

## 2. Get the frames out

```bash
astro cadu sync --input bin --frame-len 260 --format hex examples/capture/capture.bin > cadus.hex

while read -r cadu; do
  printf '%s\n' "$cadu" | astro cadu unwrap --input hex --derandomize
done < cadus.hex | tr -d '\n' > frames.hex
```

Two things about that.

`--derandomize` is a guess too. CCSDS randomization is optional and not signalled. Get it wrong and the next step produces a frame header full of nonsense, which is how you find out.

`cadu unwrap` takes one CADU, not a stream, which is why the loop is there. Feeding it a concatenation strips only the first sync marker and de-randomizes the rest as though it were frame data.

## 3. Read one frame header

```bash
head -1 cadus.hex | astro cadu unwrap --input hex --derandomize | astro tm inspect --input hex
```

```
Primary Header (6 bytes)
  Version .............. 0
  Spacecraft ID ........ 26 (0x01A)
  Virtual Channel ID ... 1
  OCF Flag ............. false
  MC Frame Count ....... 0
  VC Frame Count ....... 0
  FSH Flag ............. false
  Sync Flag ............ false
  Packet Order Flag .... false
  Segment Length ID .... 3
  First Header Ptr ..... 0 (0x000)
```

This is the moment the guessing pays off. `Version 0` and a plausible spacecraft ID mean the frame length and the randomization were both right. A wrong guess on either shows up here as a version of 2 or 3, a spacecraft ID in the hundreds, and a First Header Pointer past the end of the frame.

So now you know: **spacecraft 26, TM frames, 256 octets, randomized, no OCF.** Four of the five things you needed. The fifth, whether there is a frame error control field, comes out of the next step.

## 4. Count what is missing

```bash
astro tm gaps --input hex --frame-len 256 frames.hex
```

```
MC gap: frame #6, 1 frame(s) missing before MC=6 (SCID=26)
VC gap: frame #8, 1 frame(s) missing before VC=2 (SCID=26 VCID=0)
Warning: frame #9 decode error: CRC mismatch: received CRC does not match computed CRC, skipping
MC gap: frame #10, 1 frame(s) missing before MC=10 (SCID=26)
VC gap: frame #10, 1 frame(s) missing before VC=7 (SCID=26 VCID=1)

Scanned 14 frame(s), found 4 gap(s), 4 frame(s) missing.
1 frame(s) could not be decoded and were skipped.
```

The CRC mismatch tells you the frame error control field is present and being checked. If the channel had no FECF, `tm gaps` would be reading two octets of real data as a CRC and rejecting nearly every frame.

Now read the four gaps carefully, because **there were only two real problems.**

One frame was lost in transit. That is the `MC gap` at frame #6, and the `VC gap` at frame #8 is the same loss seen from the virtual channel: the missing frame was VC0's, so VC0's count skips.

One frame arrived corrupted. That is the CRC mismatch at frame #9. Then the gaps at frame #10 are that same frame counted again: it was skipped, so both counters appear to jump.

**A corrupted frame reports twice**, once as a CRC failure and again as a gap. Adding up gap counts without allowing for that overstates the loss. Two frames went wrong here, and the tool reported four gaps and one reject.

## 5. Split the virtual channels

```bash
astro tm demux --input hex --frame-len 256 --vcid 0 frames.hex | tail -3
astro tm demux --input hex --frame-len 256 --vcid 1 frames.hex | tail -3
```

```
Matched 5 of 14 frame(s) on VCID=0.
Matched 9 of 14 frame(s) on VCID=1.
```

Two channels, and they are getting very different shares of the link. That is a priority setting, and here it says whatever is on VC1 is being given most of the downlink.

## 6. Get the packets

Frames carry packets, so pull the data field out of each frame and read the stream:

```bash
astro tm demux --input hex --frame-len 256 --vcid 0 --format hex frames.hex \
  | while read -r frame; do
      printf '%s\n' "$frame" | astro tm decode --input hex --format json
    done \
  | jq -r '.data_field' | tr -d '\n' > vc0-data.hex

astro spp stream --input hex --crc vc0-data.hex
```

```
--- Packet #1 (offset 0, 21 bytes) ---
  Type: TM  APID: 100  SeqFlags: unsegmented  SeqCount: 0  DataLen: 13
  Data: 000003e841e0cccd41b4000001
--- Packet #2 (offset 21, 21 bytes) ---
  Type: TM  APID: 100  SeqFlags: unsegmented  SeqCount: 1  DataLen: 13
...
--- Packet #11 (offset 210, 21 bytes) ---
  Type: TM  APID: 100  SeqFlags: unsegmented  SeqCount: 10  DataLen: 13
packet #12 at offset 231: CRC validation failed
```

Eleven packets from APID 100, sequence counts 0 to 10 with no gaps in them, then it stops.

The same on VC1:

```
--- Packet #1 (offset 0, 408 bytes) ---
  Type: TM  APID: 200  SeqFlags: unsegmented  SeqCount: 0  DataLen: 400
--- Packet #2 (offset 408, 408 bytes) ---
--- Packet #3 (offset 816, 408 bytes) ---
packet #4 at offset 1224: CRC validation failed
```

Three science packets of 400 octets each, then the same wall.

`--crc` is worth using here. Without it a bad packet decodes into a plausible-looking one and the stream keeps going, further and further out of step. With it, the decode stops where the data actually stopped being real.

## 7. Where the command line runs out

Both streams stop at the frame that went missing, and that is not a limitation of the tools. Concatenating data fields assumes they are contiguous. Once a frame is gone, a partial packet is left dangling and everything after it reads at the wrong offset.

Recovering from that is what the First Header Pointer is for, and it needs the [receiving service](/docs/guides/lossy-link), which throws away the partial packet and starts again at the next frame's FHP:

```go
service := tmdl.NewVirtualChannelPacketService(
    spacecraftID, vcid, vc, config, nil)
service.SetPacketSizer(spp.PacketSizer)
```

So the shape of the work is: **command line to find the parameters, Go to process the stream properly.** Twenty minutes of piping tells you the frame length, the randomization, the spacecraft, the channels and the APIDs, and then you write the fifteen lines that actually decode it.

## What each field means

Once packets are coming out, the layout of their payloads is a separate question, and the answer should not be a Go struct. Put it in a [mission database](/docs/guides/xtce-database):

```bash
astro xtce list examples/xtce/mission.xml --kind containers
astro xtce match examples/xtce/mission.xml packet.hex --root /Demosat/PrimaryHeader
```

```
Matched container: PowerReport
────────────────────────────────────────────────────────────────────
OFFSET   WIDTH    NAME                         VALUE
────────────────────────────────────────────────────────────────────
5        11       /Demosat/APID                100
18       14       /Demosat/SequenceCount       7
48       16       /Demosat/BusVoltage          28.14
64       16       /Demosat/BusCurrent          4.199981689453125
80       8        /Demosat/Mode                Science
```

`match` works out which container the packet is from its APID and decodes it
against that, so raw counts come back as volts and amperes. That database is
[`examples/xtce/mission.xml`](https://github.com/ravisuhag/astro/tree/main/examples/xtce),
which describes a different mission from this capture; the point is the shape
of the command.

## A checklist

When a capture will not decode, in the order worth trying:

1. **Frame length.** Wrong length finds sync markers at irregular offsets, or none.
2. **Randomization.** Wrong guess gives a garbage frame header. Try both.
3. **Frame error control.** Present when it is not expected eats two octets of data; expected when absent fails every CRC.
4. **The OCF.** Same problem, four octets. `astro tm inspect` reports the flag, and the flag is in the frame, so this one you can read rather than guess.
5. **Reed-Solomon.** A capture from a deep space link probably has an RS codeblock between the frame and the sync marker, so the "frame" is 255 octets of codeword rather than your frame. Interleaving depth is another guess.
6. **The frame type.** TM, [AOS](/docs/guides/aos-high-rate) and USLP all start with a version number, and it is the first two bits: 00 for TM, 01 for AOS, 1100 for USLP. Check that before anything else.

## Things that will bite you

**Nothing in the file states its own configuration.** Every parameter above is a guess checked against behaviour. Write down what you worked out, because the next person will have to do it again otherwise.

**A wrong guess rarely errors.** It produces plausible garbage. A frame header with version 0 and a sensible spacecraft ID is the strongest evidence you will get that the guess was right.

**Sync marker spacing is your best diagnostic.** Even stride means the frame length is right. Nothing else about a capture is that unambiguous.

**Count corrupted frames once.** They report as both a CRC failure and a counter gap.

**`--crc` on `spp stream` stops the bleeding.** Without it, one bad packet takes the rest of the stream with it silently.

**Do not trust a CADU count as a frame count.** Frames that fail CRC are still CADUs.

## Next

- [Handle a lossy link](/docs/guides/lossy-link), what the receiving service does that a pipeline cannot
- [Decode from a mission database](/docs/guides/xtce-database), for what the payload octets mean
- [CLI reference](/cli) | [CADU](/cli/cadu) | [TM](/cli/tm) | [SPP](/cli/spp)
