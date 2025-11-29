package repo

import (
	"fmt"
	"log"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
)

func (r *GitRepo) Clone() error {
	err := r.SetupAuth()
	if err != nil {
		return err
	}
  r.options.ReferenceName = plumbing.NewBranchReferenceName(r.branch)
	r.options.SingleBranch = true
	r.options.Depth = 1
	log.Printf("Cloning %s (branch=%s)\n", r.project, r.branch)
	repo, err := git.Clone(r.storage, r.fs, &git.CloneOptions{
		URL: fmt.Sprintf(
			"%s/%s/%s", pkg.Cfg.VcsVendor, pkg.Cfg.VcsUser, r.project,
		),
	})
	if err != nil {
		return fmt.Errorf("error cloning repo: %s", err)
	}
  w, err := repo.Worktree()
	if err != nil{
		return fmt.Errorf("error getting worktree: %s", err)
	}
	r.worktree = w
	return nil
}

func (r *GitRepo) Commit(files ...string) error {
	return nil
}

func (r *GitRepo) Push() error {
	return nil
}

func (r *GitRepo) Init(remoteUrl string) error {
	return nil
}


