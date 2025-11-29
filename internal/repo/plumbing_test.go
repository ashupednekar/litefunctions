package repo

import (
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
	PrintFS(r.fs)
}
