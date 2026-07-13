// Package env provides a minimal .env loader for local development, shared by the
// service entrypoints so the behaviour lives in one place.
package env

import (
	"bufio"
	"os"
	"strings"
)

// Load reads KEY=VALUE pairs from path into the process environment, without
// overriding variables that are already set. A missing file is not an error.
func Load(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
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
		if key = strings.TrimSpace(key); key != "" {
			if _, exists := os.LookupEnv(key); !exists {
				os.Setenv(key, strings.TrimSpace(value))
			}
		}
	}
}
