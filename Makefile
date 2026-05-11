.PHONY: build test race lint vet cover

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
