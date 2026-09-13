// Package autodiscover implements MS-OXDISCO/MS-ASAB POX Autodiscover for
// Exchange ActiveSync mobile clients. It returns the EAS endpoint URL for a
// given user e-mail address by issuing POX (Plain Old XML) requests over
// HTTPS to a list of candidate hosts and, as a last resort, by following an
// _autodiscover._tcp DNS SRV record.
package autodiscover

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

const (
	// responseSchema identifies the matching mobilesync POX response payload.
	responseSchema = "http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006"

	// path is the canonical Autodiscover POX path.
	path = "/autodiscover/autodiscover.xml"

	// maxRedirects bounds Action/Redirect + Action/RedirectAddr chains.
	maxRedirects = 10
)

// Result describes the outcome of a successful Autodiscover lookup.
type Result struct {
	URL          string
	DisplayName  string
	EMailAddress string
	RedirectURL  string
	RedirectAddr string
}

// Credentials carries optional Basic credentials used for the Autodiscover
// request itself (some servers require authentication even for discovery).
type Credentials struct {
	Username string
	Password string
}

// Discoverer performs MS-OXDISCO POX Autodiscover lookups.
type Discoverer struct {
	HTTPClient *http.Client

	// candidatesOverride lets tests inject a deterministic list of candidate
	// URLs. When nil, the production candidate list is used.
	candidatesOverride func(domain string) []string
	// srvResolver, when set, replaces net.DefaultResolver.LookupSRV. It must
	// return host:port (port omitted means 443).
	srvResolver func(ctx context.Context, name string) (string, error)
}

// New returns a Discoverer that uses the given HTTP client. If client is nil,
// http.DefaultClient is used.
func New(client *http.Client) *Discoverer {
	if client == nil {
		client = http.DefaultClient
	}
	return &Discoverer{HTTPClient: client}
}

// Discover resolves the EAS endpoint URL for emailAddress.
func (d *Discoverer) Discover(ctx context.Context, emailAddress string, creds *Credentials) (*Result, error) {
	current := emailAddress
	for i := 0; i < maxRedirects; i++ {
		domain, err := domainOf(current)
		if err != nil {
			return nil, err
		}

		candidates := d.candidates(domain)
		var lastErr error
		for _, u := range candidates {
			res, err := d.requestOne(ctx, u, current, creds)
			if err != nil {
				lastErr = err
				continue
			}
			if res.RedirectURL != "" {
				res2, err := d.followRedirectURL(ctx, res.RedirectURL, current, creds)
				if err != nil {
					lastErr = err
					continue
				}
				return res2, nil
			}
			if res.RedirectAddr != "" {
				current = res.RedirectAddr
				lastErr = nil
				goto next
			}
			if res.URL != "" {
				return res, nil
			}
			lastErr = fmt.Errorf("autodiscover: empty response")
		}

		if host, err := d.lookupSRV(ctx, domain); err == nil && host != "" {
			u := "https://" + host + path
			res, err := d.requestOne(ctx, u, current, creds)
			if err == nil {
				if res.URL != "" {
					return res, nil
				}
				if res.RedirectAddr != "" {
					current = res.RedirectAddr
					goto next
				}
				if res.RedirectURL != "" {
					return d.followRedirectURL(ctx, res.RedirectURL, current, creds)
				}
			} else {
				lastErr = err
			}
		}

		if lastErr == nil {
			lastErr = fmt.Errorf("autodiscover: no candidates succeeded for %s", domain)
		}
		return nil, lastErr
	next:
	}
	return nil, fmt.Errorf("autodiscover: too many redirects")
}

func (d *Discoverer) followRedirectURL(ctx context.Context, redirectURL, email string, creds *Credentials) (*Result, error) {
	res, err := d.requestOne(ctx, redirectURL, email, creds)
	if err != nil {
		return nil, err
	}
	if res.URL == "" {
		return nil, fmt.Errorf("autodiscover: redirect target returned no Url")
	}
	return res, nil
}

func (d *Discoverer) candidates(domain string) []string {
	if d.candidatesOverride != nil {
		return d.candidatesOverride(domain)
	}
	return []string{
		"https://autodiscover." + domain + path,
		"https://" + domain + path,
	}
}

