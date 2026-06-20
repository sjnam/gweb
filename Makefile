# GWEB: a literate programming system for Go (CWEB for Go).

GO  ?= go
BIN ?= bin

.PHONY: all build test install clean example tangle bootstrap

all: build

build:
	$(GO) build -o $(BIN)/gtangle ./cmd/gtangle
	$(GO) build -o $(BIN)/gweave  ./cmd/gweave

# GWEB is self-hosted: its own Go sources are tangled from the literate sources
# in lit/*.w. Each .w names its output files with @(path@>=, so we tangle from
# the repository root. The generated .go files are committed too, so a fresh
# checkout still builds without first having gtangle.
WEBS = $(wildcard lit/*.w)

tangle: build
	@for w in $(WEBS); do $(BIN)/gtangle "$$w"; done

# Prove the system reproduces itself: tangle every lit/*.w into a scratch tree
# and check the result is byte-identical to the committed Go sources (the
# fixpoint). Tests stay as ordinary .go, so they are excluded from the compare.
bootstrap: build
	@rm -rf .bootstrap
	@for w in $(WEBS); do $(BIN)/gtangle -o .bootstrap "$$w" >/dev/null; done
	@ok=1; for d in internal/tangle internal/weave internal/web cmd/gtangle cmd/gweave; do \
		diff -r "$$d" ".bootstrap/$$d" --exclude='*_test.go' >/dev/null || { echo "DIFF in $$d"; ok=0; }; \
	done; \
	rm -rf .bootstrap; \
	[ $$ok = 1 ] && echo "bootstrap: lit/*.w reproduce the Go tree byte-for-byte"

test:
	$(GO) test ./...

install:
	$(GO) install ./cmd/gtangle ./cmd/gweave

# Tangle and weave the bundled examples (needs a TeX engine for the PDFs).
example: build
	$(MAKE) -C examples NAME=wc
	$(MAKE) -C examples NAME=pmap

clean:
	rm -rf $(BIN)
	$(MAKE) -C examples clean
