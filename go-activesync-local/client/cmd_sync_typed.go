package client

import (
	"context"

	"github.com/remdev/go-activesync/eas"
)

// SyncTyped is a generic wrapper around Client.Sync that decodes every
// SyncAdd / SyncChange ApplicationData into the requested type T. It is
// intended for the common case of a Sync request that targets a single
// collection of a single Class (Email, Calendar, Contacts, Tasks).
//
// SyncTyped does not surface the raw eas.SyncResponse; callers that need
// access to the wire-level structure (e.g. to introspect Class / Status per
// collection while still picking the right T per collection at runtime)
// should call (*Client).Sync directly and project the response with
// eas.NewTypedSyncResponse[T] themselves.
//
// SPEC: MS-ASCMD/sync.typed
func SyncTyped[T any](ctx context.Context, c *Client, user string, req *eas.SyncRequest) (*eas.TypedSyncResponse[T], error) {
	resp, err := c.Sync(ctx, user, req)
	if err != nil {
		return nil, err
	}
	return eas.NewTypedSyncResponse[T](resp)
}
