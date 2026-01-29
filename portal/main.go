package main

import (
	"github.com/ashupednekar/litefunctions/portal/cmd"
	"github.com/ashupednekar/litefunctions/portal/pkg"
)

func main() {
	pkg.LoadCfg()
	cmd.Execute()
}
