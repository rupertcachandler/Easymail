package eas

import (
	"testing"

	"github.com/remdev/go-activesync/wbxml"
)

func TestItemOperationsMarshalUnmarshal(t *testing.T) {
	req := &ItemOperationsRequest{Fetch: ItemOperationsFetch{FileReference: "mail/ABC/123/1"}}
	body, err := wbxml.Marshal(req)
	if err != nil { t.Fatalf("marshal: %v", err) }
	t.Logf("request body %d bytes", len(body))

	// Simulate SOGo reply: ItemOperations/Response/Fetch/Status 1, Properties/{ContentType,Data(base64)}
	resp := &ItemOperationsResponse{
		Status: 1,
		Response: ItemOperationsResponseBlock{Fetch: []ItemOperationsFetchResult{{
			Status: 1,
			FileReference: "mail/ABC/123/1",
			Properties: ItemOperationsProperties{ContentType: "application/pdf", Data: "aGVsbG8="},
		}}},
	}
	rb, err := wbxml.Marshal(resp)
	if err != nil { t.Fatalf("marshal resp: %v", err) }
	var got ItemOperationsResponse
	if err := wbxml.Unmarshal(rb, &got); err != nil { t.Fatalf("unmarshal: %v", err) }
	t.Logf("resp.Status=%d fetch[0].Status=%d ct=%q data=%q", got.Status, got.Response.Fetch[0].Status, got.Response.Fetch[0].Properties.ContentType, got.Response.Fetch[0].Properties.Data)
	if got.Response.Fetch[0].Properties.Data != "aGVsbG8=" { t.Fatalf("data mismatch %q", got.Response.Fetch[0].Properties.Data) }
}
