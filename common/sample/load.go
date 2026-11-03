package sample

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed all:_go all:python all:ts all:lua all:rust
var files embed.FS

type variants struct {
	sync  string
	async string
}

var paths = map[string]variants{
	"go": {
		sync:  "_go/sample_sync.go",
		async: "_go/sample_async.go",
	},
	"python": {
		sync:  "python/sample_sync.py",
		async: "python/sample_async.py",
	},
	"ts": {
		sync:  "ts/sample_sync.ts",
		async: "ts/sample_async.ts",
	},
	"lua": {
		sync:  "lua/sample_sync.lua",
		async: "lua/sample_async.lua",
	},
	"rust": {
		sync:  "rust/sample_sync.rs",
		async: "rust/sample_async.rs",
	},
}

func LoadSync(language string) ([]byte, error) {
	return load(language, false)
}

func LoadAsync(language string) ([]byte, error) {
	return load(language, true)
}

func load(language string, isAsync bool) ([]byte, error) {
	lang := normalize(language)

	variant, ok := paths[lang]
	if !ok {
		return nil, fmt.Errorf("no default sample for language %q", language)
	}

	path := variant.sync
	if isAsync {
		path = variant.async
	}

	body, err := files.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load default sample for %q: %w", language, err)
	}
	return body, nil
}

func normalize(language string) string {
	lang := strings.ToLower(strings.TrimSpace(language))
	switch lang {
	case "golang":
		return "go"
	case "py":
		return "python"
	case "rs":
		return "rust"
	case "typescript":
		return "ts"
	default:
		return lang
	}
}
