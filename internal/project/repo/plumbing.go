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
	r.Options.ReferenceName = plumbing.NewBranchReferenceName(r.Branch)
	r.Options.SingleBranch = true
	r.Options.Depth = 1
	log.Printf("Cloning %s (branch=%s)\n", r.Project, r.Branch)
	repo, err := git.Clone(r.Storage, r.Fs, r.Options)
	if err != nil {
		return fmt.Errorf("error cloning repo: %s", err)
	}
	r.Repo = repo
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("error getting worktree: %s", err)
	}
	r.Worktree = w
	return nil
}

func (r *GitRepo) Commit(files ...string) error {
	if r.Worktree == nil {
		return fmt.Errorf("worktree not initialized; call Clone() first")
	}

	for _, f := range files {
		_, err := r.Fs.Stat(f)
		if err != nil {
			_, err = r.Worktree.Remove(f)
			if err != nil {
				return fmt.Errorf("failed to stage deletion of %s: %w", f, err)
			}
		} else {
			_, err := r.Worktree.Add(f)
			if err != nil {
				return fmt.Errorf("failed to stage file %s: %w", f, err)
			}
		}
	}

	_, err := r.Worktree.Commit(
		fmt.Sprintf(
			"Update %s (%s)", r.Project, time.Now().Format(time.RFC3339),
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
	if r.Repo == nil {
		return fmt.Errorf("repo not cloned or initialized")
	}
	fmt.Printf("auth: %v\n", r.Options.Auth)
	err := r.Repo.Push(&git.PushOptions{
		Auth:     r.Options.Auth,
		Progress: r.Options.Progress,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("push failed: %w", err)
	}

	return nil
}

func (r *GitRepo) Pull() error {
	wt, err := r.Repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree error: %w", err)
	}

	err = wt.Pull(&git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(r.Branch),
		Auth:          r.Options.Auth,
		Force:         true,
	})

	if err == git.NoErrAlreadyUpToDate {
		return nil
	}

	if err != nil {
		return fmt.Errorf("pull error: %w", err)
	}

	ref, err := r.Repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Hash:  ref.Hash(),
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("failed to checkout: %w", err)
	}

	r.Worktree = wt

	return nil
}
