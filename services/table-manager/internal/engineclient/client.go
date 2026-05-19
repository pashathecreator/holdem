package engineclient

import (
	"fmt"

	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn   *grpc.ClientConn
	client enginev1.GameEngineClient
}

func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial engine grpc: %w", err)
	}

	return &Client{
		conn:   conn,
		client: enginev1.NewGameEngineClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) GameEngine() enginev1.GameEngineClient {
	return c.client
}
