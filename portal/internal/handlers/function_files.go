package handlers

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v6"
)

// functionFile is a file inside a function package. Path is always relative to
// the function directory (for example "handle.py" or "internal/types.py").
type functionFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// functionTreeNode is the nested representation of a function package used by
// the editor file tree.
type functionTreeNode struct {
	Name     string              `json:"name"`
	Path     string              `json:"path"`
	Type     string              `json:"type"`
	Main     bool                `json:"main,omitempty"`
	Children []*functionTreeNode `json:"children,omitempty"`
}

// readFunctionFiles recursively reads every file below dir and returns paths
// relative to dir, sorted for deterministic responses.
func readFunctionFiles(fs billy.Filesystem, dir string) ([]functionFile, error) {
	files := []functionFile{}

	var walk func(string) error
	walk = func(current string) error {
		entries, err := fs.ReadDir(current)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			full := path.Join(current, entry.Name())
			if entry.IsDir() {
				if entry.Name() == ".git" {
					continue
				}
				if err := walk(full); err != nil {
					return err
				}
				continue
			}

			fh, err := fs.Open(full)
			if err != nil {
				return err
			}
			data, err := io.ReadAll(fh)
			fh.Close()
			if err != nil {
				return err
			}

			rel := strings.TrimPrefix(strings.TrimPrefix(full, dir), "/")
			files = append(files, functionFile{Path: rel, Content: string(data)})
		}
		return nil
	}

	if err := walk(dir); err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// buildFunctionTree converts a flat relative file list into a nested tree.
func buildFunctionTree(files []functionFile, entrypoint string) []*functionTreeNode {
	root := &functionTreeNode{Type: "dir", Children: []*functionTreeNode{}}

	for _, file := range files {
		parts := strings.Split(file.Path, "/")
		current := root

		for i, part := range parts {
			isFile := i == len(parts)-1
			var next *functionTreeNode
			for _, child := range current.Children {
				if child.Name == part {
					next = child
					break
				}
			}
			if next == nil {
				next = &functionTreeNode{
					Name: part,
					Path: strings.Join(parts[:i+1], "/"),
					Type: "dir",
				}
				if isFile {
					next.Type = "file"
					next.Main = file.Path == entrypoint
				} else {
					next.Children = []*functionTreeNode{}
				}
				current.Children = append(current.Children, next)
			}
			current = next
		}
	}

	sortFunctionTree(root.Children)
	return root.Children
}

func sortFunctionTree(nodes []*functionTreeNode) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "dir"
		}
		return nodes[i].Name < nodes[j].Name
	})
	for _, node := range nodes {
		sortFunctionTree(node.Children)
	}
}

// cleanFunctionPath resolves a package-relative path below dir and rejects
// absolute paths, traversal and empty names.
func cleanFunctionPath(dir, rel string) (string, bool) {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" || strings.HasPrefix(rel, "/") {
		return "", false
	}
	clean := path.Clean(path.Join(dir, rel))
	if clean == dir || !strings.HasPrefix(clean, dir+"/") {
		return "", false
	}
	return clean, true
}

// removeFunctionFile removes one package file. Missing files are ignored so
// deleting an already-removed file stays idempotent.
func removeFunctionFile(fs billy.Filesystem, dir, rel string) error {
	clean, ok := cleanFunctionPath(dir, rel)
	if !ok {
		return fmt.Errorf("invalid file path %q", rel)
	}
	if _, err := fs.Stat(clean); err != nil {
		return nil
	}
	return fs.Remove(clean)
}

// writeFunctionFile writes one package file, creating parent directories.
func writeFunctionFile(fs billy.Filesystem, dir, rel, content string) error {
	clean, ok := cleanFunctionPath(dir, rel)
	if !ok {
		return fmt.Errorf("invalid file path %q", rel)
	}
	if d := path.Dir(clean); d != dir {
		if err := fs.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	fh, err := fs.Create(clean)
	if err != nil {
		return err
	}
	if _, err := fh.Write([]byte(content)); err != nil {
		fh.Close()
		return err
	}
	return fh.Close()
}
