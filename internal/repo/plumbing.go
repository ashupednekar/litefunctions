package repo

import (
	"fmt"
	"log"
	"time"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
)

func (r *GitRepo) Clone() error {	
  r.options.ReferenceName = plumbing.NewBranchReferenceName(r.branch)
	r.options.SingleBranch = true
	r.options.Depth = 1
	log.Printf("Cloning %s (branch=%s)\n", r.project, r.branch)
	repo, err := git.Clone(r.storage, r.fs, r.options)
	if err != nil {
		return fmt.Errorf("error cloning repo: %s", err)
	}
	r.repo = repo
  w, err := repo.Worktree()
	if err != nil{
		return fmt.Errorf("error getting worktree: %s", err)
	}
	r.worktree = w
	return nil
}

func (r *GitRepo) Commit(files ...string) error {
	if r.worktree == nil {
		return fmt.Errorf("worktree not initialized; call Clone() first")
	}

	for _, f := range files {
		_, err := r.worktree.Add(f)
		if err != nil {
			return fmt.Errorf("failed to stage file %s: %w", f, err)
		}
	}

	_, err := r.worktree.Commit(
		fmt.Sprintf(
			"Update %s (%s)", r.project, time.Now().Format(time.RFC3339),
		),
		&git.CommitOptions{
			Author: &object.Signature{
				Name:  pkg.Cfg.VcsUser,
				Email: pkg.Cfg.VcsUser,
				When:  time.Now(),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}


func (r *GitRepo) Push() error {
	if r.repo == nil {
		return fmt.Errorf("repo not cloned or initialized")
	}
	fmt.Printf("auth: %v\n", r.options.Auth)
	err := r.repo.Push(&git.PushOptions{
		Auth:     r.options.Auth,
		Progress: r.options.Progress,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("push failed: %w", err)
	}

	return nil
}
