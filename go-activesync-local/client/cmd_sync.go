package client

import (
	"context"

	"github.com/remdev/go-activesync/eas"
)

// Sync performs a single Sync request and returns the server response.
// The caller owns the SyncKey lifecycle through SyncStateStore; this method
// only sends the request as-is.
func (c *Client) Sync(ctx context.Context, user string, req *eas.SyncRequest) (*eas.SyncResponse, error) {
	var resp eas.SyncResponse
	if err := c.do(ctx, CmdSync, user, req, &resp); err != nil {
		return nil, err
	}
	if resp.Status != 0 && resp.Status != eas.SyncStatusSuccess {
		return &resp, &StatusError{Command: "Sync", Status: resp.Status}
	}
	return &resp, nil
}
