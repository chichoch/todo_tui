package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type config struct {
	FilePath    string
	FileCmdSave string
	FileCmdLoad string
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "todo-tui", "todo-tui.conf")
}

func loadConfig() (config, error) {
	return loadConfigFrom(configPath())
}

func loadConfigFrom(path string) (config, error) {
	var cfg config
	if path == "" {
		return cfg, nil
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch key {
		case "file-path":
			cfg.FilePath = expandHome(value)
		case "file-cmd-save":
			cfg.FileCmdSave = value
		case "file-cmd-load":
			cfg.FileCmdLoad = value
		}
	}

	return cfg, scanner.Err()
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func resolveFilePath(cfg config) string {
	if cfg.FilePath != "" {
		return cfg.FilePath
	}
	return "TODO_tui.md"
}

func runFileCmd(cmdTemplate, filePath string) error {
	expanded := strings.ReplaceAll(cmdTemplate, "$FILE", filePath)
	cmd := exec.Command("sh", "-c", expanded)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %q failed: %w", expanded, err)
	}
	return nil
}
