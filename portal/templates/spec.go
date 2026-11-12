package templates

import (
	"strconv"
	"strings"
	"time"
)

func itoa(n int) string { return strconv.Itoa(n) }

func shortID(id string) string {
	if len(id) > 10 {
		return id[:10]
	}
	return id
}

func svgSearch() string {
	return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="m21 21-4.35-4.35"/></svg>`
}

func svgPlus() string {
	return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>`
}

func svgCpu() string {
	return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M15 2v2M15 20v2M2 15h2M2 9h2M20 15h2M20 9h2M9 2v2M9 20v2"/></svg>`
}

func svgShield() string {
	return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1Z"/></svg>`
}

func svgExternal() string {
	return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/></svg>`
}

func svgTable() string {
	return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M3 15h18M9 3v18"/></svg>`
}

func svgBraces() string {
	return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M8 3H7a2 2 0 0 0-2 2v5a2 2 0 0 1-2 2 2 2 0 0 1 2 2v5a2 2 0 0 0 2 2h1"/><path d="M16 3h1a2 2 0 0 1 2 2v5a2 2 0 0 0 2 2 2 2 0 0 0-2 2v5a2 2 0 0 1-2 2h-1"/></svg>`
}

func svgClock() string {
	return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>`
}

func svgRefresh() string {
	return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 0 1 15-6.7L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-15 6.7L3 16"/><path d="M3 21v-5h5"/></svg>`
}

// DashboardData is the view model for the workspace overview.
type DashboardData struct {
	Username        string
	Projects        []Project
	ActiveProjectID string
	ActiveProject   string
	ActiveRole      string
	FunctionCount   int
	EndpointCount   int
}

// ShellData is the authenticated application chrome passed to BaseLayout.
type ShellData struct {
	Title       string
	Section     string
	Active      string
	Username    string
	Role        string
	ProjectID   string
	ProjectName string
	// Dummy is true when the page renders fixture data instead of live data.
	// The shell shows a "dummy data" badge so it is never mistaken for real state.
	Dummy bool
}

// PageUsesDummyData lists the UI-only surfaces that currently render fixtures.
func PageUsesDummyData(active string) bool {
	switch active {
	case "overview", "dependencies", "data-model", "agent", "data",
		"pubsub", "traffic", "compute", "observability", "configuration",
		"access", "audit":
		return true
	}
	return false
}

func projectLabel(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Select a project"
	}
	return name
}

func projectRole(role string) string {
	if strings.TrimSpace(role) == "" {
		return "workspace"
	}
	return role
}

func projectInitial(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "∅"
	}
	return strings.ToUpper(name[:1])
}

func userInitial(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	parts := strings.Fields(name)
	if len(parts) > 1 {
		return strings.ToUpper(parts[0][:1] + parts[1][:1])
	}
	if len(name) >= 2 {
		return strings.ToUpper(name[:2])
	}
	return strings.ToUpper(name[:1])
}

func lines(s string) []string { return strings.Split(strings.TrimRight(s, "\n"), "\n") }

func tabStyle(first bool) string {
	if first {
		return ""
	}
	return "display:none"
}

type Dependency struct {
	Name    string
	Version string
	Scope   string
	Status  string
	Note    string
}

type DependencyManifest struct {
	ID       string
	Language string
	File     string
	Code     string
	Deps     []Dependency
	Count    int
	Updates  int
	Vulns    int
}

