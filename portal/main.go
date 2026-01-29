package main

import (
	"github.com/ashupednekar/litewebservices-portal/cmd"
	"github.com/ashupednekar/litewebservices-portal/pkg"
)

func main() {
	pkg.LoadCfg()
	cmd.Execute()
}
