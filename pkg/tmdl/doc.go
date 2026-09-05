// Package tmdl implements CCSDS 132.0-B-3, the TM Space Data Link Protocol.
//
// TM carries telemetry from a spacecraft to the ground in fixed-length
// transfer frames. The frame length is chosen once per physical channel
// and never changes. Inside a frame you usually find Space Packets (see
// pkg/spp), packed end to end and spanning frame boundaries when they are
// too big to fit. One spacecraft owns a master channel, which is split
// into up to eight virtual channels that share the downlink. See
// pkg/stack for how this sits under the packet layer and over the coding
// layer.
//
// # What it implements
//
// The transfer frame format, and all three services the standard
// defines: VCP and VCF (clause 3.4), and VCA. Master and virtual channel
// multiplexing, frame gap detection, and idle frame generation with the
// mandatory PN fill are all here. ChannelConfig.Validate also enforces
// the ECSS-E-ST-50-03C 2048-octet frame ceiling, for missions that opt
// into it — a CCSDS-only mission may legitimately run longer frames, so
// this check is not applied for you.
//
// # What is somewhere else
//
// The sync layer is not here: Attached Sync Markers, pseudo-randomization,
// and CADU wrapping live in pkg/tmsc. PhysicalChannel does master channel
// multiplexing and nothing below it.
//
// # What it leaves to you
//
// Secondary header contents, and the CLCW placed in the Operational
// Control Field. pkg/cop builds CLCWs if you need one.
package tmdl
