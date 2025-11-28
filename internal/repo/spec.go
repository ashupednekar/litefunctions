package repo

import (
	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage/memory"
)

type VCS interface{
	Init(remoteUrl string) error
	Clone(branch string, dest string) error
	Commit(files ...string) error
	Push() error
}

type GitRepo struct{
	url string
	branch string
	storage *memory.Storage
	authMode *AuthMode
}


func NewGitRepo(url string, branch string) *GitRepo {
	return &GitRepo{
		url: url, branch: branch, storage: memory.NewStorage(),
	}
}

func (r *GitRepo) SetupAuth() error {

	return nil
}



