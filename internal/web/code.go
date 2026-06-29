//line lit/web.w:812
package web

//line lit/web.w:814
import "strings"

// AtomKind classifies a piece of a code part.
//
//line lit/web.w:816
//line lit/web.w:817
type AtomKind int

//line lit/web.w:819
const (
//line lit/web.w:820
	AText AtomKind = iota // ordinary Go source text
//line lit/web.w:821
	ARef // @<name@> reference to a named section
//line lit/web.w:822
	AVerbatim // @=text@> passed verbatim to tangled output
//line lit/web.w:823
	ATeX // @t text@> TeX text for the woven output
//line lit/web.w:824
	AIndex // @^/@./@: index entry
//line lit/web.w:825
	APaste // @& join (delete surrounding whitespace)
//line lit/web.w:826
	ALayout // @, @/ @| @# woven-output layout hints
//line lit/web.w:827
	AIndexDef // @! force the next identifier to index as a definition
//line lit/web.w:828
)

// Atom is one element of a scanned code part.
//
//line lit/web.w:830
//line lit/web.w:831
type Atom struct {
//line lit/web.w:832
	Kind AtomKind
//line lit/web.w:833
	Text string // payload for AText/AVerbatim/ATeX/AIndex; name for ARef
//line lit/web.w:834
	Index byte // '^','.',':' for AIndex; ',' '/' '|' '#' for ALayout
//line lit/web.w:835
}

// ScanCode splits a raw code part into atoms, interpreting in-code control
// codes. "@@" becomes a literal '@' folded into the surrounding text.
//
//line lit/web.w:841
//line lit/web.w:842
//line lit/web.w:843
func ScanCode(code string) []Atom {
//line lit/web.w:844
	var atoms []Atom
//line lit/web.w:845
	var buf strings.Builder
//line lit/web.w:846
	flush := func() {
//line lit/web.w:847
		if buf.Len() > 0 {
//line lit/web.w:848
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
//line lit/web.w:849
			buf.Reset()
//line lit/web.w:850
		}
//line lit/web.w:851
	}
//line lit/web.w:852
	n := len(code)
//line lit/web.w:853
	i := 0
//line lit/web.w:854
	for i < n {
//line lit/web.w:855
		c := code[i]
//line lit/web.w:856
		if c != '@' || i+1 >= n {
//line lit/web.w:857
			buf.WriteByte(c)
//line lit/web.w:858
			i++
//line lit/web.w:859
			continue
//line lit/web.w:860
		}
//line lit/web.w:861
		switch d := code[i+1]; d {
//line lit/web.w:862
		case '@':
//line lit/web.w:863
			buf.WriteByte('@')
//line lit/web.w:864
			i += 2
//line lit/web.w:865
		case '&':
//line lit/web.w:866
			flush()
//line lit/web.w:867
			atoms = append(atoms, Atom{Kind: APaste})
//line lit/web.w:868
			i += 2
//line lit/web.w:869
		case '<':
//line lit/web.w:870
			end := indexFrom(code, "@>", i+2)
//line lit/web.w:871
			if end < 0 {
//line lit/web.w:872
				buf.WriteString(code[i:])
//line lit/web.w:873
				i = n
//line lit/web.w:874
				continue
//line lit/web.w:875
			}
//line lit/web.w:876
			flush()
//line lit/web.w:877
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
//line lit/web.w:878
			i = end + 2
//line lit/web.w:879
		case '=':
//line lit/web.w:880
			end := indexFrom(code, "@>", i+2)
//line lit/web.w:881
			if end < 0 {
//line lit/web.w:882
				i = n
//line lit/web.w:883
				continue
//line lit/web.w:884
			}
//line lit/web.w:885
			flush()
//line lit/web.w:886
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
//line lit/web.w:887
			i = end + 2
//line lit/web.w:888
		case 't':
//line lit/web.w:889
			end := indexFrom(code, "@>", i+2)
//line lit/web.w:890
			if end < 0 {
//line lit/web.w:891
				i = n
//line lit/web.w:892
				continue
//line lit/web.w:893
			}
//line lit/web.w:894
			flush()
//line lit/web.w:895
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
//line lit/web.w:896
			i = end + 2
//line lit/web.w:897
		case '^', '.', ':':
//line lit/web.w:898
			end := indexFrom(code, "@>", i+2)
//line lit/web.w:899
			if end < 0 {
//line lit/web.w:900
				i = n
//line lit/web.w:901
				continue
//line lit/web.w:902
			}
//line lit/web.w:903
			flush()
//line lit/web.w:904
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
//line lit/web.w:905
			i = end + 2
//line lit/web.w:906
		case 'q':
//line lit/web.w:907
			end := indexFrom(code, "@>", i+2)
//line lit/web.w:908
			if end < 0 {
//line lit/web.w:909
				i = n
//line lit/web.w:910
				continue
//line lit/web.w:911
			}
//line lit/web.w:912
			i = end + 2 // ignored material
//line lit/web.w:913
		case '%':
//line lit/web.w:914
			j := i + 2
//line lit/web.w:915
			for j < n && code[j] != '\n' {
//line lit/web.w:916
				j++
//line lit/web.w:917
			}
//line lit/web.w:918
			i = j
//line lit/web.w:919
		case '>':
//line lit/web.w:920
			i += 2 // stray terminator
//line lit/web.w:921
		case ',', '/', '|', '#':
//line lit/web.w:922
			// Woven-output layout hints: thin space, line break, optional line
//line lit/web.w:923
			// break, and break-plus-blank-line. Ignored by gtangle.
//line lit/web.w:924
			flush()
//line lit/web.w:925
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
//line lit/web.w:926
			i += 2
//line lit/web.w:927
		case '!':
//line lit/web.w:928
			// Force the next identifier's index entry to be a definition,
//line lit/web.w:929
			// overriding the heuristic. Produces no output by itself.
//line lit/web.w:930
			flush()
//line lit/web.w:931
			atoms = append(atoms, Atom{Kind: AIndexDef})
//line lit/web.w:932
			i += 2
//line lit/web.w:933
		case '+', '[', ']', ';':
//line lit/web.w:934
			// CWEB prettyprinter hints (cancel break, expression brackets,
//line lit/web.w:935
			// invisible semicolon). GWEB mirrors the source instead of reflowing
//line lit/web.w:936
			// it, so these have no effect; accept and drop them for portability.
//line lit/web.w:937
			i += 2
//line lit/web.w:938
		default:
//line lit/web.w:939
			i += 2 // unknown @x: drop it rather than corrupt the output
//line lit/web.w:940
		}
//line lit/web.w:941
	}
//line lit/web.w:942
	flush()
//line lit/web.w:943
	return atoms
//line lit/web.w:944
}
