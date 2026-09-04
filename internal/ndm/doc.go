// Package ndm holds what the CCSDS navigation data messages have in common.
//
// Four standards define these messages, and astro gives each its own package:
// orbit data (CCSDS 502.0-B-3, pkg/odm), tracking data (CCSDS 503.0-B-2,
// pkg/tdm), attitude data (CCSDS 504.0-B-2, pkg/adm) and conjunction data
// (CCSDS 508.0-B-1, pkg/cdm). They revise on separate schedules and a caller
// who wants one should not have to import the other three.
//
// What they share is real, though. All four are plain ASCII files built from
// 'keyword = value' lines. All four open with the same header keywords. All
// four write times in the same two formats, take the same optional unit
// suffix, and follow the same rules about what counts as a blank line and
// which characters may appear. CCSDS 505.0-B-3 then defines one XML container
// that any of them can be carried in.
//
// This package carries that common ground: the line syntax, the value
// formats, and the header. It lives in internal because it carries no API
// commitment of its own — the shape it should take is whatever the four
// packages above need.
//
// # What it does not decide
//
// It does not say which header fields are mandatory. That differs: MESSAGE_ID
// is optional in an orbit message and mandatory in a conjunction message,
// CLASSIFICATION exists only in orbit and attitude messages, and MESSAGE_FOR
// only in conjunction messages. Each package validates its own standard's
// table, because that table is the thing the standard is.
//
// It does not know any keyword beyond the header. Every other keyword belongs
// to a message type, and the tables that define them are what the four
// packages are made of.
package ndm
