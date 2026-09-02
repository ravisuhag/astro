package cfdp

import "errors"

// Sentinel errors returned by the CFDP protocol machinery.
var (
	// ErrDataTooShort indicates the input ended before a field it must contain.
	ErrDataTooShort = errors.New("data too short for the PDU field being read")

	// ErrInvalidVersion indicates a PDU version other than the '001' of
	// CCSDS 727.0-B-5 table 5-1.
	ErrInvalidVersion = errors.New("invalid PDU version: this implementation speaks version 1")

	// ErrInvalidEntityIDWidth indicates an entity ID or transaction sequence
	// number width outside the 1-to-8 octet range the 3-bit length field allows.
	ErrInvalidEntityIDWidth = errors.New("invalid entity ID or sequence number width: must be 1 to 8 octets")

	// ErrEntityIDOverflow indicates a value too large for its declared width.
	ErrEntityIDOverflow = errors.New("entity ID or sequence number does not fit its declared width")

	// ErrValueTooLong indicates an LV or TLV value beyond the 255 octets its
	// 8-bit length field can describe.
	ErrValueTooLong = errors.New("LV or TLV value exceeds 255 octets")

	// ErrInvalidDirectiveCode indicates a reserved or unrecognized directive code.
	ErrInvalidDirectiveCode = errors.New("invalid or reserved file directive code")

	// ErrWrongDirectiveCode indicates a decode call for one directive was
	// handed the bytes of another.
	ErrWrongDirectiveCode = errors.New("directive code does not match the PDU being decoded")

	// ErrNotFileDirective indicates a file directive operation on a File Data PDU.
	ErrNotFileDirective = errors.New("PDU is file data, not a file directive")

	// ErrNotFileData indicates a file data operation on a File Directive PDU.
	ErrNotFileData = errors.New("PDU is a file directive, not file data")

	// ErrCRCMismatch indicates the optional PDU CRC failed validation, so the
	// PDU must be discarded per clause 4.1.2.
	ErrCRCMismatch = errors.New("PDU CRC mismatch: received CRC does not match computed CRC")

	// ErrDataLengthMismatch indicates the header's PDU data field length does
	// not match the bytes supplied.
	ErrDataLengthMismatch = errors.New("PDU data field length does not match the data supplied")

	// ErrUnsupportedChecksumType indicates a checksum algorithm this
	// implementation does not provide.
	ErrUnsupportedChecksumType = errors.New("unsupported checksum type")

	// ErrChecksumFailure indicates the received file did not match the
	// checksum in its EOF PDU.
	ErrChecksumFailure = errors.New("file checksum failure")

	// ErrFileSizeError indicates received file data extends beyond the size
	// declared in the EOF PDU.
	ErrFileSizeError = errors.New("file size error: data extends past the declared file size")

	// ErrInvalidTransmissionMode indicates an operation that the transaction's
	// class does not support.
	ErrInvalidTransmissionMode = errors.New("invalid transmission mode for this operation")

	// ErrTransactionFinished indicates the transaction has already completed.
	ErrTransactionFinished = errors.New("transaction has already finished")

	// ErrSuspended indicates the transaction is suspended and will emit
	// nothing until resumed.
	ErrSuspended = errors.New("transaction is suspended")

	// ErrFileNotFound indicates the filestore has no such file.
	ErrFileNotFound = errors.New("file not found in the filestore")

	// ErrFilestoreRejection indicates the filestore refused the operation.
	ErrFilestoreRejection = errors.New("filestore rejected the operation")

	// ErrUnsupportedAction indicates a filestore action code this
	// implementation does not execute.
	ErrUnsupportedAction = errors.New("unsupported filestore action")

	// ErrSegmentTooLarge indicates segment metadata beyond the 63 octets its
	// 6-bit length field can describe.
	ErrSegmentTooLarge = errors.New("segment metadata exceeds 63 octets")

	// ErrInvalidFaultHandler indicates a fault handler override TLV whose
	// handler code is not one of the four defined by clause 5.4.4.
	ErrInvalidFaultHandler = errors.New("invalid fault handler code")

	// ErrNotUserMessage indicates a Message to User TLV that does not open
	// with the "cfdp" identifier, so it is an application message rather than
	// a Reserved CFDP Message. It is how a receiver tells the two apart, not
	// a malformed protocol message.
	ErrNotUserMessage = errors.New("message to user is not a Reserved CFDP Message")

	// ErrReservedBitsSet indicates a field the standard requires to be zero
	// that is not. A sender setting one is using something this issue has not
	// defined, and reading the rest of the field out from under it would be a
	// guess.
	ErrReservedBitsSet = errors.New("a field reserved for future use is not zero")
)
