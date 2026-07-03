.PHONY: build test race lint vet cover fuzz-smoke

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
	go test -run '^$$' -fuzz FuzzDecodeSecurityHeader -fuzztime $(FUZZTIME) ./pkg/sdls/
	go test -run '^$$' -fuzz FuzzProcessSecurity -fuzztime $(FUZZTIME) ./pkg/sdls/
	go test -run '^$$' -fuzz FuzzDecodePDU -fuzztime $(FUZZTIME) ./pkg/cfdp/
	go test -run '^$$' -fuzz FuzzDecodeTLV -fuzztime $(FUZZTIME) ./pkg/cfdp/
	go test -run '^$$' -fuzz FuzzReceiverHandle -fuzztime $(FUZZTIME) ./pkg/cfdp/
	go test -run '^$$' -fuzz FuzzDecodeTCHeader -fuzztime $(FUZZTIME) ./pkg/pus/
	go test -run '^$$' -fuzz FuzzDecodeTMHeader -fuzztime $(FUZZTIME) ./pkg/pus/
	go test -run '^$$' -fuzz FuzzRegistryDecode -fuzztime $(FUZZTIME) ./pkg/pus/
	go test -run '^$$' -fuzz FuzzDecode -fuzztime $(FUZZTIME) ./pkg/sdnv/
	go test -run '^$$' -fuzz FuzzRoundTrip -fuzztime $(FUZZTIME) ./pkg/sdnv/
