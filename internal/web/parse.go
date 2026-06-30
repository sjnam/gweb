//line internal/web/web.w:389
package web

//line internal/web/web.w:391
import (
//line internal/web/web.w:392
	"fmt"
//line internal/web/web.w:393
	"strings"
//line internal/web/web.w:394
)

// ctrlKind classifies a structural control code found while scanning.
//
//line internal/web/web.w:396
//line internal/web/web.w:397
type ctrlKind int

//line internal/web/web.w:399
const (
//line internal/web/web.w:400
	cEOF ctrlKind = iota
//line internal/web/web.w:401
	cSection
//line internal/web/web.w:402
	cCode // @c (or its synonym @p)
//line internal/web/web.w:403
	cNamed // @<name@>= or @(file@>=
//line internal/web/web.w:404
	cDefn // @d
//line internal/web/web.w:405
	cFormat

//line internal/web/web.w:406
)

//line internal/web/web.w:408
type ctrl struct {
//line internal/web/web.w:409
	kind ctrlKind
//line internal/web/web.w:410
	pos int // index of the leading '@'
//line internal/web/web.w:411
	end int // index just past the control token
//line internal/web/web.w:412
	depth int // for cSection: -1 unstarred (or @** top level), else starred depth
//line internal/web/web.w:413
	starred bool // for cSection (distinguishes @** from an unstarred section)
//line internal/web/web.w:414
	name string // for cNamed
//line internal/web/web.w:415
	isFile bool // for cNamed (@( vs @<)
//line internal/web/web.w:416
	noIndex bool // for cFormat (@s)
//line internal/web/web.w:417
}

// scanStruct finds the next structural control at or after i. It skips literal
// "@@" and argument-terminated codes (@<...@>, @=...@>, etc.) so their contents
// never trigger a false section break. A "@<...@>" not followed by "=" is a
// reference, not a definition, and is skipped.
//
//line internal/web/web.w:424
//line internal/web/web.w:425
//line internal/web/web.w:426
//line internal/web/web.w:427
//line internal/web/web.w:428
func scanStruct(src string, i int) ctrl {
//line internal/web/web.w:429
	n := len(src)
//line internal/web/web.w:430
	for i < n {
//line internal/web/web.w:431
		if src[i] != '@' {
//line internal/web/web.w:432
			i++
//line internal/web/web.w:433
			continue
//line internal/web/web.w:434
		}
//line internal/web/web.w:435
		if i+1 >= n {
//line internal/web/web.w:436
			break
//line internal/web/web.w:437
		}
//line internal/web/web.w:438
		switch c := src[i+1]; {
//line internal/web/web.w:439
		case c == '@':
//line internal/web/web.w:440
			i += 2
//line internal/web/web.w:441
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line internal/web/web.w:442
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line internal/web/web.w:443
		case c == '*':
//line internal/web/web.w:444
			j := i + 2
//line internal/web/web.w:445
			depth := 0
//line internal/web/web.w:446
			if j < n && src[j] == '*' {
//line internal/web/web.w:447
				j++
//line internal/web/web.w:448
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line internal/web/web.w:449
			} else {
//line internal/web/web.w:450
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line internal/web/web.w:451
					depth = depth*10 + int(src[j]-'0')
//line internal/web/web.w:452
					j++
//line internal/web/web.w:453
				}
//line internal/web/web.w:454
			}
//line internal/web/web.w:455
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line internal/web/web.w:456
		case c == 'c' || c == 'p':
//line internal/web/web.w:457
			return ctrl{kind: cCode, pos: i, end: i + 2}
//line internal/web/web.w:458
		case c == 'd':
//line internal/web/web.w:459
			return ctrl{kind: cDefn, pos: i, end: i + 2}
//line internal/web/web.w:460
		case c == 'f':
//line internal/web/web.w:461
			return ctrl{kind: cFormat, pos: i, end: i + 2}
//line internal/web/web.w:462
		case c == 's':
//line internal/web/web.w:463
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
//line internal/web/web.w:464
		case c == '<' || c == '(':
//line internal/web/web.w:465
			end := indexFrom(src, "@>", i+2)
//line internal/web/web.w:466
			if end < 0 {
//line internal/web/web.w:467
				return ctrl{kind: cEOF, pos: n, end: n}
//line internal/web/web.w:468
			}
//line internal/web/web.w:469
			after := end + 2
//line internal/web/web.w:470
			k := after
//line internal/web/web.w:471
			for k < n && (src[k] == ' ' || src[k] == '\t') {
//line internal/web/web.w:472
				k++
//line internal/web/web.w:473
			}
//line internal/web/web.w:474
			if k < n && src[k] == '=' {
//line internal/web/web.w:475
				return ctrl{kind: cNamed, pos: i, end: k + 1,
//line internal/web/web.w:476
					name: canonName(src[i+2 : end]), isFile: c == '('}
//line internal/web/web.w:477
			}
//line internal/web/web.w:478
			i = after // a reference, not a definition
//line internal/web/web.w:479
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line internal/web/web.w:480
			end := indexFrom(src, "@>", i+2)
//line internal/web/web.w:481
			if end < 0 {
//line internal/web/web.w:482
				return ctrl{kind: cEOF, pos: n, end: n}
//line internal/web/web.w:483
			}
//line internal/web/web.w:484
			i = end + 2
//line internal/web/web.w:485
		case c == '%':
//line internal/web/web.w:486
			j := i + 2
//line internal/web/web.w:487
			for j < n && src[j] != '\n' {
//line internal/web/web.w:488
				j++
//line internal/web/web.w:489
			}
//line internal/web/web.w:490
			i = j
//line internal/web/web.w:491
		default:
//line internal/web/web.w:492
			i += 2
//line internal/web/web.w:493
		}
//line internal/web/web.w:494
	}
