//line lit/weave.w:1392
package weave

//line lit/weave.w:1394
import (
//line lit/weave.w:1395
	"bufio"
//line lit/weave.w:1396
	"fmt"
//line lit/weave.w:1397
	"sort"
//line lit/weave.w:1398
	"strings"

//line lit/weave.w:1400
	"github.com/sjnam/gweb/internal/web"
//line lit/weave.w:1401
)

// xref accumulates cross-reference information while a web is woven:
//   - where each identifier is used and (heuristically) defined;
//   - where each named section is defined and used;
//   - manual index entries from @^ @. @: control codes.
//
// It is populated during a first (discarded) weaving pass and then consulted
// during the real pass and when emitting the back matter.
//
//line lit/weave.w:1405
//line lit/weave.w:1406
//line lit/weave.w:1407
//line lit/weave.w:1408
//line lit/weave.w:1409
//line lit/weave.w:1410
//line lit/weave.w:1411
//line lit/weave.w:1412
type xref struct {
//line lit/weave.w:1413
	identUse map[string]map[int]bool
//line lit/weave.w:1414
	identDef map[string]map[int]bool
//line lit/weave.w:1415
	sectionDefs map[string]map[int]bool
//line lit/weave.w:1416
	sectionUses map[string]map[int]bool
//line lit/weave.w:1417
	manualIndex []manualEntry
//line lit/weave.w:1418
}

//line lit/weave.w:1420
type manualEntry struct {
//line lit/weave.w:1421
	kind byte // '^', '.', ':'
//line lit/weave.w:1422
	text string
//line lit/weave.w:1423
	sec int
//line lit/weave.w:1424
}

//line lit/weave.w:1428
func newXref() *xref {
//line lit/weave.w:1429
	return &xref{
//line lit/weave.w:1430
		identUse: map[string]map[int]bool{},
//line lit/weave.w:1431
		identDef: map[string]map[int]bool{},
//line lit/weave.w:1432
		sectionDefs: map[string]map[int]bool{},
//line lit/weave.w:1433
		sectionUses: map[string]map[int]bool{},
//line lit/weave.w:1434
	}
//line lit/weave.w:1435
}

//line lit/weave.w:1437
func addTo(m map[string]map[int]bool, key string, sec int) {
//line lit/weave.w:1438
	if m[key] == nil {
//line lit/weave.w:1439
		m[key] = map[int]bool{}
//line lit/weave.w:1440
	}
//line lit/weave.w:1441
	m[key][sec] = true
//line lit/weave.w:1442
}

//line lit/weave.w:1444
func (x *xref) addIdentUse(name string, sec int) { addTo(x.identUse, name, sec) }

//line lit/weave.w:1445
func (x *xref) addIdentDef(name string, sec int) { addTo(x.identDef, name, sec) }

//line lit/weave.w:1446
func (x *xref) addSectionDef(name string, sec int) { addTo(x.sectionDefs, name, sec) }

//line lit/weave.w:1447
func (x *xref) addSectionUse(name string, sec int) { addTo(x.sectionUses, name, sec) }

//line lit/weave.w:1448
func (x *xref) addManualIndex(kind byte, text string, sec int) {
//line lit/weave.w:1449
	x.manualIndex = append(x.manualIndex, manualEntry{kind, text, sec})
//line lit/weave.w:1450
}

// sortedKeys returns the keys of a section set in ascending order.
//
//line lit/weave.w:1455
//line lit/weave.w:1456
func sortedKeys(m map[int]bool) []int {
//line lit/weave.w:1457
	ks := make([]int, 0, len(m))
//line lit/weave.w:1458
	for k := range m {
//line lit/weave.w:1459
		ks = append(ks, k)
//line lit/weave.w:1460
	}
//line lit/weave.w:1461
	sort.Ints(ks)
//line lit/weave.w:1462
	return ks
//line lit/weave.w:1463
}

