package autodiscover

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"
)

// SPEC: MS-OXDISCO/transport.candidates
func TestDiscover_AllCandidatesFail(t *testing.T) {
	d := New(nil)
	d.candidatesOverride = func(string) []string { return []string{"https://127.0.0.1:1/autodiscover/autodiscover.xml"} }
	d.srvResolver = func(ctx context.Context, name string) (string, error) { return "", errors.New("no SRV") }
	if _, err := d.Discover(context.Background(), "user@example.com", nil); err == nil {
		t.Fatalf("expected error when all candidates fail")
	}
}

// SPEC: MS-OXDISCO/transport.candidates
func TestDiscover_InvalidEmail(t *testing.T) {
	d := New(nil)
	if _, err := d.Discover(context.Background(), "bogus", nil); err == nil {
		t.Fatalf("expected error for invalid email")
	}
}

// SPEC: MS-OXDISCO/response.error
func TestDiscover_NonOKHTTP(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", 500)
	}))
	t.Cleanup(srv.Close)

	d := New(srv.Client())
	d.candidatesOverride = func(string) []string { return []string{srv.URL + "/autodiscover/autodiscover.xml"} }
	if _, err := d.Discover(context.Background(), "user@example.com", nil); err == nil {
		t.Fatalf("expected error on 500")
	}
}

// SPEC: MS-OXDISCO/response.url
func TestDiscover_WithCredentials(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "user@example.com" || p != "pass" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Action><Settings><Server><Type>MobileSync</Type><Url>https://eas/example</Url></Server></Settings></Action>
  </Response>
</Autodiscover>`)
	}))
	t.Cleanup(srv.Close)
	d := New(srv.Client())
	d.candidatesOverride = func(string) []string { return []string{srv.URL + "/autodiscover/autodiscover.xml"} }
	res, err := d.Discover(context.Background(), "user@example.com", &Credentials{Username: "user@example.com", Password: "pass"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if res.URL != "https://eas/example" {
		t.Fatalf("URL = %q", res.URL)
	}
}

// SPEC: MS-OXDISCO/transport.srv
func TestDiscover_SRVResolverError(t *testing.T) {
	d := New(nil)
	d.candidatesOverride = func(string) []string { return nil }
	d.srvResolver = func(ctx context.Context, name string) (string, error) { return "", errors.New("nope") }
	if _, err := d.Discover(context.Background(), "user@example.com", nil); err == nil {
		t.Fatalf("expected error on SRV failure")
	}
}

// SPEC: MS-OXDISCO/response.url
func TestParseResponse_BadXML(t *testing.T) {
	if _, err := parseResponse([]byte("not xml")); err == nil {
		t.Fatalf("expected error on bad xml")
	}
}

// SPEC: MS-OXDISCO/response.url
func TestParseResponse_NoServer(t *testing.T) {
	const body = `<Autodiscover><Response><Action><Settings></Settings></Action></Response></Autodiscover>`
	res, err := parseResponse([]byte(body))
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if res.URL != "" {
		t.Fatalf("URL = %q, want empty", res.URL)
	}
}

// SPEC: MS-OXDISCO/transport.candidates
func TestNew_DefaultsToHTTPDefaultClient(t *testing.T) {
	d := New(nil)
	if d.HTTPClient != http.DefaultClient {
		t.Fatalf("HTTPClient not default")
	}
}

// SPEC: MS-OXDISCO/transport.candidates
func TestCandidates_Default(t *testing.T) {
	d := New(nil)
	got := d.candidates("example.com")
	if len(got) != 2 || !strings.Contains(got[0], "autodiscover.example.com") || !strings.Contains(got[1], "/example.com/") {
		t.Fatalf("candidates = %v", got)
	}
}

// SPEC: MS-OXDISCO/response.redirect-url
func TestDiscover_FollowRedirectURL(t *testing.T) {
	final := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Action><Settings><Server><Type>MobileSync</Type><Url>https://eas/final</Url></Server></Settings></Action>
  </Response>
</Autodiscover>`)
	}))
	t.Cleanup(final.Close)

	first := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Action><Redirect>%s/autodiscover/autodiscover.xml</Redirect></Action>
  </Response>
</Autodiscover>`, final.URL)
	}))
	t.Cleanup(first.Close)

	d := New(final.Client())
	d.candidatesOverride = func(string) []string { return []string{first.URL + "/autodiscover/autodiscover.xml"} }
	res, err := d.Discover(context.Background(), "user@example.com", nil)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if res.URL != "https://eas/final" {
		t.Fatalf("URL = %q", res.URL)
	}
}

// SPEC: MS-OXDISCO/response.redirect-url
func TestDiscover_FollowRedirectURLFails(t *testing.T) {
	first := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Action><Redirect>https://127.0.0.1:1/x</Redirect></Action>
  </Response>
</Autodiscover>`)
	}))
	t.Cleanup(first.Close)

	d := New(first.Client())
	d.candidatesOverride = func(string) []string { return []string{first.URL + "/autodiscover/autodiscover.xml"} }
	if _, err := d.Discover(context.Background(), "user@example.com", nil); err == nil {
		t.Fatal("expected redirect-fetch error")
	}
}

// SPEC: MS-OXDISCO/response.redirect-url
func TestFollowRedirectURL_EmptyResult(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<Autodiscover><Response><Action><Settings/></Action></Response></Autodiscover>`)
	}))
	t.Cleanup(srv.Close)
	d := New(srv.Client())
	if _, err := d.followRedirectURL(context.Background(), srv.URL, "user@example.com", nil); err == nil {
		t.Fatal("expected empty-target error")
	}
}

// SPEC: MS-OXDISCO/transport.srv
func TestDiscover_SRVFallbackEmpty(t *testing.T) {
	d := New(nil)
	d.candidatesOverride = func(string) []string { return nil }
	d.srvResolver = func(ctx context.Context, name string) (string, error) { return "", nil }
	if _, err := d.Discover(context.Background(), "user@example.com", nil); err == nil {
		t.Fatal("expected error when SRV returns empty host")
	}
}

// SPEC: MS-OXDISCO/transport.srv
func TestDiscover_SRVThenSuccess_Redirect(t *testing.T) {
	final := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Action><RedirectAddr>real@x.com</RedirectAddr></Action>
  </Response>
</Autodiscover>`)
	}))
	t.Cleanup(final.Close)

	target, _ := neturl.Parse(final.URL)
	d := New(final.Client())
	calls := 0
	d.candidatesOverride = func(string) []string { return nil }
	d.srvResolver = func(ctx context.Context, name string) (string, error) {
		calls++
		if calls > 1 {
			return "", errors.New("no")
		}
		return target.Host, nil
	}
	if _, err := d.Discover(context.Background(), "user@example.com", nil); err == nil {
		t.Fatal("expected error after redirect from SRV path leads nowhere")
	}
}