//line internal/web/web.w:495
	return ctrl{kind: cEOF, pos: n, end: n}
//line internal/web/web.w:496
}

// findNextSection scans forward to the next section break (@ or @*), skipping
// everything else including argument-terminated codes. Used inside code parts,
// where @c/@d/@f never legitimately appear.
//
//line internal/web/web.w:502
//line internal/web/web.w:503
//line internal/web/web.w:504
//line internal/web/web.w:505
func findNextSection(src string, i int) ctrl {
//line internal/web/web.w:506
	n := len(src)
//line internal/web/web.w:507
	for i < n {
//line internal/web/web.w:508
		if src[i] != '@' {
//line internal/web/web.w:509
			i++
//line internal/web/web.w:510
			continue
//line internal/web/web.w:511
		}
//line internal/web/web.w:512
		if i+1 >= n {
//line internal/web/web.w:513
			break
//line internal/web/web.w:514
		}
//line internal/web/web.w:515
		switch c := src[i+1]; {
//line internal/web/web.w:516
		case c == '@':
//line internal/web/web.w:517
			i += 2
//line internal/web/web.w:518
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line internal/web/web.w:519
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line internal/web/web.w:520
		case c == '*':
//line internal/web/web.w:521
			j := i + 2
//line internal/web/web.w:522
			depth := 0
//line internal/web/web.w:523
			if j < n && src[j] == '*' {
//line internal/web/web.w:524
				j++
//line internal/web/web.w:525
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line internal/web/web.w:526
			} else {
//line internal/web/web.w:527
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line internal/web/web.w:528
					depth = depth*10 + int(src[j]-'0')
//line internal/web/web.w:529
					j++
//line internal/web/web.w:530
				}
//line internal/web/web.w:531
			}
//line internal/web/web.w:532
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line internal/web/web.w:533
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line internal/web/web.w:534
			end := indexFrom(src, "@>", i+2)
//line internal/web/web.w:535
			if end < 0 {
//line internal/web/web.w:536
				return ctrl{kind: cEOF, pos: n, end: n}
//line internal/web/web.w:537
			}
//line internal/web/web.w:538
			i = end + 2
//line internal/web/web.w:539
		case c == '%':
//line internal/web/web.w:540
			j := i + 2
//line internal/web/web.w:541
			for j < n && src[j] != '\n' {
//line internal/web/web.w:542
				j++
//line internal/web/web.w:543
			}
//line internal/web/web.w:544
			i = j
//line internal/web/web.w:545
		default:
//line internal/web/web.w:546
			i += 2
//line internal/web/web.w:547
		}
//line internal/web/web.w:548
	}
//line internal/web/web.w:549
	return ctrl{kind: cEOF, pos: n, end: n}
//line internal/web/web.w:550
}

