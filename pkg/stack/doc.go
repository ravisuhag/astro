// Package stack wires the protocol layers into a working pipeline.
//
// Every other package in this repository implements one standard well and
// leaves composition to the caller. That is the right shape for a protocol
// library, but it means a working downlink is about forty lines of setup:
// a channel configuration, a physical channel, a master channel, a virtual
// channel and a packet service for each stream, a shared frame counter, a
// packet sizer on every service, and then the CADU wrapping. The ground side
// is the same forty lines again.
//
// The second forty lines are the problem. The two ends have to agree on the
// frame length, on whether there is a frame error control field, on whether
// the frames are randomized, and on which virtual channels exist. Nothing
// checks that they do. A ground station configured with a frame length two
// octets different from the spacecraft's decodes nothing, and the failure
// looks like a bad link rather than a typo.
//
// So this package takes one configuration value and builds both ends from it:
//
//	cfg := stack.Downlink{
//	    SpacecraftID: 42,
//	    FrameLength:  1115,
//	    FECF:         true,
//	    Channels: []stack.VC{
//	        {ID: 0, Priority: 3}, // housekeeping
//	        {ID: 1, Priority: 1}, // science
//	    },
//	}
//
//	sender, err := stack.NewSender(cfg)
//	...
//	sender.Send(0, packet)
//	sender.Flush()
//	for cadu, err := range sender.CADUs() { ... }
//
//	receiver, err := stack.NewReceiver(cfg) // the same cfg
//	...
//	receiver.Accept(cadu)
//	packet, ok, err := receiver.Next(0)
//
// The layers are still there and still separately usable. This is a
// convenience over them, not a replacement: anything the composer cannot
// express is a reason to drop to the packages underneath, not a reason to
// grow this one until it can express everything.
//
// # What it composes
//
// Downlink is packets to CADUs and back: Space Packets (CCSDS 133.0-B-2)
// through TM Transfer Frames (132.0-B-3) into Channel Access Data Units
// (131.0-B-5), with the frame error control field and pseudo-randomization
// as configured.
//
// Uplink is packets to CLTUs and back: Space Packets through TC Transfer
// Frames (232.0-B-4) into Command Link Transmission Units (231.0-B-4), with
// COP-1 (232.1-B-2) providing sequence-controlled delivery.
//
// The uplink is deliberately not the downlink backwards. Commanding is a
// conversation: FOP-1 on the ground will not send past its sliding window
// until a CLCW comes back on the telemetry link saying what the spacecraft
// accepted. So a Commander sends packets and accepts CLCWs, while an Onboard
// accepts frames and produces them — asymmetric in a way the two downlink
// ends are not. See uplink.go for why that shapes the API.
//
// Reed-Solomon is deliberately absent. CCSDS 131.0 puts the codeblock between
// the frame and the sync marker, and a caller who wants it can run
// pkg/tmsc over the encoded frame before handing the octets on — but the
// interleaving depth and the shortened-codeblock choices are real decisions
// that a composer guessing at them would get wrong.
package stack
