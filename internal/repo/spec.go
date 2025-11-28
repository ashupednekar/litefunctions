package repo

import (
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
}


func NewGitRepo(project string, branch string) *GitRepo {
	return &GitRepo{
		project: project, 
		branch: branch, 
		storage: memory.NewStorage(),
		options: &git.CloneOptions{},
	}
}
