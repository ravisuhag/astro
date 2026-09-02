---
title: Encrypt and authenticate a link
short: Secure a link
description: Anyone with a dish can read a downlink. Anyone with a transmitter can forge an uplink.
order: 10
---

Every other guide here sends plaintext. That is a reasonable way to learn the protocols and a bad way to fly. A downlink is readable by anyone with a dish, and an uplink is forgeable by anyone with a transmitter and your frame format, which is published.

[SDLS](/protocols/data-link/sdls) fixes both, and this guide shows a secured link in each direction plus three attacks failing.

The complete program is [`examples/sdls`](https://github.com/ravisuhag/astro/tree/main/examples/sdls). Run it:

```bash
go run ./examples/sdls/
```

## Where it sits

SDLS protects the **data field** of a frame. It adds a security header in front and a trailer behind, and the frame packages need no changes at all:

```
                  ┌──────────── frame data field ────────────────┐
frame header ...  │ Security Header │ ciphertext │ MAC (trailer) │
                  └──────────────────────────────────────────────┘
```

So the shape of the work is: build the protected data field with `pkg/sdls`, then hand it to `tmdl.NewTMTransferFrame` as ordinary octets.

## A Security Association is the agreement

Nothing on the wire describes the layout. Field widths, the algorithm, the key: all of it comes from a Security Association that both ends configured before the link opened. Only the Security Parameter Index travels, and it is just a pointer to the agreement.

```go
sa := &sdls.SecurityAssociation{
    SPI:          1,
    Mode:         sdls.AuthenticatedEncryption,
    Key:          downlinkKey,
    FieldLengths: tmBaseline,
    AuthMask:     sdls.BaselineAuthMaskTM(0, tmBaseline),
    Channels: []sdls.ChannelID{{
        TFVN: 0, SCID: spacecraftID, VCID: vcidSecure, MAPID: sdls.NoMAP,
    }},
}
if err := sa.Validate(); err != nil {
    log.Fatalf("the downlink SA is invalid: %v", err)
}
```

An SA is **not** safe for concurrent use. It holds the sender's IV counter and the receiver's anti-replay state, both of which change on every frame. Give each direction of each channel its own value.

Use one key per direction, too. A link that encrypts and decrypts with the same key lets a recorded downlink be replayed as an uplink.

## Two baselines, two jobs

`pkg/sdls` ships the annex baselines, and the difference between them is worth understanding.

**Clause E1, the telemetry baseline.** AES-256-GCM, encrypting and authenticating together:

```go
var tmBaseline = sdls.FieldLengths{IV: sdls.GCMIVSize, SeqNum: 0, PadLen: 0, MAC: 16}
```

No sequence number, because GCM's initialization vector is already the anti-replay mechanism. Fourteen octets of header and sixteen of trailer, so thirty octets of overhead per frame.

**Clause E2, the telecommand baseline.** AES-CMAC, authenticating without encrypting:

```go
var tcBaseline = sdls.FieldLengths{IV: 0, SeqNum: 4, PadLen: 0, MAC: 16}
```

That looks like the weaker choice and is usually the right one. The threat to commanding is a **forged** command, not a read one. Authentication stops the forgery, and leaving the command readable means a station can see what it is relaying and a recording is diagnosable years later.

```
  plaintext ........... "SET MODE 3"
  protected ........... 32 octets
  readable on the wire  true, clause E2 authenticates without encrypting
  authenticated ....... true
```

## The mask is the subtle part

A frame picks up fields after it leaves the security function. The Master Channel Frame Count is written by the master channel multiplexer, downstream of the virtual channel that built the data field. Authenticate that octet and every frame fails at the receiver.

So clause 4.2.2.6.2 defines a mask, ANDed over the frame header and security header before the MAC is computed. Use the per-frame-type constructor and it applies the mandatory exclusions for you:

```go
AuthMask: sdls.BaselineAuthMaskTM(0, tmBaseline),
```

There is one for each carrier: `BaselineAuthMaskTM`, `BaselineAuthMaskTC`, `BaselineAuthMaskAOS`, `BaselineAuthMaskUSLP`. They know that TM excludes the MC frame count, AOS also excludes the Frame Header Error Control and the insert zone, and the IV is excluded in every case.

Here is the mask earning its keep. Security runs first, the multiplexer stamps the frame count afterwards, and the MAC still verifies:

```
  frame ............... 72 octets, MC count 137
  SPI on the wire ..... 1
  recovered ........... "BATT 28.1V TEMP 22.5C MODE SCIENCE"
  MAC verified ........ true despite the frame count changing
```

Note which count is which. The **virtual channel** frame count is known when the data field is built, because the VC service assigns it, so it stays under the MAC. The **master channel** count is not, so it does not. Putting them the other way round produces a link where nothing verifies and the reason is four layers away.

## The header comes before the data field

That ordering is real and worth planning for:

```go
header := tmdl.PrimaryHeader{
    SpacecraftID:     spacecraftID,
    VirtualChannelID: vcidSecure,
    SegmentLengthID:  0b11,
    VCFrameCount:     12,
}
headerBytes, err := header.Encode()

protected, err := transmit.ApplySecurity(headerBytes, telemetry)
frame, err := tmdl.NewTMTransferFrame(spacecraftID, vcidSecure, protected, nil, nil)
```

It works because the frame length is fixed for the physical channel, so the header is knowable before its contents exist.

## Bind the SA to its channels

An SA serves an agreed set of channels, and clause 4.2.4.3 makes the receiver check it:

```go
securityHeader, recovered, err := sdls.ProcessSecurityForChannel(
    decoded.DataField, receivedHeader, channel, sdls.StaticLookup(receive))
```

`ProcessSecurity` skips that check. `ProcessSecurityForChannel` enforces it, before doing any cryptographic work. Fill in `Channels` and use the second one.

## The attacks

```
  1. one flipped bit ........ rejected: authentication failed
  2. the same frame again ... rejected: replay detected: sequence number rejected
  3. wrong virtual channel .. rejected: security association is not bound to this channel
     refused before any cryptographic work
```

The third is the interesting one. Nothing is cryptographically wrong with that frame. It is genuine, correctly signed, and it was moved to a channel this SA does not serve. Without the channel binding it would be accepted, and an attacker who can move a frame between channels can confuse a receiver about which subsystem sent what.

## Things that will bite you

**A nil `AuthMask` means "authenticate everything".** That is stricter than the standard asks and, for TM and AOS, it breaks the mandatory exclusions. Every frame will fail at the receiver. Only leave it nil when the header you pass genuinely has no downstream-written field.

**`SeqWindow: 0` disables anti-replay.** The zero value turns the check off completely. It looks like a sensible default and is only ever right in a test.

**`MAPID` zero is MAP 0, not "no MAP".** `ChannelID` needs `MAPID: sdls.NoMAP` when the frames have no MAP or the binding stops at the virtual channel. The zero value silently means a real MAP.

**One SA, one direction, one channel.** They mutate. Sharing one across goroutines corrupts the IV counter, and reusing an IV under GCM is the failure that loses you the key stream.

**SDLS adds thirty octets to a TM frame.** With a 256-octet frame that is 12% of your downlink. It is worth choosing the frame length with the security overhead in mind rather than after.

**Key management is not here.** `pkg/sdls` takes the key octets and does not load, store or rotate them. That is deliberate, and it means the hard part is still yours.

## Next

- [Build a downlink](/docs/guides/downlink), the same chain without the security layer
- [Build an uplink](/docs/guides/uplink), which is what the clause E2 baseline protects
- [SDLS protocol page](/protocols/data-link/sdls) | [Conformance](/conformance/sdls) | [CLI](/cli/sdls)
