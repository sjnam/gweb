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
	match    []string // lines to find in the master source
	repl     []string // lines to substitute for them
	line     int      // 1-based line of the @x in the change file (for diagnostics)
	replLine int      // 1-based change-file line of the first replacement line
}

// srcLoc identifies the origin (file and 1-based line) of a line of the
// includes-expanded, change-applied source, so diagnostics can point back to
// the file the user actually wrote.
type srcLoc struct {
	file string
	line int
}

func (l srcLoc) String() string {
	if l.file == "" {
		return fmt.Sprintf("line %d", l.line)
	}
	return fmt.Sprintf("%s:%d", l.file, l.line)
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
		c.replLine = i + 1
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

// applyChanges returns src with the changes applied (string convenience form,
// used by tests). See applyChangesMapped for the origin-tracking version.
func applyChanges(src string, changes []change, chFile string) (string, error) {
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
	if err != nil {
		return "", err
	}
	return strings.Join(out, "\n"), nil
}

// applyChangesMapped applies changes to master, keeping a parallel origin map in
// step: passed-through lines keep their origin, and replacement lines are
// attributed to the change file. locs may be nil if origins are not tracked.
// chFile names the change file for diagnostics. It is an error if a change's
// first line is never found, or is found but the rest of the block does not
// match.
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) ([]string, []srcLoc, error) {
	loc := func(i int) srcLoc {
		if locs != nil && i < len(locs) {
			return locs[i]
		}
		return srcLoc{line: i + 1}
	}
	out := make([]string, 0, len(master))
	var outLocs []srcLoc
	ci := 0
	for i := 0; i < len(master); {
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
			if !blockMatches(master, i, changes[ci].match) {
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
					chFile, changes[ci].line, loc(i))
			}
			for r, rl := range changes[ci].repl {
				out = append(out, rl)
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
			}
			i += len(changes[ci].match)
			ci++
			continue
		}
		out = append(out, master[i])
		outLocs = append(outLocs, loc(i))
		i++
	}
	if ci < len(changes) {
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
			chFile, changes[ci].line, changes[ci].match[0])
	}
	return out, outLocs, nil
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
