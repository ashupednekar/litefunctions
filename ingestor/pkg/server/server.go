package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ashupednekar/litefunctions/ingestor/pkg"
	"github.com/ashupednekar/litefunctions/common/proto"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Server struct {
	port       int
	nc         *nats.Conn
	logger     *slog.Logger
	grpcClient proto.FunctionServiceClient
	grpcConn   *grpc.ClientConn
}

func NewServer(nc *nats.Conn) (*Server, error) {
	conn, err := grpc.NewClient(pkg.Settings.OperatorUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	client := proto.NewFunctionServiceClient(conn)

	return &Server{
		port:       pkg.Settings.ListenPort,
		nc:         nc,
		logger:     slog.Default(),
		grpcClient: client,
		grpcConn:   conn,
	}, nil
}

func (s *Server) Start() error {
	defer s.grpcConn.Close()
	s.BuildRoutes()
	slog.Info("ingestor server listening", "port", s.port)
	return http.ListenAndServe(fmt.Sprintf(":%d", s.port), nil)
}

func (s *Server) activateFunction(project, name string) (*proto.ActivateResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &proto.ActivateRequest{
		Namespace: "default",
		Name:      name,
	}

	resp, err := s.grpcClient.Activate(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to call operator gRPC: %w", err)
	}

	slog.Info("successfully activated function", "project", project, "name", name, "language", resp.Language)
	return resp, nil
}
