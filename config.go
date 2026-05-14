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
	FileName    string
	FilePath    string
	FileCmdSave string
	FileCmdLoad string
	HistoryFile string
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
		case "$FILE":
			if err := validateFileName(value); err != nil {
				return cfg, fmt.Errorf("$FILE %q: %w", value, err)
			}
			cfg.FileName = value
		case "file-path":
			cfg.FilePath = expandHome(value)
		case "file-cmd-save":
			cfg.FileCmdSave = value
		case "file-cmd-load":
			cfg.FileCmdLoad = value
		case "history-file":
			cfg.HistoryFile = expandHome(value)
		}
	}

	if err := scanner.Err(); err != nil {
		return cfg, err
	}

	if (cfg.FileCmdSave == "") != (cfg.FileCmdLoad == "") {
		return cfg, fmt.Errorf("file-cmd-save and file-cmd-load must both be set if either is used")
	}

	return cfg, nil
}

func validateFileName(name string) error {
	if name == "" {
		return fmt.Errorf("empty name")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("must not contain '..'")
	}
	const bad = "/\\;&|`$()<>\"'*?\n\r\t"
	if strings.ContainsAny(name, bad) {
		return fmt.Errorf("contains forbidden character")
	}
	return nil
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

func resolveFileName(cfg config) string {
	if cfg.FileName != "" {
		return cfg.FileName + ".md"
	}
	return "TODO_tui.md"
}

func resolveFilePath(cfg config) string {
	name := resolveFileName(cfg)
	if cfg.FilePath != "" {
		return filepath.Join(cfg.FilePath, name)
	}
	if cfg.FileCmdSave != "" || cfg.FileCmdLoad != "" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".cache", "todo-tui", name)
		}
	}
	return name
}

func runFileCmd(cmdTemplate, dir, fileName string) error {
	cmd := exec.Command("sh", "-c", cmdTemplate)
	cmd.Env = append(os.Environ(),
		"TODO_PATH="+dir,
		"TODO_FILE="+fileName,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %q failed: %w", cmdTemplate, err)
	}
	return nil
}

func configFileName(cfg config) string {
	if cfg.FileName != "" {
		return cfg.FileName
	}
	return "TODO_tui"
}

func cachePath(cfg config) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "todo-tui", configFileName(cfg)+".base.md")
}

func ensureCacheDir(cfg config) error {
	p := cachePath(cfg)
	if p == "" {
		return fmt.Errorf("could not resolve cache dir")
	}
	return os.MkdirAll(filepath.Dir(p), 0o755)
}
