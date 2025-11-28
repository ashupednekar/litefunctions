package repo

import (
	"fmt"

	"github.com/go-git/go-git/v6"
)

func (r *GitRepo) Clone(path string, branch string, dest string) error {
	//TODO: auth
  re, err := git.Clone(r.storage, nil, &git.CloneOptions{
      URL: "https://github.com/go-git/go-billy",
  })
	if err != nil{
		return fmt.Errorf("error cloning repo: %s", err)
	}
	fmt.Printf("repo %v\n", re)
	return nil
}

func (r *GitRepo) Commit(files ...string) error {
	return nil
}

func (r *GitRepo) Push() error{
	return nil
} 

func (r *GitRepo) Init(remoteUrl string) error {
	return nil
}

