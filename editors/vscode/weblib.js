// Pure helpers for the GWEB extension: parsing the //line directives gtangle
// leaves in its output (they form a line-accurate source map between a .w file
// and its tangled .go), and scanning .w text for @<section name@> spans.
// No dependency on the 'vscode' module, so this file is testable with plain node.
'use strict';

const path = require('path');

// ---------------------------------------------------------------------------
// //line directives.
//
// Go's semantics: a comment `//line file:N` in column 1 declares that the NEXT
// line came from file:N; following lines increment N until the next directive.
// gtangle emits one for virtually every line, so the map is essentially exact.

// parseLineDirectives returns `origins`, indexed by 1-based .go line number:
// origins[goLine] = { file, line } or null (for directive lines themselves and
// any preamble before the first directive).
function parseLineDirectives(goText) {
  const lines = goText.split('\n');
  const origins = new Array(lines.length + 1).fill(null);
  let cur = null; // origin of the next line
  for (let i = 0; i < lines.length; i++) {
    const m = /^\/\/line (.+?):(\d+)(?::\d+)?$/.exec(lines[i]);
    if (m) {
      cur = { file: m[1], line: parseInt(m[2], 10) };
    } else if (cur) {
      origins[i + 1] = { file: cur.file, line: cur.line };
      cur = { file: cur.file, line: cur.line + 1 };
    }
  }
  return origins;
}

// originMatchesW: does a //line path refer to the .w file at wPath?  The path
// gtangle records is whatever it was invoked with (often workspace-relative),
// so try resolving against the .go file's directory, then fall back to a
// basename match (the .go and .w conventionally sit in the same directory).
function originMatchesW(originFile, goDir, wPath) {
  if (path.resolve(goDir, originFile) === path.resolve(wPath)) return true;
  return path.basename(originFile) === path.basename(wPath);
}

// ---------------------------------------------------------------------------
// Identifier occurrence matching.
//
// gofmt re-indents the tangled code, so columns in the .go differ from the .w.
// Lines map exactly (via //line); within a line we locate the identifier by
// text, matching the occurrence ordinal so `x = x + 1` resolves the right `x`.

const IDENT_CHAR = /[\p{L}\p{Nl}\p{Nd}_]/u;

function isIdentChar(ch) {
  return ch !== undefined && IDENT_CHAR.test(ch);
}

// wordOccurrences: columns of every whole-word occurrence of `word` in `text`.
function wordOccurrences(text, word) {
  const cols = [];
  if (!word) return cols;
  let idx = 0;
  while ((idx = text.indexOf(word, idx)) !== -1) {
    if (!isIdentChar(text[idx - 1]) && !isIdentChar(text[idx + word.length])) {
      cols.push(idx);
    }
    idx += word.length;
  }
  return cols;
}

// occurrenceAt: which occurrence (0-based) of `word` covers column `col`; -1
// if none does.
function occurrenceAt(text, word, col) {
  const cols = wordOccurrences(text, word);
  return cols.findIndex((c) => col >= c && col <= c + word.length);
}

// pickOccurrence: column of the n-th occurrence, clamped to what exists; -1 if
// the word does not occur at all.
function pickOccurrence(text, word, n) {
  const cols = wordOccurrences(text, word);
  if (cols.length === 0) return -1;
  return cols[Math.max(0, Math.min(n, cols.length - 1))];
}

// ---------------------------------------------------------------------------
// @<section name@> spans.

// canonName mirrors the parser: whitespace runs collapse to one space, ends
// trimmed.
function canonName(s) {
  return s.trim().replace(/\s+/g, ' ');
}

// sectionSpansInLine finds every @<...@> on one line (names spanning lines are
// not supported here). `@@` is a literal at-sign both outside and inside a
// name. A span immediately followed by `=` (but not `==`) is a definition.
function sectionSpansInLine(lineText) {
  const spans = [];
  for (let i = 0; i < lineText.length; i++) {
    if (lineText[i] !== '@') continue;
    const c = lineText[i + 1];
    if (c === '@') { i++; continue; }
    if (c !== '<') continue;
    let j = i + 2;
    let name = '';
    let closed = false;
    while (j < lineText.length) {
      if (lineText[j] === '@') {
        const d = lineText[j + 1];
        if (d === '@') { name += '@'; j += 2; continue; }
        if (d === '>') { closed = true; break; }
      }
      name += lineText[j];
      j++;
    }
    if (!closed) break;
    const end = j + 2; // past "@>"
    const isDef = /^[ \t]*=(?!=)/.test(lineText.slice(end));
    spans.push({ name: canonName(name), start: i, end, isDef });
    i = end - 1;
  }
  return spans;
}

