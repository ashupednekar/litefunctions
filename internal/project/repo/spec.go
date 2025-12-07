package repo

import (
	"fmt"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/storage/memory"
)

type VCS interface {
	Clone(project string, branch string) error
	Commit(files ...string) error
	Push() error
	Pull() error
}

type GitRepo struct {
	Project  string
	Branch   string
	Storage  *memory.Storage
	Options  *git.CloneOptions
	Fs       billy.Filesystem
	Worktree *git.Worktree
	Repo     *git.Repository
}

var repos = make(map[string]*GitRepo)

func NewGitRepo(project string, branch *string) (*GitRepo, error) {
	fs := memfs.New()
	var b string
	if branch != nil {
		b = *branch
	} else {
		b = "main"
	}
	repo, ok := repos[project]
	if ok {
		fmt.Printf("[DEBUG] Found cached repo for %s, calling Pull()...\n", project)
		err := repo.Pull()
		if err != nil {
			fmt.Printf("[ERROR] Pull() failed: %v\n", err)
			return nil, err
		}
		fmt.Printf("[DEBUG] Pull() completed successfully for %s\n", project)
		return repo, nil
	} else {
		r := GitRepo{
			Project: project,
			Branch:  b,
			Fs:      fs,
			Storage: memory.NewStorage(),
			Options: &git.CloneOptions{
				URL: fmt.Sprintf("%s/%s/%s.git", pkg.Cfg.VcsBaseUrl, pkg.Cfg.VcsUser, project),
			},
		}
		err := r.SetupAuth()
		if err != nil {
			return nil, fmt.Errorf("auth setup failed: %w", err)
		}
		if err := r.Clone(); err != nil {
			return nil, fmt.Errorf("clone failed for %s: %w", project, err)
		}
		repos[project] = &r
		return &r, nil
	}
}
