package sle_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/sle"
)

// The wire vectors for this package live in vectors/sle/. They pin the BER
// primitives and the ISP1 TML framing — the foundation everything else in
// SLE sits on. A wrong INTEGER length or length-form boundary corrupts
// every operation encoded above it, which is why these come first.
//
// The association machine needs a sequence of calls, which no vector kind
// expresses. The operation encodings above these primitives are not
// pinned: several GET-PARAMETER alternatives have no published vectors to
// test a typed shape against.

func TestBERVectors(t *testing.T) {
	vectors.RunFile(t, "sle/ber.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			kind, err := f.Str("kind")
			if err != nil {
				return nil, err
			}
			switch kind {
			case "integer":
				v, err := f.Uint("value")
				if err != nil {
					// Negative values arrive as JSON numbers, which Uint
					// refuses; read them through the raw field instead.
					var signed int64
					if n, ok := f["value"]; ok {
						if _, serr := fmt.Sscan(fmt.Sprint(n), &signed); serr == nil {
							return sle.AppendInteger(nil, signed), nil
						}
					}
					return nil, err
				}
				return sle.AppendInteger(nil, int64(v)), nil

			case "octet_string":
				v, err := f.Hex("value")
				if err != nil {
					return nil, err
				}
				return sle.AppendOctetString(nil, v), nil

			case "null":
				return sle.AppendNull(nil), nil

			case "sequence":
				content, err := f.Hex("content")
				if err != nil {
					return nil, err
				}
				return sle.AppendSequence(nil, content), nil

			case "length":
				v, err := f.Uint("value")
				if err != nil {
					return nil, err
				}
				return sle.AppendLength(nil, int(v)), nil

			case "tml":
				mt, err := f.Uint("message_type")
				if err != nil {
					return nil, err
				}
				body, err := f.Hex("body")
				if err != nil {
					return nil, err
				}
				m := sle.Message{Type: sle.MessageType(mt), Body: body}
				var buf bytes.Buffer
				if err := sle.WriteMessage(&buf, &m); err != nil {
					return nil, err
				}
				return buf.Bytes(), nil

			default:
				return nil, fmt.Errorf("unknown kind %q", kind)
			}
		},

		ConstructFn: func(f, _ vectors.Fields) error {
			mt, err := f.Uint("message_type")
			if err != nil {
				return err
			}
			body, err := f.Hex("body")
			if err != nil {
				return err
			}
			m := sle.Message{Type: sle.MessageType(mt), Body: body}
			_, err = m.Encode()
			return err
		},

		DecodeFn: func(input []byte, config vectors.Fields) (vectors.Fields, error) {
			kind, err := config.Str("kind")
			if err != nil {
				return nil, err
			}
			if kind != "tml" {
				return nil, fmt.Errorf("no decoder wired for kind %q", kind)
			}
			m, err := sle.ReadMessage(bytes.NewReader(input), 1<<20)
			if err != nil {
				return nil, err
			}
			return vectors.Fields{
				"message_type": uint8(m.Type),
				"body":         m.Body,
			}, nil
		},
	})
}
