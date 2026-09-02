package sle

import (
	"encoding/binary"
	"fmt"
	"io"
)

// The Transport Mapping Layer, per CCSDS 913.1-B-2 clause 3.3.
//
// TML is a thin framing over TCP. One SLE association maps to exactly one TCP
// connection (clause 3.3.1), and everything on it is one of three message types:
// an SLE PDU, a context message opening the connection, or a heartbeat keeping
// an idle one alive.

// TMLHeaderSize is the width of a TML message header in octets (clause 3.3.2.2.1).
const TMLHeaderSize = 8

// MessageType is the TML message type identifier of table 3-1.
type MessageType uint8

const (
	// MessageSLEPDU carries an encoded SLE protocol data unit.
	MessageSLEPDU MessageType = 1
	// MessageContext carries the TML initialization parameters. It is the
	// first message on a connection.
	MessageContext MessageType = 2
	// MessageHeartbeat probes an idle connection. Its body is empty.
	MessageHeartbeat MessageType = 3
)

// String names the message type.
func (m MessageType) String() string {
	switch m {
	case MessageSLEPDU:
		return "SLE PDU"
	case MessageContext:
		return "context"
	case MessageHeartbeat:
		return "heartbeat"
	default:
		return fmt.Sprintf("invalid(%d)", uint8(m))
	}
}

// Valid reports whether the type is one of the three of table 3-1.
func (m MessageType) Valid() bool {
	return m >= MessageSLEPDU && m <= MessageHeartbeat
}

// ProtocolID is the identification field of a context message: the characters
// 'I' 'S' 'P' '1' (clause 3.3.2.2.4 a).
var ProtocolID = [4]byte{'I', 'S', 'P', '1'}

// ProtocolVersion is the version a context message carries (clause 3.3.2.2.4 c).
const ProtocolVersion uint8 = 1

// ContextBodySize is the width of a context message body (clause 3.3.2.2.4).
const ContextBodySize = 12

// DefaultMaxMessageSize bounds a TML message body when no limit is given:
// 16 MiB. The length field is 32 bits, so without a cap one message could
// name four gigabytes.
const DefaultMaxMessageSize = 16 << 20

// Message is one TML message.
type Message struct {
	Type MessageType
	// Body is the message body: an encoded SLE PDU, a context body, or empty
	// for a heartbeat.
	Body []byte
}

// Encode serializes the message per figure 3-3: a one-octet type, three
// reserved zero octets, a four-octet body length, then the body.
//
// Clause 3.3.2.2.6: every integer is big-endian.
func (m *Message) Encode() ([]byte, error) {
	if !m.Type.Valid() {
		return nil, ErrInvalidMessageType
	}
	if m.Type == MessageHeartbeat && len(m.Body) != 0 {
		return nil, ErrNonEmptyHeartbeat
	}
	if m.Type == MessageContext && len(m.Body) != ContextBodySize {
		return nil, ErrInvalidContextLength
	}

	out := make([]byte, TMLHeaderSize, TMLHeaderSize+len(m.Body))
	out[0] = byte(m.Type)
	// Octets 1 to 3 are the reserved field, left zero.
	binary.BigEndian.PutUint32(out[4:8], uint32(len(m.Body)))
	return append(out, m.Body...), nil
}

// DecodeMessage parses one complete TML message from the front of data,
// returning it and the octets consumed.
func DecodeMessage(data []byte) (*Message, int, error) {
	return DecodeMessageWithLimit(data, DefaultMaxMessageSize)
}

// DecodeMessageWithLimit parses one message, refusing a body beyond maxBody.
func DecodeMessageWithLimit(data []byte, maxBody int) (*Message, int, error) {
	if maxBody <= 0 {
		maxBody = DefaultMaxMessageSize
	}
	if len(data) < TMLHeaderSize {
		return nil, 0, ErrDataTooShort
	}

	m := &Message{Type: MessageType(data[0])}
	if !m.Type.Valid() {
		return nil, 0, ErrInvalidMessageType
	}

	length := int(binary.BigEndian.Uint32(data[4:8]))
	if length > maxBody {
		return nil, 0, ErrMessageTooLarge
	}
	if len(data) < TMLHeaderSize+length {
		return nil, 0, ErrDataTooShort
	}

	// Clause 3.3.2.2.5: a heartbeat is a header with a zero length.
	if m.Type == MessageHeartbeat && length != 0 {
		return nil, 0, ErrNonEmptyHeartbeat
	}
	if m.Type == MessageContext && length != ContextBodySize {
		return nil, 0, ErrInvalidContextLength
	}

	if length > 0 {
		m.Body = make([]byte, length)
		copy(m.Body, data[TMLHeaderSize:TMLHeaderSize+length])
	}
	return m, TMLHeaderSize + length, nil
}

