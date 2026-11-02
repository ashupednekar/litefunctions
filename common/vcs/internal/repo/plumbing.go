package repo

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ashupednekar/litefunctions/common/vcs/internal/config"
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
	return r.CommitMessage(path, "update "+path)
}

func (r *GitRepo) CommitMessage(path, message string) error {
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

	_, err = wt.Commit(message, &git.CommitOptions{
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

// EnsureFile writes and pushes body to path when the file is not already
// present in the project repository. It returns true when a commit was pushed.
func EnsureFile(cfg config.Config, project, path string, body []byte) (bool, error) {
	r, err := New(cfg, project, nil)
	if err != nil {
		return false, err
	}

	if _, err := r.Fs.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}

	if err := writeAndPush(r, path, body, "add "+path); err != nil {
		return false, err
	}
	return true, nil
}

func WriteFile(
	cfg config.Config,
	project string,
	path string,
	body []byte,
	commitMessage string,
) error {
	r, err := New(cfg, project, nil)
	if err != nil {
		slog.Error("error in repo new", "error", err)
		return err
	}
	return writeAndPush(r, path, body, commitMessage)
}

func writeAndPush(r *GitRepo, path string, body []byte, commitMessage string) error {
	if dir := filepath.Dir(path); dir != "." && dir != "/" {
		if err := r.Fs.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory for %s - %s", path, err)
		}
	}

	slog.Info("writing", "file", path, "repo", r.Project)
	fh, err := r.Fs.Create(path)
	if err != nil {
		return fmt.Errorf("error getting file handle on %s - %s", path, err)
	}
	if _, err := fh.Write(body); err != nil {
		fh.Close()
		return fmt.Errorf("error writing %s - %s", path, err)
	}
	fh.Close()

	slog.Info("commiting file", "path", path)
	if err := r.CommitMessage(path, commitMessage); err != nil && !errors.Is(err, git.ErrEmptyCommit) {
		slog.Error("error commiting file", "path", path, "error", err)
		return fmt.Errorf("commit error %s", err)
	}

	slog.Info("pushing file", "path", path)
	if err := r.Push(); err != nil {
		return fmt.Errorf("push error %s", err)
	}
	return nil
}
