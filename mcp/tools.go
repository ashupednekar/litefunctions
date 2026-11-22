package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/ashupednekar/litefunctions/common"
	"github.com/ashupednekar/litefunctions/common/sample"
	vcs "github.com/ashupednekar/litefunctions/common/vcs/pkg"
)

type tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     func(ctx context.Context, raw json.RawMessage) (string, error)
}

var toolOrder = []string{
	"list_projects",
	"create_project",
	"select_project",
	"list_project_files",
	"read_project_file",
	"write_project_file",
	"list_functions",
	"create_function",
	"update_function",
	"delete_function",
	"create_endpoint",
}

var functionNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$`)

func (s *Server) buildTools() map[string]tool {
	tools := map[string]tool{
		"list_projects": {
			Name:        "list_projects",
			Description: "List LiteFunctions projects visible to the agent. Use this before creating a project or when the user has not chosen one.",
			InputSchema: objectSchema(map[string]any{}),
			Handler:     s.listProjects,
		},
		"create_project": {
			Name:        "create_project",
			Description: "Create a new LiteFunctions project. This provisions the Git repository, webhook, and build workflow. Use a short lowercase name.",
			InputSchema: objectSchema(map[string]any{
				"name": stringSchema("Project name (lowercase letters, numbers, dash, underscore)"),
			}, "name"),
			Handler: s.createProject,
		},
		"select_project": {
			Name:        "select_project",
			Description: "Select an existing project and return its id. Pass the returned project name to the other tools.",
			InputSchema: objectSchema(map[string]any{
				"name": stringSchema("Project name"),
			}, "name"),
			Handler: s.getProject,
		},
		"list_project_files": {
			Name:        "list_project_files",
			Description: "List files in a LiteFunctions project repository. Use this before creating functions.",
			InputSchema: objectSchema(map[string]any{
				"project": stringSchema("Project name"),
				"path":    stringSchema("Optional subdirectory. Defaults to the repo root."),
			}, "project"),
			Handler: s.listProjectFiles,
		},
		"read_project_file": {
			Name:        "read_project_file",
			Description: "Read one text file from a project repository.",
			InputSchema: objectSchema(map[string]any{
				"project": stringSchema("Project name"),
				"path":    stringSchema("Repo-relative file path"),
			}, "project", "path"),
			Handler: s.readProjectFile,
		},
		"write_project_file": {
			Name:        "write_project_file",
			Description: "Write and push an arbitrary text file in a project repository.",
			InputSchema: objectSchema(map[string]any{
				"project":        stringSchema("Project name"),
				"path":           stringSchema("Repo-relative file path"),
				"content":        stringSchema("File content"),
				"commit_message": stringSchema("Optional commit message"),
			}, "project", "path", "content"),
			Handler: s.writeProjectFile,
		},
		"list_functions": {
			Name:        "list_functions",
			Description: "List functions in a project, including async flags and endpoints from the LiteFunctions manifest.",
			InputSchema: objectSchema(map[string]any{
				"project": stringSchema("Project name"),
			}, "project"),
			Handler: s.listFunctions,
		},
		"create_function": {
			Name:        "create_function",
			Description: "Create a function package in the project repo and register its Function CR. Use surface=http for sync HTTP, async for a NATS consumer, ws for WebSocket, or sse for server-sent events.",
			InputSchema: objectSchema(map[string]any{
				"project":   stringSchema("Project name"),
				"name":      stringSchema("Function name (letters, numbers, dash, underscore)"),
				"language":  enumSchema("Runtime language", "go", "python", "ts", "rust", "lua"),
				"surface":   enumSchema("Execution surface. http is sync HTTP; async is a NATS consumer; ws and sse are streaming functions.", "http", "async", "ws", "sse"),
				"is_async":  boolSchema("Explicit async flag. Overrides surface when set."),
				"ui":        boolSchema("True when this function serves server-rendered HTML. The portal previews it in an iframe."),
				"code":      stringSchema("Full entrypoint source. Defaults to the language sample."),
				"files":     filesSchema(),
				"endpoints": endpointsSchema(),
			}, "project", "name", "language"),
			Handler: s.createFunction,
		},
		"update_function": {
			Name:        "update_function",
			Description: "Update a function entrypoint, extra files, async flag, or endpoints. The function must already exist in the repo.",
			InputSchema: objectSchema(map[string]any{
				"project":   stringSchema("Project name"),
				"name":      stringSchema("Function name"),
				"language":  enumSchema("Runtime language", "go", "python", "ts", "rust", "lua"),
				"surface":   enumSchema("Execution surface", "http", "async", "ws", "sse"),
				"is_async":  boolSchema("Explicit async flag"),
				"ui":        boolSchema("True when this function serves server-rendered HTML"),
				"code":      stringSchema("Replacement entrypoint source"),
				"files":     filesSchema(),
				"endpoints": endpointsSchema(),
			}, "project", "name", "language"),
			Handler: s.updateFunction,
		},
		"delete_function": {
			Name:        "delete_function",
			Description: "Delete a function package and its manifest entry from the project repo.",
			InputSchema: objectSchema(map[string]any{
				"project":  stringSchema("Project name"),
				"name":     stringSchema("Function name"),
				"language": enumSchema("Runtime language", "go", "python", "ts", "rust", "lua"),
			}, "project", "name", "language"),
			Handler: s.deleteFunction,
		},
		"create_endpoint": {
			Name:        "create_endpoint",
			Description: "Add or replace an endpoint entry for an existing function. The portal creates the route during its next repo sync.",
			InputSchema: objectSchema(map[string]any{
				"project":  stringSchema("Project name"),
				"name":     stringSchema("Function name"),
				"language": enumSchema("Runtime language", "go", "python", "ts", "rust", "lua"),
				"endpoint": stringSchema("Optional endpoint path. Defaults to /<project>/<function>."),
				"method":   stringSchema("HTTP method or WS/SSE. Defaults to POST for sync and GET for async."),
				"scope":    enumSchema("Endpoint scope", "public", "authn"),
			}, "project", "name", "language"),
			Handler: s.createEndpoint,
		},
	}
	return tools
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func boolSchema(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func enumSchema(description string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

func filesSchema() map[string]any {
	return map[string]any{
		"type":        "array",
		"description": "Extra files to add under the function directory.",
		"items": objectSchema(map[string]any{
			"path":    stringSchema("Path relative to the function directory"),
			"content": stringSchema("File content"),
		}, "path", "content"),
	}
}

func endpointsSchema() map[string]any {
	return map[string]any{
		"type":        "array",
		"description": "Endpoints to create for this function.",
		"items": objectSchema(map[string]any{
			"name":   stringSchema("Endpoint path"),
			"method": stringSchema("HTTP method, WS, or SSE"),
			"scope":  enumSchema("Endpoint scope", "public", "authn"),
		}),
	}
}

type fileArg struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type endpointArg struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Scope  string `json:"scope"`
}

type functionArgs struct {
	Project   string        `json:"project"`
	Name      string        `json:"name"`
	Language  string        `json:"language"`
	Surface   string        `json:"surface"`
	IsAsync   *bool         `json:"is_async"`
	UI        *bool         `json:"ui"`
	Code      string        `json:"code"`
	Files     []fileArg     `json:"files"`
	Endpoints []endpointArg `json:"endpoints"`
}

func (s *Server) listProjectFiles(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Project string `json:"project"`
		Path    string `json:"path"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if err := validateProject(args.Project); err != nil {
		return "", err
	}
	root := strings.TrimSpace(args.Path)
	if root != "" {
		clean, err := cleanRepoPath(root)
		if err != nil {
			return "", err
		}
		root = clean
	}

	s.repoMu.Lock()
	defer s.repoMu.Unlock()
	r, err := vcs.NewRepo(s.vcs, args.Project, nil)
	if err != nil {
		return "", err
	}
	files, err := listRepoFiles(r, root)
	if err != nil {
		return "", err
	}
	return marshalResult(map[string]any{"project": args.Project, "path": root, "files": files})
}

