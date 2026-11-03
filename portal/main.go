package main

import (
	"github.com/ashupednekar/litefunctions/portal/cmd"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
)

func main() {
	config.LoadCfg()
	cmd.Execute()
}
