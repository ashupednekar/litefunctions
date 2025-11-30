package repo

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-billy/v6"
)

func PrintFS(fs billy.Filesystem) {
	walk(fs, "/", "")
}

func walk(fs billy.Filesystem, dir, indent string) {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		fmt.Printf("%s[ERROR reading %s]: %s\n", indent, dir, err)
		return
	}

	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(dir, name)

		if e.IsDir() {
			fmt.Printf("%s📁 %s/\n", indent, name)
			walk(fs, path, indent+"  ")
		} else {
			fmt.Printf("%s📄 %s\n", indent, name)

			// If you want file contents:
			f, err := fs.Open(path)
			if err == nil {
				buf := make([]byte, 1024)
				n, _ := f.Read(buf)
				fmt.Printf("%s    └─ contents: %q\n", indent, string(buf[:n]))
			}
		}
	}
}
