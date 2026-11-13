package sample

import (
	"strings"
	"testing"
)

func TestLoadPackageAllLanguages(t *testing.T) {
	cases := []struct {
		lang     string
		ext      string
		async    bool
		extras   []string
		contains string
	}{
		{"go", "go", false, []string{"go.mod", "internal/types/types.go"}, "runtimes/go/functions/hu/internal/types"},
		{"go", "go", true, []string{"go.mod", "internal/types/types.go"}, "StreamHandler"},
		{"rust", "rs", false, []string{"Cargo.toml"}, "pub async fn handle"},
		{"rust", "rs", true, []string{"Cargo.toml"}, "stream_handler"},
		{"python", "py", false, []string{"pyproject.toml", "internal/__init__.py", "internal/types.py"}, "from .internal.types"},
		{"python", "py", true, []string{"pyproject.toml"}, "yield"},
		{"ts", "ts", false, []string{"package.json", "internal/types.ts"}, `from "./internal/types"`},
		{"ts", "ts", true, []string{"package.json"}, "yield"},
		{"lua", "lua", false, []string{"internal/types.lua"}, `require("internal.types")`},
		{"java", "java", false, []string{"types/Input.java", "types/Output.java"}, "class Handler"},
		{"java", "java", true, []string{"types/Input.java", "types/Output.java"}, "streamHandler"},
	}

	for _, tc := range cases {
		pkg, err := LoadPackage(tc.lang, "hu", tc.async)
		if err != nil {
			t.Fatalf("%s async=%v: %v", tc.lang, tc.async, err)
		}
		if pkg.Entrypoint != "handle."+tc.ext {
			t.Fatalf("%s: entrypoint = %q", tc.lang, pkg.Entrypoint)
		}

		paths := map[string][]byte{}
		for _, file := range pkg.Files {
			paths[file.Path] = file.Content
		}

		entrypoint, ok := paths[pkg.Entrypoint]
		if !ok {
			t.Fatalf("%s: entrypoint missing from files", tc.lang)
		}
		if !strings.Contains(string(entrypoint), tc.contains) {
			t.Fatalf("%s async=%v: entrypoint does not contain %q:\n%s", tc.lang, tc.async, tc.contains, entrypoint)
		}
		if strings.Contains(string(entrypoint), "{{name}}") {
			t.Fatalf("%s: entrypoint still contains the name placeholder", tc.lang)
		}
		for _, extra := range tc.extras {
			if _, ok := paths[extra]; !ok {
				t.Fatalf("%s: missing extra %q", tc.lang, extra)
			}
		}
	}
}

func TestLoadSyncLeavesNamePlaceholder(t *testing.T) {
	body, err := LoadSync("go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "{{name}}") {
		t.Fatal("expected the editor sample to keep the {{name}} placeholder")
	}
}
