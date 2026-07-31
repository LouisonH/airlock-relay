package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"
)

// Client connects to the owner-only local control endpoint. It sets the control
// protocol version and authentication token itself, so callers never need to
// include those values in an operation request.
type Client struct {
	socketPath string
	token      string
}

func NewClient(socketPath, token string) (*Client, error) {
	if !validControlEndpoint(socketPath) || len(token) < 32 || len(token) > 128 {
		return nil, errors.New("invalid control client configuration")
	}
	return &Client{socketPath: socketPath, token: token}, nil
}

func (c *Client) Do(ctx context.Context, request Request) (Response, error) {
	if c == nil {
		return Response{}, errors.New("control client is unavailable")
	}
	request.Version = protocolVersion
	request.Token = c.token
	exchangeTimeout := controlExchangeTimeout
	if request.Action == "test_route_health" {
		exchangeTimeout = controlHealthCheckTimeout
	}
	connection, err := dialControlEndpoint(ctx, c.socketPath)
	if err != nil {
		return Response{}, errors.New("connect to airlockd control endpoint")
	}
	defer connection.Close()
	deadline := time.Now().Add(exchangeTimeout)
	if requestedDeadline, ok := ctx.Deadline(); ok && requestedDeadline.Before(deadline) {
		deadline = requestedDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return Response{}, errors.New("protect control socket exchange")
	}
	if err := json.NewEncoder(connection).Encode(request); err != nil {
		return Response{}, errors.New("write control request")
	}
	request.Token = ""
	decoder := json.NewDecoder(io.LimitReader(connection, maxMessageBytes))
	var response Response
	if err := decoder.Decode(&response); err != nil {
		return Response{}, errors.New("read control response")
	}
	return response, nil
}
