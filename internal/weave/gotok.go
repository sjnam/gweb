//line lit/weave.w:873
package weave

//line lit/weave.w:875
// A small, line-oriented Go lexer for the woven output. Unlike go/scanner it
//line lit/weave.w:876
// tolerates the partial fragments found in web sections and reports whitespace,
//line lit/weave.w:877
// newlines, and comments as tokens so the pretty-printer can preserve layout.
//line lit/weave.w:878
// State (open block comment / raw string) is carried across calls because a
//line lit/weave.w:879
// code part may be interrupted by @<...@> references.

//line lit/weave.w:881
type tokKind int

//line lit/weave.w:885
const (
//line lit/weave.w:886
	tkIdent tokKind = iota // ordinary identifier
//line lit/weave.w:887
	tkKeyword // Go reserved word
//line lit/weave.w:888
	tkBuiltin // predeclared type or constant (also set bold)
//line lit/weave.w:889
	tkNumber // numeric literal
//line lit/weave.w:890
	tkString // "..." or `...` or '...'
//line lit/weave.w:891
	tkComment // // or /* */ text (no trailing newline)
//line lit/weave.w:892
	tkOp // operator or punctuation run
//line lit/weave.w:893
	tkSpace // a run of spaces/tabs
//line lit/weave.w:894
	tkNewline // a single '\n'
//line lit/weave.w:895
	tkMacro // typewriter: an  name or a predeclared constant
//line lit/weave.w:896
)

//line lit/weave.w:901
type token struct {
//line lit/weave.w:902
	kind tokKind
//line lit/weave.w:903
	text string
//line lit/weave.w:904
}

// lexState carries lexer state across fragments of one code part.
//
//line lit/weave.w:906
//line lit/weave.w:907
type lexState struct {
//line lit/weave.w:908
	inBlockComment bool
//line lit/weave.w:909
	inRawString bool
//line lit/weave.w:910
}

//line lit/weave.w:914
var goKeywords = map[string]bool{
//line lit/weave.w:915
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
//line lit/weave.w:916
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
//line lit/weave.w:917
	"func": true, "go": true, "goto": true, "if": true, "import": true,
//line lit/weave.w:918
	"interface": true, "map": true, "package": true, "range": true, "return": true,
//line lit/weave.w:919
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
//line lit/weave.w:920
}

//line lit/weave.w:922
var goBuiltins = map[string]bool{
//line lit/weave.w:923
	"bool": true, "byte": true, "complex64": true, "complex128": true, "error": true,
//line lit/weave.w:924
	"float32": true, "float64": true, "int": true, "int8": true, "int16": true,
//line lit/weave.w:925
	"int32": true, "int64": true, "rune": true, "string": true, "uint": true,
//line lit/weave.w:926
	"uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
//line lit/weave.w:927
	"any": true, "comparable": true,
//line lit/weave.w:928
}

// The predeclared constant values are set in typewriter (like a const), not bold
// like the predeclared types; they denote values, not types. (nil is the
// exception: renderToken shows it as a symbol, the way cweave shows C's NULL.)
//
//line lit/weave.w:930
//line lit/weave.w:931
//line lit/weave.w:932
//line lit/weave.w:933
var goConstants = map[string]bool{"nil": true, "true": true, "false": true, "iota": true}

//line lit/weave.w:940
func classifyWord(w string) tokKind {
//line lit/weave.w:941
	switch {
//line lit/weave.w:942
	case goKeywords[w]:
//line lit/weave.w:943
		return tkKeyword
//line lit/weave.w:944
	case goConstants[w]:
//line lit/weave.w:945
		return tkMacro // a predeclared constant: typewriter, like a const
//line lit/weave.w:946
	case goBuiltins[w]:
//line lit/weave.w:947
		return tkBuiltin
//line lit/weave.w:948
	default:
//line lit/weave.w:949
		return tkIdent
//line lit/weave.w:950
	}
//line lit/weave.w:951
}

//line lit/weave.w:953
func isIdentStart(c byte) bool {
//line lit/weave.w:954
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
//line lit/weave.w:955
}

