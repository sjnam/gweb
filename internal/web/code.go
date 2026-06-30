//line internal/web/web.w:812
package web

//line internal/web/web.w:814
import "strings"

// AtomKind classifies a piece of a code part.
//
//line internal/web/web.w:816
//line internal/web/web.w:817
type AtomKind int

//line internal/web/web.w:819
const (
//line internal/web/web.w:820
	AText AtomKind = iota // ordinary Go source text
//line internal/web/web.w:821
	ARef // @<name@> reference to a named section
//line internal/web/web.w:822
	AVerbatim // @=text@> passed verbatim to tangled output
//line internal/web/web.w:823
	ATeX // @t text@> TeX text for the woven output
//line internal/web/web.w:824
	AIndex // @^/@./@: index entry
//line internal/web/web.w:825
	APaste // @& join (delete surrounding whitespace)
//line internal/web/web.w:826
	ALayout // @, @/ @| @# woven-output layout hints
//line internal/web/web.w:827
	AIndexDef // @! force the next identifier to index as a definition
//line internal/web/web.w:828
)

// Atom is one element of a scanned code part.
//
//line internal/web/web.w:830
//line internal/web/web.w:831
type Atom struct {
//line internal/web/web.w:832
	Kind AtomKind
//line internal/web/web.w:833
	Text string // payload for AText/AVerbatim/ATeX/AIndex; name for ARef
//line internal/web/web.w:834
	Index byte // '^','.',':' for AIndex; ',' '/' '|' '#' for ALayout
//line internal/web/web.w:835
}

// ScanCode splits a raw code part into atoms, interpreting in-code control
// codes. "@@" becomes a literal '@' folded into the surrounding text.
//
//line internal/web/web.w:841
//line internal/web/web.w:842
//line internal/web/web.w:843
func ScanCode(code string) []Atom {
//line internal/web/web.w:844
	var atoms []Atom
//line internal/web/web.w:845
	var buf strings.Builder
//line internal/web/web.w:846
	flush := func() {
//line internal/web/web.w:847
		if buf.Len() > 0 {
//line internal/web/web.w:848
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
//line internal/web/web.w:849
			buf.Reset()
//line internal/web/web.w:850
		}
//line internal/web/web.w:851
	}
//line internal/web/web.w:852
	n := len(code)
//line internal/web/web.w:853
	i := 0
//line internal/web/web.w:854
	for i < n {
//line internal/web/web.w:855
		c := code[i]
//line internal/web/web.w:856
		if c != '@' || i+1 >= n {
//line internal/web/web.w:857
			buf.WriteByte(c)
//line internal/web/web.w:858
			i++
//line internal/web/web.w:859
			continue
//line internal/web/web.w:860
		}
//line internal/web/web.w:861
		switch d := code[i+1]; d {
//line internal/web/web.w:862
		case '@':
//line internal/web/web.w:863
			buf.WriteByte('@')
//line internal/web/web.w:864
			i += 2
//line internal/web/web.w:865
		case '&':
//line internal/web/web.w:866
			flush()
//line internal/web/web.w:867
			atoms = append(atoms, Atom{Kind: APaste})
//line internal/web/web.w:868
			i += 2
//line internal/web/web.w:869
		case '<':
//line internal/web/web.w:870
			end := indexFrom(code, "@>", i+2)
//line internal/web/web.w:871
			if end < 0 {
//line internal/web/web.w:872
				buf.WriteString(code[i:])
//line internal/web/web.w:873
				i = n
//line internal/web/web.w:874
				continue
//line internal/web/web.w:875
			}
//line internal/web/web.w:876
			flush()
//line internal/web/web.w:877
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
//line internal/web/web.w:878
			i = end + 2
//line internal/web/web.w:879
		case '=':
//line internal/web/web.w:880
			end := indexFrom(code, "@>", i+2)
//line internal/web/web.w:881
			if end < 0 {
//line internal/web/web.w:882
				i = n
//line internal/web/web.w:883
				continue
//line internal/web/web.w:884
			}
//line internal/web/web.w:885
			flush()
//line internal/web/web.w:886
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
//line internal/web/web.w:887
			i = end + 2
//line internal/web/web.w:888
		case 't':
//line internal/web/web.w:889
			end := indexFrom(code, "@>", i+2)
//line internal/web/web.w:890
			if end < 0 {
//line internal/web/web.w:891
				i = n
//line internal/web/web.w:892
				continue
//line internal/web/web.w:893
			}
//line internal/web/web.w:894
			flush()
//line internal/web/web.w:895
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
//line internal/web/web.w:896
			i = end + 2
//line internal/web/web.w:897
		case '^', '.', ':':
//line internal/web/web.w:898
			end := indexFrom(code, "@>", i+2)
//line internal/web/web.w:899
			if end < 0 {
//line internal/web/web.w:900
				i = n
//line internal/web/web.w:901
				continue
//line internal/web/web.w:902
			}
//line internal/web/web.w:903
			flush()
//line internal/web/web.w:904
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
//line internal/web/web.w:905
			i = end + 2
//line internal/web/web.w:906
		case 'q':
//line internal/web/web.w:907
			end := indexFrom(code, "@>", i+2)
//line internal/web/web.w:908
			if end < 0 {
//line internal/web/web.w:909
				i = n
//line internal/web/web.w:910
				continue
//line internal/web/web.w:911
			}
//line internal/web/web.w:912
			i = end + 2 // ignored material
//line internal/web/web.w:913
		case '%':
//line internal/web/web.w:914
			j := i + 2
//line internal/web/web.w:915
			for j < n && code[j] != '\n' {
//line internal/web/web.w:916
				j++
//line internal/web/web.w:917
			}
//line internal/web/web.w:918
			i = j
//line internal/web/web.w:919
		case '>':
//line internal/web/web.w:920
			i += 2 // stray terminator
//line internal/web/web.w:921
		case ',', '/', '|', '#':
//line internal/web/web.w:922
			// Woven-output layout hints: thin space, line break, optional line
//line internal/web/web.w:923
			// break, and break-plus-blank-line. Ignored by gtangle.
//line internal/web/web.w:924
			flush()
//line internal/web/web.w:925
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
//line internal/web/web.w:926
			i += 2
//line internal/web/web.w:927
		case '!':
//line internal/web/web.w:928
			// Force the next identifier's index entry to be a definition,
//line internal/web/web.w:929
			// overriding the heuristic. Produces no output by itself.
//line internal/web/web.w:930
			flush()
//line internal/web/web.w:931
			atoms = append(atoms, Atom{Kind: AIndexDef})
//line internal/web/web.w:932
			i += 2
//line internal/web/web.w:933
		case '+', '[', ']', ';':
//line internal/web/web.w:934
			// CWEB prettyprinter hints (cancel break, expression brackets,
//line internal/web/web.w:935
			// invisible semicolon). GWEB mirrors the source instead of reflowing
//line internal/web/web.w:936
			// it, so these have no effect; accept and drop them for portability.
//line internal/web/web.w:937
			i += 2
//line internal/web/web.w:938
		default:
//line internal/web/web.w:939
			i += 2 // unknown @x: drop it rather than corrupt the output
//line internal/web/web.w:940
		}
//line internal/web/web.w:941
	}
//line internal/web/web.w:942
	flush()
//line internal/web/web.w:943
	return atoms
//line internal/web/web.w:944
}
