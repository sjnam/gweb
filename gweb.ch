% Change file for the self-documentation. gweb.w @i-includes the three
% component webs; each carries, in its own limbo, a \def\title, \def\topofcontents,
% and \def\botofcontents so it weaves nicely on its own. Spliced into the combined
% document those would fight the master's own title and contents page, so here we
% strip them---the same trick CWEB uses to fold its component webs into the
% manual's appendices (comm-man.ch and friends). Each web likewise ends in its
% own `@** Index.' section, so that it has an index when woven alone, as CWEB's
% common.w, ctangle.w, and cweave.w do; the combined document has one index, at
% the end of gweb.w, so here we strip the three. Each index section's text names
% its own web, so no @x block below can match another's. Apply with
% `gweave gweb.w gweb.ch'; `make selfdoc' does this for you.
%
% Only the \def block is matched here---never the `@i types.w' line above it.
% Changes are matched after @i includes are expanded (see the Change files
% section of common.w), so by matching time that line is gone, replaced by the
% contents of types.w; an @x block naming it can never match. Spelling out
% those contents instead would work, but then this file would have to mirror
% types.w forever and would break every time it changed. Leaving the include
% alone costs nothing: types.w holds only @d and @s format hints, which are
% invisible and harmless when the three webs each bring in a copy.

% ---- common.w ----------------------------------------------------------------
@x
\def\title{Common code for GTANGLE and GWEAVE (Version 0.10.3)}
\def\topofcontents{\null\vfill
  \centerline{\titlefont Common code for {\ttitlefont GTANGLE} and
    {\ttitlefont GWEAVE}}
  \vskip 15pt
  \centerline{(Version 0.10.3)}
  \vfill}
\def\botofcontents{\vfill\centerline{\smallfont
  Copyright \copyright\ 2026 Soojin Nam. MIT License.}}
@y
@z

@x
@** Index.
This index covers the common code alone, as it stands when \.{common.w} is
woven by itself: every identifier it uses (a section number is underlined where
the identifier is defined), together with the manual index entries. In the
combined \.{GWEB} document \.{gweb.ch} removes this section, and the index at
the end of \.{gweb.w} covers all three webs. The list of section names follows.
@y
@z

% ---- gtangle.w ---------------------------------------------------------------
@x
\def\title{GTANGLE (Version 0.10.3)}
\def\topofcontents{\null\vfill
  \centerline{\titlefont The {\ttitlefont GTANGLE} processor}
  \vskip 15pt
  \centerline{(Version 0.10.3)}
  \vfill}
\def\botofcontents{\vfill\centerline{\smallfont
  Copyright \copyright\ 2026 Soojin Nam. MIT License.}}
@y
@z

@x
@** Index.
This index covers \.{GTANGLE} alone, as it stands when \.{gtangle.w} is woven
by itself: every identifier it uses (a section number is underlined where the
identifier is defined), together with the manual index entries. In the combined
\.{GWEB} document \.{gweb.ch} removes this section, and the index at the end of
\.{gweb.w} covers all three webs. The list of section names follows.
@y
@z

% ---- gweave.w ----------------------------------------------------------------
@x
\def\title{GWEAVE (Version 0.10.3)}
\def\topofcontents{\null\vfill
  \centerline{\titlefont The {\ttitlefont GWEAVE} processor}
  \vskip 15pt
  \centerline{(Version 0.10.3)}
  \vfill}
\def\botofcontents{\vfill\centerline{\smallfont
  Copyright \copyright\ 2026 Soojin Nam. MIT License.}}
@y
@z

@x
@** Index.
This index covers \.{GWEAVE} alone, as it stands when \.{gweave.w} is woven by
itself: every identifier it uses (a section number is underlined where the
identifier is defined), together with the manual index entries. In the combined
\.{GWEB} document \.{gweb.ch} removes this section, and the index at the end of
\.{gweb.w} covers all three webs. The list of section names follows.
@y
@z
