//line lit/web.w:389
package web

//line lit/web.w:391
import (
//line lit/web.w:392
	"fmt"
//line lit/web.w:393
	"strings"
//line lit/web.w:394
)

// ctrlKind classifies a structural control code found while scanning.
//
//line lit/web.w:396
//line lit/web.w:397
type ctrlKind int

//line lit/web.w:399
const (
//line lit/web.w:400
	cEOF ctrlKind = iota
//line lit/web.w:401
	cSection
//line lit/web.w:402
	cCode // @c (or its synonym @p)
//line lit/web.w:403
	cNamed // @<name@>= or @(file@>=
//line lit/web.w:404
	cDefn // @d
//line lit/web.w:405
	cFormat

//line lit/web.w:406
)

//line lit/web.w:408
type ctrl struct {
//line lit/web.w:409
	kind ctrlKind
//line lit/web.w:410
	pos int // index of the leading '@'
//line lit/web.w:411
	end int // index just past the control token
//line lit/web.w:412
	depth int // for cSection: -1 unstarred (or @** top level), else starred depth
//line lit/web.w:413
	starred bool // for cSection (distinguishes @** from an unstarred section)
//line lit/web.w:414
	name string // for cNamed
//line lit/web.w:415
	isFile bool // for cNamed (@( vs @<)
//line lit/web.w:416
	noIndex bool // for cFormat (@s)
//line lit/web.w:417
}

// scanStruct finds the next structural control at or after i. It skips literal
// "@@" and argument-terminated codes (@<...@>, @=...@>, etc.) so their contents
// never trigger a false section break. A "@<...@>" not followed by "=" is a
// reference, not a definition, and is skipped.
//
//line lit/web.w:424
//line lit/web.w:425
//line lit/web.w:426
//line lit/web.w:427
//line lit/web.w:428
func scanStruct(src string, i int) ctrl {
//line lit/web.w:429
	n := len(src)
//line lit/web.w:430
	for i < n {
//line lit/web.w:431
		if src[i] != '@' {
//line lit/web.w:432
			i++
//line lit/web.w:433
			continue
//line lit/web.w:434
		}
//line lit/web.w:435
		if i+1 >= n {
//line lit/web.w:436
			break
//line lit/web.w:437
		}
//line lit/web.w:438
		switch c := src[i+1]; {
//line lit/web.w:439
		case c == '@':
//line lit/web.w:440
			i += 2
//line lit/web.w:441
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line lit/web.w:442
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line lit/web.w:443
		case c == '*':
//line lit/web.w:444
			j := i + 2
//line lit/web.w:445
			depth := 0
//line lit/web.w:446
			if j < n && src[j] == '*' {
//line lit/web.w:447
				j++
//line lit/web.w:448
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line lit/web.w:449
			} else {
//line lit/web.w:450
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line lit/web.w:451
					depth = depth*10 + int(src[j]-'0')
//line lit/web.w:452
					j++
//line lit/web.w:453
				}
//line lit/web.w:454
			}
//line lit/web.w:455
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line lit/web.w:456
		case c == 'c' || c == 'p':
//line lit/web.w:457
			return ctrl{kind: cCode, pos: i, end: i + 2}
//line lit/web.w:458
		case c == 'd':
//line lit/web.w:459
			return ctrl{kind: cDefn, pos: i, end: i + 2}
//line lit/web.w:460
		case c == 'f':
//line lit/web.w:461
			return ctrl{kind: cFormat, pos: i, end: i + 2}
//line lit/web.w:462
		case c == 's':
//line lit/web.w:463
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
//line lit/web.w:464
		case c == '<' || c == '(':
//line lit/web.w:465
			end := indexFrom(src, "@>", i+2)
//line lit/web.w:466
			if end < 0 {
//line lit/web.w:467
				return ctrl{kind: cEOF, pos: n, end: n}
//line lit/web.w:468
			}
//line lit/web.w:469
			after := end + 2
//line lit/web.w:470
			k := after
//line lit/web.w:471
			for k < n && (src[k] == ' ' || src[k] == '\t') {
//line lit/web.w:472
				k++
//line lit/web.w:473
			}
//line lit/web.w:474
			if k < n && src[k] == '=' {
//line lit/web.w:475
				return ctrl{kind: cNamed, pos: i, end: k + 1,
//line lit/web.w:476
					name: canonName(src[i+2 : end]), isFile: c == '('}
//line lit/web.w:477
			}
//line lit/web.w:478
			i = after // a reference, not a definition
//line lit/web.w:479
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line lit/web.w:480
			end := indexFrom(src, "@>", i+2)
//line lit/web.w:481
			if end < 0 {
//line lit/web.w:482
				return ctrl{kind: cEOF, pos: n, end: n}
//line lit/web.w:483
			}
//line lit/web.w:484
			i = end + 2
//line lit/web.w:485
		case c == '%':
//line lit/web.w:486
			j := i + 2
//line lit/web.w:487
			for j < n && src[j] != '\n' {
//line lit/web.w:488
				j++
//line lit/web.w:489
			}
//line lit/web.w:490
			i = j
//line lit/web.w:491
		default:
//line lit/web.w:492
			i += 2
//line lit/web.w:493
		}
//line lit/web.w:494
	}