// parse splits source into limbo and sections.
//
//line internal/web/web.w:555
//line internal/web/web.w:556
func parse(src string) *Web {
//line internal/web/web.w:557
	w := &Web{}
//line internal/web/web.w:558
	n := len(src)

//line internal/web/web.w:560
	// Limbo runs until the first section break. Format directives placed there
//line internal/web/web.w:561
	// (@f / @s, a common CWEB idiom) are extracted and removed from the copied
//line internal/web/web.w:562
	// TeX so they apply globally rather than printing literally.
//line internal/web/web.w:563
	first := findNextSection(src, 0)
//line internal/web/web.w:564
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
//line internal/web/web.w:565
	i := first.pos

//line internal/web/web.w:567
	num := 0
//line internal/web/web.w:568
	for i < n {
//line internal/web/web.w:569
		// We are positioned at a section break.
//line internal/web/web.w:570
		hdr := src[i+1]
//line internal/web/web.w:571
		num++
//line internal/web/web.w:572
		sec := &Section{Number: num, Line: lineAt(src, i)}
//line internal/web/web.w:573
		if hdr == '*' {
//line internal/web/web.w:574
			h := findSectionHeaderEnd(src, i)
//line internal/web/web.w:575
			sec.Starred = true
//line internal/web/web.w:576
			sec.Depth = h.depth
//line internal/web/web.w:577
			i = h.end
//line internal/web/web.w:578
		} else {
//line internal/web/web.w:579
			i += 2
//line internal/web/web.w:580
		}

//line internal/web/web.w:582
		// TeX part: from here to the next structural control.
//line internal/web/web.w:583
		ct := scanStruct(src, i)
//line internal/web/web.w:584
		sec.Tex = src[i:ct.pos]
//line internal/web/web.w:585
		if sec.Starred {
//line internal/web/web.w:586
			sec.Title = extractTitle(sec.Tex)
//line internal/web/web.w:587
		}

//line internal/web/web.w:589
		// Definition part: a run of @d / @f / @s.
//line internal/web/web.w:590
		for ct.kind == cDefn || ct.kind == cFormat {
//line internal/web/web.w:591
			nx := scanStruct(src, ct.end)
//line internal/web/web.w:592
			seg := src[ct.end:nx.pos]
//line internal/web/web.w:593
			// @d has no Go analogue (Go has no preprocessor), so it never tangles
//line internal/web/web.w:594
			// to code; gweave uses it only to set the named identifier in
//line internal/web/web.w:595
			// typewriter, as cweave sets a macro. @f/@s format like another word.
//line internal/web/web.w:596
			if ct.kind == cDefn {
//line internal/web/web.w:597
				if f, ok := parseMacro(seg); ok {
//line internal/web/web.w:598
					sec.Formats = append(sec.Formats, f)
//line internal/web/web.w:599
				}
//line internal/web/web.w:600
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
//line internal/web/web.w:601
				sec.Formats = append(sec.Formats, f)
//line internal/web/web.w:602
			}
//line internal/web/web.w:603
			ct = nx
//line internal/web/web.w:604
		}

//line internal/web/web.w:606
		switch ct.kind {
//line internal/web/web.w:607
		case cCode:
//line internal/web/web.w:608
			sec.HasCode = true
//line internal/web/web.w:609
			sec.CodeLine = lineAt(src, ct.end)
//line internal/web/web.w:610
			nx := findNextSection(src, ct.end)
//line internal/web/web.w:611
			sec.Code = src[ct.end:nx.pos]
//line internal/web/web.w:612
			i = nx.pos
//line internal/web/web.w:613
		case cNamed:
//line internal/web/web.w:614
			sec.HasCode = true
//line internal/web/web.w:615
			sec.Name = ct.name
//line internal/web/web.w:616
			sec.IsFile = ct.isFile
//line internal/web/web.w:617
			sec.CodeLine = lineAt(src, ct.end)
//line internal/web/web.w:618
			nx := findNextSection(src, ct.end)
//line internal/web/web.w:619
			sec.Code = src[ct.end:nx.pos]
//line internal/web/web.w:620
			i = nx.pos
//line internal/web/web.w:621
		default: // cSection or cEOF: a documentation-only section
//line internal/web/web.w:622
			i = ct.pos
//line internal/web/web.w:623
		}

//line internal/web/web.w:625
		w.Sections = append(w.Sections, sec)
//line internal/web/web.w:626
		if ct.kind == cEOF && sec.Code == "" {
//line internal/web/web.w:627
			break
//line internal/web/web.w:628
		}
//line internal/web/web.w:629
		if i >= n {
//line internal/web/web.w:630
			break
//line internal/web/web.w:631
		}
//line internal/web/web.w:632
	}
//line internal/web/web.w:633
	return w
//line internal/web/web.w:634
}

