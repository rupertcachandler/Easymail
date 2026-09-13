package autodiscover

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// SPEC: MS-OXDISCO/request.schema
func TestRequestXML_MobileSyncSchema(t *testing.T) {
	body, err := buildRequestXML("user@example.com")
	if err != nil {
		t.Fatalf("buildRequestXML: %v", err)
	}
	if !strings.Contains(body, "mobilesync/requestschema/2006") {
		t.Errorf("request schema missing in: %s", body)
	}
	if !strings.Contains(body, "mobilesync/responseschema/2006") {
		t.Errorf("AcceptableResponseSchema missing in: %s", body)
	}
	if !strings.Contains(body, "<EMailAddress>user@example.com</EMailAddress>") {
		t.Errorf("EMailAddress missing in: %s", body)
	}
}

// SPEC: MS-OXDISCO/response.url
func TestParseResponse_URL(t *testing.T) {
	const xmlBody = `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <User>
      <DisplayName>John Doe</DisplayName>
      <EMailAddress>user@example.com</EMailAddress>
    </User>
    <Action>
      <Settings>
        <Server>
          <Type>MobileSync</Type>
          <Url>https://eas.example.com/Microsoft-Server-ActiveSync</Url>
          <Name>https://eas.example.com/Microsoft-Server-ActiveSync</Name>
        </Server>
      </Settings>
    </Action>
  </Response>
</Autodiscover>`
	res, err := parseResponse([]byte(xmlBody))
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if res.URL != "https://eas.example.com/Microsoft-Server-ActiveSync" {
		t.Errorf("URL = %q", res.URL)
	}
	if res.DisplayName != "John Doe" {
		t.Errorf("DisplayName = %q", res.DisplayName)
	}
}

// SPEC: MS-OXDISCO/response.redirect-url
func TestParseResponse_RedirectURL(t *testing.T) {
	const xmlBody = `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Action>
      <Redirect>https://other.example.com/autodiscover/autodiscover.xml</Redirect>
    </Action>
  </Response>
</Autodiscover>`
	res, err := parseResponse([]byte(xmlBody))
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if res.RedirectURL != "https://other.example.com/autodiscover/autodiscover.xml" {
		t.Errorf("RedirectURL = %q", res.RedirectURL)
	}
}

// SPEC: MS-OXDISCO/response.redirect-addr
func TestParseResponse_RedirectAddr(t *testing.T) {
	const xmlBody = `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Action>
      <RedirectAddr>user@other.example.com</RedirectAddr>
    </Action>
  </Response>
</Autodiscover>`
	res, err := parseResponse([]byte(xmlBody))
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if res.RedirectAddr != "user@other.example.com" {
		t.Errorf("RedirectAddr = %q", res.RedirectAddr)
	}
}

// SPEC: MS-OXDISCO/response.error
func TestParseResponse_Error(t *testing.T) {
	const xmlBody = `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Error>
      <Status>2</Status>
      <Message>Account not found</Message>
    </Error>
  </Response>
</Autodiscover>`
	if _, err := parseResponse([]byte(xmlBody)); err == nil {
		t.Fatalf("parseResponse: expected error for Error/Status=2")
	}
}

// SPEC: MS-OXDISCO/request.path
// SPEC: MS-OXDISCO/transport.candidates
func TestDiscover_HappyPath(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/autodiscover/autodiscover.xml" {
			http.Error(w, "wrong path", 404)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			XMLName xml.Name `xml:"Autodiscover"`
			Request struct {
				EMailAddress string `xml:"EMailAddress"`
			} `xml:"Request"`
		}
		if err := xml.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <User><EMailAddress>%s</EMailAddress></User>
    <Action><Settings><Server><Type>MobileSync</Type><Url>https://eas.example.com/Microsoft-Server-ActiveSync</Url></Server></Settings></Action>
  </Response>
</Autodiscover>`, req.Request.EMailAddress)
	}))
	t.Cleanup(srv.Close)

	d := New(srv.Client())
	d.candidatesOverride = func(domain string) []string {
		return []string{srv.URL + "/autodiscover/autodiscover.xml"}
	}
	res, err := d.Discover(context.Background(), "user@example.com", nil)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if res.URL != "https://eas.example.com/Microsoft-Server-ActiveSync" {
		t.Errorf("URL = %q", res.URL)
	}
}

// SPEC: MS-OXDISCO/transport.302
func TestDiscover_HTTP302(t *testing.T) {
	final := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Action><Settings><Server><Type>MobileSync</Type><Url>https://eas.example.com/Microsoft-Server-ActiveSync</Url></Server></Settings></Action>
  </Response>
</Autodiscover>`)
	}))
	t.Cleanup(final.Close)

	first := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/autodiscover/autodiscover.xml", http.StatusFound)
	}))
	t.Cleanup(first.Close)

	d := New(final.Client())
	d.candidatesOverride = func(domain string) []string {
		return []string{first.URL + "/autodiscover/autodiscover.xml"}
	}
	res, err := d.Discover(context.Background(), "user@example.com", nil)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if res.URL != "https://eas.example.com/Microsoft-Server-ActiveSync" {
		t.Errorf("URL = %q", res.URL)
	}
}

// SPEC: MS-OXDISCO/response.redirect-addr
func TestDiscover_FollowsRedirectAddr(t *testing.T) {
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls++
		if calls == 1 {
			fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Action><RedirectAddr>real@example.com</RedirectAddr></Action>
  </Response>
</Autodiscover>`)
			return
		}
		if !strings.Contains(string(body), "<EMailAddress>real@example.com</EMailAddress>") {
			http.Error(w, "expected redirected email", 400)
			return
		}
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Action><Settings><Server><Type>MobileSync</Type><Url>https://eas.example.com/Microsoft-Server-ActiveSync</Url></Server></Settings></Action>
  </Response>
</Autodiscover>`)
	}))
	t.Cleanup(srv.Close)

	d := New(srv.Client())
	d.candidatesOverride = func(domain string) []string {
		return []string{srv.URL + "/autodiscover/autodiscover.xml"}
	}
	res, err := d.Discover(context.Background(), "user@example.com", nil)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if res.URL != "https://eas.example.com/Microsoft-Server-ActiveSync" {
		t.Errorf("URL = %q", res.URL)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

// SPEC: MS-OXDISCO/transport.srv
func TestDiscover_SRVFallback(t *testing.T) {
	srvHost := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006">
    <Action><Settings><Server><Type>MobileSync</Type><Url>https://eas.example.com/Microsoft-Server-ActiveSync</Url></Server></Settings></Action>
  </Response>
</Autodiscover>`)
	}))
	t.Cleanup(srvHost.Close)
	target, err := url.Parse(srvHost.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	d := New(srvHost.Client())
	d.candidatesOverride = func(domain string) []string { return nil }
	d.srvResolver = func(ctx context.Context, name string) (string, error) {
		if !strings.HasPrefix(name, "_autodiscover._tcp.") {
			return "", errors.New("unexpected SRV name")
		}
		return target.Host, nil
	}
	res, err := d.Discover(context.Background(), "user@example.com", nil)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if res.URL != "https://eas.example.com/Microsoft-Server-ActiveSync" {
		t.Errorf("URL = %q", res.URL)
	}
}
