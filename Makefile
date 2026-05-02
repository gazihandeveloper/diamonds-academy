.PHONY: dev run build templ tidy clean migrate-create

GO       := go
TEMPL    := $(shell go env GOPATH)/bin/templ
AIR      := $(shell go env GOPATH)/bin/air
APP_NAME := diamonds
BIN_DIR  := bin

## dev: hot-reload (templ generate + air)
dev:
	$(TEMPL) generate --watch --proxy="http://localhost:8080" --cmd="$(AIR)"

## run: tek seferlik çalıştır
run: templ
	$(GO) run ./cmd/server

## templ: .templ -> _templ.go üret
templ:
	$(TEMPL) generate

## build: prod binary
build: templ
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "-s -w" -o $(BIN_DIR)/$(APP_NAME) ./cmd/server

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR) tmp
	find . -name "*_templ.go" -delete
