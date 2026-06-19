This change file turns wc into a CSV reporter: counts become comma-separated
instead of column-aligned. Apply it with

    gtangle wc.w wc.ch        (or: gweave wc.w wc.ch)

@x
		fmt.Printf("%8d %8d %8d\n", c.lines, c.words, c.chars)
@y
		fmt.Printf("%d,%d,%d\n", c.lines, c.words, c.chars)
@z
@x
		fmt.Printf("%8d %8d %8d %s\n", c.lines, c.words, c.chars, name)
@y
		fmt.Printf("%d,%d,%d,%s\n", c.lines, c.words, c.chars, name)
@z