//line lit/web.w:495
	return ctrl{kind: cEOF, pos: n, end: n}
//line lit/web.w:496
}

// findNextSection scans forward to the next section break (@ or @*), skipping
// everything else including argument-terminated codes. Used inside code parts,
// where @c/@d/@f never legitimately appear.
//
//line lit/web.w:502
//line lit/web.w:503
//line lit/web.w:504
//line lit/web.w:505
func findNextSection(src string, i int) ctrl {
//line lit/web.w:506
	n := len(src)
//line lit/web.w:507
	for i < n {
//line lit/web.w:508
		if src[i] != '@' {
//line lit/web.w:509
			i++
//line lit/web.w:510
			continue
//line lit/web.w:511
		}
//line lit/web.w:512
		if i+1 >= n {
//line lit/web.w:513
			break
//line lit/web.w:514
		}
//line lit/web.w:515
		switch c := src[i+1]; {
//line lit/web.w:516
		case c == '@':
//line lit/web.w:517
			i += 2
//line lit/web.w:518
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line lit/web.w:519
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line lit/web.w:520
		case c == '*':
//line lit/web.w:521
			j := i + 2
//line lit/web.w:522
			depth := 0
//line lit/web.w:523
			if j < n && src[j] == '*' {
//line lit/web.w:524
				j++
//line lit/web.w:525
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line lit/web.w:526
			} else {
//line lit/web.w:527
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line lit/web.w:528
					depth = depth*10 + int(src[j]-'0')
//line lit/web.w:529
					j++
//line lit/web.w:530
				}
//line lit/web.w:531
			}
//line lit/web.w:532
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line lit/web.w:533
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line lit/web.w:534
			end := indexFrom(src, "@>", i+2)
//line lit/web.w:535
			if end < 0 {
//line lit/web.w:536
				return ctrl{kind: cEOF, pos: n, end: n}
//line lit/web.w:537
			}
//line lit/web.w:538
			i = end + 2
//line lit/web.w:539
		case c == '%':
//line lit/web.w:540
			j := i + 2
//line lit/web.w:541
			for j < n && src[j] != '\n' {
//line lit/web.w:542
				j++
//line lit/web.w:543
			}
//line lit/web.w:544
			i = j
//line lit/web.w:545
		default:
//line lit/web.w:546
			i += 2
//line lit/web.w:547
		}
//line lit/web.w:548
	}
//line lit/web.w:549
	return ctrl{kind: cEOF, pos: n, end: n}
//line lit/web.w:550
}

