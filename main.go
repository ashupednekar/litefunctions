package main

import (
	"github.com/ashupednekar/lwsportal/cmd"
	"github.com/ashupednekar/lwsportal/pkg"
)

func main() {
	pkg.LoadCfg()
	cmd.Execute()
}
