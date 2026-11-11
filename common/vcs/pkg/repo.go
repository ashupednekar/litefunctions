package pkg

import (
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

// File is a path/content pair relative to a project repository.
type File struct {
	Path string
	Body []byte
}

func NewRepo(cfg Config, project string, branch *string) (Repo, error) {
	return repo.New(&cfg, project, branch)
}

func EnsureFile(cfg Config, project, path string, body []byte) (bool, error) {
	return repo.EnsureFile(&cfg, project, path, body)
}

// EnsureFiles writes a whole function package in one commit. It is a no-op when
// any of the files already exist, so it never overwrites user edits.
func EnsureFiles(cfg Config, project string, files []File) (bool, error) {
	internalFiles := make([]repo.File, 0, len(files))
	for _, file := range files {
		internalFiles = append(internalFiles, repo.File{Path: file.Path, Body: file.Body})
	}
	return repo.EnsureFiles(&cfg, project, internalFiles)
}

func WriteFile(cfg Config, project, path string, body []byte, commitMessage string) error {
	return repo.WriteFile(&cfg, project, path, body, commitMessage)
}
