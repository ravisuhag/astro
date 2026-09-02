---
title: Put a real timestamp on telemetry
short: Time correlation
description: A spacecraft counts its own ticks. Turning that count into a time is a separate job from decoding it.
order: 8
---

A spacecraft does not know what time it is. It counts ticks of an oscillator, and that count drifts against UTC by parts per million, which is seconds per day. Telemetry is stamped with the count, so every timestamp that reaches the ground is wrong until somebody corrects it.

That makes time two problems, not one. Writing an instant as a CCSDS time code is the easy half. Working out which instant the spacecraft meant is the half that produces the bugs.

The complete program is [`examples/timecorrelation`](https://github.com/ravisuhag/astro/tree/main/examples/timecorrelation). Run it:

```bash
go run ./examples/timecorrelation/
```

## The four formats

[CCSDS 301.0](/protocols/mission/tcf) defines four ways to write a time. Here is the same instant in all of them:

```
  The instant .......... 2026-04-17T08:30:15.25Z

  CUC level 1           7 octets  1e 80 74 4e 3c 40 00
  CUC level 2           7 octets  2e 00 8c 36 97 40 00
  CDS                   9 octets  41 61 6f 01 d3 26 d2 00 00
  CCS                  10 octets  52 20 26 04 17 08 30 15 25 00
  ASCII type A         24 octets  2026-04-17T08:30:15.250Z
```

**CUC** is a binary counter: coarse seconds and fine fractions, and nothing else. It is the smallest and the one to use on a link. Fine resolution is `2^-(8n)` seconds for `n` fine octets, so two octets gives about 15 microseconds and three gives 60 nanoseconds.

**CDS** segments into a day count, milliseconds of day, and optional sub-milliseconds. Two octets wider, and you can read the day straight off a hex dump.

**CCS** is BCD calendar fields. Widest, and the only binary one a human reads without arithmetic.

**ASCII** is text. It has no place on a space link and every place in a log file or a product filename.

```go
cuc, err := tcf.NewCUC(instant,
    tcf.WithCUCCoarseBytes(4), tcf.WithCUCFineBytes(2))
encoded, err := cuc.Encode()
```

## Level 1 counts TAI, not UTC

This is the first trap. A CUC Level 1 code counts seconds from 1958-01-01 on the **TAI** scale, and TAI does not have leap seconds:

```
      coarse 2155105852 s, fine 16384
      TAI is 37 s ahead of UTC at this instant
```

So the coarse count is not the UTC seconds since 1958. It is 37 seconds more, and it was 36 before 2017. `pkg/tcf` carries the full historical table and applies it both ways, so `NewCUC` and `Time` hide the arithmetic. `TAIUTCOffsetAt` exposes it when you need to check.

Two boundaries are worth knowing about. Instants before 1972 use an offset of zero, because UTC used fractional adjustments back then that have no integer form. And an instant inside an inserted leap second comes back as the following `00:00:00`, because Go's `time.Time` cannot hold second 60.

**Level 2** uses a mission epoch instead and is purely arithmetic, no leap-second correction in either direction:

```go
cuc, err := tcf.NewCUC(instant, tcf.WithCUCEpoch(missionEpoch), ...)
```

Smaller counter, simpler arithmetic, and completely meaningless to anyone who does not have your epoch. Write the epoch into the mission database and the archive format, not just the flight software.

## Then the actual problem

Here is the same clock stamping three packets during a pass, against what the ground independently knows those instants to be:

```
    on-board says          actually was           error
    08:28:11.133           08:30:00.000           -108.867 s
    08:33:11.129           08:35:00.000           -108.870 s
    08:38:11.126           08:40:00.000           -108.874 s
```

Every one of those time codes decoded perfectly. They are all nearly two minutes wrong, and the error is growing by about 3.5 milliseconds every five minutes.

Two things are going on. The clock was set late, which is the constant part. And it is running fast, which is the growing part. You need both to fix a timestamp.

## Where the correlation points come from

A correlation point is a pair: what the clock said, and when that really was. The second half has to come from somewhere the clock is not involved in, such as

- a ranging solution, which gives the light travel time and so the transmit instant,
- a GPS-stamped pulse the spacecraft reports having seen,
- or the station's receive time less a known travel time, which is the cheap version.

Whatever the source, you get pairs, and you fit a line through them.

## Fitting the line

```go
type correlationPoint struct {
    onboard float64 // seconds since the mission epoch, as the clock said
    ground  float64 // seconds since the mission epoch, as it really was
}

rate, offset := fit(points)
```

Two points give an exact answer. More points and ordinary least squares averages out the noise in each measurement, which is what a real system does. The fit in the example is about twenty lines of arithmetic and no library.

Working in seconds since the mission epoch, rather than since 1958, keeps the numbers small enough that `float64` has plenty of precision left. Fitting against a 2.1-billion-second TAI count throws away most of it.

```
  Fitted clock model:
    rate ..... 1.000011974 ground seconds per on-board second
    offset ... -1.163950 s at the mission epoch
    drift .... +12.0 ppm
```

Twelve parts per million. That is a perfectly ordinary oscillator, and it is a second a day.

## Applying it

```go
func apply(claimed time.Time, rate, offset float64) time.Time {
    onboard := claimed.Sub(missionEpoch).Seconds()
    ground := rate*onboard + offset
    return missionEpoch.Add(time.Duration(ground * float64(time.Second)))
}
```

And here it is on a science observation from the middle of the pass, which had no correlation point of its own:

```
    on-board time ..... 08:35:41.127899
    corrected ......... 08:37:29.999995
    truth ............. 08:37:30.000000

    error uncorrected . -108.872101 s
    error corrected ... -0.000004 s
```

From two minutes wrong to four microseconds. And that residual is not the clock any more, it is the time code: two fine octets quantise to about 15 microseconds, so this is as good as the format allows.

## Things that will bite you

**A perfect decode can still be the wrong time.** This is the whole point. Nothing about the codec catches an uncorrelated clock, and the number looks entirely reasonable.

**Level 1 and Level 2 codes are not interchangeable.** They have different epochs and different leap-second behaviour, and the P-field is the only thing that says which you have. A T-field on its own, which is what a packet usually carries, says nothing. The format has to come from the mission database.

**CDS cannot represent a leap second.** Milliseconds-of-day is capped at 86399999, so `23:59:60` has no encoding. On a leap-second day a CDS code is a UTC day and time-of-day label rather than a true elapsed count. If you need real elapsed time, use CUC Level 1.

**`NewCUC` truncates, it does not round.** The fractional second is truncated toward zero to the fine-time resolution. Over many timestamps that is a small consistent bias rather than noise, which is worse for a fit.

**One fit does not last the mission.** Oscillator rate moves with temperature, and a spacecraft's thermal environment changes every orbit. Correlate every pass, and keep the history: a rate that suddenly jumps is telling you something about the hardware.

**Extrapolating backwards is worse than forwards.** A fit from points late in a pass, applied to data from the start of it, carries the rate error over the whole gap. Bracket the data you care about.

## Next

- [Build a PUS service model](/docs/guides/pus-services), where the time field lives in the TM header
- [Decode from a mission database](/docs/guides/xtce-database), which is where the time format should be recorded
- [Time code formats page](/protocols/mission/tcf) | [Conformance](/conformance/tcf) | [CLI](/cli/time)
