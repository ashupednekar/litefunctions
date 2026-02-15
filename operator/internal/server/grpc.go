package server

import (
	"context"
	"time"

	functionproto "github.com/ashupednekar/litefunctions/common/proto"
	"github.com/ashupednekar/litefunctions/operator/internal/client"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FunctionServer struct {
	functionproto.UnimplementedFunctionServiceServer
	Client           *client.Client
	Log              logr.Logger
	KeepWarmDuration time.Duration
}

func NewFunctionServer(k8sClient *client.Client, log logr.Logger, keepWarmDuration time.Duration) *FunctionServer {
	return &FunctionServer{
		Client:           k8sClient,
		Log:              log,
		KeepWarmDuration: keepWarmDuration,
	}
}

func (s *FunctionServer) CreateFunction(ctx context.Context, req *functionproto.CreateFunctionRequest) (*functionproto.CreateFunctionResponse, error) {
	if req.Namespace == "" || req.Name == "" || req.Project == "" || req.Language == "" {
		return nil, status.Error(codes.InvalidArgument, "namespace, name, project, and language are required")
	}

	created, err := s.Client.CreateFunctionIfNotExists(ctx, req.Namespace, req.Name, req.Project, req.Language, req.GitCreds, req.IsAsync)
	if err != nil {
		s.Log.Error(err, "Failed to create function", "namespace", req.Namespace, "name", req.Name)
		return nil, status.Error(codes.Internal, "Failed to create function: "+err.Error())
	}

	return &functionproto.CreateFunctionResponse{
		Created: created,
	}, nil
}

func (s *FunctionServer) Activate(ctx context.Context, req *functionproto.ActivateRequest) (*functionproto.ActivateResponse, error) {
	if req.Namespace == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "namespace and name are required")
	}

	keepWarm := s.KeepWarmDuration
	if keepWarm <= 0 {
		keepWarm = 5 * time.Minute
	}
	fn, err := s.Client.MarkFunctionActive(ctx, req.Namespace, req.Name, keepWarm)
	if err != nil {
		s.Log.Error(err, "Failed to mark function as active", "namespace", req.Namespace, "name", req.Name)
		return nil, status.Error(codes.Internal, "Failed to activate function: "+err.Error())
	}

	resp := &functionproto.ActivateResponse{
		IsActive: true,
		Language: fn.Spec.Language,
		IsAsync:  fn.Spec.IsAsync,
		Project:  fn.Spec.Project,
		Name:     fn.Spec.Name,
		Method: fn.Spec.Method,
	}
	if supportsHTTP(fn.Spec.Language) {
		resp.ServiceName = client.GetServiceName(fn)
		resp.ServicePort = 8080
	}

	return resp, nil
}

func supportsHTTP(lang string) bool {
	switch lang {
	case "go", "rust", "rs", "python":
		return true
	default:
		return false
	}
}

func (s *FunctionServer) GetStatus(ctx context.Context, req *functionproto.StatusRequest) (*functionproto.StatusResponse, error) {
	if req.Namespace == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "namespace and name are required")
	}

	isActive, err := s.Client.IsFunctionActive(ctx, req.Namespace, req.Name)
	if err != nil {
		s.Log.Error(err, "Failed to check function active status", "namespace", req.Namespace, "name", req.Name)
		return nil, status.Error(codes.Internal, "Failed to check function status: "+err.Error())
	}

	if isActive {
		keepWarm := s.KeepWarmDuration
		if keepWarm <= 0 {
			keepWarm = 5 * time.Minute
		}
		if _, err := s.Client.ExtendFunctionLease(ctx, req.Namespace, req.Name, keepWarm); err != nil {
			s.Log.Error(err, "Failed to extend function lease", "namespace", req.Namespace, "name", req.Name)
			return nil, status.Error(codes.Internal, "Failed to extend function lease: "+err.Error())
		}
	}

	return &functionproto.StatusResponse{
		IsActive: isActive,
	}, nil
}
