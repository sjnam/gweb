% Change file for the self-documentation. doc/gweb.w @i-includes the three
% component webs; each carries, in its own limbo, a \def\title, \def\topofcontents,
% and \def\botofcontents so it weaves nicely on its own. Spliced into the combined
% document those would fight the master's own title and contents page, so here we
% strip them---the same trick CWEB uses to fold its component webs into the
% manual's appendices (comm-man.ch and friends). Apply with
% `gweave doc/gweb.w doc/gweb.ch'; `make selfdoc' does this for you.

% ---- common.w ----------------------------------------------------------------
@x
\def\title{Common code for GTANGLE and GWEAVE (Version 0.8.0)}
\def\topofcontents{\null\vfill
  \centerline{\titlefont Common code for {\ttitlefont GTANGLE} and
    {\ttitlefont GWEAVE}}
  \vskip 15pt
  \centerline{(Version 0.8.0)}
  \vfill}
\def\botofcontents{\vfill\centerline{\Gsmallfont
  Copyright \copyright\ 2026 Soojin Nam. MIT License.}}
@y
@z

% ---- gtangle.w ---------------------------------------------------------------
@x
\def\title{GTANGLE (Version 0.8.0)}
\def\topofcontents{\null\vfill
  \centerline{\titlefont The {\ttitlefont GTANGLE} processor}
  \vskip 15pt
  \centerline{(Version 0.8.0)}
  \vfill}
\def\botofcontents{\vfill\centerline{\Gsmallfont
  Copyright \copyright\ 2026 Soojin Nam. MIT License.}}
@y
@z

% ---- gweave.w ----------------------------------------------------------------
@x
\def\title{GWEAVE (Version 0.8.0)}
\def\topofcontents{\null\vfill
  \centerline{\titlefont The {\ttitlefont GWEAVE} processor}
  \vskip 15pt
  \centerline{(Version 0.8.0)}
  \vfill}
\def\botofcontents{\vfill\centerline{\Gsmallfont
  Copyright \copyright\ 2026 Soojin Nam. MIT License.}}
@y
@z
