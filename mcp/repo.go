package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/ashupednekar/litefunctions/common"
	vcs "github.com/ashupednekar/litefunctions/common/vcs/pkg"
)

func cleanRepoPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "", errors.New("path is required")
	}
	clean := path.Clean(p)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid path %q", p)
	}
	return clean, nil
}

func readRepoFile(r vcs.Repo, name string) ([]byte, error) {
	file, err := r.Filesystem().Open(name)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return body, nil
}

func writeRepoFile(r vcs.Repo, name string, body []byte) error {
	if dir := path.Dir(name); dir != "." && dir != "/" {
		if err := r.Filesystem().MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", name, err)
		}
	}
	file, err := r.Filesystem().Create(name)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	if _, err := file.Write(body); err != nil {
		file.Close()
		return fmt.Errorf("write %s: %w", name, err)
	}
	return file.Close()
}

func removeRepoPath(r vcs.Repo, name string) error {
	info, err := r.Filesystem().Stat(name)
	if err != nil {
		return fmt.Errorf("stat %s: %w", name, err)
	}
	if !info.IsDir() {
		return r.Filesystem().Remove(name)
	}
	entries, err := r.Filesystem().ReadDir(name)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", name, err)
	}
	for _, entry := range entries {
		if err := removeRepoPath(r, path.Join(name, entry.Name())); err != nil {
			return err
		}
	}
	return r.Filesystem().Remove(name)
}

func listRepoFiles(r vcs.Repo, root string) ([]string, error) {
	if root == "" {
		root = "."
	}
	var out []string
	var walk func(string) error
	walk = func(dir string) error {
		entries, err := r.Filesystem().ReadDir(dir)
		if err != nil {
			if dir == "." {
				return nil
			}
			return err
		}
		for _, entry := range entries {
			name := path.Join(dir, entry.Name())
			if entry.Name() == ".git" {
				continue
			}
			if entry.IsDir() {
				if err := walk(name); err != nil {
					return err
				}
				continue
			}
			out = append(out, strings.TrimPrefix(name, "./"))
			if len(out) > 5000 {
				return fmt.Errorf("too many files; narrow the path")
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func loadManifest(r vcs.Repo) (*common.FunctionsManifest, error) {
	body, err := readRepoFile(r, common.FunctionsManifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrInvalid) || strings.Contains(err.Error(), "file does not exist") || strings.Contains(err.Error(), "no such file") {
			return common.ParseFunctionsManifest(nil)
		}
		return nil, err
	}
	return common.ParseFunctionsManifest(body)
}

func saveManifest(r vcs.Repo, manifest *common.FunctionsManifest) error {
	body, err := manifest.Marshal()
	if err != nil {
		return err
	}
	return writeRepoFile(r, common.FunctionsManifestPath, body)
}

func commitPush(r vcs.Repo, message string, paths ...string) error {
	for _, p := range paths {
		if err := r.CommitMessage(p, message); err != nil {
			if strings.Contains(err.Error(), "clean working tree") || strings.Contains(err.Error(), "empty commit") {
				continue
			}
			return fmt.Errorf("commit %s: %w", p, err)
		}
	}
	if err := r.Push(); err != nil {
		return fmt.Errorf("push: %w", err)
	}
	return nil
}
