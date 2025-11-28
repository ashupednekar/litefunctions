package repo

import (
	"github.com/go-git/go-git/v6/storage"
)


type GitRepo interface{
	Init(remoteUrl string) error
	Clone(branch string, dest string) error
	Commit(files ...string) error
	Push(path string) error
}

type Repo struct{
	url string
	branch string
	storage *storage.Storer
}







