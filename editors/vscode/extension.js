// GWEB language features for VS Code.
//
// The idea: gtangle already writes a `//line file.w:N` directive for virtually
// every line of its output, so the tangled .go on disk carries a line-accurate
// source map back to the .w. This extension turns that into IDE features by
// request forwarding: a Go to Definition (or Hover) in a .w code part is mapped
// through the //line table to the corresponding position in the tangled .go,
// handed to whatever Go language support is installed (gopls, via the standard
// `vscode.execute...Provider` commands), and the answer is mapped back the same
// way. @<section name@> navigation is handled natively, and saving a .w
// re-tangles it so the map stays fresh.
'use strict';

const vscode = require('vscode');
const cp = require('child_process');
const fs = require('fs');
const path = require('path');
const lib = require('./weblib');

let out; // output channel, created on activation

// ---------------------------------------------------------------------------
// Source maps: for a given .w file, every .go in its directory whose //line
// directives mention it, parsed into per-file tables. Cached on the .go mtimes.

const mapCache = new Map(); // wPath -> { stamp, entries }

function getMaps(wPath) {
  const dir = path.dirname(wPath);
  let names;
  try {
    names = fs.readdirSync(dir).filter((n) => n.endsWith('.go'));
  } catch {
    return [];
  }
  const stamp = names
    .map((n) => {
      try {
        return n + ':' + fs.statSync(path.join(dir, n)).mtimeMs;
      } catch {
        return n;
      }
    })
    .join('|');
  const hit = mapCache.get(wPath);
  if (hit && hit.stamp === stamp) return hit.entries;

  const entries = [];
  for (const n of names) {
    const goPath = path.join(dir, n);
    let text;
    try {
      text = fs.readFileSync(goPath, 'utf8');
    } catch {
      continue;
    }
    if (!text.includes('//line ')) continue;
    const origins = lib.parseLineDirectives(text);
    const wToGo = new Map(); // .w line (1-based) -> [.go lines]
    for (let gl = 1; gl < origins.length; gl++) {
      const o = origins[gl];
      if (!o || !lib.originMatchesW(o.file, dir, wPath)) continue;
      let a = wToGo.get(o.line);
      if (!a) wToGo.set(o.line, (a = []));
      a.push(gl);
    }
    if (wToGo.size) entries.push({ goPath, wToGo });
  }
  mapCache.set(wPath, { stamp, entries });
  return entries;
}

// Origins of an arbitrary .go file (for mapping results back), cached on mtime.
const originCache = new Map(); // goPath -> { mtimeMs, origins }

function getOrigins(goPath) {
  let mtimeMs;
  try {
    mtimeMs = fs.statSync(goPath).mtimeMs;
  } catch {
    return null;
  }
  const hit = originCache.get(goPath);
  if (hit && hit.mtimeMs === mtimeMs) return hit.origins;
  let text;
  try {
    text = fs.readFileSync(goPath, 'utf8');
  } catch {
    return null;
  }
  const origins = text.includes('//line ') ? lib.parseLineDirectives(text) : null;
  originCache.set(goPath, { mtimeMs, origins });
  return origins;
}

// resolveOriginPath: the path in a //line directive, resolved to a real file.
function resolveOriginPath(originFile, goDir) {
  const cands = [path.resolve(goDir, originFile)];
  for (const f of vscode.workspace.workspaceFolders || []) {
    cands.push(path.resolve(f.uri.fsPath, originFile));
  }
  cands.push(path.join(goDir, path.basename(originFile)));
  for (const c of cands) {
    try {
      if (fs.statSync(c).isFile()) return c;
    } catch {
      /* keep trying */
    }
  }
  return null;
}

// ---------------------------------------------------------------------------
// Forwarding a position: .w -> tangled .go.
//
// Returns { goUri, goPos } or null. Lines map exactly; the column is found by
// locating the same identifier occurrence in the mapped line (gofmt re-indents,
// so raw columns differ).
async function mapToGo(document, position, word, wordStart) {
  const wLine = position.line + 1;
  const srcText = document.lineAt(position.line).text;
  const occ = lib.occurrenceAt(srcText, word, wordStart);
  for (const e of getMaps(document.uri.fsPath)) {
    const gls = e.wToGo.get(wLine);
    if (!gls) continue;
    let goDoc;
    try {
      goDoc = await vscode.workspace.openTextDocument(e.goPath);
    } catch {
      continue;
    }
    for (const gl of gls) {
      if (gl - 1 >= goDoc.lineCount) continue;
      const col = lib.pickOccurrence(goDoc.lineAt(gl - 1).text, word, occ >= 0 ? occ : 0);
      if (col < 0) continue;
      return { goUri: goDoc.uri, goPos: new vscode.Position(gl - 1, col) };
    }
  }
  return null;
}

// Mapping a result back: a location in some tangled .go whose line carries a
// //line origin becomes a location in the .w; anything else (stdlib, plain Go
// packages) is returned as it came.
async function mapLocationBack(uri, range) {
  if (uri.scheme === 'file' && uri.fsPath.endsWith('.go')) {
    const origins = getOrigins(uri.fsPath);
    const o = origins && origins[range.start.line + 1];
    if (o) {
      const wPath = resolveOriginPath(o.file, path.dirname(uri.fsPath));
      if (wPath) {
        try {
          const goDoc = await vscode.workspace.openTextDocument(uri);
          const wDoc = await vscode.workspace.openTextDocument(wPath);
          const word = goDoc.getText(range);
          const goCol = range.start.character;
          const occ = lib.occurrenceAt(goDoc.lineAt(range.start.line).text, word, goCol);
          let col = 0;
          if (o.line - 1 < wDoc.lineCount) {
            const c = lib.pickOccurrence(wDoc.lineAt(o.line - 1).text, word, occ >= 0 ? occ : 0);
            if (c >= 0) col = c;
          }
          const pos = new vscode.Position(o.line - 1, col);
          return new vscode.Location(wDoc.uri, new vscode.Range(pos, pos.translate(0, word.length)));
        } catch {
          /* fall through to the raw location */
        }
      }
    }
  }
  return new vscode.Location(uri, range);
}

