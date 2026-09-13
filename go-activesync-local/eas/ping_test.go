package eas

import (
	"reflect"
	"testing"

	"github.com/remdev/go-activesync/wbxml"
)

// SPEC: MS-ASCMD/ping.request
func TestPingRequest_RoundTrip(t *testing.T) {
	in := PingRequest{
		HeartbeatInterval: 480,
		Folders: PingFolders{
			Folder: []PingFolder{
				{ID: "1", Class: "Email"},
				{ID: "2", Class: "Calendar"},
			},
		},
	}
	data, err := wbxml.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out PingRequest
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

// SPEC: MS-ASCMD/ping.response
// SPEC: MS-ASCMD/ping.status.changes
func TestPingResponse_ChangesAvailable(t *testing.T) {
	in := PingResponse{
		Status: 2,
		Folders: PingResponseFolders{
			Folder: []string{"1", "2"},
		},
	}
	if !PingHasChanges(in.Status) {
		t.Fatalf("PingHasChanges(2) = false")
	}
	data, err := wbxml.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out PingResponse
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}