//line lit/weave.w:956
func isIdentPart(c byte) bool {
//line lit/weave.w:957
	return isIdentStart(c) || (c >= '0' && c <= '9')
//line lit/weave.w:958
}

//line lit/weave.w:959
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// lexGo tokenizes src, updating *st. Newlines and whitespace runs are returned
// as their own tokens.
//
//line lit/weave.w:964
//line lit/weave.w:965
//line lit/weave.w:966
func lexGo(src string, st *lexState) []token {
//line lit/weave.w:967
	var toks []token
//line lit/weave.w:968
	n := len(src)
//line lit/weave.w:969
	i := 0
//line lit/weave.w:970
	for i < n {
//line lit/weave.w:971
		// Resume an open block comment.
//line lit/weave.w:972
		if st.inBlockComment {
//line lit/weave.w:973
			if end := indexStr(src, "*/", i); end >= 0 {
//line lit/weave.w:974
				toks = append(toks, token{tkComment, src[i : end+2]})
//line lit/weave.w:975
				st.inBlockComment = false
//line lit/weave.w:976
				i = end + 2
//line lit/weave.w:977
			} else if nl := indexByte(src, '\n', i); nl >= 0 {
//line lit/weave.w:978
				if nl > i {
//line lit/weave.w:979
					toks = append(toks, token{tkComment, src[i:nl]})
//line lit/weave.w:980
				}
//line lit/weave.w:981
				toks = append(toks, token{tkNewline, "\n"})
//line lit/weave.w:982
				i = nl + 1
//line lit/weave.w:983
			} else {
//line lit/weave.w:984
				toks = append(toks, token{tkComment, src[i:]})
//line lit/weave.w:985
				i = n
//line lit/weave.w:986
			}
//line lit/weave.w:987
			continue
//line lit/weave.w:988
		}
//line lit/weave.w:989
		// Resume an open raw string.
//line lit/weave.w:990
		if st.inRawString {
//line lit/weave.w:991
			if end := indexByte(src, '`', i); end >= 0 {
//line lit/weave.w:992
				toks = append(toks, token{tkString, src[i : end+1]})
//line lit/weave.w:993
				st.inRawString = false
//line lit/weave.w:994
				i = end + 1
//line lit/weave.w:995
			} else if nl := indexByte(src, '\n', i); nl >= 0 {
//line lit/weave.w:996
				if nl > i {
//line lit/weave.w:997
					toks = append(toks, token{tkString, src[i:nl]})
//line lit/weave.w:998
				}
//line lit/weave.w:999
				toks = append(toks, token{tkNewline, "\n"})
//line lit/weave.w:1000
				i = nl + 1
//line lit/weave.w:1001
			} else {
//line lit/weave.w:1002
				toks = append(toks, token{tkString, src[i:]})
//line lit/weave.w:1003
				i = n
//line lit/weave.w:1004
			}
//line lit/weave.w:1005
			continue
//line lit/weave.w:1006
		}

//line lit/weave.w:1008
		c := src[i]
//line lit/weave.w:1009
		switch {
//line lit/weave.w:1010
		case c == '\n':
//line lit/weave.w:1011
			toks = append(toks, token{tkNewline, "\n"})
//line lit/weave.w:1012
			i++
//line lit/weave.w:1013
		case c == ' ' || c == '\t' || c == '\r':
//line lit/weave.w:1014
			j := i
//line lit/weave.w:1015
			for j < n && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r') {
//line lit/weave.w:1016
				j++
//line lit/weave.w:1017
			}
//line lit/weave.w:1018
			toks = append(toks, token{tkSpace, src[i:j]})
//line lit/weave.w:1019
			i = j
//line lit/weave.w:1020
		case c == '/' && i+1 < n && src[i+1] == '/':
//line lit/weave.w:1021
			j := indexByte(src, '\n', i)
//line lit/weave.w:1022
			if j < 0 {
//line lit/weave.w:1023
				j = n
//line lit/weave.w:1024
			}
//line lit/weave.w:1025
			toks = append(toks, token{tkComment, src[i:j]})
//line lit/weave.w:1026
			i = j
//line lit/weave.w:1027
		case c == '/' && i+1 < n && src[i+1] == '*':
//line lit/weave.w:1028
			if end := indexStr(src, "*/", i+2); end >= 0 {
//line lit/weave.w:1029
				toks = append(toks, token{tkComment, src[i : end+2]})
//line lit/weave.w:1030
				i = end + 2
//line lit/weave.w:1031
			} else if nl := indexByte(src, '\n', i); nl >= 0 {
//line lit/weave.w:1032
				toks = append(toks, token{tkComment, src[i:nl]})
//line lit/weave.w:1033
				toks = append(toks, token{tkNewline, "\n"})
//line lit/weave.w:1034
				st.inBlockComment = true
//line lit/weave.w:1035
				i = nl + 1
//line lit/weave.w:1036
			} else {
//line lit/weave.w:1037
				toks = append(toks, token{tkComment, src[i:]})
//line lit/weave.w:1038
				st.inBlockComment = true
//line lit/weave.w:1039
				i = n
//line lit/weave.w:1040
			}
//line lit/weave.w:1041
		case c == '"':
//line lit/weave.w:1042
			i = lexQuoted(src, i, '"', &toks)
//line lit/weave.w:1043
		case c == '\'':
//line lit/weave.w:1044
			i = lexQuoted(src, i, '\'', &toks)
//line lit/weave.w:1045
		case c == '`':
//line lit/weave.w:1046
			if end := indexByte(src, '`', i+1); end >= 0 {
//line lit/weave.w:1047
				toks = append(toks, token{tkString, src[i : end+1]})
//line lit/weave.w:1048
				i = end + 1
//line lit/weave.w:1049
			} else if nl := indexByte(src, '\n', i+1); nl >= 0 {
//line lit/weave.w:1050
				toks = append(toks, token{tkString, src[i:nl]})
//line lit/weave.w:1051
				toks = append(toks, token{tkNewline, "\n"})
//line lit/weave.w:1052
				st.inRawString = true
//line lit/weave.w:1053
				i = nl + 1
//line lit/weave.w:1054
			} else {
//line lit/weave.w:1055
				toks = append(toks, token{tkString, src[i:]})
//line lit/weave.w:1056
				st.inRawString = true
//line lit/weave.w:1057
				i = n
//line lit/weave.w:1058
			}
//line lit/weave.w:1059
		case isIdentStart(c):
//line lit/weave.w:1060
			j := i + 1
//line lit/weave.w:1061
			for j < n && isIdentPart(src[j]) {
//line lit/weave.w:1062
				j++
//line lit/weave.w:1063
			}
//line lit/weave.w:1064
			w := src[i:j]
//line lit/weave.w:1065
			toks = append(toks, token{classifyWord(w), w})
//line lit/weave.w:1066
			i = j
//line lit/weave.w:1067
		case isDigit(c) || (c == '.' && i+1 < n && isDigit(src[i+1])):
//line lit/weave.w:1068
			j := i + 1
//line lit/weave.w:1069
			for j < n && isNumberPart(src[j]) {
//line lit/weave.w:1070
				j++
//line lit/weave.w:1071
			}
//line lit/weave.w:1072
			toks = append(toks, token{tkNumber, src[i:j]})
//line lit/weave.w:1073
			i = j
//line lit/weave.w:1074
		default:
//line lit/weave.w:1075
			if l := matchOp(src, i); l > 0 {
//line lit/weave.w:1076
				toks = append(toks, token{tkOp, src[i : i+l]})
//line lit/weave.w:1077
				i += l
//line lit/weave.w:1078
			} else {
//line lit/weave.w:1079
				toks = append(toks, token{tkOp, string(c)})
//line lit/weave.w:1080
				i++
//line lit/weave.w:1081
			}
//line lit/weave.w:1082
		}
//line lit/weave.w:1083
	}
