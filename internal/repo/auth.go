package repo

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
)

type AuthMode interface {
	UpdateOptions(options *git.CloneOptions) error
}

type SshAuth struct {
	privKey  string
	password string
}

type TokenAuth struct {
	token string
}

func (s *SshAuth) UpdateOptions(options *git.CloneOptions) error {
	_, err := os.Stat(s.privKey)
	if err != nil {
		return fmt.Errorf("read file %s failed %s\n", s.privKey, err.Error())
	}
	publicKeys, err := ssh.NewPublicKeysFromFile("git", s.privKey, s.password)
	if err != nil {
		return fmt.Errorf("error getting pubkey: %s", err)
	}
	options.Auth = publicKeys
	options.Progress = os.Stdout
	return nil
}

func (t *TokenAuth) UpdateOptions(options *git.CloneOptions) error {
	options.Auth = &http.BasicAuth{Username: "git", Password: t.token}
	return nil
}

func (r *GitRepo) SetupAuth() error {
	log.Printf("setting up %s auth\n", pkg.Cfg.VcsAuthMode)
	switch pkg.Cfg.VcsAuthMode{
	case "ssh":
    vendor := strings.TrimPrefix(strings.TrimPrefix(pkg.Cfg.VcsVendor, "https://"), "http://")
		r.options.URL = fmt.Sprintf(
        "git@%s:%s/%s.git",
        vendor, 
        pkg.Cfg.VcsUser,
        r.project,
    )
		auth := &SshAuth{
			privKey: pkg.Cfg.VcsPrivKeyPath,
			password: pkg.Cfg.VcsPrivKeyPassword,
		}
		auth.UpdateOptions(r.options)
	case "token":
		auth := &TokenAuth{token: pkg.Cfg.VcsToken}
		auth.UpdateOptions(r.options)
	default:
		return fmt.Errorf("invalid auth mode: %s", pkg.Cfg.VcsAuthMode)
	}
	return nil
}
