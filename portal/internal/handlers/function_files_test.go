package handlers

import (
	"io"
	"path"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
)

func writeTestFile(t *testing.T, fs billy.Filesystem, name, content string) {
	t.Helper()
	if dir := path.Dir(name); dir != "." {
		if err := fs.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	fh, err := fs.Create(name)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	if _, err := fh.Write([]byte(content)); err != nil {
		fh.Close()
		t.Fatalf("write %s: %v", name, err)
	}
	if err := fh.Close(); err != nil {
		t.Fatalf("close %s: %v", name, err)
	}
}

func TestReadFunctionFiles(t *testing.T) {
	fs := memfs.New()
	dir := "functions/python/demo"
	writeTestFile(t, fs, dir+"/handle.py", "print('hi')\n")
	writeTestFile(t, fs, dir+"/README.md", "# demo\n")
	writeTestFile(t, fs, dir+"/internal/types.py", "types = {}\n")

	files, err := readFunctionFiles(fs, dir)
	if err != nil {
		t.Fatalf("readFunctionFiles: %v", err)
	}

	want := []string{"README.md", "handle.py", "internal/types.py"}
	if len(files) != len(want) {
		t.Fatalf("got %d files, want %d: %#v", len(files), len(want), files)
	}
	for i, file := range files {
		if file.Path != want[i] {
			t.Fatalf("file %d path = %q, want %q", i, file.Path, want[i])
		}
	}
	if files[1].Content != "print('hi')\n" {
		t.Fatalf("unexpected content: %q", files[1].Content)
	}
}

func TestBuildFunctionTree(t *testing.T) {
	files := []functionFile{
		{Path: "README.md"},
		{Path: "handle.py"},
		{Path: "internal/sub/util.py"},
		{Path: "internal/types.py"},
	}

	tree := buildFunctionTree(files, "handle.py")
	if len(tree) != 3 {
		t.Fatalf("root has %d nodes, want 3", len(tree))
	}
	if tree[0].Name != "internal" || tree[0].Type != "dir" {
		t.Fatalf("first node = %#v, want internal dir", tree[0])
	}
	if len(tree[0].Children) != 2 {
		t.Fatalf("internal has %d children, want 2", len(tree[0].Children))
	}
	if tree[0].Children[0].Name != "sub" || tree[0].Children[0].Type != "dir" {
		t.Fatalf("first internal child = %#v, want sub dir", tree[0].Children[0])
	}
	if tree[0].Children[1].Name != "types.py" || tree[0].Children[1].Type != "file" {
		t.Fatalf("second internal child = %#v, want types.py", tree[0].Children[1])
	}
	if !tree[2].Main {
		t.Fatalf("handle.py should be marked main: %#v", tree[2])
	}
}

func TestCleanFunctionPath(t *testing.T) {
	dir := "functions/go/demo"
	valid := map[string]string{
		"handle.go":         dir + "/handle.go",
		"internal/types.go": dir + "/internal/types.go",
		"./internal/new.go": dir + "/internal/new.go",
		"a/../b.go":         dir + "/b.go",
	}
	for rel, want := range valid {
		got, ok := cleanFunctionPath(dir, rel)
		if !ok || got != want {
			t.Fatalf("cleanFunctionPath(%q) = %q, %v; want %q, true", rel, got, ok, want)
		}
	}

	invalid := []string{"", ".", "..", "../escape.go", "internal/../../escape.go", "/etc/passwd"}
	for _, rel := range invalid {
		if got, ok := cleanFunctionPath(dir, rel); ok {
			t.Fatalf("cleanFunctionPath(%q) = %q, true; want rejected", rel, got)
		}
	}
}

func TestRemoveFunctionFile(t *testing.T) {
	fs := memfs.New()
	dir := "functions/go/demo"
	writeTestFile(t, fs, dir+"/internal/old.go", "package old\n")

	if err := removeFunctionFile(fs, dir, "internal/old.go"); err != nil {
		t.Fatalf("removeFunctionFile: %v", err)
	}
	if _, err := fs.Stat(dir + "/internal/old.go"); err == nil {
		t.Fatal("file should have been removed")
	}
	if err := removeFunctionFile(fs, dir, "internal/old.go"); err != nil {
		t.Fatalf("removing a missing file should be idempotent: %v", err)
	}
	if err := removeFunctionFile(fs, dir, "../escape.go"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestWriteFunctionFileCreatesNestedDirs(t *testing.T) {
	fs := memfs.New()
	dir := "functions/ts/demo"
	if err := fs.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := writeFunctionFile(fs, dir, "internal/new.ts", "export {};\n"); err != nil {
		t.Fatalf("writeFunctionFile: %v", err)
	}
	fh, err := fs.Open(dir + "/internal/new.ts")
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	data, err := io.ReadAll(fh)
	fh.Close()
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "export {};\n" {
		t.Fatalf("unexpected content: %q", data)
	}

	if err := writeFunctionFile(fs, dir, "../escape.ts", ""); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
	if _, err := fs.Stat("functions/ts/escape.ts"); err == nil {
		t.Fatal("traversal file should not exist")
	}
}
