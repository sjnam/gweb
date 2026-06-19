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

# Let the TeX engine find gwebmac.tex.
export TEXINPUTS := $(ROOT)/tex:$(TEXINPUTS)

.PHONY: all go pdf clean

all: go pdf

go: $(NAME).go
$(NAME).go: $(NAME).w
	$(GTANGLE) $(NAME).w

pdf: $(NAME).pdf
$(NAME).pdf: $(NAME).w
	$(GWEAVE) $(NAME).w
	$(TEXENGINE) $(NAME).tex

clean:
	rm -f *.go *.tex *.pdf *.log *.dvi *.toc wc pmap
