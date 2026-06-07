GO_FILES := $(shell find . -name '*.go' -not -path './vendor/*')

.PHONY: build test

external_payments: $(GO_FILES)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOEXPERIMENT=jsonv2 go build -ldflags="-s -w" -o external_payments main/* && upx -9 --force-macos external_payments

build: external_payments

test:
	go test ./...
