# GWEB: a literate programming system for Go (CWEB for Go).

GO  ?= go
BIN ?= bin

.PHONY: all build generate test install install-tools uninstall clean example tangle bootstrap selfdoc manual bump

# GWEB is self-hosted: its Go is tangled from the literate .w sources, each kept
# next to its output. Following cweb's tradition, the repository commits only the
# Go needed to build gtangle the first time -- the shared common package and
# cmd/gtangle (its main and the tangle engine). Everything else is tangled on
# demand by `generate' and is git-ignored: all of gweave (its main and the weave
# engine) and every test, which live in the .w sources too.
#
# Each command is a single web: cmd/gtangle/gtangle.w is the gtangle front end
# plus the tangle engine, cmd/gweave/gweave.w the gweave front end plus the weave
# engine, and common/common.w the shared parser. doc/gweb.w is the weave-only
# master (it just @i-includes the three, so it is not tangled).
WEBS   = common/common.w cmd/gtangle/gtangle.w cmd/gweave/gweave.w
# The non-committed Go that `generate' produces (removed by `clean').
GEN_GO = cmd/gtangle/gtangle_test.go \
         cmd/gweave/gweave.go cmd/gweave/gweave_test.go \
         common/common_test.go

all: build

# The two commands. `generate' first tangles the tree (building a bootstrap
# gtangle from the committed Go to do so); we then build both binaries from the
# freshly generated sources, so each reflects the current .w (e.g. the version).
build: generate
	$(GO) build -o $(BIN)/gtangle ./cmd/gtangle
	$(GO) build -o $(BIN)/gweave  ./cmd/gweave

# Build a bootstrap gtangle from the committed sources, then tangle every web, so
# the non-committed Go exists and the tree compiles. This rewrites the committed
# bootstrap Go in place too (identically, unless a .w changed); Go's build cache
# absorbs the no-op rewrites. gtangle always emits //line directives (as cweb's
# ctangle does), so the Go points back at the .w source, and editing a .w
# reshuffles those line numbers. `tangle' is a synonym.
generate tangle:
	$(GO) build -o $(BIN)/gtangle ./cmd/gtangle
	@for w in $(WEBS); do $(BIN)/gtangle "$$w"; done

# Prove the bootstrap reproduces itself: tangle every web into a scratch tree
# and check the committed Go is byte-identical (the fixpoint). Only the committed
# bootstrap dirs are compared; the generated Go (weave package, gweave, tests) has
# no committed counterpart, and tests are excluded from the compare anyway.
bootstrap:
	@$(GO) build -o $(BIN)/gtangle ./cmd/gtangle
	@rm -rf .bootstrap
	@for w in $(WEBS); do $(BIN)/gtangle -o ".bootstrap/$$(dirname "$$w")" "$$w" >/dev/null; done
	@ok=1; for d in common cmd/gtangle; do \
		for f in .bootstrap/$$d/*.go; do \
			case "$$f" in *_test.go) continue ;; esac; \
			b=`basename "$$f"`; \
			cmp -s "$$d/$$b" "$$f" || { echo "DIFF in $$d/$$b"; ok=0; }; \
		done; \
	done; \
	rm -rf .bootstrap; \
	[ $$ok = 1 ] && echo "bootstrap: the .w sources reproduce the committed Go byte-for-byte"

# Weave GWEB's own sources into a typeset PDF of the whole system. gweb.w is the
# master that @i-includes the three component webs in reading order. Needs a
# TeX engine (pdftex) that can find tex/gwebmac.tex.
selfdoc: build
	@mkdir -p build
	$(BIN)/gweave -o build doc/gweb.w doc/gweb.ch
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

# Bump the release version everywhere it appears. Usage:
#   make bump VERSION=0.5.2
# The current version is read from common.w's Version constant, then replaced --
# in every tracked .w/.tex/.ch/.md that carries it -- with VERSION. That is why
# it exists: the version now lives in each component web's \title and
# \topofcontents, in doc/gweb.ch (which must keep matching those lines), and in
# the manual, besides common.w's constant; this moves them together. The
# committed Go is regenerated afterwards. Review the diff, then commit, tag, and
# release by hand (make bootstrap && make test first).
bump:
	@test -n "$(VERSION)" || { echo "usage: make bump VERSION=x.y.z"; exit 2; }
	@old=`sed -n 's/.*const Version = "\([^"]*\)".*/\1/p' common/common.w`; \
	test -n "$$old" || { echo "cannot read the current version from common/common.w"; exit 1; }; \
	if [ "$$old" = "$(VERSION)" ]; then echo "already at $$old"; exit 0; fi; \
	files=`git grep -lF "$$old" -- '*.w' '*.tex' '*.ch' '*.md'`; \
	echo "bumping $$old -> $(VERSION) in:"; echo "$$files" | sed 's/^/  /'; \
	for f in $$files; do perl -pi -e "s/\Q$$old\E(?![0-9.])/$(VERSION)/g" $$f; done
	@$(MAKE) --no-print-directory generate >/dev/null
	@echo "bumped to $(VERSION); verify with 'make bootstrap test', then commit, tag, push"

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
	$(MAKE) -C examples NAME=floyd
	$(MAKE) -C examples NAME=pairsums
	$(MAKE) -C examples NAME=rps TEXENGINE=luatex	
	$(MAKE) -C examples NAME=seq
	$(MAKE) -C examples NAME=sham
	$(MAKE) -C examples NAME=squint
	$(MAKE) -C examples NAME=torture
	$(MAKE) -C examples NAME=wc	

clean:
	rm -rf $(BIN) build $(GEN_GO)
	@for d in common cmd/gtangle cmd/gweave; do b=`basename $$d`; \
		rm -f $$d/$$b.tex $$d/$$b.pdf $$d/$$b.dvi $$d/$$b.ps $$d/$$b.log $$d/$$b.toc $$d/$$b.idx $$d/$$b.scn; \
	done
	$(MAKE) -C examples clean
