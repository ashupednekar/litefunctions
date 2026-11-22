package repo

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ashupednekar/litefunctions/common/vcs/internal"
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
	cfg      *internal.Config
}

var (
	repos   = make(map[string]*GitRepo)
	reposMu sync.Mutex
)

func New(cfg *internal.Config, project string, branch *string) (*GitRepo, error) {
	reposMu.Lock()
	defer reposMu.Unlock()

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
		if err := repo.Pull(); err == nil {
			fmt.Printf("[DEBUG] Pull() completed successfully for %s\n", project)
			return repo, nil
		} else {
			// A cached in-memory repo can be left without the remote's objects
			// after a failed pull. Drop it and clone again so one bad sync does
			// not poison the project for the life of the process.
			fmt.Printf("[ERROR] Pull() failed: %v; evicting cached repo\n", err)
			delete(repos, project)
		}
	}
	{
		r := GitRepo{
			Project: project,
			Branch:  b,
			Fs:      fs,
			Storage: memory.NewStorage(),
			cfg:     cfg,
			Options: &git.CloneOptions{
				URL: fmt.Sprintf("%s/%s/%s.git", strings.TrimRight(cfg.BaseUrl, "/"), cfg.User, project),
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

func (r *GitRepo) ProjectName() string {
	return r.Project
}

func (r *GitRepo) Filesystem() billy.Filesystem {
	return r.Fs
}
