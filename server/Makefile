# AI Skope Server
BINARY  := aiss
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/ai-skope/aiss/internal/version.Version=$(VERSION)

.PHONY: build install test race e2e lint tidy clean run

build:            ## build ./aiss
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/aiss

install:          ## install aiss into GOBIN
	go install -ldflags "$(LDFLAGS)" ./cmd/aiss

test:             ## unit and integration tests
	go test ./...

race:             ## tests under the race detector
	go test -race ./...

e2e:              ## end-to-end: real server, real HTTP, fake agent
	./scripts/e2e.sh

lint:
	go vet ./...
	gofmt -l . | grep -v '^$$' && exit 1 || true

tidy:
	go mod tidy

run: build        ## run in the foreground with debug logging
	AISS_LOG_LEVEL=debug ./$(BINARY) start --foreground

clean:
	rm -f $(BINARY)
