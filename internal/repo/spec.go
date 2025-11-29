package repo

import (
	"fmt"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/storage/memory"
)

type VCS interface {
	Init(remoteUrl string) error
	Clone(project string, branch string) error
	Commit(files ...string) error
	Push() error
}

type GitRepo struct {
	project string
	branch   string
	storage  *memory.Storage
	options *git.CloneOptions
	worktree *git.Worktree
}

var repos map[string]*memory.Storage

func NewGitRepo(project string, branch string) (*GitRepo, error) {
	s, ok := repos[project]
	if ok{
		r, err := git.Open(s, nil)
		if err != nil{
			return nil, fmt.Errorf("error opening repo: %s", err)
		}
		w, err := r.Worktree()
		if err != nil{
			return nil, fmt.Errorf("error getting worktree: %s", err)
		}
	  return &GitRepo{
	  	project: project, 
	  	branch: branch, 
	  	storage: s,
	  	options: &git.CloneOptions{},
			worktree: w,
	  }, nil
	}else{
		return &GitRepo{
	  	project: project, 
	  	branch: branch, 
	  	storage: s,
	  	options: &git.CloneOptions{},
	  }, nil
	}
}
