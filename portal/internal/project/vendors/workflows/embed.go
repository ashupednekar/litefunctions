package workflows

import _ "embed"

//go:embed github.yaml
var GitHubWorkflow []byte

//go:embed gitea.yaml
var GiteaWorkflow []byte


