package client

import (
	"context"
	"net/http"
	"testing"
)

// SPEC: MS-ASHTTP/auth.basic
func TestBasicAuth_AppliesAuthorizationHeader(t *testing.T) {
	auth := BasicAuth{Username: "Aladdin", Password: "open sesame"}
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://example.com", nil)
	if err := auth.Apply(req); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==" {
		t.Errorf("Authorization = %q", got)
	}
}