func (s *Server) readProjectFile(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Project string `json:"project"`
		Path    string `json:"path"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if err := validateProject(args.Project); err != nil {
		return "", err
	}
	name, err := cleanRepoPath(args.Path)
	if err != nil {
		return "", err
	}

	s.repoMu.Lock()
	defer s.repoMu.Unlock()
	r, err := vcs.NewRepo(s.vcs, args.Project, nil)
	if err != nil {
		return "", err
	}
	body, err := readRepoFile(r, name)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (s *Server) writeProjectFile(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Project       string `json:"project"`
		Path          string `json:"path"`
		Content       string `json:"content"`
		CommitMessage string `json:"commit_message"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if err := validateProject(args.Project); err != nil {
		return "", err
	}
	name, err := cleanRepoPath(args.Path)
	if err != nil {
		return "", err
	}
	if name == common.FunctionsManifestPath {
		return "", fmt.Errorf("%s is managed by create_function/update_function", common.FunctionsManifestPath)
	}
	message := strings.TrimSpace(args.CommitMessage)
	if message == "" {
		message = "agent: update " + name
	}

	s.repoMu.Lock()
	defer s.repoMu.Unlock()
	r, err := vcs.NewRepo(s.vcs, args.Project, nil)
	if err != nil {
		return "", err
	}
	if err := writeRepoFile(r, name, []byte(args.Content)); err != nil {
		return "", err
	}
	if err := commitPush(r, message, name); err != nil {
		return "", err
	}
	return marshalResult(map[string]any{"project": args.Project, "path": name, "status": "saved"})
}