func (d *Discoverer) lookupSRV(ctx context.Context, domain string) (string, error) {
	name := "_autodiscover._tcp." + domain
	if d.srvResolver != nil {
		return d.srvResolver(ctx, name)
	}
	_, srvs, err := net.DefaultResolver.LookupSRV(ctx, "autodiscover", "tcp", domain)
	if err != nil {
		return "", err
	}
	if len(srvs) == 0 {
		return "", fmt.Errorf("autodiscover: no SRV records")
	}
	host := strings.TrimSuffix(srvs[0].Target, ".")
	if srvs[0].Port != 0 && srvs[0].Port != 443 {
		host = fmt.Sprintf("%s:%d", host, srvs[0].Port)
	}
	return host, nil
}

func (d *Discoverer) requestOne(ctx context.Context, urlStr, email string, creds *Credentials) (*Result, error) {
	body, err := buildRequestXML(email)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	if creds != nil && creds.Username != "" {
		req.SetBasicAuth(creds.Username, creds.Password)
	}
	resp, err := d.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("autodiscover: %s -> %s", urlStr, resp.Status)
	}
	return parseResponse(respBytes)
}

// buildRequestXML produces a mobilesync Autodiscover POX request payload.
func buildRequestXML(emailAddress string) (string, error) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	buf.WriteString(`<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/mobilesync/requestschema/2006">` + "\n")
	buf.WriteString(`  <Request>` + "\n")
	buf.WriteString(`    <EMailAddress>`)
	if err := xml.EscapeText(&buf, []byte(emailAddress)); err != nil {
		return "", err
	}
	buf.WriteString(`</EMailAddress>` + "\n")
	buf.WriteString(`    <AcceptableResponseSchema>` + responseSchema + `</AcceptableResponseSchema>` + "\n")
	buf.WriteString(`  </Request>` + "\n")
	buf.WriteString(`</Autodiscover>` + "\n")
	return buf.String(), nil
}

// rawResponse mirrors the POX response structure as defined by MS-OXDISCO
// §3.1.5. We use namespace-less local names because Autodiscover responses in
// the wild mix the outer and mobilesync namespaces and Go's xml decoder
// matches local names by default.
type rawResponse struct {
	XMLName  xml.Name `xml:"Autodiscover"`
	Response struct {
		User struct {
			DisplayName  string `xml:"DisplayName"`
			EMailAddress string `xml:"EMailAddress"`
		} `xml:"User"`
		Action struct {
			Redirect     string `xml:"Redirect"`
			RedirectAddr string `xml:"RedirectAddr"`
			Settings     struct {
				Server []struct {
					Type string `xml:"Type"`
					URL  string `xml:"Url"`
					Name string `xml:"Name"`
				} `xml:"Server"`
			} `xml:"Settings"`
		} `xml:"Action"`
		Error *struct {
			Status  string `xml:"Status"`
			Message string `xml:"Message"`
		} `xml:"Error"`
	} `xml:"Response"`
}

func parseResponse(b []byte) (*Result, error) {
	var raw rawResponse
	if err := xml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("autodiscover: parse response: %w", err)
	}
	if raw.Response.Error != nil && raw.Response.Error.Status != "" && raw.Response.Error.Status != "0" {
		return nil, fmt.Errorf("autodiscover: server error status=%s message=%s",
			raw.Response.Error.Status, raw.Response.Error.Message)
	}
	out := &Result{
		DisplayName:  raw.Response.User.DisplayName,
		EMailAddress: raw.Response.User.EMailAddress,
		RedirectURL:  raw.Response.Action.Redirect,
		RedirectAddr: raw.Response.Action.RedirectAddr,
	}
	for _, s := range raw.Response.Action.Settings.Server {
		if strings.EqualFold(s.Type, "MobileSync") && s.URL != "" {
			out.URL = s.URL
			break
		}
	}
	return out, nil
}

func domainOf(emailAddress string) (string, error) {
	at := strings.LastIndex(emailAddress, "@")
	if at < 0 || at == len(emailAddress)-1 {
		return "", fmt.Errorf("autodiscover: invalid e-mail address %q", emailAddress)
	}
	return emailAddress[at+1:], nil
}
