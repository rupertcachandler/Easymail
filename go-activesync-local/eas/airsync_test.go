package eas

import (
	"reflect"
	"testing"

	"github.com/remdev/go-activesync/wbxml"
)

// SPEC: MS-ASCMD/sync.request
func TestSyncRequest_RoundTrip(t *testing.T) {
	in := SyncRequest{
		Collections: SyncCollections{
			Collection: []SyncCollection{{
				SyncKey:      "0",
				CollectionID: "1",
				GetChanges:   1,
				WindowSize:   25,
				Options: &SyncOptions{
					FilterType:     5,
					MIMESupport:    2,
					BodyPreference: []BodyPreference{{Type: 2, TruncationSize: 4096}},
				},
			}},
		},
	}
	data, err := wbxml.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out SyncRequest
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

// SPEC: MS-ASCMD/sync.response
func TestSyncResponse_RoundTrip(t *testing.T) {
	in := SyncResponse{
		Collections: SyncCollections{
			Collection: []SyncCollection{{
				SyncKey:       "abc",
				CollectionID:  "1",
				Status:        1,
				MoreAvailable: 1,
			}},
		},
	}
	data, err := wbxml.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out SyncResponse
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}
