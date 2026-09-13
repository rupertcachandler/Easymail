package eas

import "errors"

// ErrNilSyncResponse is returned by NewTypedSyncResponse / SyncTyped when the
// underlying *SyncResponse is nil. It allows callers to distinguish "the
// transport handed us nothing" from a wbxml decoding error.
var ErrNilSyncResponse = errors.New("eas: nil *SyncResponse")

// TypedSyncResponse mirrors SyncResponse but exposes typed ApplicationData
// values for callers that operate on a single payload class per Sync.
//
// SPEC: MS-ASCMD/sync.typed
type TypedSyncResponse[T any] struct {
	Status      int32
	Collections []TypedSyncCollection[T]
}

// TypedSyncCollection mirrors SyncCollection with typed Add/Change items.
//
// SPEC: MS-ASCMD/sync.typed
type TypedSyncCollection[T any] struct {
	SyncKey       string
	CollectionID  string
	Class         string
	Status        int32
	MoreAvailable int32
	Add           []TypedItem[T]
	Change        []TypedItem[T]
	Delete        []string // ServerId values from Delete commands
}

// TypedItem is a SyncAdd or SyncChange whose ApplicationData has been
// decoded into a typed value of T. ApplicationData is nil when the wire
// element was empty or absent.
//
// SPEC: MS-ASCMD/sync.typed
type TypedItem[T any] struct {
	ServerID        string
	ClientID        string
	ApplicationData *T
}

// NewTypedSyncResponse projects a SyncResponse into a TypedSyncResponse[T] by
// decoding every SyncAdd/SyncChange ApplicationData via
// UnmarshalApplicationData. Empty ApplicationData fields are preserved as
// nil ApplicationData on the TypedItem; any other decode error fails the
// projection.
//
// SPEC: MS-ASCMD/sync.typed
func NewTypedSyncResponse[T any](resp *SyncResponse) (*TypedSyncResponse[T], error) {
	if resp == nil {
		return nil, ErrNilSyncResponse
	}
	out := &TypedSyncResponse[T]{Status: resp.Status}
	for _, col := range resp.Collections.Collection {
		tcol := TypedSyncCollection[T]{
			SyncKey:       col.SyncKey,
			CollectionID:  col.CollectionID,
			Class:         col.Class,
			Status:        col.Status,
			MoreAvailable: col.MoreAvailable,
		}
		if col.Commands != nil {
			adds, err := convertAdds[T](col.Commands.Add)
			if err != nil {
				return nil, err
			}
			tcol.Add = adds
			changes, err := convertChanges[T](col.Commands.Change)
			if err != nil {
				return nil, err
			}
			tcol.Change = changes
			tcol.Delete = make([]string, 0, len(col.Commands.Delete))
			for _, d := range col.Commands.Delete {
				tcol.Delete = append(tcol.Delete, d.ServerID)
			}
		}
		out.Collections = append(out.Collections, tcol)
	}
	return out, nil
}

func convertAdds[T any](src []SyncAdd) ([]TypedItem[T], error) {
	if len(src) == 0 {
		return nil, nil
	}
	out := make([]TypedItem[T], 0, len(src))
	for _, a := range src {
		item := TypedItem[T]{ServerID: a.ServerID, ClientID: a.ClientID}
		if a.ApplicationData != nil && len(a.ApplicationData.Bytes) > 0 {
			v, err := UnmarshalApplicationData[T](a.ApplicationData)
			if err != nil {
				return nil, err
			}
			item.ApplicationData = v
		}
		out = append(out, item)
	}
	return out, nil
}

func convertChanges[T any](src []SyncChange) ([]TypedItem[T], error) {
	if len(src) == 0 {
		return nil, nil
	}
	out := make([]TypedItem[T], 0, len(src))
	for _, c := range src {
		item := TypedItem[T]{ServerID: c.ServerID}
		if c.ApplicationData != nil && len(c.ApplicationData.Bytes) > 0 {
			v, err := UnmarshalApplicationData[T](c.ApplicationData)
			if err != nil {
				return nil, err
			}
			item.ApplicationData = v
		}
		out = append(out, item)
	}
	return out, nil
}