// parse splits source into limbo and sections.
//
//line lit/web.w:555
//line lit/web.w:556
func parse(src string) *Web {
//line lit/web.w:557
	w := &Web{}
//line lit/web.w:558
	n := len(src)

//line lit/web.w:560
	// Limbo runs until the first section break. Format directives placed there
//line lit/web.w:561
	// (@f / @s, a common CWEB idiom) are extracted and removed from the copied
//line lit/web.w:562
	// TeX so they apply globally rather than printing literally.
//line lit/web.w:563
	first := findNextSection(src, 0)
//line lit/web.w:564
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
//line lit/web.w:565
	i := first.pos

//line lit/web.w:567
	num := 0
//line lit/web.w:568
	for i < n {
//line lit/web.w:569
		// We are positioned at a section break.
//line lit/web.w:570
		hdr := src[i+1]
//line lit/web.w:571
		num++
//line lit/web.w:572
		sec := &Section{Number: num, Line: lineAt(src, i)}
//line lit/web.w:573
		if hdr == '*' {
//line lit/web.w:574
			h := findSectionHeaderEnd(src, i)
//line lit/web.w:575
			sec.Starred = true
//line lit/web.w:576
			sec.Depth = h.depth
//line lit/web.w:577
			i = h.end
//line lit/web.w:578
		} else {
//line lit/web.w:579
			i += 2
//line lit/web.w:580
		}

//line lit/web.w:582
		// TeX part: from here to the next structural control.
//line lit/web.w:583
		ct := scanStruct(src, i)
//line lit/web.w:584
		sec.Tex = src[i:ct.pos]
//line lit/web.w:585
		if sec.Starred {
//line lit/web.w:586
			sec.Title = extractTitle(sec.Tex)
//line lit/web.w:587
		}

//line lit/web.w:589
		// Definition part: a run of @d / @f / @s.
//line lit/web.w:590
		for ct.kind == cDefn || ct.kind == cFormat {
//line lit/web.w:591
			nx := scanStruct(src, ct.end)
//line lit/web.w:592
			seg := src[ct.end:nx.pos]
//line lit/web.w:593
			// @d has no Go analogue (Go has no preprocessor), so it never tangles
//line lit/web.w:594
			// to code; gweave uses it only to set the named identifier in
//line lit/web.w:595
			// typewriter, as cweave sets a macro. @f/@s format like another word.
//line lit/web.w:596
			if ct.kind == cDefn {
//line lit/web.w:597
				if f, ok := parseMacro(seg); ok {
//line lit/web.w:598
					sec.Formats = append(sec.Formats, f)
//line lit/web.w:599
				}
//line lit/web.w:600
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
//line lit/web.w:601
				sec.Formats = append(sec.Formats, f)
//line lit/web.w:602
			}
//line lit/web.w:603
			ct = nx
//line lit/web.w:604
		}

//line lit/web.w:606
		switch ct.kind {
//line lit/web.w:607
		case cCode:
//line lit/web.w:608
			sec.HasCode = true
//line lit/web.w:609
			sec.CodeLine = lineAt(src, ct.end)
//line lit/web.w:610
			nx := findNextSection(src, ct.end)
//line lit/web.w:611
			sec.Code = src[ct.end:nx.pos]
//line lit/web.w:612
			i = nx.pos
//line lit/web.w:613
		case cNamed:
//line lit/web.w:614
			sec.HasCode = true
//line lit/web.w:615
			sec.Name = ct.name
//line lit/web.w:616
			sec.IsFile = ct.isFile
//line lit/web.w:617
			sec.CodeLine = lineAt(src, ct.end)
//line lit/web.w:618
			nx := findNextSection(src, ct.end)
//line lit/web.w:619
			sec.Code = src[ct.end:nx.pos]
//line lit/web.w:620
			i = nx.pos
//line lit/web.w:621
		default: // cSection or cEOF: a documentation-only section
//line lit/web.w:622
			i = ct.pos
//line lit/web.w:623
		}

//line lit/web.w:625
		w.Sections = append(w.Sections, sec)
//line lit/web.w:626
		if ct.kind == cEOF && sec.Code == "" {
//line lit/web.w:627
			break
//line lit/web.w:628
		}
//line lit/web.w:629
		if i >= n {
//line lit/web.w:630
			break
//line lit/web.w:631
		}
//line lit/web.w:632
	}
//line lit/web.w:633
	return w
//line lit/web.w:634
}