func dependencyManifests() []DependencyManifest {
	return []DependencyManifest{
		{
			ID: "go", Language: "Go", File: "go.mod", Count: 12, Updates: 2, Vulns: 0,
			Code: "module github.com/acme/orders\n\ngo 1.26.0\n\nrequire (\n\tgithub.com/a-h/templ v0.3.977\n\tgithub.com/gin-gonic/gin v1.11.0\n\tgithub.com/jackc/pgx/v5 v5.8.0\n\tgithub.com/nats-io/nats.go v1.55.0\n)\n\nrequire (\n\tgolang.org/x/sync v0.23.0 // indirect\n)\n",
			Deps: []Dependency{
				{Name: "github.com/a-h/templ", Version: "v0.3.977", Scope: "direct", Status: "current"},
				{Name: "github.com/gin-gonic/gin", Version: "v1.11.0", Scope: "direct", Status: "current"},
				{Name: "github.com/jackc/pgx/v5", Version: "v5.8.0", Scope: "direct", Status: "update", Note: "v5.9.1"},
				{Name: "github.com/nats-io/nats.go", Version: "v1.55.0", Scope: "direct", Status: "current"},
				{Name: "golang.org/x/sync", Version: "v0.23.0", Scope: "indirect", Status: "current"},
			},
		},
		{
			ID: "ts", Language: "Node / TS", File: "package.json", Count: 9, Updates: 1, Vulns: 0,
			Code: "{\n  \"name\": \"orders-api\",\n  \"private\": true,\n  \"type\": \"module\",\n  \"scripts\": {\n    \"dev\": \"vite\",\n    \"build\": \"tsc && vite build\"\n  },\n  \"dependencies\": {\n    \"@litefunctions/sdk\": \"^0.4.2\",\n    \"zod\": \"^3.24.1\"\n  },\n  \"devDependencies\": {\n    \"typescript\": \"^5.7.3\",\n    \"vite\": \"^6.0.7\"\n  }\n}\n",
			Deps: []Dependency{
				{Name: "@litefunctions/sdk", Version: "^0.4.2", Scope: "runtime", Status: "current"},
				{Name: "zod", Version: "^3.24.1", Scope: "runtime", Status: "update", Note: "3.25.0"},
				{Name: "typescript", Version: "^5.7.3", Scope: "dev", Status: "current"},
				{Name: "vite", Version: "^6.0.7", Scope: "dev", Status: "current"},
			},
		},
		{
			ID: "python", Language: "Python", File: "requirements.txt", Count: 6, Updates: 1, Vulns: 0,
			Code: "fastapi==0.115.6\nuvicorn[standard]==0.34.0\npydantic==2.10.4\nhttpx==0.28.1\npython-json-logger==3.2.1\n",
			Deps: []Dependency{
				{Name: "fastapi", Version: "0.115.6", Scope: "runtime", Status: "current"},
				{Name: "uvicorn", Version: "0.34.0", Scope: "runtime", Status: "current"},
				{Name: "pydantic", Version: "2.10.4", Scope: "runtime", Status: "update", Note: "2.10.6"},
				{Name: "httpx", Version: "0.28.1", Scope: "runtime", Status: "current"},
			},
		},
		{
			ID: "rust", Language: "Rust", File: "Cargo.toml", Count: 5, Updates: 0, Vulns: 0,
			Code: "[package]\nname = \"orders\"\nversion = \"0.1.0\"\nedition = \"2024\"\n\n[dependencies]\ntokio = { version = \"1\", features = [\"full\"] }\nserde = { version = \"1\", features = [\"derive\"] }\naxum = \"0.8\"\nserde_json = \"1\"\n",
			Deps: []Dependency{
				{Name: "tokio", Version: "1.43.0", Scope: "runtime", Status: "current"},
				{Name: "serde", Version: "1.0.217", Scope: "runtime", Status: "current"},
				{Name: "axum", Version: "0.8.1", Scope: "runtime", Status: "current"},
				{Name: "serde_json", Version: "1.0.138", Scope: "runtime", Status: "current"},
			},
		},
		{
			ID: "lua", Language: "Lua", File: "orders-0.1.0-1.rockspec", Count: 3, Updates: 0, Vulns: 0,
			Code: "package = \"orders\"\nversion = \"0.1.0-1\"\nsource = { url = \"git+https://github.com/acme/orders\" }\ndescription = { summary = \"Orders function\" }\ndependencies = {\n  \"lua >= 5.4\",\n  \"lua-cjson >= 2.1.0\",\n  \"luasocket >= 3.1.0\",\n}\nbuild = { type = \"builtin\", modules = { orders = \"orders.lua\" } }\n",
			Deps: []Dependency{
				{Name: "lua", Version: ">= 5.4", Scope: "runtime", Status: "current"},
				{Name: "lua-cjson", Version: ">= 2.1.0", Scope: "runtime", Status: "current"},
				{Name: "luasocket", Version: ">= 3.1.0", Scope: "runtime", Status: "current"},
			},
		},
	}
}

func agentExampleCode() string {
	return `// generated: functions/orders/create.go
package orders

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/acme/orders/internal/store"
)

type CreateOrderInput struct {
	CustomerID string  ` + "`json:\"customer_id\"`" + `
	Items      []Item  ` + "`json:\"items\"`" + `
	Currency   string  ` + "`json:\"currency\"`" + `
}

func Create(ctx context.Context, req CreateOrderInput) (Order, error) {
	if err := req.Validate(); err != nil {
		return Order{}, err
	}
	order, err := store.InsertOrder(ctx, req)
	if err != nil {
		return Order{}, err
	}
	return order, store.PublishOrderCreated(ctx, order)
}
`
}

func dataModelSQL() string {
	return `-- schema: orders · dialect: postgresql 16
CREATE TYPE order_status AS ENUM (
  'pending', 'paid', 'shipped', 'cancelled'
);

CREATE TABLE customers (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email       TEXT NOT NULL UNIQUE,
  name        TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE orders (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID NOT NULL REFERENCES customers(id),
  status      order_status NOT NULL DEFAULT 'pending',
  currency    CHAR(3) NOT NULL DEFAULT 'USD',
  total_cents BIGINT NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_customer ON orders(customer_id, created_at DESC);

-- name: CreateOrder :one
INSERT INTO orders (customer_id, currency, total_cents)
VALUES ($1, $2, $3)
RETURNING *;
`
}

func dataModelGenerated() string {
	return `// Code generated by sqlc. DO NOT EDIT.
package db

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type Order struct {
	ID         pgtype.UUID
	CustomerID pgtype.UUID
	Status     OrderStatus
	Currency   string
	TotalCents int64
	CreatedAt  pgtype.Timestamptz
}

type CreateOrderParams struct {
	CustomerID pgtype.UUID
	Currency   string
	TotalCents int64
}
`
}

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

type CodeSample struct {
	Sync  string `json:"sync"`
	Async string `json:"async"`
}
