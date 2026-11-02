package vcs


type Config struct {
	UsePrivateRepo       bool   `env:"VCS_USE_PRIVATE_REPO" default:"false"`
	AuthMode             string `env:"VCS_AUTH_MODE" default:"ssh"`
	PrivKeyPath          string `env:"VCS_PRIVATE_KEY_PATH" default:"/app/.ssh/privkey.pem"`
	PrivKeyPassword      string `env:"VCS_PRIVATE_KEY_PASSWORD"`
	Token                string `env:"VCS_TOKEN"`
	User                 string `env:"VCS_USER"`
	Vendor               string `env:"VCS_VENDOR"`
	BaseUrl              string `env:"VCS_BASE_URL"`
	PublicBaseUrl        string `env:"VCS_PUBLIC_BASE_URL"`
	FQDN            string
}

