package cfdp_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/cfdp"
)

// The wire vectors for this package live in vectors/cfdp/. They cover the
// checksum and the LV/TLV encodings. The transaction machines need a
// sequence of calls, which no vector kind expresses.
//
// The annex F checksum is one of very few genuinely published CCSDS test
// vectors, so it leads the file.

func TestWireVectors(t *testing.T) {
	vectors.RunFile(t, "cfdp/wire.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			// A checksum vector carries a file rather than a kind.
			if f.Has("file") {
				file, err := f.Hex("file")
				if err != nil {
					return nil, err
				}
				c, err := cfdp.NewChecksum(cfdp.ChecksumModular)
				if err != nil {
					return nil, err
				}
				c.Update(0, file)
				out := make([]byte, 4)
				binary.BigEndian.PutUint32(out, c.Sum())
				return out, nil
			}

			kind, err := f.Str("kind")
			if err != nil {
				return nil, err
			}
			value, err := f.Hex("value")
			if err != nil {
				return nil, err
			}
			switch kind {
			case "lv":
				return cfdp.LV{Value: value}.Encode()
			case "tlv":
				typ, err := f.Uint("type")
				if err != nil {
					return nil, err
				}
				return cfdp.TLV{Type: cfdp.TLVType(typ), Value: value}.Encode()
			default:
				return nil, fmt.Errorf("unknown kind %q", kind)
			}
		},

		DecodeFn: func(input []byte, config vectors.Fields) (vectors.Fields, error) {
			// Which encoding is present is prior agreement, not wire
			// content: both LV and TLV open with an octet that could be a
			// length or a type, so the octets alone cannot say. Guessing
			// from them reads a TLV as an LV, which is why kind lives in
			// config.
			kind, err := config.Str("kind")
			if err != nil {
				return nil, err
			}
			switch kind {
			case "lv":
				lv, n, err := cfdp.DecodeLV(input)
				if err != nil {
					return nil, err
				}
				return vectors.Fields{"value": lv.Value, "consumed": n}, nil
			case "tlv":
				tlv, n, err := cfdp.DecodeTLV(input)
				if err != nil {
					return nil, err
				}
				return vectors.Fields{
					"type": uint8(tlv.Type), "value": tlv.Value, "consumed": n,
				}, nil
			default:
				return nil, fmt.Errorf("unknown kind %q", kind)
			}
		},
	})
}

// TestPDUInteropVectors runs PDUs captured from spacepackets, a Python
// implementation written from the standards by other authors.
//
// The fixed header packs six single-bit flags into its first octet, and two of
// them change how a PDU is handled without changing how it parses. The
// transmission mode bit is the sharper one, because table 5-1 inverts it: '0'
// means acknowledged. An implementation using the obvious sense marks every
// acknowledged transaction unacknowledged, works perfectly against itself, and
// confuses anything else.
func TestPDUInteropVectors(t *testing.T) {
	vectors.RunFile(t, "cfdp/interop.json", vectors.Impl{
		EncodeFn: encodeInteropPDU,
	})
}

// encodeInteropPDU builds whichever PDU the vector's config names, wraps it in
// a fixed header and encodes the whole thing.
func encodeInteropPDU(f, config vectors.Fields) ([]byte, error) {
	structure, err := config.Str("structure")
	if err != nil {
		return nil, err
	}
	direction, err := config.Uint("direction")
	if err != nil {
		return nil, err
	}
	acknowledged, err := config.Bool("acknowledged")
	if err != nil {
		return nil, err
	}
	crcFlag, err := config.BoolOr("crc_flag", false)
	if err != nil {
		return nil, err
	}

	body, isFileData, err := interopBody(structure, f)
	if err != nil {
		return nil, err
	}

	header := &cfdp.PDUHeader{
		IsFileData:     isFileData,
		Direction:      cfdp.Direction(direction),
		Acknowledged:   acknowledged,
		CRCFlag:        crcFlag,
		DataLength:     uint16(len(body)),
		Source:         cfdp.NewEntityID(1),
		Destination:    cfdp.NewEntityID(2),
		TransactionSeq: cfdp.NewEntityID(7),
	}
	pdu := &cfdp.PDU{Header: header, Data: body}
	return pdu.Encode()
}

func interopBody(structure string, f vectors.Fields) (body []byte, isFileData bool, err error) {
	switch structure {
	case "metadata":
		checksumType, err := f.Uint("checksum_type")
		if err != nil {
			return nil, false, err
		}
		fileSize, err := f.Uint("file_size")
		if err != nil {
			return nil, false, err
		}
		source, err := f.Hex("source_file_name")
		if err != nil {
			return nil, false, err
		}
		dest, err := f.Hex("destination_file_name")
		if err != nil {
			return nil, false, err
		}
		m := &cfdp.MetadataPDU{
			ChecksumType:        uint8(checksumType),
			FileSize:            fileSize,
			SourceFileName:      cfdp.LV{Value: source},
			DestinationFileName: cfdp.LV{Value: dest},
		}
		body, err = m.Encode(false)
		return body, false, err

	case "eof":
		code, err := f.Uint("condition_code")
		if err != nil {
			return nil, false, err
		}
		checksum, err := f.Uint("file_checksum")
		if err != nil {
			return nil, false, err
		}
		fileSize, err := f.Uint("file_size")
		if err != nil {
			return nil, false, err
		}
		e := &cfdp.EOFPDU{
			ConditionCode: cfdp.ConditionCode(code),
			FileChecksum:  uint32(checksum),
			FileSize:      fileSize,
		}
		body, err = e.Encode(false)
		return body, false, err

	case "nak":
		start, err := f.Uint("start_of_scope")
		if err != nil {
			return nil, false, err
		}
		end, err := f.Uint("end_of_scope")
		if err != nil {
			return nil, false, err
		}
		reqStart, err := f.Uint("request_start")
		if err != nil {
			return nil, false, err
		}
		reqEnd, err := f.Uint("request_end")
		if err != nil {
			return nil, false, err
		}
		n := &cfdp.NAKPDU{
			StartOfScope: start,
			EndOfScope:   end,
			Requests:     []cfdp.SegmentRequest{{StartOffset: reqStart, EndOffset: reqEnd}},
		}
		body, err = n.Encode(false)
		return body, false, err

	case "file_data":
		offset, err := f.Uint("offset")
		if err != nil {
			return nil, false, err
		}
		data, err := f.Hex("file_data")
		if err != nil {
			return nil, false, err
		}
		fd := &cfdp.FileDataPDU{Offset: offset, Data: data}
		body, err = fd.Encode(false, false)
		return body, true, err
	}
	return nil, false, fmt.Errorf("unknown CFDP structure %q", structure)
}
