package client

import (
	"context"

	"github.com/remdev/go-activesync/eas"
)

// MoveItems moves one or more items between folders on the server and returns
// the server response. SrcFldID is each item's current folder, DstFldID is the
// target folder. EAS MoveItems is a true move (not a copy); there is no
// server-side copy command in ActiveSync.
func (c *Client) MoveItems(ctx context.Context, user string, moves []eas.MoveItemEntry) (*eas.MoveItemsResponse, error) {
	req := eas.MoveItemsRequest{Moves: moves}
	var resp eas.MoveItemsResponse
	if err := c.do(ctx, CmdMoveItems, user, &req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// FolderCreate creates a new folder of the given type under parentID ("0" for
// the mailbox root; type 12 = user-created mail folder) and returns the new
// ServerId.
func (c *Client) FolderCreate(ctx context.Context, user, parentID, displayName string, folderType int32) (*eas.FolderCreateResponse, error) {
	req := eas.FolderCreateRequest{
		ParentID:    parentID,
		DisplayName: displayName,
		Type:        folderType,
	}
	var resp eas.FolderCreateResponse
	if err := c.do(ctx, CmdFolderCreate, user, &req, &resp); err != nil {
		return nil, err
	}
	if resp.Status != int32(eas.StatusSuccess) {
		return &resp, &StatusError{Command: "FolderCreate", Status: resp.Status}
	}
	return &resp, nil
}

// FolderDelete deletes a folder (and its contents) by ServerID.
func (c *Client) FolderDelete(ctx context.Context, user, serverID string) (*eas.FolderDeleteResponse, error) {
	req := eas.FolderDeleteRequest{ServerID: serverID}
	var resp eas.FolderDeleteResponse
	if err := c.do(ctx, CmdFolderDelete, user, &req, &resp); err != nil {
		return nil, err
	}
	if resp.Status != int32(eas.StatusSuccess) {
		return &resp, &StatusError{Command: "FolderDelete", Status: resp.Status}
	}
	return &resp, nil
}
