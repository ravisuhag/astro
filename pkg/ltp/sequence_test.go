package ltp_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/ltp"
)

// The sequence vectors in vectors/ltp/session.json drive a sender and a
// receiver across a link that can drop segments.
//
// Segment octets are pinned in ltp/header.json. What these vectors pin is the
// exchange: which segment prompts which answer, what a lost segment costs, and
// where the protocol stops without a caller's timer. None of that fits in an
// input-and-output pair, because every answer depends on what came before.
func TestSessionSequenceVectors(t *testing.T) {
	vectors.RunFile(t, "ltp/session.json", vectors.Impl{MachineFn: newLinkMachine})
}

// link is a sender, a receiver, and the one-way path between them. Segments
// cross only when a step says so, which is what lets a vector lose one on
// purpose.
type link struct {
	sender   *ltp.Sender
	receiver *ltp.Receiver
	block    []byte
	lost     int

	// reports counts the report segments the receiver produced. A run that
	// lost nothing needs exactly one; every extra report is a round trip the
	// loss cost.
	reports int

	// Pending slots hold one segment already pulled from each end.
	//
	// They exist because NextSegment consumes: there is no way to ask
	// "have you anything to send" without taking it. A vector that checks
	// whether an end has gone quiet — which is the whole point of the lost
	// checkpoint run — needs that question answered without changing the
	// answer. Holding one segment here is the honest way to ask it, rather
	// than adding a peek to the package for the benefit of a test.
	senderPending   *ltp.Segment
	receiverPending *ltp.Segment
}

func newLinkMachine(init, config vectors.Fields) (vectors.Machine, error) {
	block, err := config.Hex("block")
	if err != nil {
		return nil, err
	}
	segmentSize, err := config.Uint("segment_size")
	if err != nil {
		return nil, err
	}
	redPartLength, err := config.Uint("red_part_length")
	if err != nil {
		return nil, err
	}

	id := ltp.SessionID{EngineID: 1, SessionNumber: 42}
	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             id,
		ClientServiceID:       1,
		SegmentSize:           int(segmentSize),
		RedPartLength:         redPartLength,
		FirstCheckpointSerial: 0x5A5B,
	})
	if err != nil {
		return nil, err
	}
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID:         id,
		FirstReportSerial: 0xA5A6,
	})
	if err != nil {
		return nil, err
	}
	return &link{sender: sender, receiver: receiver, block: block}, nil
}

func (l *link) Step(call string, fields vectors.Fields) ([]byte, vectors.Fields, error) {
	switch call {
	case "send":
		// One segment from the sender, delivered.
		if _, err := l.move(false); err != nil {
			return nil, nil, err
		}

	case "drop":
		// One segment from the sender, thrown away. The sender is never told.
		if _, err := l.move(true); err != nil {
			return nil, nil, err
		}

	case "flush":
		if err := l.pump(-1); err != nil {
			return nil, nil, err
		}

	case "flush_dropping_last":
		// Deliver everything except the sender's final segment, which is the
		// checkpoint. Nothing then prompts a report.
		if err := l.pumpDroppingLast(); err != nil {
			return nil, nil, err
		}

	case "resend_checkpoint":
		// The caller's timer firing. This package owns no clock.
		l.sender.ResendCheckpoint()

	case "settle":
		// Run both ends until neither has anything left to say.
		if err := l.pump(-1); err != nil {
			return nil, nil, err
		}

	default:
		return nil, nil, fmt.Errorf("unknown LTP call %q", call)
	}

	return nil, l.state(), nil
}

// move takes one segment from the sender and either delivers or drops it.
func (l *link) move(drop bool) (bool, error) {
	seg, err := l.fromSender()
	if err != nil || seg == nil {
		return false, err
	}
	if drop {
		l.lost++
		return true, nil
	}
	return true, l.deliver(seg, l.receiver.HandleSegment)
}

// deliver puts a segment through encode and decode before handing it over, so
// the exchange is tested across the wire format rather than by passing structs.
func (l *link) deliver(seg *ltp.Segment, to func(*ltp.Segment) error) error {
	encoded, err := seg.Encode()
	if err != nil {
		return err
	}
	decoded, err := ltp.DecodeSegment(encoded)
	if err != nil {
		return err
	}
	return to(decoded)
}

// fromSender takes the pending sender segment, pulling a fresh one if the
// slot is empty. It returns nil when the sender has nothing to say.
func (l *link) fromSender() (*ltp.Segment, error) {
	if l.senderPending == nil {
		seg, ok, err := l.sender.NextSegment()
		if err != nil || !ok {
			return nil, err
		}
		l.senderPending = seg
	}
	seg := l.senderPending
	l.senderPending = nil
	return seg, nil
}

// fromReceiver is the same for the receiver's side.
func (l *link) fromReceiver() (*ltp.Segment, error) {
	if l.receiverPending == nil {
		seg, ok, err := l.receiver.NextSegment()
		if err != nil || !ok {
			return nil, err
		}
		l.receiverPending = seg
	}
	seg := l.receiverPending
	l.receiverPending = nil
	return seg, nil
}

// pump runs both ends until neither produces a segment, or until limit
// segments have crossed. A negative limit means no limit.
func (l *link) pump(limit int) error {
	for moved := 0; limit < 0 || moved < limit; moved++ {
		progressed := false

		seg, err := l.fromSender()
		if err != nil {
			return err
		}
		if seg != nil {
			if err := l.deliver(seg, l.receiver.HandleSegment); err != nil {
				return err
			}
			progressed = true
		}

		seg, err = l.fromReceiver()
		if err != nil {
			return err
		}
		if seg != nil {
			if seg.Report != nil {
				l.reports++
			}
			if err := l.deliver(seg, l.sender.HandleSegment); err != nil {
				return err
			}
			progressed = true
		}

		if !progressed {
			return nil
		}
	}
	return nil
}

// pumpDroppingLast delivers every sender segment but the last, which is the
// checkpoint closing the red part.
func (l *link) pumpDroppingLast() error {
	var pending []*ltp.Segment
	for {
		seg, err := l.fromSender()
		if err != nil {
			return err
		}
		if seg == nil {
			break
		}
		pending = append(pending, seg)
	}
	if len(pending) == 0 {
		return nil
	}
	for _, seg := range pending[:len(pending)-1] {
		if err := l.deliver(seg, l.receiver.HandleSegment); err != nil {
			return err
		}
	}
	l.lost++
	return nil
}

func (l *link) state() vectors.Fields {
	// Fill the pending slots so "has output" can be answered without
	// consuming what it finds.
	if l.senderPending == nil {
		if seg, ok, err := l.sender.NextSegment(); err == nil && ok {
			l.senderPending = seg
		}
	}
	if l.receiverPending == nil {
		if seg, ok, err := l.receiver.NextSegment(); err == nil && ok {
			l.receiverPending = seg
		}
	}

	return vectors.Fields{
		"red_part_complete":       l.receiver.RedPartComplete(),
		"sender_done":             l.sender.Done(),
		"segments_lost":           uint64(l.lost),
		"reports_sent":            uint64(l.reports),
		"receiver_missing_ranges": uint64(len(l.receiver.MissingRanges())),
		"sender_has_output":       l.senderPending != nil,
		"receiver_has_output":     l.receiverPending != nil,
		"block_identical":         bytes.Equal(l.receiver.RedPart(), l.block[:len(l.receiver.RedPart())]),
	}
}
