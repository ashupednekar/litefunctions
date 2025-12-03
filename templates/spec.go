package templates

type Project struct {
	ID   string
	Name string
}

type Function struct {
	ID          string
	Name        string
	Language    string
	Description string
	Icon        string
}

type Lang struct {
	ID      string
	Label   string
	Icon    string
	AceMode string
}