//line internal/web/web.w:638
func findSectionHeaderEnd(src string, i int) ctrl {
//line internal/web/web.w:639
	n := len(src)
//line internal/web/web.w:640
	j := i + 2
//line internal/web/web.w:641
	depth := 0
//line internal/web/web.w:642
	if j < n && src[j] == '*' {
//line internal/web/web.w:643
		j++
//line internal/web/web.w:644
		depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line internal/web/web.w:645
	} else {
//line internal/web/web.w:646
		for j < n && src[j] >= '0' && src[j] <= '9' {
//line internal/web/web.w:647
			depth = depth*10 + int(src[j]-'0')
//line internal/web/web.w:648
			j++
//line internal/web/web.w:649
		}
//line internal/web/web.w:650
	}
//line internal/web/web.w:651
	return ctrl{end: j, depth: depth}
//line internal/web/web.w:652
}

// extractTitle returns the text of a starred section up to its terminating
// period, with whitespace collapsed, for use in the table of contents. The
// terminator is the first period at end of text or followed by whitespace, so a
// period inside a control sequence such as \.{web} does not end the title early.
//
//line internal/web/web.w:657
//line internal/web/web.w:658
//line internal/web/web.w:659
//line internal/web/web.w:660
//line internal/web/web.w:661
func extractTitle(tex string) string {
//line internal/web/web.w:662
	t := strings.TrimLeft(tex, " \t\n")
//line internal/web/web.w:663
	if i := titleEnd(t); i >= 0 {
//line internal/web/web.w:664
		t = t[:i]
//line internal/web/web.w:665
	}
//line internal/web/web.w:666
	return strings.Join(strings.Fields(t), " ")
//line internal/web/web.w:667
}

// titleEnd returns the index of the period that ends a starred-section title --
// the first '.' at end of s or followed by whitespace -- or -1 if there is none.
//
//line internal/web/web.w:669
//line internal/web/web.w:670
//line internal/web/web.w:671
func titleEnd(s string) int {
//line internal/web/web.w:672
	for i := 0; i < len(s); i++ {
//line internal/web/web.w:673
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
//line internal/web/web.w:674
			s[i+1] == '\n' || s[i+1] == '\r') {
//line internal/web/web.w:675
			return i
//line internal/web/web.w:676
		}
//line internal/web/web.w:677
	}
//line internal/web/web.w:678
	return -1
//line internal/web/web.w:679
}

// scanDiagnostics walks the source looking for malformed control codes —
// currently argument-terminated codes (@<, @(, @=, @t, @^, @., @:, @q) that are
// missing their closing @> — and returns one warning per problem.
//
//line internal/web/web.w:684
//line internal/web/web.w:685
//line internal/web/web.w:686
//line internal/web/web.w:687
func (w *Web) scanDiagnostics(src string) []string {
//line internal/web/web.w:688
	var warns []string
//line internal/web/web.w:689
	n := len(src)
//line internal/web/web.w:690
	i := 0
//line internal/web/web.w:691
	for i < n {
//line internal/web/web.w:692
		if src[i] != '@' || i+1 >= n {
//line internal/web/web.w:693
			i++
//line internal/web/web.w:694
			continue
//line internal/web/web.w:695
		}
//line internal/web/web.w:696
		switch c := src[i+1]; c {
//line internal/web/web.w:697
		case '@':
//line internal/web/web.w:698
			i += 2
//line internal/web/web.w:699
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line internal/web/web.w:700
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line internal/web/web.w:701
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
//line internal/web/web.w:702
				i = n
//line internal/web/web.w:703
			} else {
//line internal/web/web.w:704
				i = end + 2
//line internal/web/web.w:705
			}
//line internal/web/web.w:706
		default:
//line internal/web/web.w:707
			i += 2
//line internal/web/web.w:708
		}
//line internal/web/web.w:709
	}
//line internal/web/web.w:710
	return warns
//line internal/web/web.w:711
}

// parseFormat parses the body of an @f/@s directive: two identifiers.
//
//line internal/web/web.w:715
//line internal/web/web.w:716
func parseFormat(seg string, noIndex bool) (Format, bool) {
//line internal/web/web.w:717
	fields := strings.Fields(seg)
//line internal/web/web.w:718
	if len(fields) < 2 {
//line internal/web/web.w:719
		return Format{}, false
//line internal/web/web.w:720
	}
//line internal/web/web.w:721
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
//line internal/web/web.w:722
}

