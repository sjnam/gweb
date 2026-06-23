# Build a GWEB example program: tangle it to Go, weave it to a PDF.
#
#   make            # tangle + weave + typeset NAME.w  (default NAME=wc)
#   make NAME=foo   # operate on foo.w instead
#   make go         # only produce NAME.go
#   make pdf        # only produce NAME.pdf

NAME ?= wc
ROOT := $(abspath ..)

GTANGLE   ?= $(ROOT)/bin/gtangle
GWEAVE    ?= $(ROOT)/bin/gweave
TEXENGINE ?= pdftex

# Extra gtangle flags. Use GTFLAGS=-line to map compile errors back to the .w
# source, e.g.  make go GTFLAGS=-line && go build .
GTFLAGS   ?=

# Let the TeX engine find gwebmac.tex.
export TEXINPUTS := $(ROOT)/tex:$(TEXINPUTS)

.PHONY: all go pdf clean

all: go pdf

go: $(NAME).go
# Depend on gtangle too, so rebuilding the tool re-tangles the example.
$(NAME).go: $(NAME).w $(GTANGLE)
	$(GTANGLE) $(GTFLAGS) $(NAME).w

pdf: $(NAME).pdf
# Depend on gweave and the macro package too, so changing either rebuilds the PDF.
$(NAME).pdf: $(NAME).w $(GWEAVE) $(ROOT)/tex/gwebmac.tex
	$(GWEAVE) $(NAME).w
	$(TEXENGINE) $(NAME).tex

clean:
	rm -f *.go *.tex *.log *.dvi *.toc *.pdf
