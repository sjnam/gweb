//line lit/web.w:951
package web

//line lit/web.w:953
import (
//line lit/web.w:954
	"fmt"
//line lit/web.w:955
	"strings"
//line lit/web.w:956
)

//line lit/web.w:958
// A change file (CWEB's ".ch" mechanism) patches the master source without
//line lit/web.w:959
// editing it. It is a sequence of changes, each of the form
//line lit/web.w:960
//
//line lit/web.w:961
//	@x
//line lit/web.w:962
//	<lines to find in the master source>
//line lit/web.w:963
//	@y
//line lit/web.w:964
//	<lines to substitute>
//line lit/web.w:965
//	@z
//line lit/web.w:966
//
//line lit/web.w:967
// Text outside an @x...@z group is ignored (it serves as commentary). Changes
//line lit/web.w:968
// are matched against the master source — after @i includes are expanded — in
//line lit/web.w:969
// the order they appear: GWEB scans the master line by line, and at the first
//line lit/web.w:970
// line equal to a change's first match line it requires the whole match block
//line lit/web.w:971
// to match, then substitutes the replacement lines.

//line lit/web.w:977
type change struct {
//line lit/web.w:978
	match []string // lines to find in the master source
//line lit/web.w:979
	repl []string // lines to substitute for them
//line lit/web.w:980
	line int // 1-based line of the @x in the change file (for diagnostics)
//line lit/web.w:981
	replLine int // 1-based change-file line of the first replacement line
//line lit/web.w:982
}

// srcLoc identifies the origin (file and 1-based line) of a line of the
// includes-expanded, change-applied source, so diagnostics can point back to
// the file the user actually wrote.
//
//line lit/web.w:984
//line lit/web.w:985
//line lit/web.w:986
//line lit/web.w:987
type srcLoc struct {
//line lit/web.w:988
	file string
//line lit/web.w:989
	line int
//line lit/web.w:990
}

//line lit/web.w:992
func (l srcLoc) String() string {
//line lit/web.w:993
	if l.file == "" {
//line lit/web.w:994
		return fmt.Sprintf("line %d", l.line)
//line lit/web.w:995
	}
//line lit/web.w:996
	return fmt.Sprintf("%s:%d", l.file, l.line)
//line lit/web.w:997
}

// isChangeCtrl reports whether line begins with the change control "@<c>"
// (c is 'x', 'y', or 'z'), which must start in the first column.
//
//line lit/web.w:1002
//line lit/web.w:1003
//line lit/web.w:1004
func isChangeCtrl(line string, c byte) bool {
//line lit/web.w:1005
	return len(line) >= 2 && line[0] == '@' && line[1] == c
//line lit/web.w:1006
}

// splitLines splits text into lines, normalizing CRLF, so that joining the
// result with "\n" reproduces the (LF-normalized) input.
//
//line lit/web.w:1008
//line lit/web.w:1009
//line lit/web.w:1010
func splitLines(s string) []string {
//line lit/web.w:1011
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
//line lit/web.w:1012
}

// sameLine compares two source lines for change matching, ignoring trailing
// whitespace (as WEB does).
//
//line lit/web.w:1014
//line lit/web.w:1015
//line lit/web.w:1016
func sameLine(a, b string) bool {
//line lit/web.w:1017
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
//line lit/web.w:1018
}

// parseChangeFile parses change-file text into an ordered list of changes.
//
//line lit/web.w:1023
//line lit/web.w:1024
func parseChangeFile(src string) ([]change, error) {
//line lit/web.w:1025
	lines := splitLines(src)
//line lit/web.w:1026
	var changes []change
//line lit/web.w:1027
	n := len(lines)
//line lit/web.w:1028
	for i := 0; i < n; {
//line lit/web.w:1029
		if !isChangeCtrl(lines[i], 'x') {
//line lit/web.w:1030
			i++ // commentary between changes
//line lit/web.w:1031
			continue
//line lit/web.w:1032
		}
//line lit/web.w:1033
		c := change{line: i + 1}
//line lit/web.w:1034
		i++
//line lit/web.w:1035
		for i < n && !isChangeCtrl(lines[i], 'y') {
//line lit/web.w:1036
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
//line lit/web.w:1037
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
//line lit/web.w:1038
			}
//line lit/web.w:1039
			c.match = append(c.match, lines[i])
//line lit/web.w:1040
			i++
//line lit/web.w:1041
		}
//line lit/web.w:1042
		if i >= n {
//line lit/web.w:1043
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
//line lit/web.w:1044
		}
//line lit/web.w:1045
		i++ // skip @y
//line lit/web.w:1046
		c.replLine = i + 1
//line lit/web.w:1047
		for i < n && !isChangeCtrl(lines[i], 'z') {
//line lit/web.w:1048
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
//line lit/web.w:1049
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
//line lit/web.w:1050
			}
//line lit/web.w:1051
			c.repl = append(c.repl, lines[i])
//line lit/web.w:1052
			i++
//line lit/web.w:1053
		}
