package repo

import (
	"fmt"
	"log"
	"os"
	"time"

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

func (r *GitRepo) Commit(path string) error {
    wt, err := r.Repo.Worktree()
    if err != nil {
        return err
    }

    _, statErr := r.Fs.Stat(path)
    if os.IsNotExist(statErr) {
        if _, err := wt.Remove(path); err != nil {
            return fmt.Errorf("failed to stage deletion: %w", err)
        }
    } else if statErr == nil {
        if _, err := wt.Add(path); err != nil {
            return fmt.Errorf("failed to stage file update: %w", err)
        }
    } else {
        return fmt.Errorf("stat error: %w", statErr)
    }

    _, err = wt.Commit("update "+path, &git.CommitOptions{
        Author: &object.Signature{
            Name:  "LiteWebServices Portal",
            Email: "noreply@example.com",
            When:  time.Now(),
        },
    })
    return err
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