// ReadMessage reads one TML message from r.
//
// It reads the eight-octet header, then exactly as many body octets as the
// header promises, and no further. That matters on a stream: reading ahead
// would swallow the start of the next message.
func ReadMessage(r io.Reader, maxBody int) (*Message, error) {
	if maxBody <= 0 {
		maxBody = DefaultMaxMessageSize
	}

	var header [TMLHeaderSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}

	m := &Message{Type: MessageType(header[0])}
	if !m.Type.Valid() {
		return nil, ErrInvalidMessageType
	}

	length := int(binary.BigEndian.Uint32(header[4:8]))
	if length > maxBody {
		return nil, ErrMessageTooLarge
	}
	if m.Type == MessageHeartbeat && length != 0 {
		return nil, ErrNonEmptyHeartbeat
	}
	if m.Type == MessageContext && length != ContextBodySize {
		return nil, ErrInvalidContextLength
	}

	if length > 0 {
		m.Body = make([]byte, length)
		if _, err := io.ReadFull(r, m.Body); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// WriteMessage writes one TML message to w.
func WriteMessage(w io.Writer, m *Message) error {
	encoded, err := m.Encode()
	if err != nil {
		return err
	}
	_, err = w.Write(encoded)
	return err
}

// Humanize returns a human-readable summary.
func (m *Message) Humanize() string {
	return fmt.Sprintf("TML Message\n  Type ... %s\n  Body ... %d octets", m.Type, len(m.Body))
}

// ContextMessage is the body of a TML context message (clause 3.3.2.2.4).
//
// It opens a connection, telling the peer how often to expect a heartbeat and
// how many missed ones mean the link is dead.
type ContextMessage struct {
	// HeartbeatInterval is how many seconds pass between heartbeats. Zero
	// disables the heartbeat entirely (clause 3.3.3).
	HeartbeatInterval uint16
	// DeadFactor is how many intervals of silence mean the peer has gone.
	DeadFactor uint16
}

// Encode serializes the context message body: 'ISP1', three reserved zeros,
// the version, then the two parameters.
func (c *ContextMessage) Encode() []byte {
	out := make([]byte, ContextBodySize)
	copy(out[0:4], ProtocolID[:])
	// Octets 4 to 6 are the reserved field, left zero.
	out[7] = ProtocolVersion
	binary.BigEndian.PutUint16(out[8:10], c.HeartbeatInterval)
	binary.BigEndian.PutUint16(out[10:12], c.DeadFactor)
	return out
}

// Message wraps the context body in a TML message.
func (c *ContextMessage) Message() *Message {
	return &Message{Type: MessageContext, Body: c.Encode()}
}

// DecodeContextMessage parses a context message body.
func DecodeContextMessage(body []byte) (*ContextMessage, error) {
	if len(body) != ContextBodySize {
		return nil, ErrInvalidContextLength
	}
	for i := range ProtocolID {
		if body[i] != ProtocolID[i] {
			return nil, ErrInvalidProtocolID
		}
	}
	if body[7] != ProtocolVersion {
		return nil, ErrInvalidProtocolVersion
	}
	return &ContextMessage{
		HeartbeatInterval: binary.BigEndian.Uint16(body[8:10]),
		DeadFactor:        binary.BigEndian.Uint16(body[10:12]),
	}, nil
}

// Humanize returns a human-readable summary.
func (c *ContextMessage) Humanize() string {
	return fmt.Sprintf("TML Context Message\n"+
		"  Protocol ............ ISP1 version %d\n"+
		"  Heartbeat interval .. %d s\n"+
		"  Dead factor ......... %d",
		ProtocolVersion, c.HeartbeatInterval, c.DeadFactor)
}

// HeartbeatMessage returns a TML heartbeat: a header with an empty body.
func HeartbeatMessage() *Message {
	return &Message{Type: MessageHeartbeat}
}
