package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var checklistPattern = regexp.MustCompile(`^- \[([ xX])\]\s?(.*)$`)

type Item struct {
	checked bool
	text    string
}

// fileContext stores the original file structure so saves can preserve
// non-checklist content (headings, prose, blank lines, etc.).
type fileContext struct {
	// lines holds every line from the original file.
	// Checklist lines are stored as nil (their content comes from items).
	lines []*string
}

func loadItems(path string) ([]Item, *fileContext, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	ctx := &fileContext{}
	items := make([]Item, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		match := checklistPattern.FindStringSubmatch(line)
		if match == nil {
			l := line
			ctx.lines = append(ctx.lines, &l)
			continue
		}

		items = append(items, Item{
			checked: strings.EqualFold(match[1], "x"),
			text:    match[2],
		})
		ctx.lines = append(ctx.lines, nil) // placeholder for this checklist item
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return items, ctx, nil
}

func saveItems(path string, items []Item, ctx *fileContext) error {
	formatItem := func(item Item) string {
		mark := " "
		if item.checked {
			mark = "x"
		}
		return fmt.Sprintf("- [%s] %s", mark, item.text)
	}

	// Build output lines from the original structure.
	// Fill original checklist slots positionally with current items.
	out := make([]string, 0, len(ctx.lines)+len(items))
	itemIdx := 0
	for _, linePtr := range ctx.lines {
		if linePtr != nil {
			out = append(out, *linePtr)
			continue
		}
		// This is an original checklist slot — fill with next item if available.
		if itemIdx < len(items) {
			out = append(out, formatItem(items[itemIdx]))
			itemIdx++
		}
		// Otherwise the item was deleted; skip the slot.
	}

	// Append any new items beyond the original count.
	for ; itemIdx < len(items); itemIdx++ {
		out = append(out, formatItem(items[itemIdx]))
	}

	content := strings.Join(out, "\n")
	if len(out) > 0 {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