// parseMacro parses an @d directive: its first word names a constant to set in
// typewriter; any value after it is ignored (Go has no preprocessor). A
// qualified name keeps its final component, so "@d http.StatusOK" and
// "@d StatusOK" both register StatusOK.
//
//line internal/web/web.w:730
//line internal/web/web.w:731
//line internal/web/web.w:732
//line internal/web/web.w:733
//line internal/web/web.w:734
func parseMacro(seg string) (Format, bool) {
//line internal/web/web.w:735
	fields := strings.Fields(seg)
//line internal/web/web.w:736
	if len(fields) == 0 {
//line internal/web/web.w:737
		return Format{}, false
//line internal/web/web.w:738
	}
//line internal/web/web.w:739
	name := fields[0]
//line internal/web/web.w:740
	if k := strings.LastIndex(name, "."); k >= 0 {
//line internal/web/web.w:741
		name = name[k+1:]
//line internal/web/web.w:742
	}
//line internal/web/web.w:743
	if name == "" {
//line internal/web/web.w:744
		return Format{}, false
//line internal/web/web.w:745
	}
//line internal/web/web.w:746
	return Format{Original: name, Macro: true}, true
//line internal/web/web.w:747
}

// extractLimboFormats pulls @d/@f/@s directives out of the limbo text
// (consuming each to end of line) and returns the cleaned text together with the
// formats. Other control codes and argument-terminated groups are copied through.
//
//line internal/web/web.w:753
//line internal/web/web.w:754
//line internal/web/web.w:755
//line internal/web/web.w:756
func extractLimboFormats(src string) (string, []Format) {
//line internal/web/web.w:757
	var b strings.Builder
//line internal/web/web.w:758
	var formats []Format
//line internal/web/web.w:759
	n := len(src)
//line internal/web/web.w:760
	i := 0
//line internal/web/web.w:761
	for i < n {
//line internal/web/web.w:762
		if src[i] != '@' || i+1 >= n {
//line internal/web/web.w:763
			b.WriteByte(src[i])
//line internal/web/web.w:764
			i++
//line internal/web/web.w:765
			continue
//line internal/web/web.w:766
		}
//line internal/web/web.w:767
		switch c := src[i+1]; c {
//line internal/web/web.w:768
		case '@':
//line internal/web/web.w:769
			b.WriteString("@@")
//line internal/web/web.w:770
			i += 2
//line internal/web/web.w:771
		case 'd', 'f', 's':
//line internal/web/web.w:772
			j := i + 2
//line internal/web/web.w:773
			for j < n && src[j] != '\n' {
//line internal/web/web.w:774
				j++
//line internal/web/web.w:775
			}
//line internal/web/web.w:776
			var f Format
//line internal/web/web.w:777
			var ok bool
//line internal/web/web.w:778
			if c == 'd' {
//line internal/web/web.w:779
				f, ok = parseMacro(src[i+2 : j])
//line internal/web/web.w:780
			} else {
//line internal/web/web.w:781
				f, ok = parseFormat(src[i+2:j], c == 's')
//line internal/web/web.w:782
			}
//line internal/web/web.w:783
			if ok {
//line internal/web/web.w:784
				formats = append(formats, f)
//line internal/web/web.w:785
			}
//line internal/web/web.w:786
			if j < n {
//line internal/web/web.w:787
				j++ // also drop the newline that ended the directive
//line internal/web/web.w:788
			}
//line internal/web/web.w:789
			i = j
//line internal/web/web.w:790
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line internal/web/web.w:791
			end := indexFrom(src, "@>", i+2)
//line internal/web/web.w:792
			if end < 0 {
//line internal/web/web.w:793
				b.WriteString(src[i:])
//line internal/web/web.w:794
				i = n
//line internal/web/web.w:795
			} else {
//line internal/web/web.w:796
				b.WriteString(src[i : end+2])
//line internal/web/web.w:797
				i = end + 2
//line internal/web/web.w:798
			}
//line internal/web/web.w:799
		default:
//line internal/web/web.w:800
			b.WriteString(src[i : i+2])
//line internal/web/web.w:801
			i += 2
//line internal/web/web.w:802
		}
//line internal/web/web.w:803
	}
//line internal/web/web.w:804
	return b.String(), formats
//line internal/web/web.w:805
}
