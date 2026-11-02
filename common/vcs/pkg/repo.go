package vcs

import (
	"github.com/ashupednekar/litefunctions/common/vcs"
	"github.com/ashupednekar/litefunctions/common/vcs/internal/repo"
	"github.com/go-git/go-billy/v6"
)

type Repo interface {
	ProjectName() string
	Filesystem() billy.Filesystem
	Clone() error
	Commit(path string) error
	CommitMessage(path, message string) error
	Push() error
	Pull() error
}

func NewRepo(cfg vcs.Config, project string, branch *string) (Repo, error) {
	return repo.New(cfg, project, branch)
}

func EnsureFile(cfg vcs.Config, project, path string, body []byte) (bool, error) {
	return repo.EnsureFile(cfg, project, path, body)
}

func WriteFile(cfg vcs.Config, project, path string, body []byte, commitMessage string) error {
	return repo.WriteFile(cfg, project, path, body, commitMessage)
}
