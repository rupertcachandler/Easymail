package client

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/remdev/go-activesync/eas"
)

// SPEC: MS-ASCMD/global.status.codes
func TestStatusError_Error(t *testing.T) {
	e := &StatusError{Command: "Sync", Status: 142}
	if msg := e.Error(); !strings.Contains(msg, "Sync") || !strings.Contains(msg, "142") {
		t.Fatalf("Error()=%q", msg)
	}
}

// SPEC: MS-ASCMD/global.status.codes
func TestNew_RequiresFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"no base url", Config{DeviceID: "d", DeviceType: "t"}},
		{"no device id", Config{BaseURL: "http://x", DeviceType: "t"}},
		{"no device type", Config{BaseURL: "http://x", DeviceID: "d"}},
	}
	for _, tc := range cases {
		if _, err := New(tc.cfg); err == nil {
			t.Errorf("%s: expected error", tc.name)
		}
	}
}

// SPEC: MS-ASHTTP/client.query-encoding
func TestNew_RejectsUnknownQueryEncoding(t *testing.T) {
	_, err := New(Config{
		BaseURL:       "http://example.invalid/Microsoft-Server-ActiveSync",
		DeviceID:      "d",
		DeviceType:    "t",
		QueryEncoding: QueryEncoding("xml"),
	})
	if err == nil {
		t.Fatal("expected unsupported QueryEncoding error")
	}
}

// SPEC: MS-ASCMD/global.status.codes
func TestGlobalStatus_AllResponses(t *testing.T) {
	cases := []any{
		&eas.FolderSyncResponse{Status: 1},
		&eas.ProvisionResponse{Status: 2},
		&eas.PingResponse{Status: 3},
		&eas.SyncResponse{Status: 4},
	}
	for _, c := range cases {
		if _, ok := globalStatus(c); !ok {
			t.Errorf("globalStatus failed for %T", c)
		}
	}
	if _, ok := globalStatus("nope"); ok {
		t.Errorf("globalStatus should reject unknown type")
	}
}

// SPEC: MS-ASCMD/global.status.codes
func TestPolicyKey_StoreError(t *testing.T) {
	c := &Client{PolicyStore: errStore{}}
	if _, err := c.policyKey(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPolicyKey_NilStore(t *testing.T) {
	c := &Client{}
	k, err := c.policyKey(context.Background())
	if err != nil || k != "0" {
		t.Fatalf("policyKey nil store: %q, %v", k, err)
	}
}

type errStore struct{}

func (errStore) Get(context.Context) (string, error) { return "", errors.New("boom") }
func (errStore) Set(context.Context, string) error   { return nil }

// SPEC: MS-ASHTTP/client.profile.force-http11
func TestNew_ForceHTTP11_DefaultTransportDisablesHTTP2(t *testing.T) {
	c, err := New(Config{
		BaseURL:     "http://example.invalid/Microsoft-Server-ActiveSync",
		DeviceID:    "d",
		DeviceType:  "t",
		ForceHTTP11: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tr, ok := c.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type %T", c.HTTPClient.Transport)
	}
	if tr.TLSNextProto == nil {
		t.Fatal("TLSNextProto is nil, want non-nil empty map")
	}
	if len(tr.TLSNextProto) != 0 {
		t.Fatalf("TLSNextProto len = %d, want 0", len(tr.TLSNextProto))
	}
}

// SPEC: MS-ASHTTP/client.profile.force-http11
func TestNew_ForceHTTP11_CustomHTTPClientUnchanged(t *testing.T) {
	base := &http.Transport{}
	hc := &http.Client{Transport: base}
	c, err := New(Config{
		BaseURL:     "http://example.invalid/Microsoft-Server-ActiveSync",
		HTTPClient:  hc,
		DeviceID:    "d",
		DeviceType:  "t",
		ForceHTTP11: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.HTTPClient != hc {
		t.Fatal("HTTPClient replaced")
	}
	if c.HTTPClient.Transport != base {
		t.Fatal("Transport replaced")
	}
	if base.TLSNextProto != nil {
		t.Fatalf("custom transport TLSNextProto = %v, want nil", base.TLSNextProto)
	}
}