//line lit/web.w:638
func findSectionHeaderEnd(src string, i int) ctrl {
//line lit/web.w:639
	n := len(src)
//line lit/web.w:640
	j := i + 2
//line lit/web.w:641
	depth := 0
//line lit/web.w:642
	if j < n && src[j] == '*' {
//line lit/web.w:643
		j++
//line lit/web.w:644
		depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line lit/web.w:645
	} else {
//line lit/web.w:646
		for j < n && src[j] >= '0' && src[j] <= '9' {
//line lit/web.w:647
			depth = depth*10 + int(src[j]-'0')
//line lit/web.w:648
			j++
//line lit/web.w:649
		}
//line lit/web.w:650
	}
//line lit/web.w:651
	return ctrl{end: j, depth: depth}
//line lit/web.w:652
}

// extractTitle returns the text of a starred section up to its terminating
// period, with whitespace collapsed, for use in the table of contents. The
// terminator is the first period at end of text or followed by whitespace, so a
// period inside a control sequence such as \.{web} does not end the title early.
//
//line lit/web.w:657
//line lit/web.w:658
//line lit/web.w:659
//line lit/web.w:660
//line lit/web.w:661
func extractTitle(tex string) string {
//line lit/web.w:662
	t := strings.TrimLeft(tex, " \t\n")
//line lit/web.w:663
	if i := titleEnd(t); i >= 0 {
//line lit/web.w:664
		t = t[:i]
//line lit/web.w:665
	}
//line lit/web.w:666
	return strings.Join(strings.Fields(t), " ")
//line lit/web.w:667
}

// titleEnd returns the index of the period that ends a starred-section title --
// the first '.' at end of s or followed by whitespace -- or -1 if there is none.
//
//line lit/web.w:669
//line lit/web.w:670
//line lit/web.w:671
func titleEnd(s string) int {
//line lit/web.w:672
	for i := 0; i < len(s); i++ {
//line lit/web.w:673
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
//line lit/web.w:674
			s[i+1] == '\n' || s[i+1] == '\r') {
//line lit/web.w:675
			return i
//line lit/web.w:676
		}
//line lit/web.w:677
	}
//line lit/web.w:678
	return -1
//line lit/web.w:679
}

// scanDiagnostics walks the source looking for malformed control codes —
// currently argument-terminated codes (@<, @(, @=, @t, @^, @., @:, @q) that are
// missing their closing @> — and returns one warning per problem.
//
//line lit/web.w:684
//line lit/web.w:685
//line lit/web.w:686
//line lit/web.w:687
func (w *Web) scanDiagnostics(src string) []string {
//line lit/web.w:688
	var warns []string
//line lit/web.w:689
	n := len(src)
//line lit/web.w:690
	i := 0
//line lit/web.w:691
	for i < n {
//line lit/web.w:692
		if src[i] != '@' || i+1 >= n {
//line lit/web.w:693
			i++
//line lit/web.w:694
			continue
//line lit/web.w:695
		}
//line lit/web.w:696
		switch c := src[i+1]; c {
//line lit/web.w:697
		case '@':
//line lit/web.w:698
			i += 2
//line lit/web.w:699
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line lit/web.w:700
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line lit/web.w:701
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
//line lit/web.w:702
				i = n
//line lit/web.w:703
			} else {
//line lit/web.w:704
				i = end + 2
//line lit/web.w:705
			}
//line lit/web.w:706
		default:
//line lit/web.w:707
			i += 2
//line lit/web.w:708
		}
//line lit/web.w:709
	}
//line lit/web.w:710
	return warns
//line lit/web.w:711
}

// parseFormat parses the body of an @f/@s directive: two identifiers.
//
//line lit/web.w:715
//line lit/web.w:716
func parseFormat(seg string, noIndex bool) (Format, bool) {
//line lit/web.w:717
	fields := strings.Fields(seg)
//line lit/web.w:718
	if len(fields) < 2 {
//line lit/web.w:719
		return Format{}, false
//line lit/web.w:720
	}
//line lit/web.w:721
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
//line lit/web.w:722
}

