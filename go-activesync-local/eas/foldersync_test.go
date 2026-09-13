package eas

import (
	"reflect"
	"testing"

	"github.com/remdev/go-activesync/wbxml"
)

// SPEC: MS-ASCMD/foldersync.request
func TestFolderSyncRequest_Initial(t *testing.T) {
	req := NewFolderSyncRequest("0")
	if req.SyncKey != "0" {
		t.Errorf("initial SyncKey = %q, want \"0\"", req.SyncKey)
	}
	data, err := wbxml.Marshal(&req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out FolderSyncRequest
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = req.XMLName
	if !reflect.DeepEqual(req, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, req)
	}
}

// SPEC: MS-ASCMD/foldersync.response
func TestFolderSyncResponse_RoundTrip(t *testing.T) {
	in := FolderSyncResponse{
		Status:  1,
		SyncKey: "abcd",
		Changes: FolderChanges{
			Count: 2,
			Add: []FolderAdd{
				{ServerID: "1", ParentID: "0", DisplayName: "Inbox", Type: 2},
				{ServerID: "2", ParentID: "0", DisplayName: "Calendar", Type: 8},
			},
			Update: []FolderUpdate{{ServerID: "3", ParentID: "0", DisplayName: "Contacts", Type: 9}},
			Delete: []FolderDelete{{ServerID: "old"}},
		},
	}
	data, err := wbxml.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out FolderSyncResponse
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}
