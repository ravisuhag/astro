.PHONY: build test race lint vet cover bench fuzz-smoke

build:
	go build ./...

test:
	go test ./...

race:
	go test -race ./...

cover:
	go test -race -covermode=atomic -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

vet:
	go vet ./...

# The paths a ground station runs at frame rate: packets, frames, the coding
# layers underneath them, and the compressors.
#
# -count 3 because a laptop's clock speed is not constant and one sample of a
# few-microsecond operation says very little; compare medians, not single runs.
BENCHTIME ?= 2s
BENCHPKGS ?= ./pkg/crc/ ./pkg/spp/ ./pkg/tmdl/ ./pkg/tcdl/ ./pkg/aos/ ./pkg/usdl/ ./pkg/tmsc/ ./pkg/tcsc/ ./pkg/ldc/ ./pkg/rhc/
bench:
	go test -bench . -benchmem -benchtime $(BENCHTIME) -count 3 $(BENCHPKGS)

lint:
	golangci-lint run

FUZZTIME ?= 15s
fuzz-smoke:
	go test -run '^$$' -fuzz FuzzDecode -fuzztime $(FUZZTIME) ./pkg/spp/
	go test -run '^$$' -fuzz FuzzDecode -fuzztime $(FUZZTIME) ./pkg/epp/
	go test -run '^$$' -fuzz FuzzDecodeTCTransferFrame -fuzztime $(FUZZTIME) ./pkg/tcdl/
	go test -run '^$$' -fuzz FuzzDecodeTMTransferFrame -fuzztime $(FUZZTIME) ./pkg/tmdl/
	go test -run '^$$' -fuzz FuzzDecodeTransferFrame -fuzztime $(FUZZTIME) ./pkg/aos/
	go test -run '^$$' -fuzz FuzzDecodeTransferFrame -fuzztime $(FUZZTIME) ./pkg/usdl/
	go test -run '^$$' -fuzz FuzzDecodeCUC -fuzztime $(FUZZTIME) ./pkg/tcf/
	go test -run '^$$' -fuzz FuzzDecodeCDS -fuzztime $(FUZZTIME) ./pkg/tcf/
	go test -run '^$$' -fuzz FuzzDecodeCCS -fuzztime $(FUZZTIME) ./pkg/tcf/
	go test -run '^$$' -fuzz FuzzSumNeverPanics -fuzztime $(FUZZTIME) ./internal/cmac/
	go test -run '^$$' -fuzz FuzzDecodeSecurityHeader -fuzztime $(FUZZTIME) ./pkg/sdls/
	go test -run '^$$' -fuzz FuzzProcessSecurity -fuzztime $(FUZZTIME) ./pkg/sdls/
	go test -run '^$$' -fuzz FuzzDecodePDU -fuzztime $(FUZZTIME) ./pkg/cfdp/
	go test -run '^$$' -fuzz FuzzDecodeTLV -fuzztime $(FUZZTIME) ./pkg/cfdp/
	go test -run '^$$' -fuzz FuzzReceiverHandle -fuzztime $(FUZZTIME) ./pkg/cfdp/
	go test -run '^$$' -fuzz FuzzDecodeUserMessage -fuzztime $(FUZZTIME) ./pkg/cfdp/
	go test -run '^$$' -fuzz FuzzDecodeTCHeader -fuzztime $(FUZZTIME) ./pkg/pus/
	go test -run '^$$' -fuzz FuzzDecodeTMHeader -fuzztime $(FUZZTIME) ./pkg/pus/
	go test -run '^$$' -fuzz FuzzRegistryDecode -fuzztime $(FUZZTIME) ./pkg/pus/
	go test -run '^$$' -fuzz FuzzDecode -fuzztime $(FUZZTIME) ./pkg/sdnv/
	go test -run '^$$' -fuzz FuzzRoundTrip -fuzztime $(FUZZTIME) ./pkg/sdnv/
	go test -run '^$$' -fuzz FuzzDecodeSegment -fuzztime $(FUZZTIME) ./pkg/ltp/
	go test -run '^$$' -fuzz FuzzReceiverHandle -fuzztime $(FUZZTIME) ./pkg/ltp/
	go test -run '^$$' -fuzz FuzzDecodeBundle -fuzztime $(FUZZTIME) ./pkg/bp/
	go test -run '^$$' -fuzz FuzzDecodeAdminRecord -fuzztime $(FUZZTIME) ./pkg/bp/
	go test -run '^$$' -fuzz FuzzReassemble -fuzztime $(FUZZTIME) ./pkg/bp/
	go test -run '^$$' -fuzz FuzzDecodeTransferFrame -fuzztime $(FUZZTIME) ./pkg/pxdl/
	go test -run '^$$' -fuzz FuzzDecodeSPDU -fuzztime $(FUZZTIME) ./pkg/pxdl/
	go test -run '^$$' -fuzz FuzzReassembler -fuzztime $(FUZZTIME) ./pkg/pxdl/
	go test -run '^$$' -fuzz FuzzUnwrapPLTU -fuzztime $(FUZZTIME) ./pkg/pxsc/
	go test -run '^$$' -fuzz FuzzSynchronizer -fuzztime $(FUZZTIME) ./pkg/pxsc/
	go test -run '^$$' -fuzz FuzzCRC32 -fuzztime $(FUZZTIME) ./pkg/pxsc/
	go test -run '^$$' -fuzz FuzzConditionRoundTrip -fuzztime $(FUZZTIME) ./pkg/ocsc/
	go test -run '^$$' -fuzz FuzzRandomizer -fuzztime $(FUZZTIME) ./pkg/ocsc/
	go test -run '^$$' -fuzz FuzzCRCVerify -fuzztime $(FUZZTIME) ./pkg/ocsc/
	go test -run '^$$' -fuzz FuzzBERDecoder -fuzztime $(FUZZTIME) ./pkg/sle/
	go test -run '^$$' -fuzz FuzzDecodeTMLMessage -fuzztime $(FUZZTIME) ./pkg/sle/
	go test -run '^$$' -fuzz FuzzDecodeSLEPDUs -fuzztime $(FUZZTIME) ./pkg/sle/
	go test -run '^$$' -fuzz FuzzCredentialsVerify -fuzztime $(FUZZTIME) ./pkg/sle/
	go test -run '^$$' -fuzz FuzzDecodeRAFPDU -fuzztime $(FUZZTIME) ./pkg/sle/
	go test -run '^$$' -fuzz FuzzDecodeRCFPDU -fuzztime $(FUZZTIME) ./pkg/sle/
	go test -run '^$$' -fuzz FuzzDecodeROCFPDU -fuzztime $(FUZZTIME) ./pkg/sle/
	go test -run '^$$' -fuzz FuzzDecodeFCLTUPDU -fuzztime $(FUZZTIME) ./pkg/sle/
	go test -run '^$$' -fuzz FuzzLoad -fuzztime $(FUZZTIME) ./pkg/xtce/
	go test -run '^$$' -fuzz FuzzDecompress -fuzztime $(FUZZTIME) ./pkg/ldc/
	go test -run '^$$' -fuzz FuzzCompressRoundTrip -fuzztime $(FUZZTIME) ./pkg/ldc/
	go test -run '^$$' -fuzz FuzzDecompressPacket -fuzztime $(FUZZTIME) ./pkg/rhc/
	go test -run '^$$' -fuzz FuzzCompressRoundTrip -fuzztime $(FUZZTIME) ./pkg/rhc/
