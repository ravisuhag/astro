package sle_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/sle"
)

func TestTMLMessageLayout(t *testing.T) {
	// CCSDS 913.1-B-2 figure 3-3: one type octet, three reserved zeros, a
	// four-octet big-endian body length, then the body.
	body := []byte("an encoded SLE PDU")
	m := &sle.Message{Type: sle.MessageSLEPDU, Body: body}

	encoded, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != sle.TMLHeaderSize+len(body) {
		t.Fatalf("encoded %d octets, want %d", len(encoded), sle.TMLHeaderSize+len(body))
	}
	if encoded[0] != byte(sle.MessageSLEPDU) {
		t.Errorf("type octet = %d, want 1", encoded[0])
	}
	// §3.3.2.2.2 b): the reserved field is three zero octets.
	if encoded[1] != 0 || encoded[2] != 0 || encoded[3] != 0 {
		t.Errorf("reserved field = % X, want 00 00 00", encoded[1:4])
	}
	// §3.3.2.2.6: big-endian.
	length := uint32(encoded[4])<<24 | uint32(encoded[5])<<16 | uint32(encoded[6])<<8 | uint32(encoded[7])
	if int(length) != len(body) {
		t.Errorf("length field = %d, want %d", length, len(body))
	}
}

func TestTMLMessageRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		name string
		msg  sle.Message
	}{
		{"SLE PDU", sle.Message{Type: sle.MessageSLEPDU, Body: []byte{1, 2, 3}}},
		{"empty SLE PDU", sle.Message{Type: sle.MessageSLEPDU}},
		{"heartbeat", sle.Message{Type: sle.MessageHeartbeat}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.msg.Encode()
			if err != nil {
				t.Fatal(err)
			}
			got, consumed, err := sle.DecodeMessage(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if consumed != len(encoded) {
				t.Errorf("consumed %d, want %d", consumed, len(encoded))
			}
			if got.Type != tt.msg.Type {
				t.Errorf("type = %s, want %s", got.Type, tt.msg.Type)
			}
			if !bytes.Equal(got.Body, tt.msg.Body) {
				t.Errorf("body = %x, want %x", got.Body, tt.msg.Body)
			}
		})
	}
}

func TestHeartbeatMustBeEmpty(t *testing.T) {
	// §3.3.2.2.5: a heartbeat is a header only.
	hb := sle.HeartbeatMessage()
	encoded, err := hb.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != sle.TMLHeaderSize {
		t.Errorf("heartbeat is %d octets, want %d", len(encoded), sle.TMLHeaderSize)
	}

	bad := &sle.Message{Type: sle.MessageHeartbeat, Body: []byte{1}}
	if _, err := bad.Encode(); !errors.Is(err, sle.ErrNonEmptyHeartbeat) {
		t.Errorf("error = %v, want ErrNonEmptyHeartbeat", err)
	}
}

func TestContextMessageLayout(t *testing.T) {
	// §3.3.2.2.4 and figure 3-4: 'ISP1', three reserved zeros, version 1,
	// then the heartbeat interval and dead factor.
	c := &sle.ContextMessage{HeartbeatInterval: 30, DeadFactor: 3}
	body := c.Encode()

	if len(body) != sle.ContextBodySize {
		t.Fatalf("context body is %d octets, want 12", len(body))
	}
	if string(body[0:4]) != "ISP1" {
		t.Errorf("protocol ID = %q, want ISP1", body[0:4])
	}
	if body[4] != 0 || body[5] != 0 || body[6] != 0 {
		t.Errorf("reserved field = % X, want zeros", body[4:7])
	}
	if body[7] != sle.ProtocolVersion {
		t.Errorf("version = %d, want 1", body[7])
	}

	got, err := sle.DecodeContextMessage(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.HeartbeatInterval != 30 || got.DeadFactor != 3 {
		t.Errorf("parameters = %d/%d, want 30/3", got.HeartbeatInterval, got.DeadFactor)
	}
}

func TestContextMessageValidation(t *testing.T) {
	valid := (&sle.ContextMessage{HeartbeatInterval: 30, DeadFactor: 3}).Encode()

	bad := append([]byte{}, valid...)
	copy(bad[0:4], "XXXX")
	if _, err := sle.DecodeContextMessage(bad); !errors.Is(err, sle.ErrInvalidProtocolID) {
		t.Errorf("wrong protocol ID: error = %v, want ErrInvalidProtocolID", err)
	}

	bad = append([]byte{}, valid...)
	bad[7] = 2
	if _, err := sle.DecodeContextMessage(bad); !errors.Is(err, sle.ErrInvalidProtocolVersion) {
		t.Errorf("wrong version: error = %v, want ErrInvalidProtocolVersion", err)
	}

	if _, err := sle.DecodeContextMessage(valid[:11]); !errors.Is(err, sle.ErrInvalidContextLength) {
		t.Errorf("short body: error = %v, want ErrInvalidContextLength", err)
	}
}

func TestDecodeMessageRejectsBadType(t *testing.T) {
	// Table 3-1 defines only 1, 2 and 3.
	data := make([]byte, sle.TMLHeaderSize)
	data[0] = 4
	if _, _, err := sle.DecodeMessage(data); !errors.Is(err, sle.ErrInvalidMessageType) {
		t.Errorf("error = %v, want ErrInvalidMessageType", err)
	}
}

func TestDecodeMessageRejectsOversizedBody(t *testing.T) {
	// The length field is 32 bits, so a hostile message can name four
	// gigabytes.
	data := make([]byte, sle.TMLHeaderSize)
	data[0] = byte(sle.MessageSLEPDU)
	data[4], data[5], data[6], data[7] = 0xFF, 0xFF, 0xFF, 0xFF

	if _, _, err := sle.DecodeMessageWithLimit(data, 1024); !errors.Is(err, sle.ErrMessageTooLarge) {
		t.Errorf("error = %v, want ErrMessageTooLarge", err)
	}
}

func TestReadMessageStopsAtTheBoundary(t *testing.T) {
	// On a stream, reading past a message would swallow the next one.
	first := &sle.Message{Type: sle.MessageSLEPDU, Body: []byte("first")}
	second := &sle.Message{Type: sle.MessageSLEPDU, Body: []byte("second")}

	var buf bytes.Buffer
	if err := sle.WriteMessage(&buf, first); err != nil {
		t.Fatal(err)
	}
	if err := sle.WriteMessage(&buf, second); err != nil {
		t.Fatal(err)
	}

	gotFirst, err := sle.ReadMessage(&buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotFirst.Body) != "first" {
		t.Errorf("first body = %q", gotFirst.Body)
	}

	gotSecond, err := sle.ReadMessage(&buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotSecond.Body) != "second" {
		t.Errorf("second body = %q", gotSecond.Body)
	}
	if buf.Len() != 0 {
		t.Errorf("%d octets left unread", buf.Len())
	}
}

func TestReadMessageRejectsTruncatedStream(t *testing.T) {
	m := &sle.Message{Type: sle.MessageSLEPDU, Body: []byte("a body")}
	encoded, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for cut := 1; cut < len(encoded); cut++ {
		if _, err := sle.ReadMessage(bytes.NewReader(encoded[:cut]), 0); err == nil {
			t.Errorf("length %d: expected an error, got nil", cut)
		}
	}
}
