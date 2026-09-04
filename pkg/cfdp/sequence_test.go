package cfdp_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/cfdp"
)

// The sequence vectors in vectors/cfdp/transaction.json drive a sender and a
// receiver across a link that can drop or alter PDUs.
//
// PDU octets are pinned in the other cfdp vector files. What these pin is the
// transaction: what a NAK means, what class 1 does not promise, and what the
// checksum catches that no length check can. A file transfer is a sequence by
// nature, so an input-and-output pair has nowhere to put it.
func TestTransactionSequenceVectors(t *testing.T) {
	vectors.RunFile(t, "cfdp/transaction.json", vectors.Impl{MachineFn: newTransferMachine})
}

const (
	sourceName = "downlink.dat"
	destName   = "uplink.dat"
)

// transfer is a sender, a receiver, their two filestores, and the one-way path
// between them. PDUs cross only when a step says so.
type transfer struct {
	sender   *cfdp.Sender
	receiver *cfdp.Receiver
	destFS   *cfdp.MemoryFilestore
	file     []byte

	lost      int
	corrupted int
	naks      int

	// pending holds one PDU already pulled from the sender, so a step can ask
	// whether the sender has more without consuming what it finds.
	pending *cfdp.PDU
}

func newTransferMachine(init, config vectors.Fields) (vectors.Machine, error) {
	acknowledged, err := config.Bool("acknowledged")
	if err != nil {
		return nil, err
	}
	segmentSize, err := config.Uint("segment_size")
	if err != nil {
		return nil, err
	}
	file, err := config.Hex("file")
	if err != nil {
		return nil, err
	}

	sourceFS := cfdp.NewMemoryFilestore()
	if err := sourceFS.WriteAt(sourceName, 0, file); err != nil {
		return nil, err
	}
	destFS := cfdp.NewMemoryFilestore()

	src, dst, seq := cfdp.NewEntityID(1), cfdp.NewEntityID(2), cfdp.NewEntityID(7)

	sender, err := cfdp.NewSender(sourceFS, cfdp.SenderConfig{
		Source:              src,
		Destination:         dst,
		TransactionSeq:      seq,
		Acknowledged:        acknowledged,
		SegmentSize:         int(segmentSize),
		SourceFileName:      sourceName,
		DestinationFileName: destName,
	})
	if err != nil {
		return nil, err
	}
	receiver := cfdp.NewReceiver(destFS, cfdp.ReceiverConfig{
		Source:         src,
		Destination:    dst,
		TransactionSeq: seq,
		Acknowledged:   acknowledged,
	})

	return &transfer{sender: sender, receiver: receiver, destFS: destFS, file: file}, nil
}

func (x *transfer) Step(call string, fields vectors.Fields) ([]byte, vectors.Fields, error) {
	switch call {
	case "send":
		if _, err := x.moveOne(deliver); err != nil {
			return nil, nil, err
		}

	case "drop":
		if _, err := x.moveOne(discard); err != nil {
			return nil, nil, err
		}

	case "corrupt_one_data_pdu":
		if err := x.corruptOneDataPDU(); err != nil {
			return nil, nil, err
		}

	case "settle":
		if err := x.settle(); err != nil {
			return nil, nil, err
		}

	default:
		return nil, nil, fmt.Errorf("unknown CFDP call %q", call)
	}

	return nil, x.state(), nil
}

type disposition int

const (
	deliver disposition = iota
	discard
	corrupt
)

// moveOne takes the next sender PDU and disposes of it as told.
func (x *transfer) moveOne(how disposition) (bool, error) {
	pdu, err := x.fromSender()
	if err != nil || pdu == nil {
		return false, err
	}

	switch how {
	case discard:
		x.lost++
		return true, nil

	case corrupt:
		// Flip a file octet. The PDU keeps its length and its offset, so
		// only the checksum can notice.
		if n := len(pdu.Data); n > 0 {
			pdu.Data[n-1] ^= 0xFF
			x.corrupted++
		}
	}

	return true, x.cross(pdu, x.receiver.HandlePDU)
}

// corruptOneDataPDU delivers PDUs until it has altered one file data PDU,
// leaving the rest of the transfer intact.
func (x *transfer) corruptOneDataPDU() error {
	for {
		pdu, err := x.fromSender()
		if err != nil {
			return err
		}
		if pdu == nil {
			return nil
		}

		// The last octet is file content; the offset sits at the front, so
		// altering the tail keeps the PDU landing where it should.
		if n := len(pdu.Data); x.corrupted == 0 && pdu.Header.IsFileData && n > 0 {
			pdu.Data[n-1] ^= 0xFF
			x.corrupted++
		}
		if err := x.cross(pdu, x.receiver.HandlePDU); err != nil {
			return err
		}
		if x.corrupted > 0 {
			return nil
		}
	}
}

// cross puts a PDU through encode and decode before handing it over, so the
// exchange runs across the wire format rather than by passing structs.
func (x *transfer) cross(pdu *cfdp.PDU, to func(*cfdp.PDU) error) error {
	encoded, err := pdu.Encode()
	if err != nil {
		return err
	}
	decoded, err := cfdp.DecodePDU(encoded)
	if err != nil {
		return err
	}
	return to(decoded)
}

func (x *transfer) fromSender() (*cfdp.PDU, error) {
	if x.pending == nil {
		pdu, ok, err := x.sender.NextPDU()
		if err != nil || !ok {
			return nil, err
		}
		x.pending = pdu
	}
	pdu := x.pending
	x.pending = nil
	return pdu, nil
}

// settle runs both ends until neither produces a PDU.
func (x *transfer) settle() error {
	for {
		progressed := false

		pdu, err := x.fromSender()
		if err != nil {
			return err
		}
		if pdu != nil {
			if err := x.cross(pdu, x.receiver.HandlePDU); err != nil {
				return err
			}
			progressed = true
		}

		reply, ok, err := x.receiver.NextPDU()
		if err != nil {
			return err
		}
		if ok {
			if !reply.Header.IsFileData && len(reply.Data) > 0 &&
				cfdp.DirectiveCode(reply.Data[0]) == cfdp.DirectiveNAK {
				x.naks++
			}
			if err := x.cross(reply, x.sender.HandlePDU); err != nil {
				return err
			}
			progressed = true
		}

		if !progressed {
			return nil
		}
	}
}

func (x *transfer) state() vectors.Fields {
	received, _ := x.destFS.Read(destName)

	return vectors.Fields{
		"sender_done":      x.sender.Done(),
		"receiver_done":    x.receiver.Done(),
		"file_identical":   bytes.Equal(received, x.file),
		"pdus_lost":        uint64(x.lost),
		"pdus_corrupted":   uint64(x.corrupted),
		"naks_sent":        uint64(x.naks),
		"checksum_failure": x.receiver.ConditionCode() == cfdp.CondFileChecksumFailure,
	}
}
