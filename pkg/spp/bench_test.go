package spp_test

import (
	"strconv"
	"testing"

	"github.com/ravisuhag/astro/pkg/spp"
)

// Benchmarks for the packet paths a ground station runs at packet rate.
//
// A downlink carries far more packets than frames — several packets fit in
// one frame — so this loop runs more often than the frame one.
//
// Run with:
//
//	go test -bench . -benchmem ./pkg/spp/

var sink []byte

// benchPayload is a typical housekeeping payload: small, fixed.
func benchPayload(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i)
	}
	return data
}

func BenchmarkEncodePacket(b *testing.B) {
	for _, size := range []int{16, 256, 4096} {
		b.Run(sizeName(size), func(b *testing.B) {
			packet, err := spp.NewTMPacket(100, benchPayload(size))
			if err != nil {
				b.Fatal(err)
			}

			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				encoded, err := packet.Encode()
				if err != nil {
					b.Fatal(err)
				}
				sink = encoded
			}
		})
	}
}

// With the error control field, which costs a CRC over the whole packet.
func BenchmarkEncodePacketWithCRC(b *testing.B) {
	for _, size := range []int{16, 256, 4096} {
		b.Run(sizeName(size), func(b *testing.B) {
			packet, err := spp.NewTMPacket(100, benchPayload(size), spp.WithErrorControl())
			if err != nil {
				b.Fatal(err)
			}

			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				encoded, err := packet.Encode()
				if err != nil {
					b.Fatal(err)
				}
				sink = encoded
			}
		})
	}
}

func BenchmarkDecodePacket(b *testing.B) {
	for _, size := range []int{16, 256, 4096} {
		b.Run(sizeName(size), func(b *testing.B) {
			packet, err := spp.NewTMPacket(100, benchPayload(size))
			if err != nil {
				b.Fatal(err)
			}
			encoded, err := packet.Encode()
			if err != nil {
				b.Fatal(err)
			}

			b.SetBytes(int64(len(encoded)))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if _, err := spp.Decode(encoded); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// PacketSizer is what a streaming reader calls on every packet boundary to
// learn how far the next one runs, so it is on the hottest path of all.
func BenchmarkPacketSizer(b *testing.B) {
	packet, err := spp.NewTMPacket(100, benchPayload(256))
	if err != nil {
		b.Fatal(err)
	}
	encoded, err := packet.Encode()
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	total := 0
	for i := 0; i < b.N; i++ {
		total += spp.PacketSizer(encoded)
	}
	if total == 0 {
		b.Fatal("the sizer never answered")
	}
}

func sizeName(size int) string {
	return strconv.Itoa(size) + "-octets"
}
