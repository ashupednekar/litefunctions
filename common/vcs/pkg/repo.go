package pkg

import (
	"context"
	"fmt"
	"strings"

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

// EnsureFiles writes a whole function package in one commit. It creates the
// project repository when needed and never overwrites user edits.
func EnsureFiles(cfg Config, project string, files []File) (bool, error) {
	if len(files) == 0 {
		return false, nil
	}
	if err := ensureRepo(cfg, project); err != nil {
		return false, err
	}

	internalFiles := make([]repo.File, 0, len(files))
	for _, file := range files {
		internalFiles = append(internalFiles, repo.File{Path: file.Path, Body: file.Body})
	}
	return repo.EnsureFiles(&cfg, project, internalFiles)
}

func ensureRepo(cfg Config, project string) error {
	client, err := NewVendorClient(cfg)
	if err != nil {
		return fmt.Errorf("init VCS client for %s: %w", project, err)
	}

	_, err = client.CreateRepo(context.Background(), CreateRepoOptions{
		Name:          project,
		Description:   "Created by LiteFunctions",
		Private:       cfg.UsePrivateRepo,
		AutoInit:      true,
		DefaultBranch: "main",
	})
	if err != nil && !repoAlreadyExists(err) {
		return fmt.Errorf("create repo %s: %w", project, err)
	}
	return nil
}

func repoAlreadyExists(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "already exists") || strings.Contains(message, "status code 409")
}

func WriteFile(cfg Config, project, path string, body []byte, commitMessage string) error {
	return repo.WriteFile(&cfg, project, path, body, commitMessage)
}
