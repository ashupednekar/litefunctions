package repo

import (
	"fmt"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/storage/memory"
)


func (r *Repo) Clone(path string, branch string, dest string) error {
	//TODO: use r.storage
  re, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
      URL: "https://github.com/go-git/go-billy",
  })
	if err != nil{
		return fmt.Errorf("error cloning repo: %s", err)
	}
	fmt.Printf("repo %v\n", re)
	return nil
}

