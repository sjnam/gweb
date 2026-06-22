# GWEB: a literate programming system for Go (CWEB for Go).

GO  ?= go
BIN ?= bin

.PHONY: all build test install install-tools uninstall clean example tangle bootstrap selfdoc manual

all: build

build:
	$(GO) build -o $(BIN)/gtangle ./cmd/gtangle
	$(GO) build -o $(BIN)/gweave  ./cmd/gweave

# GWEB is self-hosted: its own Go sources are tangled from the literate sources
# in lit/*.w. Each .w names its output files with @(path@>=, so we tangle from
# the repository root. The generated .go files are committed too, so a fresh
# checkout still builds without first having gtangle.
# All component webs (lit/gweb.w is the weave-only master; it just @i-includes
# the components, so it is not tangled).
WEBS = $(filter-out lit/gweb.w,$(wildcard lit/*.w))

# gtangle emits //line directives by default; we turn them off when tangling
# GWEB itself so the committed Go stays clean (and the fixpoint below is stable).
tangle: build
	@for w in $(WEBS); do $(BIN)/gtangle -line=false -o . "$$w"; done

# Prove the system reproduces itself: tangle every lit/*.w into a scratch tree
# and check the result is byte-identical to the committed Go sources (the
# fixpoint). Tests stay as ordinary .go, so they are excluded from the compare.
bootstrap: build
	@rm -rf .bootstrap
	@for w in $(WEBS); do $(BIN)/gtangle -line=false -o .bootstrap "$$w" >/dev/null; done
	@ok=1; for d in internal/tangle internal/weave internal/web cmd/gtangle cmd/gweave; do \
		diff -r "$$d" ".bootstrap/$$d" --exclude='*_test.go' >/dev/null || { echo "DIFF in $$d"; ok=0; }; \
	done; \
	rm -rf .bootstrap; \
	[ $$ok = 1 ] && echo "bootstrap: lit/*.w reproduce the Go tree byte-for-byte"

# Weave GWEB's own sources into a typeset PDF of the whole system. lit/gweb.w is
# the master that @i-includes the five component webs in reading order. Needs a
# TeX engine (pdftex) that can find tex/gwebmac.tex.
selfdoc: build
	@mkdir -p build
	$(BIN)/gweave -o build lit/gweb.w
	cd build && TEXINPUTS="$(CURDIR)/tex:" pdftex -interaction=nonstopmode gweb.tex
	@echo "selfdoc: wrote build/gweb.pdf"

# The GWEB manual: a plain-TeX document that \input's gwebmac, formatted in the
# manner of Knuth and Levy's cwebman.tex. Needs a TeX engine that can find
# tex/gwebmac.tex.
manual:
	@mkdir -p build
	cd build && TEXINPUTS="$(CURDIR)/tex:" pdftex -interaction=nonstopmode "$(CURDIR)/doc/gwebman.tex"
	@echo "manual: wrote build/gwebman.pdf"

test:
	$(GO) test ./...

# Full install: the commands, gwebmac.tex, and the man pages. Pass options
# through, e.g.  make install ARGS=--prefix=$$HOME/.local . May need sudo for a
# system prefix. See install.sh --help.
install:
	./install.sh $(ARGS)

# Remove what `make install' put down (same ARGS for non-default locations).
uninstall:
	./install.sh --uninstall $(ARGS)

# Just the two commands, the Go way (into $GOBIN); no macros or man pages.
install-tools:
	$(GO) install ./cmd/gtangle ./cmd/gweave

# Tangle and weave the bundled examples (needs a TeX engine for the PDFs).
example: build
	$(MAKE) -C examples NAME=wc
	$(MAKE) -C examples NAME=pmap
	$(MAKE) -C examples NAME=seq
	$(MAKE) -C examples NAME=floyd
	$(MAKE) -C examples NAME=sham

clean:
	rm -rf $(BIN) build
	$(MAKE) -C examples clean
