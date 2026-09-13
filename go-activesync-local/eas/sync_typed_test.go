package eas

import (
	"errors"
	"testing"

	"github.com/remdev/go-activesync/wbxml"
)

// SPEC: MS-ASCMD/sync.typed
func TestNewTypedSyncResponse_NilInput(t *testing.T) {
	if _, err := NewTypedSyncResponse[Email](nil); !errors.Is(err, ErrNilSyncResponse) {
		t.Fatalf("nil resp: want ErrNilSyncResponse, got %v", err)
	}
}

// SPEC: MS-ASCMD/sync.typed
func TestNewTypedSyncResponse_EmptyAndDeletes(t *testing.T) {
	resp := &SyncResponse{
		Status: int32(SyncStatusSuccess),
		Collections: SyncCollections{Collection: []SyncCollection{{
			SyncKey:      "S-1",
			CollectionID: "1",
			Class:        "Email",
			Status:       int32(SyncStatusSuccess),
			Commands: &SyncCommands{
				Add:    []SyncAdd{{ServerID: "1", ApplicationData: nil}},
				Change: []SyncChange{{ServerID: "1"}},
				Delete: []SyncDelete{{ServerID: "del-1"}, {ServerID: "del-2"}},
			},
		}}},
	}
	tr, err := NewTypedSyncResponse[Email](resp)
	if err != nil {
		t.Fatalf("NewTypedSyncResponse: %v", err)
	}
	if tr.Status != int32(SyncStatusSuccess) {
		t.Fatalf("Status: %d", tr.Status)
	}
	if len(tr.Collections) != 1 {
		t.Fatalf("collections: %d", len(tr.Collections))
	}
	col := tr.Collections[0]
	if len(col.Add) != 1 || col.Add[0].ApplicationData != nil {
		t.Fatalf("Add: %+v", col.Add)
	}
	if len(col.Change) != 1 || col.Change[0].ApplicationData != nil {
		t.Fatalf("Change: %+v", col.Change)
	}
	if len(col.Delete) != 2 || col.Delete[0] != "del-1" || col.Delete[1] != "del-2" {
		t.Fatalf("Delete: %+v", col.Delete)
	}
}

// SPEC: MS-ASCMD/sync.typed
func TestNewTypedSyncResponse_NilCommands(t *testing.T) {
	resp := &SyncResponse{
		Collections: SyncCollections{Collection: []SyncCollection{{SyncKey: "S-1"}}},
	}
	tr, err := NewTypedSyncResponse[Email](resp)
	if err != nil {
		t.Fatalf("NewTypedSyncResponse: %v", err)
	}
	if len(tr.Collections) != 1 {
		t.Fatalf("collections: %d", len(tr.Collections))
	}
	if tr.Collections[0].Add != nil || tr.Collections[0].Change != nil || tr.Collections[0].Delete != nil {
		t.Fatalf("expected nil command slices for empty Commands")
	}
}

// SPEC: MS-ASCMD/sync.typed
func TestNewTypedSyncResponse_AddDecodeError(t *testing.T) {
	bad := &wbxml.RawElement{Page: 0xFF, Bytes: []byte{0x01}}
	resp := &SyncResponse{
		Collections: SyncCollections{Collection: []SyncCollection{{
			Commands: &SyncCommands{Add: []SyncAdd{{ApplicationData: bad}}},
		}}},
	}
	if _, err := NewTypedSyncResponse[Email](resp); err == nil {
		t.Fatal("expected decode error for bad Add ApplicationData")
	}
}

// SPEC: MS-ASCMD/sync.typed
func TestNewTypedSyncResponse_ChangeDecodeError(t *testing.T) {
	bad := &wbxml.RawElement{Page: 0xFF, Bytes: []byte{0x01}}
	resp := &SyncResponse{
		Collections: SyncCollections{Collection: []SyncCollection{{
			Commands: &SyncCommands{Change: []SyncChange{{ApplicationData: bad}}},
		}}},
	}
	if _, err := NewTypedSyncResponse[Email](resp); err == nil {
		t.Fatal("expected decode error for bad Change ApplicationData")
	}
}

// SPEC: MS-ASCMD/sync.typed
func TestNewTypedSyncResponse_PopulatedAddChange(t *testing.T) {
	body := captureApplicationDataBody(t, &Email{Subject: "ok"})
	resp := &SyncResponse{
		Collections: SyncCollections{Collection: []SyncCollection{{
			Commands: &SyncCommands{
				Add: []SyncAdd{{
					ServerID:        "a-1",
					ClientID:        "c-1",
					ApplicationData: &wbxml.RawElement{Page: wbxml.PageAirSync, Bytes: body},
				}},
				Change: []SyncChange{{
					ServerID:        "a-1",
					ApplicationData: &wbxml.RawElement{Page: wbxml.PageAirSync, Bytes: body},
				}},
			},
		}}},
	}
	tr, err := NewTypedSyncResponse[Email](resp)
	if err != nil {
		t.Fatalf("NewTypedSyncResponse: %v", err)
	}
	col := tr.Collections[0]
	if col.Add[0].ApplicationData == nil || col.Add[0].ApplicationData.Subject != "ok" {
		t.Fatalf("Add ApplicationData mismatch: %+v", col.Add[0].ApplicationData)
	}
	if col.Add[0].ClientID != "c-1" {
		t.Fatalf("Add ClientID: %q", col.Add[0].ClientID)
	}
	if col.Change[0].ApplicationData == nil || col.Change[0].ApplicationData.Subject != "ok" {
		t.Fatalf("Change ApplicationData mismatch: %+v", col.Change[0].ApplicationData)
	}
}