// secList renders a set of section numbers as hyperlinks, with the defining
// sections (those in def) additionally underlined.
//
//line lit/weave.w:1465
//line lit/weave.w:1466
//line lit/weave.w:1467
func secList(secs, def map[int]bool) string {
//line lit/weave.w:1468
	nums := sortedKeys(secs)
//line lit/weave.w:1469
	parts := make([]string, len(nums))
//line lit/weave.w:1470
	for i, n := range nums {
//line lit/weave.w:1471
		if def != nil && def[n] {
//line lit/weave.w:1472
			parts[i] = fmt.Sprintf("\\GsD{%d}", n)
//line lit/weave.w:1473
		} else {
//line lit/weave.w:1474
			parts[i] = fmt.Sprintf("\\Gs{%d}", n)
//line lit/weave.w:1475
		}
//line lit/weave.w:1476
	}
//line lit/weave.w:1477
	return strings.Join(parts, ", ")
//line lit/weave.w:1478
}

// writeBackMatter emits the index, the list of named sections, and the table of
// contents that close a woven document.
//
//line lit/weave.w:1483
//line lit/weave.w:1484
//line lit/weave.w:1485
func (wv *Weaver) writeBackMatter(bw *bufio.Writer) {
//line lit/weave.w:1486
	wv.writeBookmarks(bw)
//line lit/weave.w:1487
	bw.WriteString("\n\\Ginx\n")
//line lit/weave.w:1488
	wv.writeIndex(bw)
//line lit/weave.w:1489
	bw.WriteString("\\Gfin\n")
//line lit/weave.w:1490
	// A destination at the top of the section-names page, targeted by the "Names
//line lit/weave.w:1491
	// of the sections" bookmark. Its number is one past the last section, so it
//line lit/weave.w:1492
	// never collides with a section's own destination.
//line lit/weave.w:1493
	fmt.Fprintf(bw, "\\Gdest{%d}%%\n", len(wv.w.Sections)+1)
//line lit/weave.w:1494
	wv.writeSectionNames(bw)
//line lit/weave.w:1495
	bw.WriteString("\\Gcon\n\\end\n")
//line lit/weave.w:1496
}

// writeBookmarks emits one \Gbookmark per starred section, in document (pre)
// order, so a PDF outline can be built whose nesting follows the @*, @*1,
// @*2 ... depths. Each entry carries its depth (the dvipdfmx route nests by
// level) and its number of direct children (pdftex's count model). A final
// top-level "Names of the sections" entry lists every section name as a
// collapsible child linking to its defining section.
//
//line lit/weave.w:1505
//line lit/weave.w:1506
//line lit/weave.w:1507
//line lit/weave.w:1508
//line lit/weave.w:1509
//line lit/weave.w:1510
//line lit/weave.w:1511
func (wv *Weaver) writeBookmarks(bw *bufio.Writer) {
//line lit/weave.w:1512
	var starred []*web.Section
//line lit/weave.w:1513
	for _, s := range wv.w.Sections {
//line lit/weave.w:1514
		if s.Starred {
//line lit/weave.w:1515
			starred = append(starred, s)
//line lit/weave.w:1516
		}
//line lit/weave.w:1517
	}
//line lit/weave.w:1518
	bw.WriteString("\n\\par")
//line lit/weave.w:1519
	topDepth := 0
//line lit/weave.w:1520
	for i, s := range starred {
//line lit/weave.w:1521
		children := 0
//line lit/weave.w:1522
		for j := i + 1; j < len(starred) && starred[j].Depth > s.Depth; j++ {
//line lit/weave.w:1523
			if starred[j].Depth == s.Depth+1 {
//line lit/weave.w:1524
				children++
//line lit/weave.w:1525
			}
//line lit/weave.w:1526
		}
//line lit/weave.w:1527
		if s.Depth < topDepth {
//line lit/weave.w:1528
			topDepth = s.Depth
//line lit/weave.w:1529
		}
//line lit/weave.w:1530
		fmt.Fprintf(bw, "\\Gbookmark{%d}{%d}{%d}{%s}%%\n", s.Depth, s.Number, children, bookmarkTitle(s.Title))
//line lit/weave.w:1531
	}
//line lit/weave.w:1532
	// A top-level "Names of the sections" entry (linking to the destination on
//line lit/weave.w:1533
	// that page, one past the last section) with every section name listed
//line lit/weave.w:1534
	// beneath it, each linking to its defining section, as cweave does. The
//line lit/weave.w:1535
	// negative child count starts the group collapsed; the reader can expand it.
//line lit/weave.w:1536
	// \Goutsecname holds the title, which the Korean backend localizes.
//line lit/weave.w:1537
	var names []string
//line lit/weave.w:1538
	for _, n := range wv.sortedSectionNames() {
//line lit/weave.w:1539
		if wv.defNum[n] > 0 {
//line lit/weave.w:1540
			names = append(names, n)
//line lit/weave.w:1541
		}
//line lit/weave.w:1542
	}
//line lit/weave.w:1543
	fmt.Fprintf(bw, "\\Gbookmark{%d}{%d}{%d}{\\Goutsecname}%%\n", topDepth, len(wv.w.Sections)+1, -len(names))
//line lit/weave.w:1544
	for _, n := range names {
//line lit/weave.w:1545
		fmt.Fprintf(bw, "\\Gbookmark{%d}{%d}{0}{%s}%%\n", topDepth+1, wv.defNum[n], bookmarkTitle(n))
//line lit/weave.w:1546
	}
//line lit/weave.w:1547
}