// ---------------------------------------------------------------------------
// Providers.

const defProvider = {
  async provideDefinition(document, position) {
    // @<section name@> navigation, handled natively.
    const spans = lib.sectionSpansInLine(document.lineAt(position.line).text);
    const span = spans.find((s) => position.character >= s.start && position.character <= s.end);
    if (span) {
      const index = lib.indexSections(document.getText());
      return lib.sectionDefSites(span.name, index).map(
        (d) =>
          new vscode.Location(
            document.uri,
            new vscode.Range(d.line, d.start, d.line, d.end)
          )
      );
    }

    // Otherwise forward to the Go language support through the tangled .go.
    const wr = document.getWordRangeAtPosition(position);
    if (!wr) return null;
    const word = document.getText(wr);
    const m = await mapToGo(document, position, word, wr.start.character);
    if (!m) return null;
    const results = await vscode.commands.executeCommand(
      'vscode.executeDefinitionProvider',
      m.goUri,
      m.goPos
    );
    if (!results || results.length === 0) return null;

    const mapped = [];
    const seen = new Set();
    for (const r of results) {
      const uri = r.targetUri || r.uri;
      const range = r.targetSelectionRange || r.targetRange || r.range;
      const loc = await mapLocationBack(uri, range);
      const key = loc.uri.toString() + ':' + loc.range.start.line + ':' + loc.range.start.character;
      if (!seen.has(key)) {
        seen.add(key);
        mapped.push(loc);
      }
    }
    return mapped;
  },
};

const hoverProvider = {
  async provideHover(document, position) {
    const spans = lib.sectionSpansInLine(document.lineAt(position.line).text);
    const span = spans.find((s) => position.character >= s.start && position.character <= s.end);
    if (span) {
      const index = lib.indexSections(document.getText());
      const sites = lib.sectionDefSites(span.name, index);
      const md = new vscode.MarkdownString();
      if (sites.length === 0) {
        md.appendMarkdown(`⟨ ${span.name} ⟩ — *no definition found in this file*`);
      } else {
        const lines = sites.map((d) => `line ${d.line + 1}`).join(', ');
        md.appendMarkdown(`⟨ ${lib.resolveName(span.name, index.fullNames) || span.name} ⟩ — defined at ${lines}`);
      }
      return new vscode.Hover(md);
    }

    const wr = document.getWordRangeAtPosition(position);
    if (!wr) return null;
    const word = document.getText(wr);
    const m = await mapToGo(document, position, word, wr.start.character);
    if (!m) return null;
    const hovers = await vscode.commands.executeCommand(
      'vscode.executeHoverProvider',
      m.goUri,
      m.goPos
    );
    for (const h of hovers || []) {
      if (h.contents && h.contents.length) return new vscode.Hover(h.contents, wr);
    }
    return null;
  },
};

// ---------------------------------------------------------------------------
// Tangling. On save (when enabled) the extension re-runs gtangle so the source
// map stays fresh -- but only for a web that already has tangled output; it
// never creates one behind your back. `GWEB: Tangle current file` always runs.

const running = new Set();

function tangle(document, force) {
  if (document.languageId !== 'gweb') return;
  const wPath = document.uri.fsPath;
  if (!wPath.endsWith('.w')) return; // change files are not tangled
  if (!force) {
    const cfg = vscode.workspace.getConfiguration('gweb');
    if (!cfg.get('tangleOnSave')) return;
    if (getMaps(wPath).length === 0) return; // keep existing output fresh only
  }
  if (running.has(wPath)) return;
  running.add(wPath);

  const gtangle = vscode.workspace.getConfiguration('gweb').get('gtanglePath') || 'gtangle';
  const folder = vscode.workspace.getWorkspaceFolder(document.uri);
  const cwd = folder ? folder.uri.fsPath : path.dirname(wPath);
  const arg = folder ? path.relative(cwd, wPath) : path.basename(wPath);

  cp.execFile(gtangle, [arg], { cwd }, (err, stdout, stderr) => {
    running.delete(wPath);
    if (err) {
      out.appendLine(`$ ${gtangle} ${arg}   (in ${cwd})`);
      if (stdout) out.append(stdout);
      if (stderr) out.append(stderr);
      out.appendLine(String(err.message || err));
      vscode.window.setStatusBarMessage('$(warning) gtangle failed — see the GWEB output channel', 5000);
      return;
    }
    if (stderr && stderr.includes('warning')) {
      out.appendLine(`$ ${gtangle} ${arg}   (in ${cwd})`);
      out.append(stderr);
    }
    vscode.window.setStatusBarMessage(`gweb: tangled ${path.basename(wPath)}`, 2500);
  });
}

// ---------------------------------------------------------------------------

function activate(context) {
  out = vscode.window.createOutputChannel('GWEB');
  const selector = { language: 'gweb' };
  context.subscriptions.push(
    out,
    vscode.languages.registerDefinitionProvider(selector, defProvider),
    vscode.languages.registerHoverProvider(selector, hoverProvider),
    vscode.workspace.onDidSaveTextDocument((doc) => tangle(doc, false)),
    vscode.commands.registerCommand('gweb.tangle', () => {
      const ed = vscode.window.activeTextEditor;
      if (ed) tangle(ed.document, true);
    })
  );
}

function deactivate() {}

module.exports = { activate, deactivate };
