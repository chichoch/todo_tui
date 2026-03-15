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

		items = append(items, Item{
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
		lines = append(lines, fmt.Sprintf("- [%s] %s", mark, item.text))
	}

	content := "# TODO\n" + strings.Join(lines, "\n")
	return os.WriteFile(path, []byte(content), 0o644)
}
