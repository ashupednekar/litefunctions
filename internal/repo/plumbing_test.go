package repo

import (
	"log"
	"testing"

	"github.com/ashupednekar/litewebservices-portal/pkg"
)

func TestClone(t *testing.T) {
	pkg.LoadCfg()
	r, err := NewGitRepo("projone", nil)
	if err != nil {
		t.Errorf("error in new repo call: %s", err)
	}
	err = r.Clone()
	if err != nil {
		t.Errorf("error cloning repo: %s", err)
	}
	log.Println("opening existing repo")
	rOpened, err := NewGitRepo("projone", nil)
	if err != nil {
		t.Errorf("error in new repo call: %s", err)
	}
	PrintFS(rOpened.fs)
}
