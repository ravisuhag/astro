package stack_test

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/stack"
	"github.com/ravisuhag/astro/pkg/tmdl"
	"github.com/ravisuhag/astro/pkg/tmsc"
)

// The sequence vectors in vectors/stack/downlink.json compose a sender and a
// receiver from one configuration and run packets through both.
//
// Every layer underneath has its own wire vectors. What these pin is the
// ordering the multiplexer imposes: a per-channel frame count, priority
// between channels with frames ready, and a flush that must emit the same
// frames in the same order on every run. None of that fits an input and output
// pair, because all three are statements about sequence rather than shape.
func TestDownlinkSequenceVectors(t *testing.T) {
	vectors.RunFile(t, "stack/downlink.json", vectors.Impl{MachineFn: newDownlinkMachine})
}

// downlink holds both ends and the CADUs in flight between them.
type downlink struct {
	sender   *stack.Sender
	receiver *stack.Receiver
	config   stack.Downlink

	cadus [][]byte
	// sent records what went in, per channel, so the far end can be compared
	// against it rather than against a restatement of the same bytes.
	sent     map[uint8][][]byte
	received map[uint8][][]byte
}

func newDownlinkMachine(init, config vectors.Fields) (vectors.Machine, error) {
	scid, err := config.Uint("spacecraft_id")
	if err != nil {
		return nil, err
	}
	frameLength, err := config.Uint("frame_length")
	if err != nil {
		return nil, err
	}
	fecf, err := config.BoolOr("fecf", false)
	if err != nil {
		return nil, err
	}
	randomize, err := config.BoolOr("randomize", false)
	if err != nil {
		return nil, err
	}
	spec, err := config.Str("channels")
	if err != nil {
		return nil, err
	}
	channels, err := parseChannels(spec)
	if err != nil {
		return nil, err
	}

	cfg := stack.Downlink{
		SpacecraftID: uint16(scid),
		FrameLength:  int(frameLength),
		FECF:         fecf,
		Randomize:    randomize,
		Channels:     channels,
	}
	sender, err := stack.NewSender(cfg)
	if err != nil {
		return nil, err
	}
	receiver, err := stack.NewReceiver(cfg)
	if err != nil {
		return nil, err
	}
	return &downlink{
		sender: sender, receiver: receiver, config: cfg,
		sent: map[uint8][][]byte{}, received: map[uint8][][]byte{},
	}, nil
}

// parseChannels reads "0,1,2" or "0:0,1:5,2:9", the second form giving each
// channel a priority.
func parseChannels(spec string) ([]stack.VC, error) {
	var out []stack.VC
	for _, part := range strings.Split(spec, ",") {
		id, priority, found := strings.Cut(part, ":")
		vcid, err := strconv.Atoi(strings.TrimSpace(id))
		if err != nil {
			return nil, fmt.Errorf("bad channel %q: %w", part, err)
		}
		vc := stack.VC{ID: uint8(vcid)}
		if found {
			p, err := strconv.Atoi(strings.TrimSpace(priority))
			if err != nil {
				return nil, fmt.Errorf("bad priority in %q: %w", part, err)
			}
			vc.Priority = p
		}
		out = append(out, vc)
	}
	return out, nil
}

func (d *downlink) Step(call string, fields vectors.Fields) ([]byte, vectors.Fields, error) {
	switch call {
	case "send":
		vcid, err := fields.Uint("vcid")
		if err != nil {
			return nil, nil, err
		}
		packet, err := fields.Hex("packet")
		if err != nil {
			return nil, nil, err
		}
		if err := d.sender.Send(uint8(vcid), packet); err != nil {
			return nil, nil, err
		}
		d.sent[uint8(vcid)] = append(d.sent[uint8(vcid)], packet)
		d.drain()

	case "send_many":
		// Several packets at once, each large enough to fill a frame, so a
		// channel ends up with frames queued rather than one part-filled one.
		// Priority only shows when more than one channel is backed up.
		vcid, err := fields.Uint("vcid")
		if err != nil {
			return nil, nil, err
		}
		count, err := fields.Uint("count")
		if err != nil {
			return nil, nil, err
		}
		size, err := fields.Uint("packet_size")
		if err != nil {
			return nil, nil, err
		}
		for i := uint64(0); i < count; i++ {
			packet := make([]byte, size)
			packet[0] = byte(vcid)
			packet[1] = byte(i)
			if err := d.sender.Send(uint8(vcid), packet); err != nil {
				return nil, nil, err
			}
		}

	case "flush":
		if err := d.sender.Flush(); err != nil {
			return nil, nil, err
		}
		d.drain()

	case "deliver_all":
		for _, cadu := range d.cadus {
			if err := d.receiver.Accept(cadu); err != nil {
				return nil, nil, err
			}
		}
		d.cadus = nil
		if err := d.collect(); err != nil {
			return nil, nil, err
		}

	default:
		return nil, nil, fmt.Errorf("unknown stack call %q", call)
	}

	return nil, d.state(), nil
}

// drain takes every CADU the sender has ready, in the order it produces them.
// That order is what the flush vectors assert.
func (d *downlink) drain() {
	for {
		cadu, ok, err := d.sender.NextCADU()
		if err != nil || !ok {
			return
		}
		d.cadus = append(d.cadus, cadu)
	}
}

// collect pulls the packets out of the receiver, per channel.
func (d *downlink) collect() error {
	for _, channel := range d.config.Channels {
		for {
			packet, ok, err := d.receiver.Next(channel.ID)
			if err != nil {
				return err
			}
			if !ok {
				break
			}
			d.received[channel.ID] = append(d.received[channel.ID], packet)
		}
	}
	return nil
}

func (d *downlink) state() vectors.Fields {
	f := vectors.Fields{
		"cadus_ready":     uint64(len(d.cadus)),
		"cadu_vcid_order": d.caduVCIDOrder(),
	}
	for _, channel := range d.config.Channels {
		f[fmt.Sprintf("received_on_vc%d", channel.ID)] = uint64(len(d.received[channel.ID]))
	}
	f["all_packets_identical"] = d.packetsMatch()
	return f
}

// caduVCIDOrder reads the virtual channel out of each waiting CADU's frame
// header, so a vector can assert the order without knowing the frame bytes.
func (d *downlink) caduVCIDOrder() string {
	var ids []string
	for _, cadu := range d.cadus {
		// Past the sync marker, a TM frame header starts the CADU.
		frame := cadu[len(tmsc.DefaultASM()):]
		if d.config.Randomize {
			// Randomized frames have to be de-randomized before the header
			// reads as anything.
			frame = tmsc.Randomize(append([]byte(nil), frame...))
		}
		var h tmdl.PrimaryHeader
		if err := h.Decode(frame); err != nil {
			ids = append(ids, "?")
			continue
		}
		ids = append(ids, strconv.Itoa(int(h.VirtualChannelID)))
	}
	return strings.Join(ids, ",")
}

func (d *downlink) packetsMatch() bool {
	for vcid, want := range d.sent {
		got := d.received[vcid]
		if len(got) != len(want) {
			return false
		}
		for i := range want {
			if !bytes.Equal(got[i], want[i]) {
				return false
			}
		}
	}
	return true
}
