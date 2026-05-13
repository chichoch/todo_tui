package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var checklistPattern = regexp.MustCompile(`^- \[([ xX])\]\s?(.*?)(?:\s*<!--id:([a-zA-Z0-9_-]+)-->)?\s*$`)

type Item struct {
	id      string
	checked bool
	text    string
}

// fileContext records the original file structure so saveItems can preserve
// non-checklist content (headings, prose, blank lines) on round-trip. Each
// entry is either a literal line or nil (placeholder for the next checklist
// item in order).
type fileContext struct {
	lines []*string
}

func newItemID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%016x", os.Getpid())
	}
	return hex.EncodeToString(b[:])
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

		id := match[3]
		if id == "" {
			id = newItemID()
		}

		items = append(items, Item{
			id:      id,
			checked: strings.EqualFold(match[1], "x"),
			text:    match[2],
		})
		ctx.lines = append(ctx.lines, nil)
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return items, ctx, nil
}

func formatItem(item Item) string {
	mark := " "
	if item.checked {
		mark = "x"
	}
	return fmt.Sprintf("- [%s] %s <!--id:%s-->", mark, item.text, item.id)
}

// saveItems writes items as a Markdown checklist. When ctx is non-nil and has
// recorded lines, original non-checklist lines are preserved positionally;
// checklist slots are filled with current items in order, surplus items are
// appended, deleted slots are dropped. When ctx is nil or empty, the output is
// a fresh "# TODO" header followed by the checklist.
//
// Items without an id are assigned a fresh id in-place so caller's slice has
// stable ids across saves.
func saveItems(path string, items []Item, ctx *fileContext) error {
	for i := range items {
		if items[i].id == "" {
			items[i].id = newItemID()
		}
	}

	var out []string
	if ctx != nil && len(ctx.lines) > 0 {
		out = make([]string, 0, len(ctx.lines)+len(items))
		itemIdx := 0
		for _, linePtr := range ctx.lines {
			if linePtr != nil {
				out = append(out, *linePtr)
				continue
			}
			if itemIdx < len(items) {
				out = append(out, formatItem(items[itemIdx]))
				itemIdx++
			}
		}
		for ; itemIdx < len(items); itemIdx++ {
			out = append(out, formatItem(items[itemIdx]))
		}
	} else {
		out = make([]string, 0, len(items)+1)
		out = append(out, "# TODO")
		for _, item := range items {
			out = append(out, formatItem(item))
		}
	}

	content := strings.Join(out, "\n")
	if len(out) > 0 {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func appendHistory(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04"), text)
	_, err = f.WriteString(line)
	return err
}