//line lit/weave.w:1084
	return toks
//line lit/weave.w:1085
}

// multiOps lists Go's multi-character operators, longest first, so matchOp can
// greedily combine them into single tokens.
//
//line lit/weave.w:1091
//line lit/weave.w:1092
//line lit/weave.w:1093
var multiOps = []string{
//line lit/weave.w:1094
	"<<=", ">>=", "&^=", "...",
//line lit/weave.w:1095
	"<-", "++", "--", "==", "!=", "<=", ">=", ":=", "&&", "||",
//line lit/weave.w:1096
	"<<", ">>", "&^", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
//line lit/weave.w:1097
	"[]", // the empty brackets of a slice/array type, kept as one token
//line lit/weave.w:1098
	"{}", // empty braces (struct{}, interface{}, T{}), kept as one token
//line lit/weave.w:1099
}

//line lit/weave.w:1101
func matchOp(src string, i int) int {
//line lit/weave.w:1102
	for _, op := range multiOps {
//line lit/weave.w:1103
		if i+len(op) <= len(src) && src[i:i+len(op)] == op {
//line lit/weave.w:1104
			return len(op)
//line lit/weave.w:1105
		}
//line lit/weave.w:1106
	}
//line lit/weave.w:1107
	return 0
//line lit/weave.w:1108
}

