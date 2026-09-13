package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/remdev/go-activesync/eas"
	"github.com/remdev/go-activesync/wbxml"
)

// Client is the high-level Exchange ActiveSync client. The zero value is not
// useful; use New to construct a fully wired Client.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Auth       Authenticator

	DeviceID   string
	DeviceType string
	UserAgent  string
	Locale     uint16

	ProtocolVersion string
	AcceptLanguage  string

	PolicyStore    PolicyStore
	SyncStateStore SyncStateStore

	// ExtraHeaders are merged into each request after mandatory headers without
	// overwriting keys already set (see Config.ExtraHeaders). Do not mutate this
	// map after New while the Client is in use; concurrent writes race with requests.
	ExtraHeaders http.Header

	// ForceHTTP11 reflects the config flag; when HTTPClient was supplied to New
	// the transport is never altered and this bit is informational only.
	ForceHTTP11 bool

	QueryEncoding QueryEncoding

	DeviceModel        string
	DeviceOS           string
	DeviceOSLanguage   string
	Carrier            string
	PhoneNumber        string
	DeviceUserAgent    string
	DeviceFriendlyName string
	DeviceIMEI         string
}

// Config bundles the values required to construct a Client.
type Config struct {
	BaseURL    string
	HTTPClient *http.Client
	Auth       Authenticator

	DeviceID string
	// DeviceType is the device class in the MS-ASHTTP query (e.g. "SmartPhone").
	// For an Outlook-style profile many servers expect DeviceType "Outlook".
	DeviceType string

	// UserAgent is sent as the mandatory User-Agent header.
	UserAgent string

	// Locale is the LCID placed in the binary query (little-endian uint16), for
	// example 0x0409 (en-US) or 0x0419 (ru-RU). Plain queries carry language
	// preferences through Accept-Language instead of a locale field.
	Locale uint16

	AcceptLanguage string

	PolicyStore    PolicyStore
	SyncStateStore SyncStateStore

	// ExtraHeaders optional integrator headers not modeled by first-class
	// Config fields. They are merged after mandatory and device-profile headers
	// and never replace keys the client already set.
	//
	// Avoid mutating this header map after passing Config to New if other
	// goroutines still hold a reference to it; New clones into the Client when non-empty.
	ExtraHeaders http.Header

	// ForceHTTP11, when true and HTTPClient is nil, builds an HTTP client whose
	// transport clones http.DefaultTransport and disables HTTP/2 by setting
	// TLSNextProto to a non-nil empty map. When HTTPClient is non-nil,
	// ForceHTTP11 is ignored and the caller's transport is not modified.
	ForceHTTP11 bool

	// QueryEncoding selects the MS-ASHTTP request URI format. The zero value
	// defaults to QueryEncodingBase64 for compatibility with earlier releases.
	QueryEncoding QueryEncoding

	// Device profile fields are sent in the Provision DeviceInformation body
	// when at least one field is set. Header-backed fields are also sent as
	// X-MS-Device* request headers.
	DeviceModel        string
	DeviceOS           string
	DeviceOSLanguage   string
	Carrier            string
	PhoneNumber        string
	DeviceUserAgent    string
	DeviceFriendlyName string
	DeviceIMEI         string
}

// New returns a Client populated with sensible defaults for any unset
// optional fields. BaseURL, DeviceID and DeviceType are mandatory.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("client: BaseURL is required")
	}
	if cfg.DeviceID == "" {
		return nil, errors.New("client: DeviceID is required")
	}
	if cfg.DeviceType == "" {
		return nil, errors.New("client: DeviceType is required")
	}
	queryEncoding := cfg.QueryEncoding
	if queryEncoding == "" {
		queryEncoding = QueryEncodingBase64
	}
	switch queryEncoding {
	case QueryEncodingBase64, QueryEncodingPlain:
	default:
		return nil, fmt.Errorf("client: unsupported QueryEncoding %q", cfg.QueryEncoding)
	}
	c := &Client{
		BaseURL:            cfg.BaseURL,
		Auth:               cfg.Auth,
		DeviceID:           cfg.DeviceID,
		DeviceType:         cfg.DeviceType,
		UserAgent:          cfg.UserAgent,
		Locale:             cfg.Locale,
		ProtocolVersion:    eas.ProtocolVersion,
		AcceptLanguage:     cfg.AcceptLanguage,
		PolicyStore:        cfg.PolicyStore,
		SyncStateStore:     cfg.SyncStateStore,
		ForceHTTP11:        cfg.ForceHTTP11,
		QueryEncoding:      queryEncoding,
		DeviceModel:        cfg.DeviceModel,
		DeviceOS:           cfg.DeviceOS,
		DeviceOSLanguage:   cfg.DeviceOSLanguage,
		Carrier:            cfg.Carrier,
		PhoneNumber:        cfg.PhoneNumber,
		DeviceUserAgent:    cfg.DeviceUserAgent,
		DeviceFriendlyName: cfg.DeviceFriendlyName,
		DeviceIMEI:         cfg.DeviceIMEI,
	}
	if len(cfg.ExtraHeaders) > 0 {
		c.ExtraHeaders = cfg.ExtraHeaders.Clone()
	}
	switch {
	case cfg.HTTPClient != nil:
		c.HTTPClient = cfg.HTTPClient
	case cfg.ForceHTTP11:
		dt, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			c.HTTPClient = http.DefaultClient
			break
		}
		tr := dt.Clone()
		tr.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
		hc := *http.DefaultClient
		hc.Transport = tr
		c.HTTPClient = &hc
	default:
		c.HTTPClient = http.DefaultClient
	}
	if c.UserAgent == "" {
		c.UserAgent = "go-activesync/0.1"
	}
	if c.Locale == 0 {
		c.Locale = 0x0409
	}
	if c.PolicyStore == nil {
		c.PolicyStore = NewInMemoryPolicyStore()
	}
	if c.SyncStateStore == nil {
		c.SyncStateStore = NewInMemorySyncStateStore()
	}
	return c, nil
}