//line lit/web.w:1054
		if i >= n {
//line lit/web.w:1055
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
//line lit/web.w:1056
		}
//line lit/web.w:1057
		i++ // skip @z
//line lit/web.w:1058
		if len(c.match) == 0 {
//line lit/web.w:1059
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
//line lit/web.w:1060
		}
//line lit/web.w:1061
		changes = append(changes, c)
//line lit/web.w:1062
	}
//line lit/web.w:1063
	return changes, nil
//line lit/web.w:1064
}

// applyChanges returns src with the changes applied (string convenience form,
// used by tests). See applyChangesMapped for the origin-tracking version.
//
//line lit/web.w:1068
//line lit/web.w:1069
//line lit/web.w:1070
func applyChanges(src string, changes []change, chFile string) (string, error) {
//line lit/web.w:1071
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
//line lit/web.w:1072
	if err != nil {
//line lit/web.w:1073
		return "", err
//line lit/web.w:1074
	}
//line lit/web.w:1075
	return strings.Join(out, "\n"), nil
//line lit/web.w:1076
}

// applyChangesMapped applies changes to master, keeping a parallel origin map in
// step: passed-through lines keep their origin, and replacement lines are
// attributed to the change file. locs may be nil if origins are not tracked.
// chFile names the change file for diagnostics. It is an error if a change's
// first line is never found, or is found but the rest of the block does not
// match.
//
//line lit/web.w:1083
//line lit/web.w:1084
//line lit/web.w:1085
//line lit/web.w:1086
//line lit/web.w:1087
//line lit/web.w:1088
//line lit/web.w:1089
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) ([]string, []srcLoc, error) {
//line lit/web.w:1090
	loc := func(i int) srcLoc {
//line lit/web.w:1091
		if locs != nil && i < len(locs) {
//line lit/web.w:1092
			return locs[i]
//line lit/web.w:1093
		}
//line lit/web.w:1094
		return srcLoc{line: i + 1}
//line lit/web.w:1095
	}
//line lit/web.w:1096
	out := make([]string, 0, len(master))
//line lit/web.w:1097
	var outLocs []srcLoc
//line lit/web.w:1098
	ci := 0
//line lit/web.w:1099
	for i := 0; i < len(master); {
//line lit/web.w:1100
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
//line lit/web.w:1101
			if !blockMatches(master, i, changes[ci].match) {
//line lit/web.w:1102
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
//line lit/web.w:1103
					chFile, changes[ci].line, loc(i))
//line lit/web.w:1104
			}
//line lit/web.w:1105
			for r, rl := range changes[ci].repl {
//line lit/web.w:1106
				out = append(out, rl)
//line lit/web.w:1107
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
//line lit/web.w:1108
			}
//line lit/web.w:1109
			i += len(changes[ci].match)
//line lit/web.w:1110
			ci++
//line lit/web.w:1111
			continue
//line lit/web.w:1112
		}
//line lit/web.w:1113
		out = append(out, master[i])
//line lit/web.w:1114
		outLocs = append(outLocs, loc(i))
//line lit/web.w:1115
		i++
//line lit/web.w:1116
	}
//line lit/web.w:1117
	if ci < len(changes) {
//line lit/web.w:1118
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
//line lit/web.w:1119
			chFile, changes[ci].line, changes[ci].match[0])
//line lit/web.w:1120
	}
//line lit/web.w:1121
	return out, outLocs, nil
//line lit/web.w:1122
}

// blockMatches reports whether match lines up with master starting at index at.
//
//line lit/web.w:1127
//line lit/web.w:1128
func blockMatches(master []string, at int, match []string) bool {
//line lit/web.w:1129
	if at+len(match) > len(master) {
//line lit/web.w:1130
		return false
//line lit/web.w:1131
	}
//line lit/web.w:1132
	for k, m := range match {
//line lit/web.w:1133
		if !sameLine(master[at+k], m) {
//line lit/web.w:1134
			return false
//line lit/web.w:1135
		}
//line lit/web.w:1136
	}
//line lit/web.w:1137
	return true
//line lit/web.w:1138
}
