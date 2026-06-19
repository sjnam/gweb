# GWEB: a literate programming system for Go (CWEB for Go).

GO  ?= go
BIN ?= bin

.PHONY: all build test install clean example

all: build

build:
	$(GO) build -o $(BIN)/gtangle ./cmd/gtangle
	$(GO) build -o $(BIN)/gweave  ./cmd/gweave

test:
	$(GO) test ./...

install:
	$(GO) install ./cmd/gtangle ./cmd/gweave

# Tangle and weave the bundled example (needs a TeX engine for the PDF).
example: build
	$(MAKE) -C examples

clean:
	rm -rf $(BIN)
	$(MAKE) -C examples clean
