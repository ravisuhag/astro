// Package epp implements CCSDS 133.1-B-3, the Encapsulation Packet Protocol.
//
// EPP wraps data that is not a Space Packet so it can travel on a CCSDS
// data link: an IP datagram, an LTP segment, anything that already carries
// its own addressing. The header is 1, 2, 4, or 8 octets, decided entirely
// by the 2-bit Length of Length field in its first octet, and does almost
// nothing except say what protocol is inside. Its first three bits are
// always 111, where a Space Packet's are 000, which is what lets both
// share one data link. See pkg/spp for the packet format EPP sits beside.
//
// # What it implements
//
// All four header sizes, every Protocol ID from clause 4.1.2.3 and the
// SANA Encapsulation Protocol ID registry including the extension
// mechanism, idle packets in both the 1-octet and fill forms, and a
// service layer over any io.ReadWriter transport.
//
// # What it leaves to you
//
// What is inside. EPP identifies the payload protocol (LTP, an IP
// extension, a mission-specific value, or a privately-assigned extended
// ID) and gets out of the way; parsing or interpreting that payload is the
// caller's job.
package epp
