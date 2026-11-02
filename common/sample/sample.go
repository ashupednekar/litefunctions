package sample

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed all:_go all:python all:ts all:lua all:rust
var files embed.FS

// Variants holds the default sync and async sources for a language.
type Variants struct {
	Sync  string `json:"sync"`
	Async string `json:"async"`
}

var paths = map[string]Variants{
	"go": {
		Sync:  "_go/sample_sync.go",
		Async: "_go/sample_async.go",
	},
	"python": {
		Sync:  "python/sample_sync.py",
		Async: "python/sample_async.py",
	},
	"ts": {
		Sync:  "ts/sample_sync.ts",
		Async: "ts/sample_async.ts",
	},
	"lua": {
		Sync:  "lua/sample_sync.lua",
		Async: "lua/sample_async.lua",
	},
	"rust": {
		Sync:  "rust/sample_sync.rs",
		Async: "rust/sample_async.rs",
	},
}

// Default returns the default function source for a language and execution mode.
func Default(language string, isAsync bool) ([]byte, error) {
	lang := normalize(language)

	variant, ok := paths[lang]
	if !ok {
		return nil, fmt.Errorf("no default sample for language %q", language)
	}

	path := variant.Sync
	if isAsync {
		path = variant.Async
	}

	body, err := files.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load default sample for %q: %w", language, err)
	}
	return body, nil
}

// All returns the default sync and async sources for every supported language.
func All() (map[string]Variants, error) {
	out := make(map[string]Variants, len(paths))
	for lang, variant := range paths {
		syncBody, err := files.ReadFile(variant.Sync)
		if err != nil {
			return nil, fmt.Errorf("failed to load default sync sample for %q: %w", lang, err)
		}
		asyncBody, err := files.ReadFile(variant.Async)
		if err != nil {
			return nil, fmt.Errorf("failed to load default async sample for %q: %w", lang, err)
		}
		out[lang] = Variants{
			Sync:  string(syncBody),
			Async: string(asyncBody),
		}
	}
	return out, nil
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