// bookmarkTitle reduces a starred-section title to plain text safe for a PDF
// outline: |code| spans keep their inner text, @@ becomes @, and TeX-special
// characters (which are rare in titles) are dropped.
//
//line lit/weave.w:1553
//line lit/weave.w:1554
//line lit/weave.w:1555
//line lit/weave.w:1556
func bookmarkTitle(raw string) string {
//line lit/weave.w:1557
	var b strings.Builder
//line lit/weave.w:1558
	n := len(raw)
//line lit/weave.w:1559
	for i := 0; i < n; i++ {
//line lit/weave.w:1560
		c := raw[i]
//line lit/weave.w:1561
		switch {
//line lit/weave.w:1562
		case c == '\\' && i+1 < n && raw[i+1] == '|':
//line lit/weave.w:1563
			b.WriteByte('|')
//line lit/weave.w:1564
			i++
//line lit/weave.w:1565
		case c == '@' && i+1 < n && raw[i+1] == '@':
//line lit/weave.w:1566
			b.WriteByte('@')
//line lit/weave.w:1567
			i++
//line lit/weave.w:1568
		case c == '|':
//line lit/weave.w:1569
			// drop the bar; keep the inline code's text
//line lit/weave.w:1570
		case c == '\\':
//line lit/weave.w:1571
			// drop a TeX control sequence (backslash plus a run of letters, or
//line lit/weave.w:1572
			// backslash plus one symbol), so e.g. \.{web} reduces to "web".
//line lit/weave.w:1573
			if i+1 < n {
//line lit/weave.w:1574
				if d := raw[i+1]; (d >= 'a' && d <= 'z') || (d >= 'A' && d <= 'Z') {
//line lit/weave.w:1575
					i++
//line lit/weave.w:1576
					for i+1 < n {
//line lit/weave.w:1577
						if e := raw[i+1]; (e >= 'a' && e <= 'z') || (e >= 'A' && e <= 'Z') {
//line lit/weave.w:1578
							i++
//line lit/weave.w:1579
						} else {
//line lit/weave.w:1580
							break
//line lit/weave.w:1581
						}
//line lit/weave.w:1582
					}
//line lit/weave.w:1583
				} else {
//line lit/weave.w:1584
					i++
//line lit/weave.w:1585
				}
//line lit/weave.w:1586
			}
//line lit/weave.w:1587
		case c == '{' || c == '}' || c == '$' || c == '&' ||
//line lit/weave.w:1588
			c == '#' || c == '%' || c == '^' || c == '_' || c == '~':
//line lit/weave.w:1589
			// TeX-special: drop
//line lit/weave.w:1590
		default:
//line lit/weave.w:1591
			b.WriteByte(c)
//line lit/weave.w:1592
		}
//line lit/weave.w:1593
	}
//line lit/weave.w:1594
	return strings.TrimSpace(b.String())
//line lit/weave.w:1595
}

// indexItem is one alphabetized entry of the identifier/manual index.
//
//line lit/weave.w:1601
//line lit/weave.w:1602
type indexItem struct {
//line lit/weave.w:1603
	sortKey string
//line lit/weave.w:1604
	render string // typeset form of the entry head (\GID{...}, \GIR{...}, ...)
//line lit/weave.w:1605
	secs map[int]bool
//line lit/weave.w:1606
	defs map[int]bool
//line lit/weave.w:1607
}