func (s *Server) listFunctions(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Project string `json:"project"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if err := validateProject(args.Project); err != nil {
		return "", err
	}

	s.repoMu.Lock()
	defer s.repoMu.Unlock()
	r, err := vcs.NewRepo(s.vcs, args.Project, nil)
	if err != nil {
		return "", err
	}
	manifest, err := loadManifest(r)
	if err != nil {
		return "", err
	}
	files, _ := listRepoFiles(r, "functions")
	return marshalResult(map[string]any{
		"project":   args.Project,
		"functions": manifest.Functions,
		"files":     files,
	})
}

func (s *Server) createFunction(ctx context.Context, raw json.RawMessage) (string, error) {
	var args functionArgs
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	return s.upsertFunction(ctx, args, true)
}

func (s *Server) updateFunction(ctx context.Context, raw json.RawMessage) (string, error) {
	var args functionArgs
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	return s.upsertFunction(ctx, args, false)
}

func (s *Server) upsertFunction(ctx context.Context, args functionArgs, create bool) (string, error) {
	if err := validateProject(args.Project); err != nil {
		return "", err
	}
	language, err := normalizeLanguage(args.Language)
	if err != nil {
		return "", err
	}
	if err := validateFunctionName(args.Name); err != nil {
		return "", err
	}
	ext := common.LangExt[language]
	if ext == "" {
		return "", fmt.Errorf("unsupported language %q", language)
	}

	s.repoMu.Lock()
	defer s.repoMu.Unlock()

	r, err := vcs.NewRepo(s.vcs, args.Project, nil)
	if err != nil {
		return "", err
	}

	manifest, err := loadManifest(r)
	if err != nil {
		return "", err
	}

	existing, inManifest := manifest.Find(language, args.Name)
	isAsync := surfaceIsAsync(args.Surface)
	if args.IsAsync != nil {
		isAsync = *args.IsAsync
	} else if !create && inManifest && strings.TrimSpace(args.Surface) == "" {
		isAsync = existing.IsAsync
	}

	isUI := false
	if args.UI != nil {
		isUI = *args.UI
	} else if !create && inManifest {
		isUI = existing.UI
	}

	functionDir := fmt.Sprintf("functions/%s/%s", language, args.Name)
	entrypoint := "handle" + ext
	if inManifest && existing.Path != "" {
		functionDir = path.Dir(existing.Path)
		entrypoint = path.Base(existing.Path)
	}
	entrypointPath := path.Join(functionDir, entrypoint)

	_, statErr := r.Filesystem().Stat(entrypointPath)
	exists := statErr == nil
	if create && exists {
		return "", fmt.Errorf("function %s/%s already exists; use update_function", language, args.Name)
	}
	if !create && !exists {
		return "", fmt.Errorf("function %s/%s not found; use create_function", language, args.Name)
	}

	code := args.Code
	if strings.TrimSpace(code) == "" && create {
		pkg, err := sample.LoadPackage(language, args.Name, isAsync)
		if err != nil {
			return "", err
		}
		for _, file := range pkg.Files {
			content := string(file.Content)
			if file.Path == pkg.Entrypoint {
				continue
			}
			if err := writeRepoFile(r, path.Join(functionDir, file.Path), []byte(content)); err != nil {
				return "", err
			}
		}
		for _, file := range pkg.Files {
			if file.Path == pkg.Entrypoint {
				code = string(file.Content)
				break
			}
		}
	}

	if strings.TrimSpace(code) != "" {
		code = strings.ReplaceAll(code, "{{name}}", args.Name)
		if err := writeRepoFile(r, entrypointPath, []byte(code)); err != nil {
			return "", err
		}
	}

	for _, file := range args.Files {
		clean, err := cleanRepoPath(file.Path)
		if err != nil {
			return "", err
		}
		if err := writeRepoFile(r, path.Join(functionDir, clean), []byte(file.Content)); err != nil {
			return "", err
		}
	}

	endpoints := args.Endpoints
	defaulted := len(endpoints) == 0
	if defaulted {
		endpoints = defaultEndpoints(args.Project, args.Name, args.Surface, isAsync)
	}
	for i := range endpoints {
		if isUI && (defaulted || strings.TrimSpace(endpoints[i].Method) == "") {
			endpoints[i].Method = http.MethodGet
		}
		normalizeEndpoint(&endpoints[i], args.Project, args.Name, isAsync)
	}

	manifest.Upsert(common.ManifestFunction{
		Name:      args.Name,
		Language:  language,
		Path:      entrypointPath,
		IsAsync:   isAsync,
		UI:        isUI,
		Endpoints: toManifestEndpoints(endpoints),
	})
	if err := saveManifest(r, manifest); err != nil {
		return "", err
	}

	if err := commitPush(r, "agent: "+args.Name+" "+language, functionDir, common.FunctionsManifestPath); err != nil {
		return "", err
	}

	warning := ""
	if _, err := common.CreateFunctionResource(ctx, s.settings.OperatorAddr, s.settings.Namespace, args.Name, args.Project, language, isAsync); err != nil {
		warning = " (function saved, but operator registration failed: " + err.Error() + ")"
	}

	return marshalResult(map[string]any{
		"project":   args.Project,
		"name":      args.Name,
		"language":  language,
		"path":      entrypointPath,
		"is_async":  isAsync,
		"ui":        isUI,
		"endpoints": endpoints,
		"status":    "saved" + warning,
	})
}

func (s *Server) deleteFunction(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Project  string `json:"project"`
		Name     string `json:"name"`
		Language string `json:"language"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if err := validateProject(args.Project); err != nil {
		return "", err
	}
	language, err := normalizeLanguage(args.Language)
	if err != nil {
		return "", err
	}
	if err := validateFunctionName(args.Name); err != nil {
		return "", err
	}

	s.repoMu.Lock()
	defer s.repoMu.Unlock()
	r, err := vcs.NewRepo(s.vcs, args.Project, nil)
	if err != nil {
		return "", err
	}
	manifest, err := loadManifest(r)
	if err != nil {
		return "", err
	}
	functionDir := fmt.Sprintf("functions/%s/%s", language, args.Name)
	if existing, ok := manifest.Find(language, args.Name); ok && existing.Path != "" {
		functionDir = path.Dir(existing.Path)
	}
	if _, err := r.Filesystem().Stat(functionDir); err != nil {
		return "", fmt.Errorf("function %s/%s not found", language, args.Name)
	}
	if err := removeRepoPath(r, functionDir); err != nil {
		return "", err
	}
	manifest.Remove(language, args.Name)
	if err := saveManifest(r, manifest); err != nil {
		return "", err
	}
	if err := commitPush(r, "agent: delete "+args.Name+" "+language, functionDir, common.FunctionsManifestPath); err != nil {
		return "", err
	}
	return marshalResult(map[string]any{"project": args.Project, "name": args.Name, "language": language, "status": "deleted"})
}

