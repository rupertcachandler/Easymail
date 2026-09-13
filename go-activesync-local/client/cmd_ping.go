package client

import (
	"context"

	"github.com/remdev/go-activesync/eas"
)

// Ping performs a single long-poll Ping request. The caller controls the
// total wait time via the context deadline.
func (c *Client) Ping(ctx context.Context, user string, req *eas.PingRequest) (*eas.PingResponse, error) {
	var resp eas.PingResponse
	if err := c.do(ctx, CmdPing, user, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
