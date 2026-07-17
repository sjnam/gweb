% GWEB, woven as a literate program describing itself.
% This master simply \i-includes the three component webs in reading order;
% it is meant for gweave only (the components are what `make tangle' builds).

@d os.Stdin os.Stdout os.Stderr
@s testing.T int
@s strings.Builder int

\def\title{GWEB (Version 0.8.0)}
\def\topofcontents{\null\vfill
  \centerline{\titlefont Literate programming system for the Go}
  \vskip 15pt
  \centerline{(Version 0.8.0)}
  \vfill}
\def\botofcontents{\vfill\centerline{\Gsmallfont
  Copyright \copyright\ 2026 Soojin Nam. MIT License.}}

@** Introduction.
This is \.{GWEB}, a literate-programming system for the \GO/ language, modeled
closely on Knuth and Levy's \.{CWEB}. You write a single |.w| source that
interleaves \TeX\ documentation with \GO/ code; \.{gtangle} extracts the \GO/ a
compiler needs, and \.{gweave} produces a typeset document for people.

\.{GWEB} is itself written this way, and this document is the proof: the very
sources printed here are what \.{gtangle} turns into the \GO/ of the program, and
what \.{gweave} turns into the pages you are reading. The system is organized as
three short webs, presented in turn:

\smallskip
\item{$\bullet$} the \.{common} package -- the shared front end that parses a |.w|
file into sections (the analogue of \.{CWEB}'s \.{common.w});
\item{$\bullet$} \.{gtangle} -- its command-line driver together with the tangle
engine that extracts the \GO/ a compiler needs;
\item{$\bullet$} \.{gweave} -- its command-line driver together with the weave
engine: a small \GO/ lexer, the pretty-printer, and the cross-reference machinery.
\smallskip

\noindent Every section is numbered; a named-section reference is a link to the
section that defines it, and the index and list of section names at the end
gather all the cross-references automatically.

@i ../common/common.w
@i ../cmd/gtangle/gtangle.w
@i ../cmd/gweave/gweave.w

@** Index.
This index lists every identifier used in the program (a section number is
underlined when the identifier is defined there) together with the manual
index entries. The list of section names follows.