//line lit/weave.w:1609
func (wv *Weaver) writeIndex(bw *bufio.Writer) {
//line lit/weave.w:1610
	items := map[string]*indexItem{}
//line lit/weave.w:1611
	get := func(render, sortKey string) *indexItem {
//line lit/weave.w:1612
		it := items[render]
//line lit/weave.w:1613
		if it == nil {
//line lit/weave.w:1614
			it = &indexItem{sortKey: sortKey, render: render,
//line lit/weave.w:1615
				secs: map[int]bool{}, defs: map[int]bool{}}
//line lit/weave.w:1616
			items[render] = it
//line lit/weave.w:1617
		}
//line lit/weave.w:1618
		return it
//line lit/weave.w:1619
	}

//line lit/weave.w:1621
	// An identifier's index head follows its display class: a typewriter name
//line lit/weave.w:1622
	// ( or a predeclared constant) is set in typewriter, everything else italic.
//line lit/weave.w:1623
	head := func(name string) string {
//line lit/weave.w:1624
		if wv.format[name] == tkMacro {
//line lit/weave.w:1625
			return "\\GMAC{" + escTT(name) + "}"
//line lit/weave.w:1626
		}
//line lit/weave.w:1627
		return "\\GID{" + escIdent(name) + "}"
//line lit/weave.w:1628
	}
//line lit/weave.w:1629
	for name, secs := range wv.xref.identUse {
//line lit/weave.w:1630
		it := get(head(name), strings.ToLower(name))
//line lit/weave.w:1631
		for s := range secs {
//line lit/weave.w:1632
			it.secs[s] = true
//line lit/weave.w:1633
		}
//line lit/weave.w:1634
	}
//line lit/weave.w:1635
	for name, secs := range wv.xref.identDef {
//line lit/weave.w:1636
		it := get(head(name), strings.ToLower(name))
//line lit/weave.w:1637
		for s := range secs {
//line lit/weave.w:1638
			it.secs[s] = true
//line lit/weave.w:1639
			it.defs[s] = true
//line lit/weave.w:1640
		}
//line lit/weave.w:1641
	}
//line lit/weave.w:1642
	for _, e := range wv.xref.manualIndex {
//line lit/weave.w:1643
		var render string
//line lit/weave.w:1644
		switch e.kind {
//line lit/weave.w:1645
		case '.':
//line lit/weave.w:1646
			render = "\\GIT{" + escTT(e.text) + "}"
//line lit/weave.w:1647
		case ':':
//line lit/weave.w:1648
			render = "\\GIC{" + e.text + "}"
//line lit/weave.w:1649
		default: // '^'
//line lit/weave.w:1650
			render = "\\GIR{" + escProse(e.text) + "}"
//line lit/weave.w:1651
		}
//line lit/weave.w:1652
		it := get(render, strings.ToLower(e.text))
//line lit/weave.w:1653
		it.secs[e.sec] = true
//line lit/weave.w:1654
	}

//line lit/weave.w:1656
	list := make([]*indexItem, 0, len(items))
//line lit/weave.w:1657
	for _, it := range items {
//line lit/weave.w:1658
		list = append(list, it)
//line lit/weave.w:1659
	}
//line lit/weave.w:1660
	sort.Slice(list, func(i, j int) bool {
//line lit/weave.w:1661
		if list[i].sortKey != list[j].sortKey {
//line lit/weave.w:1662
			return list[i].sortKey < list[j].sortKey
//line lit/weave.w:1663
		}
//line lit/weave.w:1664
		return list[i].render < list[j].render
//line lit/weave.w:1665
	})
//line lit/weave.w:1666
	for _, it := range list {
//line lit/weave.w:1667
		fmt.Fprintf(bw, "\\GII{%s}{%s}\n", it.render, secList(it.secs, it.defs))
//line lit/weave.w:1668
	}
//line lit/weave.w:1669
}

// writeSectionNames emits the list of named sections with their defining and
// using section numbers.
//
//line lit/weave.w:1675
//line lit/weave.w:1676
//line lit/weave.w:1677
func (wv *Weaver) writeSectionNames(bw *bufio.Writer) {
//line lit/weave.w:1678
	for _, n := range wv.sortedSectionNames() {
//line lit/weave.w:1679
		fmt.Fprintf(bw, "\\GNS{%s}{%d}{%s}\n",
//line lit/weave.w:1680
			wv.renderName(n), wv.defNum[n], usedNote(wv.xref.sectionUses[n]))
//line lit/weave.w:1681
	}
//line lit/weave.w:1682
}

