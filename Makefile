# GWEB: a literate programming system for Go (CWEB for Go).

GO  ?= go
BIN ?= bin

.PHONY: all build generate test install install-tools uninstall clean example tangle bootstrap selfdoc manual

# GWEB is self-hosted: its Go is tangled from the literate sources in lit/*.w.
# Following cweb's tradition, the repository commits only the Go needed to build
# gtangle the first time -- internal/web, internal/tangle, cmd/gtangle (plus the
# hand-written *_test.go). Everything else (the weave package and gweave's main)
# is tangled on demand by `generate' and is git-ignored.
#
# All component webs (lit/gweb.w is the weave-only master; it just @i-includes
# the components, so it is not tangled).
WEBS     = $(filter-out lit/gweb.w,$(wildcard lit/*.w))
# The .w sources whose Go is not committed, and the files they produce.
GEN_WEBS = lit/weave.w lit/gweave.w
GEN_GO   = internal/weave/weave.go internal/weave/tex.go internal/weave/gotok.go \
           internal/weave/xref.go cmd/gweave/main.go
# The committed Go that builds the bootstrap gtangle.
BOOT_GO  = cmd/gtangle/main.go internal/tangle/tangle.go $(wildcard internal/web/*.go)

all: build

# The two commands. gtangle builds straight from the committed bootstrap Go;
# gweave needs its sources generated first.
build: $(BIN)/gtangle generate
	$(GO) build -o $(BIN)/gweave ./cmd/gweave

$(BIN)/gtangle: $(BOOT_GO)
	$(GO) build -o $@ ./cmd/gtangle

# Tangle the non-committed Go (the weave package and gweave's main) so the tree
# compiles. cmd/gweave/main.go is the sentinel for the whole GEN_GO group.
generate: cmd/gweave/main.go
cmd/gweave/main.go: $(GEN_WEBS) $(BIN)/gtangle
	@for w in $(GEN_WEBS); do $(BIN)/gtangle -o . "$$w"; done

# Regenerate every Go source from lit/*.w (committed bootstrap and generated
# alike). gtangle always emits //line directives (as cweb's ctangle does), so the
# Go points back at the .w source; editing a .w reshuffles those line numbers.
tangle: $(BIN)/gtangle
	@for w in $(WEBS); do $(BIN)/gtangle -o . "$$w"; done

# Prove the bootstrap reproduces itself: tangle every lit/*.w into a scratch tree
# and check the committed Go is byte-identical (the fixpoint). Only the committed
# bootstrap dirs are compared; the generated weave Go has no committed counterpart.
# Tests stay as ordinary .go, so they are excluded from the compare.
bootstrap: $(BIN)/gtangle
	@rm -rf .bootstrap
	@for w in $(WEBS); do $(BIN)/gtangle -o .bootstrap "$$w" >/dev/null; done
	@ok=1; for d in internal/web internal/tangle cmd/gtangle; do \
		diff -r "$$d" ".bootstrap/$$d" --exclude='*_test.go' >/dev/null || { echo "DIFF in $$d"; ok=0; }; \
	done; \
	rm -rf .bootstrap; \
	[ $$ok = 1 ] && echo "bootstrap: lit/*.w reproduce the committed Go byte-for-byte"

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

test: generate
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
# gweave's source is generated first, since it is not committed.
install-tools: generate
	$(GO) install ./cmd/gtangle ./cmd/gweave

# Tangle and weave the bundled examples (needs a TeX engine for the PDFs).
example: build
	$(MAKE) -C examples NAME=wc
	$(MAKE) -C examples NAME=pmap
	$(MAKE) -C examples NAME=seq
	$(MAKE) -C examples NAME=floyd
	$(MAKE) -C examples NAME=sham
	$(MAKE) -C examples NAME=fast_cancel
	$(MAKE) -C examples NAME=prjeuler152
	$(MAKE) -C examples NAME=topswops
	$(MAKE) -C examples NAME=topswops_fwd
	$(MAKE) -C examples NAME=squint
	$(MAKE) -C examples NAME=pairsums
	$(MAKE) -C examples NAME=pipeline
	$(MAKE) -C examples NAME=hangul TEXENGINE=luatex
	$(MAKE) -C examples NAME=slidingmax TEXENGINE=luatex
	$(MAKE) -C examples NAME=waiter TEXENGINE=luatex
	$(MAKE) -C examples NAME=trucktour TEXENGINE=luatex
	$(MAKE) -C examples NAME=poison TEXENGINE=luatex
	$(MAKE) -C examples NAME=runningmedian TEXENGINE=luatex
	$(MAKE) -C examples NAME=convmod TEXENGINE=luatex
	$(MAKE) -C examples NAME=intersect TEXENGINE=luatex
	$(MAKE) -C examples NAME=suffixautomaton TEXENGINE=luatex
	$(MAKE) -C examples NAME=pqundo TEXENGINE=luatex

clean:
	rm -rf $(BIN) build $(GEN_GO)
	$(MAKE) -C examples clean
