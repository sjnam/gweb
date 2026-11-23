# Build a GWEB example program: tangle it to Go, weave it to a PDF.
#
#   make            # tangle + weave + typeset NAME.gw  (default NAME=wc)
#   make NAME=foo   # operate on foo.gw instead
#   make go         # only produce NAME.go
#   make pdf        # only produce NAME.pdf

NAME ?= wc
ROOT := $(abspath ..)

GTANGLE   ?= $(ROOT)/bin/gtangle
GWEAVE    ?= $(ROOT)/bin/gweave
TEXENGINE ?= pdftex

# Let the TeX engine find gwebmac.tex.
export TEXINPUTS := $(ROOT):$(TEXINPUTS)

.PHONY: all go pdf clean

all: go pdf

go: $(NAME).go
# Depend on gtangle too, so rebuilding the tool re-tangles the example.
$(NAME).go: $(NAME).gw $(GTANGLE)
	$(GTANGLE) $(NAME).gw

pdf: $(NAME).pdf
# Depend on gweave and the macro package too, so changing either rebuilds the PDF.
$(NAME).pdf: $(NAME).gw $(GWEAVE) $(ROOT)/gwebmac.tex
	$(GWEAVE) $(NAME).gw
	$(TEXENGINE) $(NAME).tex

clean:
	rm -f *.go *.tex *.log *.dvi *.toc *.pdf *.idx *.scn