func (s *Server) createEndpoint(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Project  string `json:"project"`
		Name     string `json:"name"`
		Language string `json:"language"`
		Endpoint string `json:"endpoint"`
		Method   string `json:"method"`
		Scope    string `json:"scope"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if err := validateProject(args.Project); err != nil {
		return "", err
	}
	language, err := normalizeLanguage(args.Language)
	if err != nil {
		return "", err
	}
	if err := validateFunctionName(args.Name); err != nil {
		return "", err
	}

	s.repoMu.Lock()
	defer s.repoMu.Unlock()
	r, err := vcs.NewRepo(s.vcs, args.Project, nil)
	if err != nil {
		return "", err
	}
	manifest, err := loadManifest(r)
	if err != nil {
		return "", err
	}
	fn, ok := manifest.Find(language, args.Name)
	if !ok {
		return "", fmt.Errorf("function %s/%s is not in the manifest; create it first", language, args.Name)
	}
	endpoint := endpointArg{Name: args.Endpoint, Method: args.Method, Scope: args.Scope}
	normalizeEndpoint(&endpoint, args.Project, args.Name, fn.IsAsync)
	fn.Endpoints = upsertEndpoint(fn.Endpoints, endpoint)
	manifest.Upsert(fn)
	if err := saveManifest(r, manifest); err != nil {
		return "", err
	}
	if err := commitPush(r, "agent: endpoint "+args.Name, common.FunctionsManifestPath); err != nil {
		return "", err
	}
	return marshalResult(map[string]any{"project": args.Project, "function": args.Name, "endpoint": endpoint, "status": "saved"})
}

func surfaceIsAsync(surface string) bool {
	switch strings.ToLower(strings.TrimSpace(surface)) {
	case "async", "ws", "sse", "websocket", "stream":
		return true
	default:
		return false
	}
}

func defaultEndpoints(project, name, surface string, isAsync bool) []endpointArg {
	switch strings.ToLower(strings.TrimSpace(surface)) {
	case "ws", "websocket":
		return []endpointArg{{Name: "/" + project + "/ws/" + name, Method: "WS", Scope: "public"}}
	case "sse", "stream":
		return []endpointArg{{Name: "/" + project + "/sse/" + name, Method: "SSE", Scope: "public"}}
	}
	method := "POST"
	if isAsync {
		method = "GET"
	}
	return []endpointArg{{Name: "/" + project + "/" + name, Method: method, Scope: "public"}}
}

func normalizeEndpoint(endpoint *endpointArg, project, name string, isAsync bool) {
	if strings.TrimSpace(endpoint.Name) == "" {
		endpoint.Name = "/" + project + "/" + name
	}
	if !strings.HasPrefix(endpoint.Name, "/") {
		endpoint.Name = "/" + endpoint.Name
	}
	// Endpoint paths are project-scoped. Keep the ingestor's
	// /lambda/<project>/<name> route and the public gateway path aligned.
	prefix := "/" + project
	if project != "" && endpoint.Name != prefix && !strings.HasPrefix(endpoint.Name, prefix+"/") {
		endpoint.Name = prefix + "/" + strings.TrimPrefix(endpoint.Name, "/")
	}
	if strings.TrimSpace(endpoint.Method) == "" {
		endpoint.Method = "POST"
		if isAsync {
			endpoint.Method = "GET"
		}
	}
	endpoint.Method = strings.ToUpper(strings.TrimSpace(endpoint.Method))
	if strings.TrimSpace(endpoint.Scope) == "" {
		endpoint.Scope = "public"
	}
}

func toManifestEndpoints(endpoints []endpointArg) []common.ManifestEndpoint {
	out := make([]common.ManifestEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		out = append(out, common.ManifestEndpoint{Name: endpoint.Name, Method: endpoint.Method, Scope: endpoint.Scope})
	}
	return out
}

func upsertEndpoint(endpoints []common.ManifestEndpoint, endpoint endpointArg) []common.ManifestEndpoint {
	for i := range endpoints {
		if endpoints[i].Name == endpoint.Name && endpoints[i].Method == endpoint.Method {
			endpoints[i].Scope = endpoint.Scope
			return endpoints
		}
	}
	return append(endpoints, common.ManifestEndpoint{Name: endpoint.Name, Method: endpoint.Method, Scope: endpoint.Scope})
}

func normalizeLanguage(language string) (string, error) {
	lang := strings.ToLower(strings.TrimSpace(language))
	switch lang {
	case "golang":
		lang = "go"
	case "py":
		lang = "python"
	case "rs":
		lang = "rust"
	case "typescript":
		lang = "ts"
	}
	if common.LangExt[lang] == "" {
		return "", fmt.Errorf("unsupported language %q", language)
	}
	return lang, nil
}

func validateProject(project string) error {
	project = strings.TrimSpace(project)
	if project == "" {
		return fmt.Errorf("project is required")
	}
	if !functionNamePattern.MatchString(project) {
		return fmt.Errorf("invalid project name %q", project)
	}
	return nil
}

func validateFunctionName(name string) error {
	if !functionNamePattern.MatchString(strings.TrimSpace(name)) {
		return fmt.Errorf("invalid function name %q", name)
	}
	return nil
}

func decodeArgs(raw json.RawMessage, out any) error {
	if len(strings.TrimSpace(string(raw))) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

func marshalResult(value any) (string, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(body), nil
}
