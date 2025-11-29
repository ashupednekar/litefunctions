package repo

import (
	"fmt"
	"os"

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
}

type GitRepo struct {
	project string
	branch   string
	storage  *memory.Storage
	options *git.CloneOptions
	fs billy.Filesystem
	worktree *git.Worktree
	repo     *git.Repository
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
	if ok{
	  return repo, nil
	}else{
		r := GitRepo{
	  	project: project, 
	  	branch: b, 
			fs: fs,
	  	storage: memory.NewStorage(),
	  	options: &git.CloneOptions{
				URL: fmt.Sprintf("%s/%s/%s", pkg.Cfg.VcsBaseUrl, pkg.Cfg.VcsUser, project),
				Progress: os.Stdout,
			},
	  }
		err := r.SetupAuth()
	  if err != nil {
	  	return nil, err
	  }
		repos[project] = &r
		return &r, nil
	}
}
