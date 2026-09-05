// Package tcdl implements CCSDS 232.0-B-4, the TC Space Data Link Protocol.
//
// TC carries commands from the ground to a spacecraft. Frames are
// variable length, up to 1024 octets, because a short command should not
// have to pay for padding to a fixed size. The design goal is different
// from TM (see pkg/tmdl): a wrong or missing command can end a mission, so
// TC is built for reliability rather than throughput. Lost frames are
// detected and recovered by COP-1 (see pkg/cop), sitting directly on top
// of this layer.
//
// # What it implements
//
// The transfer frame format, the MAP sublayer with segmentation, all
// three services the standard defines (MAP Packet, MAP Access, and VC
// Frame), and the Type-BC control command frames — Unlock and Set V(R) —
// that COP-1 needs to operate.
//
// # What is somewhere else
//
// Retransmission logic is pkg/cop. CLTU wrapping and BCH coding are
// pkg/tcsc. This package only builds and parses frames.
package tcdl
