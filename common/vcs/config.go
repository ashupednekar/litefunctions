package vcs

type Config struct {
	AuthMode        string
	PrivKeyPath     string
	PrivKeyPassword string
	Token           string
	User            string
	Vendor          string
	BaseURL         string
	PublicBaseURL   string
	FQDN            string
}
