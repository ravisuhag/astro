// Example: Encrypting and authenticating a link
//
// Every other example here sends plaintext. Anyone with a dish can read a
// downlink, and worse, anyone with a transmitter can forge an uplink. SDLS is
// the layer that fixes both.
//
//	Downlink (clause E1 baseline):
//	  AES-256-GCM. The telemetry is encrypted and authenticated together.
//
//	Uplink (clause E2 baseline):
//	  AES-CMAC. The telecommand travels in the clear but cannot be forged,
//	  which is what commanding actually needs.
//
//	Three attacks that fail:
//	  1. A flipped bit in the ciphertext
//	  2. A replayed frame
//	  3. A valid frame injected on the wrong virtual channel
//
// SDLS protects the data field of a frame. The carrier packages need no
// changes: this package builds the protected data field and the frame
// constructor takes it as ordinary octets.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"

	"github.com/ravisuhag/astro/pkg/sdls"
	"github.com/ravisuhag/astro/pkg/tmdl"
)

const (
	spacecraftID = 42
	vcidSecure   = 1
	vcidOther    = 2

	spiDownlink uint16 = 1
	spiUplink   uint16 = 7
)

// Two keys, one per direction. Never one key for both: a link that encrypts
// and decrypts with the same key lets a recorded downlink be replayed as an
// uplink. Key management is out of scope for pkg/sdls, which takes the octets
// and does not store them.
var (
	downlinkKey = mustKey("downlink key: 32 octets exactly!")
	uplinkKey   = mustKey("uplink key: also 32 octets long!")
)

// tmBaseline is the CCSDS 355.0-B-2 clause E1 telemetry baseline: a 96-bit
// GCM initialization vector, no sequence number (GCM's IV is the anti-replay
// mechanism), no pad length, a 128-bit MAC.
var tmBaseline = sdls.FieldLengths{IV: sdls.GCMIVSize, SeqNum: 0, PadLen: 0, MAC: 16}

// tcBaseline is the clause E2 telecommand baseline: no IV, a 32-bit sequence
// number for anti-replay, no padding, a 128-bit MAC. Six octets of header.
var tcBaseline = sdls.FieldLengths{IV: 0, SeqNum: 4, PadLen: 0, MAC: 16}

func main() {
	fmt.Println("--- A secured downlink (AES-256-GCM) ---")
	fmt.Println()
	securedDownlink()

	fmt.Println("--- A secured uplink (AES-CMAC) ---")
	fmt.Println()
	securedUplink()

	fmt.Println("--- Three attacks ---")
	fmt.Println()
	attacks()
}

// downlinkSA builds one end of the telemetry Security Association. Both ends
// need a matching value, and only the SPI travels on the wire.
func downlinkSA() *sdls.SecurityAssociation {
	sa := &sdls.SecurityAssociation{
		SPI:          spiDownlink,
		Mode:         sdls.AuthenticatedEncryption, // always AES-GCM
		Key:          downlinkKey,
		FieldLengths: tmBaseline,

		// The mask is what keeps the MAC stable across the fields a frame
		// picks up downstream. For TM that is the Master Channel Frame Count,
		// which the virtual channel service writes after the data field
		// exists. Authenticate it and every frame fails at the receiver.
		AuthMask: sdls.BaselineAuthMaskTM(0, tmBaseline),

		// An SA serves an agreed set of channels, and the receiver checks it.
		Channels: []sdls.ChannelID{{
			TFVN: 0, SCID: spacecraftID, VCID: vcidSecure, MAPID: sdls.NoMAP,
		}},
	}
	if err := sa.Validate(); err != nil {
		log.Fatalf("the downlink SA is invalid: %v", err)
	}
	return sa
}

