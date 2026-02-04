package adaptors

import (
	"context"
	"fmt"
	"time"

	functionproto "github.com/ashupednekar/litefunctions/ingestor/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultGrpcTimeout = 5 * time.Second

func CreateFunctionCRD(ctx context.Context, operatorAddr, namespace, name, project, language, gitCreds string) (bool, error) {
	if operatorAddr == "" {
		return false, fmt.Errorf("operator address is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithTimeout(ctx, defaultGrpcTimeout)
	defer cancel()

	conn, err := grpc.NewClient(operatorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return false, fmt.Errorf("failed to create gRPC client: %w", err)
	}
	defer conn.Close()

	client := functionproto.NewFunctionServiceClient(conn)
	resp, err := client.CreateFunction(ctx, &functionproto.CreateFunctionRequest{
		Namespace: namespace,
		Name:      name,
		Project:   project,
		Language:  language,
		GitCreds:  gitCreds,
	})
	if err != nil {
		return false, err
	}
	return resp.Created, nil
}