// lexQuoted scans an interpreted string ("...") or rune ('...') starting at i,
// honoring backslash escapes, and appends a tkString token. It stops at the
// closing quote or end of line (unterminated literals are tolerated).
//
//line lit/weave.w:1113
//line lit/weave.w:1114
//line lit/weave.w:1115
//line lit/weave.w:1116
func lexQuoted(src string, i int, quote byte, toks *[]token) int {
//line lit/weave.w:1117
	n := len(src)
//line lit/weave.w:1118
	j := i + 1
//line lit/weave.w:1119
	for j < n {
//line lit/weave.w:1120
		if src[j] == '\\' && j+1 < n {
//line lit/weave.w:1121
			j += 2
//line lit/weave.w:1122
			continue
//line lit/weave.w:1123
		}
//line lit/weave.w:1124
		if src[j] == quote || src[j] == '\n' {
//line lit/weave.w:1125
			break
//line lit/weave.w:1126
		}
//line lit/weave.w:1127
		j++
//line lit/weave.w:1128
	}
//line lit/weave.w:1129
	if j < n && src[j] == quote {
//line lit/weave.w:1130
		j++
//line lit/weave.w:1131
	}
//line lit/weave.w:1132
	*toks = append(*toks, token{tkString, src[i:j]})
//line lit/weave.w:1133
	return j
//line lit/weave.w:1134
}

//line lit/weave.w:1138
func isNumberPart(c byte) bool {
//line lit/weave.w:1139
	// Note: '+'/'-' (exponent signs) are intentionally excluded so that "1+2"
//line lit/weave.w:1140
	// is not swallowed as a single number; "1e+10" splits harmlessly instead.
//line lit/weave.w:1141
	return isDigit(c) || c == '.' || c == '_' ||
//line lit/weave.w:1142
		(c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') ||
//line lit/weave.w:1143
		c == 'x' || c == 'X' || c == 'o' || c == 'O' || c == 'b' || c == 'B' ||
//line lit/weave.w:1144
		c == 'p' || c == 'P'
//line lit/weave.w:1145
}

//line lit/weave.w:1147
func indexByte(s string, b byte, from int) int {
//line lit/weave.w:1148
	for i := from; i < len(s); i++ {
//line lit/weave.w:1149
		if s[i] == b {
//line lit/weave.w:1150
			return i
//line lit/weave.w:1151
		}
//line lit/weave.w:1152
	}
//line lit/weave.w:1153
	return -1
//line lit/weave.w:1154
}

//line lit/weave.w:1156
func indexStr(s, sub string, from int) int {
//line lit/weave.w:1157
	for i := from; i+len(sub) <= len(s); i++ {
//line lit/weave.w:1158
		if s[i:i+len(sub)] == sub {
//line lit/weave.w:1159
			return i
//line lit/weave.w:1160
		}
//line lit/weave.w:1161
	}
//line lit/weave.w:1162
	return -1
//line lit/weave.w:1163
}