// parseMacro parses an @d directive: its first word names a constant to set in
// typewriter; any value after it is ignored (Go has no preprocessor). A
// qualified name keeps its final component, so "@d http.StatusOK" and
// "@d StatusOK" both register StatusOK.
//
//line lit/web.w:730
//line lit/web.w:731
//line lit/web.w:732
//line lit/web.w:733
//line lit/web.w:734
func parseMacro(seg string) (Format, bool) {
//line lit/web.w:735
	fields := strings.Fields(seg)
//line lit/web.w:736
	if len(fields) == 0 {
//line lit/web.w:737
		return Format{}, false
//line lit/web.w:738
	}
//line lit/web.w:739
	name := fields[0]
//line lit/web.w:740
	if k := strings.LastIndex(name, "."); k >= 0 {
//line lit/web.w:741
		name = name[k+1:]
//line lit/web.w:742
	}
//line lit/web.w:743
	if name == "" {
//line lit/web.w:744
		return Format{}, false
//line lit/web.w:745
	}
//line lit/web.w:746
	return Format{Original: name, Macro: true}, true
//line lit/web.w:747
}

// extractLimboFormats pulls @d/@f/@s directives out of the limbo text
// (consuming each to end of line) and returns the cleaned text together with the
// formats. Other control codes and argument-terminated groups are copied through.
//
//line lit/web.w:753
//line lit/web.w:754
//line lit/web.w:755
//line lit/web.w:756
func extractLimboFormats(src string) (string, []Format) {
//line lit/web.w:757
	var b strings.Builder
//line lit/web.w:758
	var formats []Format
//line lit/web.w:759
	n := len(src)
//line lit/web.w:760
	i := 0
//line lit/web.w:761
	for i < n {
//line lit/web.w:762
		if src[i] != '@' || i+1 >= n {
//line lit/web.w:763
			b.WriteByte(src[i])
//line lit/web.w:764
			i++
//line lit/web.w:765
			continue
//line lit/web.w:766
		}
//line lit/web.w:767
		switch c := src[i+1]; c {
//line lit/web.w:768
		case '@':
//line lit/web.w:769
			b.WriteString("@@")
//line lit/web.w:770
			i += 2
//line lit/web.w:771
		case 'd', 'f', 's':
//line lit/web.w:772
			j := i + 2
//line lit/web.w:773
			for j < n && src[j] != '\n' {
//line lit/web.w:774
				j++
//line lit/web.w:775
			}
//line lit/web.w:776
			var f Format
//line lit/web.w:777
			var ok bool
//line lit/web.w:778
			if c == 'd' {
//line lit/web.w:779
				f, ok = parseMacro(src[i+2 : j])
//line lit/web.w:780
			} else {
//line lit/web.w:781
				f, ok = parseFormat(src[i+2:j], c == 's')
//line lit/web.w:782
			}
//line lit/web.w:783
			if ok {
//line lit/web.w:784
				formats = append(formats, f)
//line lit/web.w:785
			}
//line lit/web.w:786
			if j < n {
//line lit/web.w:787
				j++ // also drop the newline that ended the directive
//line lit/web.w:788
			}
//line lit/web.w:789
			i = j
//line lit/web.w:790
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line lit/web.w:791
			end := indexFrom(src, "@>", i+2)
//line lit/web.w:792
			if end < 0 {
//line lit/web.w:793
				b.WriteString(src[i:])
//line lit/web.w:794
				i = n
//line lit/web.w:795
			} else {
//line lit/web.w:796
				b.WriteString(src[i : end+2])
//line lit/web.w:797
				i = end + 2
//line lit/web.w:798
			}
//line lit/web.w:799
		default:
//line lit/web.w:800
			b.WriteString(src[i : i+2])
//line lit/web.w:801
			i += 2
//line lit/web.w:802
		}
//line lit/web.w:803
	}
//line lit/web.w:804
	return b.String(), formats
//line lit/web.w:805
}
