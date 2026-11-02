package sample

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed all:_go all:python all:ts all:lua all:rust
var files embed.FS

var paths = map[string]string{
	"go":     "_go/sample.go",
	"python": "python/sample.py",
	"ts":     "ts/sample.ts",
	"lua":    "lua/sample.lua",
	"rust":   "rust/sample.rs",
}

// Default returns the default function source for a language.
func Default(language string) ([]byte, error) {
	lang := strings.ToLower(strings.TrimSpace(language))
	switch lang {
	case "golang":
		lang = "go"
	case "py":
		lang = "python"
	case "rs":
		lang = "rust"
	case "typescript":
		lang = "ts"
	}

	path, ok := paths[lang]
	if !ok {
		return nil, fmt.Errorf("no default sample for language %q", language)
	}

	body, err := files.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load default sample for %q: %w", language, err)
	}
	return body, nil
}
