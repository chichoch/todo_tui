package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var checklistPattern = regexp.MustCompile(`^- \[([ xX])\]\s?(.*?)(?:\s*<!--id:([a-zA-Z0-9_-]+)-->)?\s*$`)

type Item struct {
	id      string
	checked bool
	text    string
}

func newItemID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%016x", os.Getpid())
	}
	return hex.EncodeToString(b[:])
}

func loadItems(path string) ([]Item, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	items := make([]Item, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		match := checklistPattern.FindStringSubmatch(line)
		if match == nil {
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
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func saveItems(path string, items []Item) error {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		mark := " "
		if item.checked {
			mark = "x"
		}
		id := item.id
		if id == "" {
			id = newItemID()
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s <!--id:%s-->", mark, item.text, id))
	}

	content := "# TODO\n" + strings.Join(lines, "\n")
	return os.WriteFile(path, []byte(content), 0o644)
}