func securedDownlink() {
	transmit, receive := downlinkSA(), downlinkSA()

	telemetry := []byte("BATT 28.1V TEMP 22.5C MODE SCIENCE")

	// The frame header has to exist before the data field, because SDLS
	// authenticates it. That is fine: the security function runs inside the
	// virtual channel service, which already knows the channel, the frame
	// length and its own frame count.
	header := tmdl.PrimaryHeader{
		SpacecraftID:     spacecraftID,
		VirtualChannelID: vcidSecure,
		SegmentLengthID:  0b11,
		VCFrameCount:     12, // the VC service knows this one
	}
	headerBytes, err := header.Encode()
	if err != nil {
		log.Fatalf("encoding the frame header: %v", err)
	}

	protected, err := transmit.ApplySecurity(headerBytes, telemetry)
	if err != nil {
		log.Fatalf("applying security: %v", err)
	}

	fmt.Printf("  plaintext ........... %d octets: %q\n", len(telemetry), telemetry)
	fmt.Printf("  protected ........... %d octets\n", len(protected))
	fmt.Printf("    security header ... %d (SPI %d + %d octet IV)\n",
		tmBaseline.HeaderSize(), spiDownlink, tmBaseline.IV)
	fmt.Printf("    ciphertext ........ %d\n", len(telemetry))
	fmt.Printf("    MAC ............... %d\n", tmBaseline.MAC)
	fmt.Printf("  readable on the wire  %t\n", bytes.Contains(protected, telemetry))
	fmt.Println()

	// The protected data field is now ordinary frame payload.
	frame, err := tmdl.NewTransferFrame(spacecraftID, vcidSecure, protected, nil, nil)
	if err != nil {
		log.Fatalf("framing: %v", err)
	}

	frame.Header.VCFrameCount = 12

	// The master channel multiplexer stamps this one, downstream, after the
	// MAC was computed. That is exactly what the mask exists for: clause
	// 4.2.2.6.2 makes excluding the Master Channel Frame Count mandatory.
	frame.Header.MCFrameCount = 137

	encoded, err := frame.Encode()
	if err != nil {
		log.Fatalf("encoding the frame: %v", err)
	}
	fmt.Printf("  frame ............... %d octets, MC count %d\n",
		len(encoded), frame.Header.MCFrameCount)

	// The ground side.
	decoded, err := tmdl.DecodeTransferFrame(encoded)
	if err != nil {
		log.Fatalf("decoding the frame: %v", err)
	}
	receivedHeader, err := decoded.Header.Encode()
	if err != nil {
		log.Fatalf("re-encoding the received header: %v", err)
	}

	channel := sdls.ChannelID{
		TFVN: 0, SCID: spacecraftID, VCID: vcidSecure, MAPID: sdls.NoMAP,
	}
	securityHeader, recovered, err := sdls.ProcessSecurityForChannel(
		decoded.DataField, receivedHeader, channel, sdls.StaticLookup(receive))
	if err != nil {
		log.Fatalf("processing security: %v", err)
	}

	fmt.Printf("  SPI on the wire ..... %d\n", securityHeader.SPI)
	fmt.Printf("  recovered ........... %q\n", recovered)
	fmt.Printf("  MAC verified ........ %t despite the frame count changing\n",
		bytes.Equal(recovered, telemetry))
	fmt.Println()
}

// uplinkSA builds one end of the telecommand SA. Authentication only, which is
// the clause E2 baseline: a forged command is the threat, a read command
// usually is not.
func uplinkSA() *sdls.SecurityAssociation {
	sa := &sdls.SecurityAssociation{
		SPI:           spiUplink,
		Mode:          sdls.Authentication,
		AuthAlgorithm: sdls.AuthCMAC,
		Key:           uplinkKey,
		FieldLengths:  tcBaseline,
		AuthMask:      sdls.BaselineAuthMaskTC(false, tcBaseline),

		// The anti-replay window. Zero disables the check entirely, which is
		// only ever right in a test.
		SeqWindow: 100,
	}
	if err := sa.Validate(); err != nil {
		log.Fatalf("the uplink SA is invalid: %v", err)
	}
	return sa
}

