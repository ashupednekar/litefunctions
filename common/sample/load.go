package sample

import (
	"embed"
	"fmt"
	"path"
	"strings"
)

//go:embed all:_go all:_rust all:_python all:_ts all:_lua
var files embed.FS

// File is one file in a starter function package. Path is relative to the
// function directory.
type File struct {
	Path    string
	Content []byte
}

// Package is the starter file set for a function.
type Package struct {
	Entrypoint string
	Files      []File
}

type language struct {
	dir        string
	extension  string
	syncFile   string
	asyncFile  string
	extraFiles []string
}

var languages = map[string]language{
	"go": {
		dir:        "_go",
		extension:  "go",
		syncFile:   "handle_sync.go",
		asyncFile:  "handle_async.go",
		extraFiles: []string{"go.mod.tmpl", "internal/types/types.go"},
	},
	"rust": {
		dir:        "_rust",
		extension:  "rs",
		syncFile:   "handle_sync.rs",
		asyncFile:  "handle_async.rs",
		extraFiles: []string{"Cargo.toml.tmpl"},
	},
	"python": {
		dir:        "_python",
		extension:  "py",
		syncFile:   "handle_sync.py",
		asyncFile:  "handle_async.py",
		extraFiles: []string{"pyproject.toml.tmpl", "internal/__init__.py", "internal/types.py"},
	},
	"ts": {
		dir:        "_ts",
		extension:  "ts",
		syncFile:   "handle_sync.ts",
		asyncFile:  "handle_async.ts",
		extraFiles: []string{"package.json.tmpl", "internal/types.ts"},
	},
	"lua": {
		dir:        "_lua",
		extension:  "lua",
		syncFile:   "handle_sync.lua",
		asyncFile:  "handle_async.lua",
		extraFiles: []string{"internal/types.lua"},
	},
}

// LoadSync returns the sync entrypoint only. The {{name}} placeholder is left
// intact so the caller can fill in the function name.
func LoadSync(language string) ([]byte, error) {
	return loadEntrypoint(language, "{{name}}", false)
}

// LoadAsync returns the async entrypoint only. The {{name}} placeholder is left
// intact so the caller can fill in the function name.
func LoadAsync(language string) ([]byte, error) {
	return loadEntrypoint(language, "{{name}}", true)
}

// LoadPackage returns the entrypoint (handle.<ext>) plus starter manifests and
// helper files. The {{name}} placeholder is replaced with name.
func LoadPackage(language, name string, isAsync bool) (Package, error) {
	lang := normalize(language)

	spec, ok := languages[lang]
	if !ok {
		return Package{}, fmt.Errorf("no default sample for language %q", language)
	}
	if name == "" {
		name = "sample"
	}

	entrySource := spec.syncFile
	if isAsync {
		entrySource = spec.asyncFile
	}

	body, err := readTemplate(spec.dir, entrySource, name)
	if err != nil {
		return Package{}, err
	}

	entrypoint := "handle." + spec.extension
	pkg := Package{
		Entrypoint: entrypoint,
		Files: []File{
			{Path: entrypoint, Content: body},
		},
	}

	for _, extra := range spec.extraFiles {
		content, err := readTemplate(spec.dir, extra, name)
		if err != nil {
			return Package{}, err
		}
		pkg.Files = append(pkg.Files, File{
			Path:    strings.TrimSuffix(extra, ".tmpl"),
			Content: content,
		})
	}

	return pkg, nil
}

func loadEntrypoint(language, name string, isAsync bool) ([]byte, error) {
	pkg, err := LoadPackage(language, name, isAsync)
	if err != nil {
		return nil, err
	}
	for _, file := range pkg.Files {
		if file.Path == pkg.Entrypoint {
			return file.Content, nil
		}
	}
	return nil, fmt.Errorf("entrypoint missing for language %q", language)
}

func readTemplate(dir, name, functionName string) ([]byte, error) {
	body, err := files.ReadFile(path.Join(dir, name))
	if err != nil {
		return nil, fmt.Errorf("failed to load default sample %q: %w", path.Join(dir, name), err)
	}
	return []byte(strings.ReplaceAll(string(body), "{{name}}", functionName)), nil
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
