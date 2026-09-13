package client

import (
	"net/http"
	"testing"
)

// SPEC: MS-ASHTTP/headers.protocol-version
// SPEC: MS-ASHTTP/headers.content-type
// SPEC: MS-ASHTTP/headers.user-agent
func TestApplyMandatoryHeaders(t *testing.T) {
	h := http.Header{}
	ApplyMandatoryHeaders(h, HeaderOptions{
		ProtocolVersion: "14.1",
		UserAgent:       "go-activesync/0.0.1",
	})
	if got := h.Get("MS-ASProtocolVersion"); got != "14.1" {
		t.Errorf("MS-ASProtocolVersion = %q", got)
	}
	if got := h.Get("Content-Type"); got != "application/vnd.ms-sync.wbxml" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := h.Get("User-Agent"); got != "go-activesync/0.0.1" {
		t.Errorf("User-Agent = %q", got)
	}
	if got := h.Get("Accept"); got != "application/vnd.ms-sync.wbxml" {
		t.Errorf("Accept = %q", got)
	}
}

// SPEC: MS-ASHTTP/headers.policy-key
func TestApplyMandatoryHeaders_PolicyKey(t *testing.T) {
	h := http.Header{}
	ApplyMandatoryHeaders(h, HeaderOptions{
		ProtocolVersion: "14.1",
		UserAgent:       "go-activesync/0.0.1",
		PolicyKey:       "12345",
	})
	if got := h.Get("X-MS-PolicyKey"); got != "12345" {
		t.Errorf("X-MS-PolicyKey = %q", got)
	}
}

// SPEC: MS-ASHTTP/headers.accept-language
func TestApplyMandatoryHeaders_AcceptLanguage(t *testing.T) {
	h := http.Header{}
	ApplyMandatoryHeaders(h, HeaderOptions{
		ProtocolVersion: "14.1",
		UserAgent:       "go-activesync/0.0.1",
		AcceptLanguage:  "en-US",
	})
	if got := h.Get("Accept-Language"); got != "en-US" {
		t.Errorf("Accept-Language = %q", got)
	}
}

// SPEC: MS-ASHTTP/client.extra-headers-merge
func TestMergeExtraHeaders(t *testing.T) {
	t.Run("addsAbsent", func(t *testing.T) {
		dst := http.Header{}
		src := http.Header{"X-Integrator": []string{"a", "b"}}
		mergeExtraHeaders(dst, src)
		got := dst.Values("X-Integrator")
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Fatalf("X-Integrator = %q", got)
		}
	})
	t.Run("skipsExistingKeys", func(t *testing.T) {
		dst := http.Header{}
		ApplyMandatoryHeaders(dst, HeaderOptions{
			ProtocolVersion: "14.1",
			UserAgent:       "official-ua",
		})
		src := http.Header{
			"User-Agent":           []string{"should-not-apply"},
			"Ms-Asprotocolversion": []string{"99.0"},
			"X-Integrator":         []string{"ok"},
		}
		mergeExtraHeaders(dst, src)
		if dst.Get("User-Agent") != "official-ua" {
			t.Errorf("User-Agent = %q", dst.Get("User-Agent"))
		}
		if dst.Get("MS-ASProtocolVersion") != "14.1" {
			t.Errorf("MS-ASProtocolVersion = %q", dst.Get("MS-ASProtocolVersion"))
		}
		if dst.Get("X-Integrator") != "ok" {
			t.Errorf("X-Integrator = %q", dst.Get("X-Integrator"))
		}
	})
	t.Run("canonicalizesKeys", func(t *testing.T) {
		dst := http.Header{}
		src := http.Header{"x-custom-device": []string{"model-9"}}
		mergeExtraHeaders(dst, src)
		if got := dst.Get("X-Custom-Device"); got != "model-9" {
			t.Errorf("X-Custom-Device = %q", got)
		}
	})
	t.Run("duplicateValuesInSrc", func(t *testing.T) {
		dst := http.Header{}
		src := http.Header{"X-Two": []string{"a", "b"}}
		mergeExtraHeaders(dst, src)
		if dst.Values("X-Two") == nil || len(dst.Values("X-Two")) != 2 {
			t.Fatalf("expected two values, got %#v", dst.Values("X-Two"))
		}
	})
}