// indexSections scans a whole .w text: every span per line, the definition
// sites keyed by (unresolved) name, and the list of full (unabbreviated)
// names, needed to resolve `prefix...` abbreviations.
function indexSections(wText) {
  const defs = new Map(); // name -> [{ line (0-based), start, end }]
  const fullNames = new Set();
  const lines = wText.split('\n');
  for (let ln = 0; ln < lines.length; ln++) {
    for (const s of sectionSpansInLine(lines[ln])) {
      if (!s.name.endsWith('...')) fullNames.add(s.name);
      if (s.isDef) {
        let a = defs.get(s.name);
        if (!a) defs.set(s.name, (a = []));
        a.push({ line: ln, start: s.start, end: s.end });
      }
    }
  }
  return { defs, fullNames: [...fullNames] };
}

// openSectionStart: given the text of a line up to the cursor, the column just
// past an `@<` whose name is still open at the cursor (no `@>` yet); -1 when
// the cursor is not inside a section name. `@@` is a literal at-sign.
function openSectionStart(linePrefix) {
  let open = -1;
  for (let i = 0; i < linePrefix.length; i++) {
    if (linePrefix[i] !== '@') continue;
    const c = linePrefix[i + 1];
    if (c === '@') { i++; continue; }
    if (c === '<') { open = i + 2; i++; continue; }
    if (c === '>') { open = -1; i++; continue; }
  }
  return open;
}

// closeAfter: given the rest of the line after the cursor, the offset just past
// a `@>` that closes the currently open name -- so completion can replace the
// tail of a name being retyped -- or -1 when no closer comes before another
// `@<`.
function closeAfter(lineSuffix) {
  for (let i = 0; i < lineSuffix.length; i++) {
    if (lineSuffix[i] !== '@') continue;
    const c = lineSuffix[i + 1];
    if (c === '@') { i++; continue; }
    if (c === '<') return -1;
    if (c === '>') return i + 2;
  }
  return -1;
}

// escapeName re-escapes a canonical section name for insertion into .w source:
// a literal at-sign is written @@.
function escapeName(name) {
  return name.replace(/@/g, '@@');
}

// includeTargets: the file names named by @i lines (quoted or bare), in order.
function includeTargets(wText) {
  const out = [];
  for (const line of wText.split('\n')) {
    const m = /^[ \t]*@i[ \t]+(?:"([^"]+)"|(\S+))/.exec(line);
    if (m) out.push(m[1] || m[2]);
  }
  return out;
}

// resolveName expands a `prefix...` abbreviation to the unique full name that
// begins with the prefix; a full name resolves to itself; an ambiguous or
// unmatched abbreviation resolves to null.
function resolveName(name, fullNames) {
  if (!name.endsWith('...')) return name;
  const prefix = name.slice(0, -3).trimEnd();
  const matches = fullNames.filter((n) => n.startsWith(prefix));
  return matches.length === 1 ? matches[0] : null;
}

// sectionDefSites: all definition sites of (possibly abbreviated) `name`,
// resolving abbreviations on both sides.
function sectionDefSites(name, index) {
  const canon = resolveName(name, index.fullNames);
  if (canon === null) return [];
  const sites = [];
  for (const [defName, arr] of index.defs) {
    if (resolveName(defName, index.fullNames) === canon) sites.push(...arr);
  }
  sites.sort((a, b) => a.line - b.line);
  return sites;
}

module.exports = {
  parseLineDirectives,
  originMatchesW,
  wordOccurrences,
  occurrenceAt,
  pickOccurrence,
  canonName,
  sectionSpansInLine,
  indexSections,
  resolveName,
  sectionDefSites,
  openSectionStart,
  closeAfter,
  escapeName,
  includeTargets,
};