// StatusError is returned when an EAS command completes with a non-success
// Status value (MS-ASCMD §2.2.4).
type StatusError struct {
	Command string
	Status  int32
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("eas: %s returned status=%d", e.Command, e.Status)
}

// do issues a single Sync/FolderSync/Ping/Provision command, marshaling
// request, applying mandatory headers, and unmarshalling the response. It
// transparently retries the request once after a fresh Provision exchange
// when the server signals an invalid policy state (Status 142/143). Calls
// to Provision itself are issued via doOnce to avoid recursion.
func (c *Client) do(ctx context.Context, cmd byte, user string, request, response any) error {
	if err := c.doOnce(ctx, cmd, user, request, response); err != nil {
		var se *StatusError
		if cmd != CmdProvision && errors.As(err, &se) && eas.ShouldReprovision(se.Status) {
			if _, perr := c.Provision(ctx, user); perr != nil {
				return fmt.Errorf("client: re-provision: %w", perr)
			}
			if response != nil {
				if r, ok := response.(interface{ Reset() }); ok {
					r.Reset()
				}
			}
			return c.doOnce(ctx, cmd, user, request, response)
		}
		return err
	}
	return nil
}

// doOnce is a single non-retrying request execution.
func (c *Client) doOnce(ctx context.Context, cmd byte, user string, request, response any) error {
	body, err := wbxml.Marshal(request)
	if err != nil {
		return fmt.Errorf("client: marshal: %w", err)
	}

	policyKey, err := c.policyKey(ctx)
	if err != nil {
		return err
	}

	q := Query{
		ProtocolVersion: 0x91,
		Cmd:             cmd,
		Locale:          c.Locale,
		DeviceID:        c.DeviceID,
		DeviceType:      c.DeviceType,
	}
	if user != "" {
		q.Params = append(q.Params, QueryParam{Tag: ParamUser, Value: []byte(user)})
	}
	encoded, plain, err := c.encodeQuery(q, user)
	if err != nil {
		return fmt.Errorf("client: encode query: %w", err)
	}
	urlStr, err := BuildURL(c.BaseURL, encoded, plain)
	if err != nil {
		return fmt.Errorf("client: build url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(body))
	if err != nil {
		return err
	}
	ApplyMandatoryHeaders(req.Header, HeaderOptions{
		ProtocolVersion: c.ProtocolVersion,
		UserAgent:       c.UserAgent,
		PolicyKey:       policyKey,
		AcceptLanguage:  c.AcceptLanguage,
	})
	ApplyDeviceProfileHeaders(req.Header, DeviceHeaderOptions{
		DeviceModel:      c.DeviceModel,
		DeviceOS:         c.DeviceOS,
		DeviceOSLanguage: c.DeviceOSLanguage,
		Carrier:          c.Carrier,
		PhoneNumber:      c.PhoneNumber,
		DeviceUserAgent:  c.DeviceUserAgent,
	})
	mergeExtraHeaders(req.Header, c.ExtraHeaders)
	if c.Auth != nil {
		if err := c.Auth.Apply(req); err != nil {
			return fmt.Errorf("client: auth: %w", err)
		}
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("client: %s -> %s", commandName(cmd), resp.Status)
	}
	// Debug: log raw response size for Sync commands
	if cmd == 0x07 { // Sync
		log.Printf("client: Sync response %d bytes, status %s", len(respBytes), resp.Status)
	}
	if response != nil && len(respBytes) > 0 {
		if err := wbxml.Unmarshal(respBytes, response); err != nil {
			log.Printf("client: unmarshal failed for %s (%d bytes): %v", commandName(cmd), len(respBytes), err)
			return fmt.Errorf("client: unmarshal: %w", err)
		}
	}
	if response != nil {
		if s, ok := globalStatus(response); ok && eas.ShouldReprovision(s) {
			return &StatusError{Command: commandName(cmd), Status: s}
		}
	}
	return nil
}

func (c *Client) encodeQuery(q Query, user string) (string, bool, error) {
	switch c.QueryEncoding {
	case "", QueryEncodingBase64:
		encoded, err := q.EncodeBase64()
		return encoded, false, err
	case QueryEncodingPlain:
		if user == "" {
			return "", true, errors.New("plain query encoding requires user")
		}
		return q.EncodePlain(), true, nil
	default:
		return "", false, fmt.Errorf("unsupported QueryEncoding %q", c.QueryEncoding)
	}
}

// globalStatus extracts a top-level Status field from a typed response, if
// the response struct exposes one. It is used to translate command-level
// re-provision codes into a StatusError that the retry layer can recognise;
// other status values are interpreted by the typed command methods.
func globalStatus(v any) (int32, bool) {
	switch r := v.(type) {
	case *eas.FolderSyncResponse:
		return r.Status, true
	case *eas.ProvisionResponse:
		return r.Status, true
	case *eas.PingResponse:
		return r.Status, true
	case *eas.SyncResponse:
		return r.Status, true
	}
	return 0, false
}

func (c *Client) policyKey(ctx context.Context) (string, error) {
	if c.PolicyStore == nil {
		return "0", nil
	}
	k, err := c.PolicyStore.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("client: policy store: %w", err)
	}
	if k == "" {
		return "0", nil
	}
	return k, nil
}
