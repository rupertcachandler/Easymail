package client

import (
	"context"

	"github.com/remdev/go-activesync/eas"
)

// ItemOperations performs a single ItemOperations Fetch request and returns
// the server response. This is how we download attachments (FileReference)
// and full untruncated email bodies from SOGo.
func (c *Client) ItemOperations(ctx context.Context, user string, req *eas.ItemOperationsRequest) (*eas.ItemOperationsResponse, error) {
	var resp eas.ItemOperationsResponse
	if err := c.do(ctx, CmdItemOperations, user, req, &resp); err != nil {
		return nil, err
	}
	if resp.Status != 0 && resp.Status != 1 {
		return &resp, &StatusError{Command: "ItemOperations", Status: resp.Status}
	}
	return &resp, nil
}
