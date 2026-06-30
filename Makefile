# GWEB: a literate programming system for Go (CWEB for Go).

GO  ?= go
BIN ?= bin

.PHONY: all build generate test install install-tools uninstall clean example tangle bootstrap selfdoc manual

# GWEB is self-hosted: its Go is tangled from the literate .w sources, each kept
# next to its output. Following cweb's tradition, the repository commits only the
# Go needed to build gtangle the first time -- the shared internal/web package and
# cmd/gtangle (its main and the tangle engine). Everything else is tangled on
# demand by `generate' and is git-ignored: all of gweave (its main and the weave
# engine) and every test, which live in the .w sources too.
#
# Each command is a single web: cmd/gtangle/gtangle.w is the gtangle front end
# plus the tangle engine, cmd/gweave/gweave.w the gweave front end plus the weave
# engine, and internal/web/web.w the shared parser. gweb.w (repo root) is the
# weave-only master (it just @i-includes the three, so it is not tangled).
WEBS   = internal/web/web.w cmd/gtangle/gtangle.w cmd/gweave/gweave.w
# The non-committed Go that `generate' produces (removed by `clean').
GEN_GO = cmd/gtangle/tangle_test.go cmd/gtangle/build_test.go \
         cmd/gweave/main.go cmd/gweave/weave.go cmd/gweave/tex.go \
         cmd/gweave/gotok.go cmd/gweave/xref.go cmd/gweave/weave_test.go \
         internal/web/web_test.go

all: build

# The two commands. gtangle compiles from the committed bootstrap Go; the rest of
# the tree (gweave, the weave package, the tangled tests) is generated first.
build: generate
	$(GO) build -o $(BIN)/gweave ./cmd/gweave

# Build a bootstrap gtangle from the committed sources, then tangle every web, so
# the non-committed Go exists and the tree compiles. This rewrites the committed
# bootstrap Go in place too (identically, unless a .w changed); Go's build cache
# absorbs the no-op rewrites. gtangle always emits //line directives (as cweb's
# ctangle does), so the Go points back at the .w source, and editing a .w
# reshuffles those line numbers. `tangle' is a synonym.
generate tangle:
	$(GO) build -o $(BIN)/gtangle ./cmd/gtangle
	@for w in $(WEBS); do $(BIN)/gtangle -o . "$$w"; done

# Prove the bootstrap reproduces itself: tangle every web into a scratch tree
# and check the committed Go is byte-identical (the fixpoint). Only the committed
# bootstrap dirs are compared; the generated Go (weave package, gweave, tests) has
# no committed counterpart, and tests are excluded from the compare anyway.
bootstrap:
	@$(GO) build -o $(BIN)/gtangle ./cmd/gtangle
	@rm -rf .bootstrap
	@for w in $(WEBS); do $(BIN)/gtangle -o .bootstrap "$$w" >/dev/null; done
	@ok=1; for d in internal/web cmd/gtangle; do \
		diff -r "$$d" ".bootstrap/$$d" --exclude='*_test.go' --exclude='*.w' >/dev/null || { echo "DIFF in $$d"; ok=0; }; \
	done; \
	rm -rf .bootstrap; \
	[ $$ok = 1 ] && echo "bootstrap: the .w sources reproduce the committed Go byte-for-byte"

# Weave GWEB's own sources into a typeset PDF of the whole system. gweb.w is the
# master that @i-includes the three component webs in reading order. Needs a
# TeX engine (pdftex) that can find tex/gwebmac.tex.
selfdoc: build
	@mkdir -p build
	$(BIN)/gweave -o build gweb.w
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
