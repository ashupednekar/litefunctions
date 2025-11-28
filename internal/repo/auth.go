package repo

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
)


type AuthMode interface{
	GetAuth() (*transport.AuthMethod, error)
}

type SshAuth struct{
	privKey string
	password string
}

type TokenAuth struct {
	token string
}

func (s *SshAuth) GetAuth(options *git.CloneOptions) (*git.CloneOptions, error){
	_, err := os.Stat(s.privKey)
	if err != nil {
		return nil, fmt.Errorf("read file %s failed %s\n", s.privKey, err.Error())
	}
	publicKeys, err := ssh.NewPublicKeysFromFile("git", s.privKey, s.password)
	if err != nil{
		return nil, fmt.Errorf("error getting pubkey: %s", err)
	}
	options.Auth = publicKeys
	options.Progress = os.Stdout
	return options, nil
}

func (t *TokenAuth) GetAuth(options *git.CloneOptions) (*git.CloneOptions, error) {
	options.Auth = &http.BasicAuth{Password: t.token}
  return options, nil
}
