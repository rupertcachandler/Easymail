package client

import (
	"context"

	"github.com/remdev/go-activesync/eas"
)

// FolderSync performs a single FolderSync request with the given SyncKey
// (use "0" for the initial sync) and returns the server response. Callers
// are responsible for persisting the new SyncKey.
func (c *Client) FolderSync(ctx context.Context, user, syncKey string) (*eas.FolderSyncResponse, error) {
	req := eas.NewFolderSyncRequest(syncKey)
	var resp eas.FolderSyncResponse
	if err := c.do(ctx, CmdFolderSync, user, &req, &resp); err != nil {
		return nil, err
	}
	if resp.Status != int32(eas.StatusSuccess) {
		return &resp, &StatusError{Command: "FolderSync", Status: resp.Status}
	}
	return &resp, nil
}
