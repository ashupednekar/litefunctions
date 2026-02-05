package templates

import "time"

type Project struct {
	ID     string
	Name   string
	Status string
	Role   string
}

type Invite struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type AccessUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type Function struct {
	ID         string
	Name       string
	Language   string
	Icon       string
	EndpointID string
}

type Lang struct {
	ID      string
	Label   string
	Icon    string
	AceMode string
}
