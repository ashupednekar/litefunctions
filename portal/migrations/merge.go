package migrations

import (
	"io/fs"
)

// Merge presents several embedded migration trees as one filesystem so the
// portal can run both portal-owned and module-owned migrations with goose.
func Merge(sources ...fs.FS) fs.FS {
	return mergedFS(sources)
}

type mergedFS []fs.FS

func (m mergedFS) Open(name string) (fs.File, error) {
	for _, source := range m {
		file, err := source.Open(name)
		if err == nil {
			return file, nil
		}
	}
	return nil, fs.ErrNotExist
}

func (m mergedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	seen := map[string]struct{}{}
	var entries []fs.DirEntry
	found := false
	for _, source := range m {
		dirEntries, err := fs.ReadDir(source, name)
		if err != nil {
			continue
		}
		found = true
		for _, entry := range dirEntries {
			if _, ok := seen[entry.Name()]; ok {
				continue
			}
			seen[entry.Name()] = struct{}{}
			entries = append(entries, entry)
		}
	}
	if !found {
		return nil, fs.ErrNotExist
	}
	return entries, nil
}