func securedUplink() {
	transmit, receive := uplinkSA(), uplinkSA()

	// A TC frame header is 5 octets.
	frameHeader := []byte{0x20, 0x00, 0x00, 0x2A, 0x00}
	telecommand := []byte("SET MODE 3")

	protected, err := transmit.ApplySecurity(frameHeader, telecommand)
	if err != nil {
		log.Fatalf("applying security: %v", err)
	}

	fmt.Printf("  plaintext ........... %q\n", telecommand)
	fmt.Printf("  protected ........... %d octets\n", len(protected))
	fmt.Printf("  readable on the wire  %t, clause E2 authenticates without encrypting\n",
		bytes.Contains(protected, telecommand))

	_, recovered, err := sdls.ProcessSecurity(
		protected, frameHeader, sdls.StaticLookup(receive))
	if err != nil {
		log.Fatalf("processing security: %v", err)
	}
	fmt.Printf("  authenticated ....... %t\n", bytes.Equal(recovered, telecommand))
	fmt.Println()
}

func attacks() {
	// 1. Flip one bit of the ciphertext.
	transmit, receive := downlinkSA(), downlinkSA()
	header, err := (&tmdl.PrimaryHeader{
		SpacecraftID: spacecraftID, VirtualChannelID: vcidSecure, SegmentLengthID: 0b11,
	}).Encode()
	if err != nil {
		log.Fatalf("encoding the frame header: %v", err)
	}

	protected, err := transmit.ApplySecurity(header, []byte("BATT 28.1V"))
	if err != nil {
		log.Fatalf("applying security: %v", err)
	}
	tampered := bytes.Clone(protected)
	tampered[tmBaseline.HeaderSize()] ^= 0x01

	_, _, err = sdls.ProcessSecurity(tampered, header, sdls.StaticLookup(receive))
	fmt.Printf("  1. one flipped bit ........ %s\n", outcome(err))

	// 2. Replay a frame that already arrived. GCM refuses because the IV
	//    repeats; a CMAC SA refuses because the sequence number does.
	uplinkTX, uplinkRX := uplinkSA(), uplinkSA()
	tcHeader := []byte{0x20, 0x00, 0x00, 0x2A, 0x00}

	first, err := uplinkTX.ApplySecurity(tcHeader, []byte("SET MODE 3"))
	if err != nil {
		log.Fatalf("applying security: %v", err)
	}
	if _, _, err := sdls.ProcessSecurity(first, tcHeader, sdls.StaticLookup(uplinkRX)); err != nil {
		log.Fatalf("the first delivery should have been accepted: %v", err)
	}
	_, _, err = sdls.ProcessSecurity(first, tcHeader, sdls.StaticLookup(uplinkRX))
	fmt.Printf("  2. the same frame again ... %s\n", outcome(err))

	// 3. A genuine frame, moved to a virtual channel this SA does not serve.
	//    Nothing cryptographic is wrong with it, and it is still refused.
	valid, err := transmit.ApplySecurity(header, []byte("BATT 28.1V"))
	if err != nil {
		log.Fatalf("applying security: %v", err)
	}
	wrongChannel := sdls.ChannelID{
		TFVN: 0, SCID: spacecraftID, VCID: vcidOther, MAPID: sdls.NoMAP,
	}
	_, _, err = sdls.ProcessSecurityForChannel(
		valid, header, wrongChannel, sdls.StaticLookup(downlinkSA()))
	fmt.Printf("  3. wrong virtual channel .. %s\n", outcome(err))
	if errors.Is(err, sdls.ErrSAChannelMismatch) {
		fmt.Println("     refused before any cryptographic work")
	}
}

func outcome(err error) string {
	if err == nil {
		return "ACCEPTED (this is a bug)"
	}
	return fmt.Sprintf("rejected: %v", err)
}

func mustKey(s string) []byte {
	if len(s) != sdls.AESKeySize {
		log.Fatalf("a key must be %d octets, %q is %d", sdls.AESKeySize, s, len(s))
	}
	return []byte(s)
}
