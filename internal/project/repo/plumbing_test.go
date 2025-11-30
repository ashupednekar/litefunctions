package repo

import (
	"log"
	"os"
	"testing"

	"github.com/ashupednekar/litewebservices-portal/pkg"
)

func TestVcs(t *testing.T) {
	pkg.LoadCfg()
	r, err := NewGitRepo("projone", nil)
	if err != nil {
		t.Errorf("error in new repo call: %s", err)
	}
	err = r.Clone()
	if err != nil {
		t.Errorf("error cloning repo: %s", err)
	}
	readme := "README.md"
  if _, err := r.fs.Stat(readme); err != nil {
      f, err := r.fs.Create(readme)
      if err != nil {
          t.Fatalf("cannot create README.md: %s", err)
      }
      _, _ = f.Write([]byte("# " + r.project + "\n"))
      f.Close()
  } else {
      f, err := r.fs.OpenFile(readme, os.O_WRONLY|os.O_APPEND, 0644)
      if err != nil {
          t.Fatalf("cannot append to README.md: %s", err)
      }
      _, _ = f.Write([]byte("\n"))
      f.Close()
  }
	if err := r.Commit(readme); err != nil {
      t.Fatalf("commit error: %s", err)
  }

  if err := r.Push(); err != nil {
      t.Fatalf("push error: %s", err)
  }
	log.Println("opening existing repo")
	rOpened, err := NewGitRepo("projone", nil)
	if err != nil {
		t.Errorf("error in new repo call: %s", err)
	}
	PrintFS(rOpened.fs)
}


