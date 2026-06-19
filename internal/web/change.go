package web

import (
	"fmt"
	"strings"
)

// A change file (CWEB's ".ch" mechanism) patches the master source without
// editing it. It is a sequence of changes, each of the form
//
//	@x
//	<lines to find in the master source>
//	@y
//	<lines to substitute>
//	@z
//
// Text outside an @x...@z group is ignored (it serves as commentary). Changes
// are matched against the master source — after @i includes are expanded — in
// the order they appear: GWEB scans the master line by line, and at the first
// line equal to a change's first match line it requires the whole match block
// to match, then substitutes the replacement lines.

type change struct {
	match []string // lines to find in the master source
	repl  []string // lines to substitute for them
	line  int      // 1-based line of the @x in the change file (for diagnostics)
}

// isChangeCtrl reports whether line begins with the change control "@<c>"
// (c is 'x', 'y', or 'z'), which must start in the first column.
func isChangeCtrl(line string, c byte) bool {
	return len(line) >= 2 && line[0] == '@' && line[1] == c
}

// splitLines splits text into lines, normalizing CRLF, so that joining the
// result with "\n" reproduces the (LF-normalized) input.
func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

// sameLine compares two source lines for change matching, ignoring trailing
// whitespace (as WEB does).
func sameLine(a, b string) bool {
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
}

// parseChangeFile parses change-file text into an ordered list of changes.
func parseChangeFile(src string) ([]change, error) {
	lines := splitLines(src)
	var changes []change
	n := len(lines)
	for i := 0; i < n; {
		if !isChangeCtrl(lines[i], 'x') {
			i++ // commentary between changes
			continue
		}
		c := change{line: i + 1}
		i++
		for i < n && !isChangeCtrl(lines[i], 'y') {
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
			}
			c.match = append(c.match, lines[i])
			i++
		}
		if i >= n {
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
		}
		i++ // skip @y
		for i < n && !isChangeCtrl(lines[i], 'z') {
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
			}
			c.repl = append(c.repl, lines[i])
			i++
		}
		if i >= n {
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
		}
		i++ // skip @z
		if len(c.match) == 0 {
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
		}
		changes = append(changes, c)
	}
	return changes, nil
}

// applyChanges returns src with the changes applied. chFile names the change
// file, for diagnostics. It is an error if a change's first line is never
// found, or if it is found but the rest of the block does not match.
func applyChanges(src string, changes []change, chFile string) (string, error) {
	master := splitLines(src)
	out := make([]string, 0, len(master))
	ci := 0
	for i := 0; i < len(master); {
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
			if !blockMatches(master, i, changes[ci].match) {
				return "", fmt.Errorf("%s:%d: change did not match the master source at line %d",
					chFile, changes[ci].line, i+1)
			}
			out = append(out, changes[ci].repl...)
			i += len(changes[ci].match)
			ci++
			continue
		}
		out = append(out, master[i])
		i++
	}
	if ci < len(changes) {
		return "", fmt.Errorf("%s:%d: change was never matched (looking for %q)",
			chFile, changes[ci].line, changes[ci].match[0])
	}
	return strings.Join(out, "\n"), nil
}

// blockMatches reports whether match lines up with master starting at index at.
func blockMatches(master []string, at int, match []string) bool {
	if at+len(match) > len(master) {
		return false
	}
	for k, m := range match {
		if !sameLine(master[at+k], m) {
			return false
		}
	}
	return true
}
