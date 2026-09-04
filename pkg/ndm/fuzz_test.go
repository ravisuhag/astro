package ndm_test

import (
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/ndm"
)

// A combined instantiation is a second front door to every navigation decoder
// in the repository at once, and it adds a nesting level none of them has: a
// root that holds messages rather than a header and a body. Its own risks are
// the attribute rules, which decide whether a file is a combined instantiation
// at all, and the dispatch on the element name.
func FuzzDecodeCombined(f *testing.F) {
	f.Add(figureG21)
	f.Add("")
	f.Add("<ndm/>")
	f.Add(`<ndm xsi:noNamespaceSchemaLocation="x"></ndm>`)

	// A root carrying the attributes clause 4.11.4 denies it, a constituent
	// carrying the ones clause 4.11.5 denies it, and a message type no
	// package here reads.
	f.Add(strings.Replace(figureG21, "-master-3.0.xsd\">", "-master-3.0.xsd\" id=\"X\" version=\"1\">", 1))
	f.Add(strings.Replace(figureG21, `<omm id=`, `<omm xmlns:xsi="x" id=`, 1))
	f.Add(strings.NewReplacer("<omm ", "<rdm ", "</omm>", "</rdm>").Replace(figureG21))

	// Nesting that the shape does not allow: a combined instantiation inside
	// itself, and a message with no body.
	f.Add(strings.Replace(figureG21, "<omm id=", "<ndm><omm id=", 1))
	f.Add(`<ndm xsi:noNamespaceSchemaLocation="x"><opm id="CCSDS_OPM_VERS" version="3.0"><header/></opm></ndm>`)

	f.Fuzz(func(t *testing.T, data string) {
		c, err := ndm.DecodeCombined([]byte(data))
		if err != nil {
			return
		}

		if c.Humanize() == "" {
			t.Fatal("Humanize returned nothing for a file that decoded")
		}
		// Anything that decoded must have been recognised as combined in the
		// first place, since that is the check a caller makes before choosing
		// this decoder.
		if !ndm.IsCombined([]byte(data)) {
			t.Fatal("a file that decoded was not recognised as combined")
		}

		encoded, err := c.Encode()
		if err != nil {
			t.Fatalf("a file that decoded failed to encode: %v", err)
		}
		again, err := ndm.DecodeCombined(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}

		if len(again.Messages) != len(c.Messages) {
			t.Fatalf("the message count changed on round trip: %d then %d",
				len(c.Messages), len(again.Messages))
		}
		for i := range c.Messages {
			if ndm.Kind(again.Messages[i]) != ndm.Kind(c.Messages[i]) {
				t.Fatalf("message %d changed type on round trip: %s then %s",
					i, ndm.Kind(c.Messages[i]), ndm.Kind(again.Messages[i]))
			}
		}

		// Every constituent must still stand on its own, which is the promise
		// that a message means the same thing inside a combined file as
		// outside one.
		for i, m := range c.Messages {
			if _, err := m.EncodeXML(); err != nil {
				t.Fatalf("constituent %d cannot be written on its own: %v", i, err)
			}
		}
	})
}
