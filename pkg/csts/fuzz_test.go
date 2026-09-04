package csts_test

import (
	"encoding/hex"
	"testing"

	"github.com/ravisuhag/astro/pkg/csts"
)

// The property is that arbitrary input never panics and never drives an
// allocation from a length field an attacker controls.
//
// BER is all length fields, which is what makes this worth doing. The codec
// under this package is fuzzed on its own in internal/ber; what is fuzzed here
// is the layer above it — twenty CHOICE alternatives, nested SEQUENCEs whose
// fields are themselves CHOICEs, and integer ranges that Go's int64 does not
// enforce.
func FuzzDecode(f *testing.F) {
	for _, seed := range []string{
		unbindInvocationHex,
		peerAbortHex,
		unbindReturnHex,
		"",
		"a000",
		"a400",
	} {
		if data, err := hex.DecodeString(seed); err == nil {
			f.Add(data)
		}
	}

	// Shapes that must be refused rather than crash: a tag in one of the gaps
	// annex F3.15 leaves, a header that ends early, an integer too large for
	// the IntUnsigned range, and a procedure role outside its CHOICE.
	f.Add([]byte{0xA5, 0x00})
	f.Add([]byte{0xA2, 0x02, 0x30, 0x00})
	f.Add([]byte{0xA2, 0x0B, 0x30, 0x09, 0x80, 0x00, 0x02, 0x05, 0x01, 0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0xA2, 0x08, 0x30, 0x06, 0x80, 0x00, 0x02, 0x01, 0x01, 0x30, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		pdu, err := csts.Decode(data)
		if err != nil {
			return
		}

		if pdu.Humanize() == "" {
			t.Fatal("Humanize returned nothing for a PDU that decoded")
		}
		// Anything that decoded must name an operation the framework defines,
		// since Decode refuses the rest.
		if !pdu.Type.Known() {
			t.Fatalf("a PDU decoded with an unknown type %d", pdu.Type)
		}

		encoded, err := pdu.Encode()
		if err != nil {
			t.Fatalf("a PDU that decoded failed to encode: %v", err)
		}
		again, err := csts.Decode(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%x", err, encoded)
		}
		if again.Type != pdu.Type {
			t.Fatalf("the operation changed on round trip: %s then %s", pdu.Type, again.Type)
		}

		// The invoke-id is what pairs a response with its invocation
		// (clause 3.3.2.4.2), so it must survive.
		if header, ok := pdu.Header(); ok {
			back, ok := again.Header()
			if !ok {
				t.Fatal("the invocation header vanished on round trip")
			}
			if back.InvokeID != header.InvokeID {
				t.Fatalf("invoke id changed: %d then %d", header.InvokeID, back.InvokeID)
			}
			if !back.Procedure.Type.Equal(header.Procedure.Type) {
				t.Fatalf("procedure type changed: %s then %s",
					header.Procedure.Type, back.Procedure.Type)
			}
		}
		if header, ok := pdu.ReturnHeader(); ok {
			back, ok := again.ReturnHeader()
			if !ok {
				t.Fatal("the return header vanished on round trip")
			}
			if back.InvokeID != header.InvokeID || back.Positive != header.Positive {
				t.Fatalf("return header changed: %+v then %+v", header, back)
			}
		}
	})
}
