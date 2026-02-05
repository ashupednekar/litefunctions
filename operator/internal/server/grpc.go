package server

import (
	"context"
	"time"

	"github.com/ashupednekar/litefunctions/operator/internal/client"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FunctionServer struct {
	UnimplementedFunctionServiceServer
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

func (s *FunctionServer) CreateFunction(ctx context.Context, req *CreateFunctionRequest) (*CreateFunctionResponse, error) {
	if req.Namespace == "" || req.Name == "" || req.Project == "" || req.Language == "" {
		return nil, status.Error(codes.InvalidArgument, "namespace, name, project, and language are required")
	}

	created, err := s.Client.CreateFunctionIfNotExists(ctx, req.Namespace, req.Name, req.Project, req.Language, req.GitCreds)
	if err != nil {
		s.Log.Error(err, "Failed to create function", "namespace", req.Namespace, "name", req.Name)
		return nil, status.Error(codes.Internal, "Failed to create function: "+err.Error())
	}

	return &CreateFunctionResponse{
		Created: created,
	}, nil
}

func (s *FunctionServer) Activate(ctx context.Context, req *ActivateRequest) (*ActivateResponse, error) {
	if req.Namespace == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "namespace and name are required")
	}

	keepWarm := s.KeepWarmDuration
	if keepWarm <= 0 {
		keepWarm = 5 * time.Minute
	}
	lang, err := s.Client.MarkFunctionActive(ctx, req.Namespace, req.Name, keepWarm)
	if err != nil {
		s.Log.Error(err, "Failed to mark function as active", "namespace", req.Namespace, "name", req.Name)
		return nil, status.Error(codes.Internal, "Failed to activate function: "+err.Error())
	}

	return &ActivateResponse{
		IsActive: true,
		Language: lang,
	}, nil
}

func (s *FunctionServer) GetStatus(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
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

	return &StatusResponse{
		IsActive: isActive,
	}, nil
}
