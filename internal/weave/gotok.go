package weave

// A small, line-oriented Go lexer for the woven output. Unlike go/scanner it
// tolerates the partial fragments found in web sections and reports whitespace,
// newlines, and comments as tokens so the pretty-printer can preserve layout.
// State (open block comment / raw string) is carried across calls because a
// code part may be interrupted by @<...@> references.

type tokKind int

const (
	tkIdent   tokKind = iota // ordinary identifier
	tkKeyword                // Go reserved word
	tkBuiltin                // predeclared type or constant (also set bold)
	tkNumber                 // numeric literal
	tkString                 // "..." or `...` or '...'
	tkComment                // // or /* */ text (no trailing newline)
	tkOp                     // operator or punctuation run
	tkSpace                  // a run of spaces/tabs
	tkNewline                // a single '\n'
	tkMacro                  // a const name (set in typewriter, like a CWEB  macro)
)

type token struct {
	kind tokKind
	text string
}

// lexState carries lexer state across fragments of one code part.
type lexState struct {
	inBlockComment bool
	inRawString    bool
}

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

var goBuiltins = map[string]bool{
	"bool": true, "byte": true, "complex64": true, "complex128": true, "error": true,
	"float32": true, "float64": true, "int": true, "int8": true, "int16": true,
	"int32": true, "int64": true, "rune": true, "string": true, "uint": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"any": true, "comparable": true,
}

// The predeclared constant values are set in typewriter (like a const), not bold
// like the predeclared types; they denote values, not types.
var goConstants = map[string]bool{"nil": true, "true": true, "false": true, "iota": true}

func classifyWord(w string) tokKind {
	switch {
	case goKeywords[w]:
		return tkKeyword
	case goConstants[w]:
		return tkMacro // a predeclared constant: typewriter, like a const
	case goBuiltins[w]:
		return tkBuiltin
	default:
		return tkIdent
	}
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}
func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// lexGo tokenizes src, updating *st. Newlines and whitespace runs are returned
// as their own tokens.
func lexGo(src string, st *lexState) []token {
	var toks []token
	n := len(src)
	i := 0
	for i < n {
		// Resume an open block comment.
		if st.inBlockComment {
			if end := indexStr(src, "*/", i); end >= 0 {
				toks = append(toks, token{tkComment, src[i : end+2]})
				st.inBlockComment = false
				i = end + 2
			} else if nl := indexByte(src, '\n', i); nl >= 0 {
				if nl > i {
					toks = append(toks, token{tkComment, src[i:nl]})
				}
				toks = append(toks, token{tkNewline, "\n"})
				i = nl + 1
			} else {
				toks = append(toks, token{tkComment, src[i:]})
				i = n
			}
			continue
		}
		// Resume an open raw string.
		if st.inRawString {
			if end := indexByte(src, '`', i); end >= 0 {
				toks = append(toks, token{tkString, src[i : end+1]})
				st.inRawString = false
				i = end + 1
			} else if nl := indexByte(src, '\n', i); nl >= 0 {
				if nl > i {
					toks = append(toks, token{tkString, src[i:nl]})
				}
				toks = append(toks, token{tkNewline, "\n"})
				i = nl + 1
			} else {
				toks = append(toks, token{tkString, src[i:]})
				i = n
			}
			continue
		}

		c := src[i]
		switch {
		case c == '\n':
			toks = append(toks, token{tkNewline, "\n"})
			i++
		case c == ' ' || c == '\t' || c == '\r':
			j := i
			for j < n && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r') {
				j++
			}
			toks = append(toks, token{tkSpace, src[i:j]})
			i = j
		case c == '/' && i+1 < n && src[i+1] == '/':
			j := indexByte(src, '\n', i)
			if j < 0 {
				j = n
			}
			toks = append(toks, token{tkComment, src[i:j]})
			i = j
		case c == '/' && i+1 < n && src[i+1] == '*':
			if end := indexStr(src, "*/", i+2); end >= 0 {
				toks = append(toks, token{tkComment, src[i : end+2]})
				i = end + 2
			} else if nl := indexByte(src, '\n', i); nl >= 0 {
				toks = append(toks, token{tkComment, src[i:nl]})
				toks = append(toks, token{tkNewline, "\n"})
				st.inBlockComment = true
				i = nl + 1
			} else {
				toks = append(toks, token{tkComment, src[i:]})
				st.inBlockComment = true
				i = n
			}
		case c == '"':
			i = lexQuoted(src, i, '"', &toks)
		case c == '\'':
			i = lexQuoted(src, i, '\'', &toks)
		case c == '`':
			if end := indexByte(src, '`', i+1); end >= 0 {
				toks = append(toks, token{tkString, src[i : end+1]})
				i = end + 1
			} else if nl := indexByte(src, '\n', i+1); nl >= 0 {
				toks = append(toks, token{tkString, src[i:nl]})
				toks = append(toks, token{tkNewline, "\n"})
				st.inRawString = true
				i = nl + 1
			} else {
				toks = append(toks, token{tkString, src[i:]})
				st.inRawString = true
				i = n
			}
		case isIdentStart(c):
			j := i + 1
			for j < n && isIdentPart(src[j]) {
				j++
			}
			w := src[i:j]
			toks = append(toks, token{classifyWord(w), w})
			i = j
		case isDigit(c) || (c == '.' && i+1 < n && isDigit(src[i+1])):
			j := i + 1
			for j < n && isNumberPart(src[j]) {
				j++
			}
			toks = append(toks, token{tkNumber, src[i:j]})
			i = j
		default:
			if l := matchOp(src, i); l > 0 {
				toks = append(toks, token{tkOp, src[i : i+l]})
				i += l
			} else {
				toks = append(toks, token{tkOp, string(c)})
				i++
			}
		}
	}
	return toks
}

// multiOps lists Go's multi-character operators, longest first, so matchOp can
// greedily combine them into single tokens.
var multiOps = []string{
	"<<=", ">>=", "&^=", "...",
	"<-", "++", "--", "==", "!=", "<=", ">=", ":=", "&&", "||",
	"<<", ">>", "&^", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
	"[]", // the empty brackets of a slice/array type, kept as one token
	"{}", // empty braces (struct{}, interface{}, T{}), kept as one token
}

func matchOp(src string, i int) int {
	for _, op := range multiOps {
		if i+len(op) <= len(src) && src[i:i+len(op)] == op {
			return len(op)
		}
	}
	return 0
}

// lexQuoted scans an interpreted string ("...") or rune ('...') starting at i,
// honoring backslash escapes, and appends a tkString token. It stops at the
// closing quote or end of line (unterminated literals are tolerated).
func lexQuoted(src string, i int, quote byte, toks *[]token) int {
	n := len(src)
	j := i + 1
	for j < n {
		if src[j] == '\\' && j+1 < n {
			j += 2
			continue
		}
		if src[j] == quote || src[j] == '\n' {
			break
		}
		j++
	}
	if j < n && src[j] == quote {
		j++
	}
	*toks = append(*toks, token{tkString, src[i:j]})
	return j
}

func isNumberPart(c byte) bool {
	// Note: '+'/'-' (exponent signs) are intentionally excluded so that "1+2"
	// is not swallowed as a single number; "1e+10" splits harmlessly instead.
	return isDigit(c) || c == '.' || c == '_' ||
		(c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') ||
		c == 'x' || c == 'X' || c == 'o' || c == 'O' || c == 'b' || c == 'B' ||
		c == 'p' || c == 'P'
}

func indexByte(s string, b byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func indexStr(s, sub string, from int) int {
	for i := from; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
