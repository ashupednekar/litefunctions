package repo

import (
	"fmt"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/go-git/go-git/v6"
)

func (r *GitRepo) Clone(project, string, branch string) error {
	err := r.SetupAuth()
	if err != nil {
		return err
	}
	repo, err := git.Clone(r.storage, nil, &git.CloneOptions{
		URL: fmt.Sprintf(
			"%s/%s/%s", pkg.Cfg.VcsVendor, pkg.Cfg.VcsUser, project,
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
