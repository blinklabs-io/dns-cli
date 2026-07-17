package demo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnvFile loads KEY=VALUE pairs from path into the process environment
// without overriding keys that are already set.
func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if key == "" {
			continue
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}

// SaveEnvVar sets the process env and rewrites path, replacing any existing KEY= line.
func SaveEnvVar(path, key, value string) error {
	if err := os.Setenv(key, value); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var lines []string
	if raw, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			trim := strings.TrimSpace(line)
			if k, _, ok := strings.Cut(trim, "="); ok && strings.TrimSpace(k) == key {
				continue
			}
			lines = append(lines, line)
		}
		for len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}
	lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	body := strings.Join(lines, "\n") + "\n"
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tmp, path)
}