// sortedSectionNames returns every section name (defined or used), ordered
// case-insensitively, as it appears on the section-names page and in the PDF
// outline beneath "Names of the sections".
//
//line lit/weave.w:1684
//line lit/weave.w:1685
//line lit/weave.w:1686
//line lit/weave.w:1687
func (wv *Weaver) sortedSectionNames() []string {
//line lit/weave.w:1688
	names := map[string]bool{}
//line lit/weave.w:1689
	for n := range wv.xref.sectionDefs {
//line lit/weave.w:1690
		names[n] = true
//line lit/weave.w:1691
	}
//line lit/weave.w:1692
	for n := range wv.xref.sectionUses {
//line lit/weave.w:1693
		names[n] = true
//line lit/weave.w:1694
	}
//line lit/weave.w:1695
	sorted := make([]string, 0, len(names))
//line lit/weave.w:1696
	for n := range names {
//line lit/weave.w:1697
		sorted = append(sorted, n)
//line lit/weave.w:1698
	}
//line lit/weave.w:1699
	sort.Slice(sorted, func(i, j int) bool {
//line lit/weave.w:1700
		return strings.ToLower(sorted[i]) < strings.ToLower(sorted[j])
//line lit/weave.w:1701
	})
//line lit/weave.w:1702
	return sorted
//line lit/weave.w:1703
}

// usedNote renders the "Used in section(s) ..." note for the section-names list,
// or "" when the section is never used. It emits a \GNused/\GNuseds macro so the
// wording can be localized, like the \GU/\GUs notes under a definition.
//
//line lit/weave.w:1710
//line lit/weave.w:1711
//line lit/weave.w:1712
//line lit/weave.w:1713
func usedNote(uses map[int]bool) string {
//line lit/weave.w:1714
	if len(uses) == 0 {
//line lit/weave.w:1715
		return ""
//line lit/weave.w:1716
	}
//line lit/weave.w:1717
	macro := "\\GNused"
//line lit/weave.w:1718
	if len(uses) > 1 {
//line lit/weave.w:1719
		macro = "\\GNuseds"
//line lit/weave.w:1720
	}
//line lit/weave.w:1721
	return macro + "{" + secList(uses, nil) + "}"
//line lit/weave.w:1722
}

// crossRefNotes returns the "also defined in"/"used in" notes printed under the
// first definition of a named section, or "" if none apply.
//
//line lit/weave.w:1727
//line lit/weave.w:1728
//line lit/weave.w:1729
func (wv *Weaver) crossRefNotes(name string, secNum int) string {
//line lit/weave.w:1730
	if wv.defNum[name] != secNum {
//line lit/weave.w:1731
		return "" // notes appear only under the first definition
//line lit/weave.w:1732
	}
//line lit/weave.w:1733
	var b strings.Builder
//line lit/weave.w:1734
	defs := wv.xref.sectionDefs[name]
//line lit/weave.w:1735
	if len(defs) > 1 {
//line lit/weave.w:1736
		others := map[int]bool{}
//line lit/weave.w:1737
		for s := range defs {
//line lit/weave.w:1738
			if s != secNum {
//line lit/weave.w:1739
				others[s] = true
//line lit/weave.w:1740
			}
//line lit/weave.w:1741
		}
//line lit/weave.w:1742
		macro := "\\GA"
//line lit/weave.w:1743
		if len(others) > 1 {
//line lit/weave.w:1744
			macro = "\\GAs"
//line lit/weave.w:1745
		}
//line lit/weave.w:1746
		fmt.Fprintf(&b, "%s{%s}%%\n", macro, secList(others, nil))
//line lit/weave.w:1747
	}
//line lit/weave.w:1748
	if uses := wv.xref.sectionUses[name]; len(uses) > 0 {
//line lit/weave.w:1749
		macro := "\\GU"
//line lit/weave.w:1750
		if len(uses) > 1 {
//line lit/weave.w:1751
			macro = "\\GUs"
//line lit/weave.w:1752
		}
//line lit/weave.w:1753
		fmt.Fprintf(&b, "%s{%s}%%\n", macro, secList(uses, nil))
//line lit/weave.w:1754
	}
//line lit/weave.w:1755
	return b.String()
//line lit/weave.w:1756
}
