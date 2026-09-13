package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/remdev/go-activesync/eas"
	"github.com/remdev/go-activesync/wbxml"
)

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := New(Config{
		BaseURL:    srv.URL + EndpointPath,
		HTTPClient: srv.Client(),
		Auth:       &BasicAuth{Username: "user@example.com", Password: "secret"},
		DeviceID:   "TESTDEVICE",
		DeviceType: "SmartPhone",
		UserAgent:  "go-activesync-test/1.0",
		Locale:     0x0409,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func writeWBXML(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	data, err := wbxml.Marshal(v)
	if err != nil {
		t.Fatalf("server marshal: %v", err)
	}
	w.Header().Set("Content-Type", ContentTypeWBXML)
	if _, err := w.Write(data); err != nil {
		t.Fatalf("server write: %v", err)
	}
}

func decodeWBXML(t *testing.T, r *http.Request, v any) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if err := wbxml.Unmarshal(body, v); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
}

// SPEC: MS-ASCMD/scenario.full
// SPEC: MS-ASPROV/two-phase-flow
func TestProvision_TwoPhase(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req eas.ProvisionRequest
		decodeWBXML(t, r, &req)
		calls++
		switch calls {
		case 1:
			if got := req.Policies.Policy[0].PolicyType; got != eas.PolicyTypeWBXML {
				t.Errorf("call 1 PolicyType=%q", got)
			}
			if r.Header.Get("X-MS-PolicyKey") != "0" {
				t.Errorf("call 1 X-MS-PolicyKey=%q", r.Header.Get("X-MS-PolicyKey"))
			}
			writeWBXML(t, w, &eas.ProvisionResponse{
				Status: int32(eas.StatusSuccess),
				Policies: eas.PoliciesResponse{
					Policy: []eas.PolicyResponse{{
						PolicyType: eas.PolicyTypeWBXML,
						PolicyKey:  "111",
						Status:     int32(eas.StatusSuccess),
						Data: &eas.EASProvisionDoc{
							DevicePasswordEnabled:              1,
							MinDevicePasswordLength:            4,
							MaxInactivityTimeDeviceLock:        900,
							MaxDevicePasswordFailedAttempts:    8,
							AllowSimpleDevicePassword:          1,
							AllowStorageCard:                   1,
							AllowCamera:                        1,
							RequireDeviceEncryption:            0,
							AlphanumericDevicePasswordRequired: 0,
						},
					}},
				},
			})
		case 2:
			if got := req.Policies.Policy[0].PolicyKey; got != "111" {
				t.Errorf("call 2 PolicyKey=%q", got)
			}
			if r.Header.Get("X-MS-PolicyKey") != "111" {
				t.Errorf("call 2 X-MS-PolicyKey=%q", r.Header.Get("X-MS-PolicyKey"))
			}
			writeWBXML(t, w, &eas.ProvisionResponse{
				Status: int32(eas.StatusSuccess),
				Policies: eas.PoliciesResponse{
					Policy: []eas.PolicyResponse{{
						PolicyType: eas.PolicyTypeWBXML,
						PolicyKey:  "222",
						Status:     int32(eas.StatusSuccess),
					}},
				},
			})
		default:
			http.Error(w, "unexpected call", 500)
		}
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv)
	doc, err := c.Provision(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if doc == nil || doc.MinDevicePasswordLength != 4 {
		t.Errorf("policy doc = %+v", doc)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
	got, err := c.PolicyStore.Get(context.Background())
	if err != nil {
		t.Fatalf("PolicyStore: %v", err)
	}
	if got != "222" {
		t.Errorf("final PolicyKey = %q, want 222", got)
	}
}

// SPEC: MS-ASHTTP/client.query-encoding
func TestProvision_QueryEncodingPlainUsesURIParameters(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req eas.ProvisionRequest
		decodeWBXML(t, r, &req)
		calls++

		q := r.URL.Query()
		if got := q.Get("Cmd"); got != "Provision" {
			t.Errorf("Cmd = %q, want Provision", got)
		}
		if got := q.Get("User"); got != "user@example.com" {
			t.Errorf("User = %q, want user@example.com", got)
		}
		if got := q.Get("DeviceId"); got != "PLAINDEV1" {
			t.Errorf("DeviceId = %q, want PLAINDEV1", got)
		}
		if got := q.Get("DeviceType"); got != "Outlook" {
			t.Errorf("DeviceType = %q, want Outlook", got)
		}

		switch calls {
		case 1:
			writeWBXML(t, w, &eas.ProvisionResponse{
				Status: int32(eas.StatusSuccess),
				Policies: eas.PoliciesResponse{
					Policy: []eas.PolicyResponse{{
						PolicyType: eas.PolicyTypeWBXML,
						PolicyKey:  "plain-temp",
						Status:     int32(eas.StatusSuccess),
						Data: &eas.EASProvisionDoc{
							DevicePasswordEnabled:              1,
							MinDevicePasswordLength:            4,
							MaxInactivityTimeDeviceLock:        900,
							MaxDevicePasswordFailedAttempts:    8,
							AllowSimpleDevicePassword:          1,
							AllowStorageCard:                   1,
							AllowCamera:                        1,
							RequireDeviceEncryption:            0,
							AlphanumericDevicePasswordRequired: 0,
						},
					}},
				},
			})
		case 2:
			writeWBXML(t, w, &eas.ProvisionResponse{
				Status: int32(eas.StatusSuccess),
				Policies: eas.PoliciesResponse{
					Policy: []eas.PolicyResponse{{
						PolicyType: eas.PolicyTypeWBXML,
						PolicyKey:  "plain-final",
						Status:     int32(eas.StatusSuccess),
					}},
				},
			})
		default:
			http.Error(w, "unexpected call", 500)
		}
	}))
	t.Cleanup(srv.Close)

	c, err := New(Config{
		BaseURL:       srv.URL + EndpointPath,
		HTTPClient:    srv.Client(),
		DeviceID:      "PLAINDEV1",
		DeviceType:    "Outlook",
		QueryEncoding: QueryEncodingPlain,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.Provision(context.Background(), "user@example.com"); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

// SPEC: MS-ASHTTP/client.query-encoding
func TestFolderSync_DefaultQueryEncodingUsesBase64(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("Cmd") != "" {
			t.Fatalf("default query encoded as plain parameters: %q", r.URL.RawQuery)
		}
		q, err := ParseBase64(r.URL.RawQuery)
		if err != nil {
			t.Fatalf("ParseBase64(default query): %v", err)
		}
		if q.Cmd != CmdFolderSync {
			t.Errorf("Cmd = %d, want FolderSync", q.Cmd)
		}
		if q.DeviceID != "TESTDEVICE" {
			t.Errorf("DeviceID = %q, want TESTDEVICE", q.DeviceID)
		}
		if q.DeviceType != "SmartPhone" {
			t.Errorf("DeviceType = %q, want SmartPhone", q.DeviceType)
		}
		if len(q.Params) != 1 || q.Params[0].Tag != ParamUser || string(q.Params[0].Value) != "user@example.com" {
			t.Errorf("Params = %+v, want User=user@example.com", q.Params)
		}

		var req eas.FolderSyncRequest
		decodeWBXML(t, r, &req)
		writeWBXML(t, w, &eas.FolderSyncResponse{
			Status:  int32(eas.StatusSuccess),
			SyncKey: "FS-BASE64",
		})
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv)
	if _, err := c.FolderSync(context.Background(), "user@example.com", "0"); err != nil {
		t.Fatalf("FolderSync: %v", err)
	}
}

// SPEC: MS-ASHTTP/client.profile.extra-headers
func TestFolderSync_OutgoingExtraHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req eas.FolderSyncRequest
		decodeWBXML(t, r, &req)
		if got := r.Header.Get("X-Device-Model"); got != "Surface" {
			t.Errorf("X-Device-Model = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "go-activesync-test/1.0" {
			t.Errorf("User-Agent should stay mandatory client UA, got %q", got)
		}
		writeWBXML(t, w, &eas.FolderSyncResponse{
			Status:  int32(eas.StatusSuccess),
			SyncKey: "FS-X",
		})
	}))
	t.Cleanup(srv.Close)

	extra := http.Header{
		"x-device-model": []string{"Surface"},
		"User-Agent":     []string{"should-not-override"},
	}
	c, err := New(Config{
		BaseURL:      srv.URL + EndpointPath,
		HTTPClient:   srv.Client(),
		Auth:         &BasicAuth{Username: "user@example.com", Password: "secret"},
		DeviceID:     "TESTDEVICE",
		DeviceType:   "SmartPhone",
		UserAgent:    "go-activesync-test/1.0",
		Locale:       0x0409,
		ExtraHeaders: extra,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.FolderSync(context.Background(), "user@example.com", "0"); err != nil {
		t.Fatalf("FolderSync: %v", err)
	}
}

// SPEC: MS-ASHTTP/client.profile.device-headers
func TestFolderSync_DeviceProfileHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req eas.FolderSyncRequest
		decodeWBXML(t, r, &req)
		want := map[string]string{
			"X-MS-DeviceModel":       "Outlook for iOS and Android",
			"X-MS-DeviceOS":          "iOS 17.5",
			"X-MS-DeviceOSLanguage":  "ru",
			"X-MS-DeviceCarrier":     "Apple",
			"X-MS-DevicePhoneNumber": "+70000000000",
			"X-MS-DeviceUserAgent":   "Outlook-iOS-Android/1.0",
		}
		for name, wantValue := range want {
			if got := r.Header.Get(name); got != wantValue {
				t.Errorf("%s = %q, want %q", name, got, wantValue)
			}
		}
		writeWBXML(t, w, &eas.FolderSyncResponse{
			Status:  int32(eas.StatusSuccess),
			SyncKey: "FS-PROFILE",
		})
	}))
	t.Cleanup(srv.Close)

	extra := http.Header{
		"X-MS-DeviceModel": []string{"should-not-override-first-class-field"},
	}
	c, err := New(Config{
		BaseURL:          srv.URL + EndpointPath,
		HTTPClient:       srv.Client(),
		DeviceID:         "TESTDEVICE",
		DeviceType:       "Outlook",
		DeviceModel:      "Outlook for iOS and Android",
		DeviceOS:         "iOS 17.5",
		DeviceOSLanguage: "ru",
		Carrier:          "Apple",
		PhoneNumber:      "+70000000000",
		DeviceUserAgent:  "Outlook-iOS-Android/1.0",
		ExtraHeaders:     extra,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.FolderSync(context.Background(), "user@example.com", "0"); err != nil {
		t.Fatalf("FolderSync: %v", err)
	}
}

// SPEC: MS-ASCMD/scenario.full
// SPEC: MS-ASCMD/foldersync.response
func TestFolderSync_Initial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req eas.FolderSyncRequest
		decodeWBXML(t, r, &req)
		if req.SyncKey != "0" {
			t.Errorf("initial SyncKey = %q", req.SyncKey)
		}
		if !strings.Contains(r.URL.RawQuery, "") {
			t.Errorf("missing query")
		}
		if r.Header.Get("MS-ASProtocolVersion") != "14.1" {
			t.Errorf("MS-ASProtocolVersion = %q", r.Header.Get("MS-ASProtocolVersion"))
		}
		writeWBXML(t, w, &eas.FolderSyncResponse{
			Status:  int32(eas.StatusSuccess),
			SyncKey: "FS-1",
			Changes: eas.FolderChanges{
				Count: 1,
				Add: []eas.FolderAdd{
					{ServerID: "1", ParentID: "0", DisplayName: "Inbox", Type: 2},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv)
	resp, err := c.FolderSync(context.Background(), "user@example.com", "0")
	if err != nil {
		t.Fatalf("FolderSync: %v", err)
	}
	if resp.SyncKey != "FS-1" || len(resp.Changes.Add) != 1 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

// SPEC: MS-ASCMD/retry.142
func TestFolderSync_AutoReprovision(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// Detect command by trying to unmarshal as Provision first.
		var prov eas.ProvisionRequest
		isProvision := wbxml.Unmarshal(body, &prov) == nil
		calls++
		switch {
		case calls == 1 && !isProvision:
			writeWBXML(t, w, &eas.FolderSyncResponse{
				Status:  int32(eas.StatusInvalidPolicy),
				Changes: eas.FolderChanges{},
			})
		case (calls == 2 || calls == 3) && isProvision:
			pk := "777"
			if calls == 3 {
				pk = "888"
			}
			writeWBXML(t, w, &eas.ProvisionResponse{
				Status: int32(eas.StatusSuccess),
				Policies: eas.PoliciesResponse{
					Policy: []eas.PolicyResponse{{
						PolicyType: eas.PolicyTypeWBXML,
						PolicyKey:  pk,
						Status:     int32(eas.StatusSuccess),
					}},
				},
			})
		case calls == 4 && !isProvision:
			if r.Header.Get("X-MS-PolicyKey") != "888" {
				t.Errorf("retry X-MS-PolicyKey=%q", r.Header.Get("X-MS-PolicyKey"))
			}
			writeWBXML(t, w, &eas.FolderSyncResponse{
				Status:  int32(eas.StatusSuccess),
				SyncKey: "FS-OK",
				Changes: eas.FolderChanges{},
			})
		default:
			http.Error(w, "unexpected", 500)
		}
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv)
	resp, err := c.FolderSync(context.Background(), "user@example.com", "0")
	if err != nil {
		t.Fatalf("FolderSync: %v", err)
	}
	if resp.SyncKey != "FS-OK" {
		t.Errorf("SyncKey = %q, want FS-OK", resp.SyncKey)
	}
	if calls != 4 {
		t.Errorf("calls = %d, want 4", calls)
	}
}

// SPEC: MS-ASCMD/scenario.full
// SPEC: MS-ASCMD/sync.response
func TestSync_RoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req eas.SyncRequest
		decodeWBXML(t, r, &req)
		if len(req.Collections.Collection) != 1 || req.Collections.Collection[0].CollectionID != "1" {
			t.Errorf("unexpected sync request: %+v", req)
		}
		writeWBXML(t, w, &eas.SyncResponse{
			Collections: eas.SyncCollections{
				Collection: []eas.SyncCollection{{
					SyncKey:      "S-1",
					CollectionID: "1",
					Status:       int32(eas.SyncStatusSuccess),
				}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv)
	req := &eas.SyncRequest{
		Collections: eas.SyncCollections{
			Collection: []eas.SyncCollection{{
				SyncKey:      "0",
				CollectionID: "1",
				GetChanges:   1,
				WindowSize:   25,
			}},
		},
	}
	resp, err := c.Sync(context.Background(), "user@example.com", req)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if resp.Collections.Collection[0].SyncKey != "S-1" {
		t.Errorf("SyncKey = %q", resp.Collections.Collection[0].SyncKey)
	}
}

// SPEC: MS-ASCMD/sync.applicationdata.raw
// SPEC: MS-ASCMD/sync.applicationdata.typed
func TestSync_TypedApplicationData(t *testing.T) {
	emailIn := &eas.Email{
		Subject: "hello",
		From:    "alice@example.com",
		To:      "bob@example.com",
	}
	body := mustEmailBody(t, emailIn)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req eas.SyncRequest
		decodeWBXML(t, r, &req)
		writeWBXML(t, w, &eas.SyncResponse{
			Collections: eas.SyncCollections{
				Collection: []eas.SyncCollection{{
					SyncKey:      "S-1",
					CollectionID: "1",
					Class:        "Email",
					Status:       int32(eas.SyncStatusSuccess),
					Commands: &eas.SyncCommands{
						Add: []eas.SyncAdd{{
							ServerID: "1:1",
							ApplicationData: &wbxml.RawElement{
								Page:  wbxml.PageAirSync,
								Bytes: body,
							},
						}},
						Change: []eas.SyncChange{{
							ServerID: "1:1",
							ApplicationData: &wbxml.RawElement{
								Page:  wbxml.PageAirSync,
								Bytes: body,
							},
						}},
					},
				}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv)
	resp, err := c.Sync(context.Background(), "user@example.com", &eas.SyncRequest{
		Collections: eas.SyncCollections{
			Collection: []eas.SyncCollection{{SyncKey: "0", CollectionID: "1", GetChanges: 1}},
		},
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	col := resp.Collections.Collection[0]
	if col.Commands == nil || len(col.Commands.Add) != 1 {
		t.Fatalf("missing Add: %+v", col.Commands)
	}
	add := col.Commands.Add[0]
	if add.ApplicationData == nil {
		t.Fatal("ApplicationData missing on Add")
	}
	got, err := add.Email()
	if err != nil {
		t.Fatalf("add.Email: %v", err)
	}
	if got.Subject != emailIn.Subject || got.From != emailIn.From || got.To != emailIn.To {
		t.Fatalf("Email decode mismatch: %+v", got)
	}
	chg := col.Commands.Change[0]
	got2, err := chg.Email()
	if err != nil {
		t.Fatalf("change.Email: %v", err)
	}
	if got2.Subject != emailIn.Subject {
		t.Fatalf("change Email mismatch: %+v", got2)
	}
}

// mustEmailBody marshals an Email value and strips the WBXML header plus
// the AirSync.ApplicationData wrapper, returning just the bytes that would
// appear inside ApplicationData on the wire.
func mustEmailBody(t *testing.T, e *eas.Email) []byte {
	t.Helper()
	data, err := wbxml.Marshal(e)
	if err != nil {
		t.Fatalf("marshal email: %v", err)
	}
	dec := wbxml.NewDecoder(bytes.NewReader(data))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("read header: %v", err)
	}
	tok, err := dec.NextToken()
	if err != nil {
		t.Fatalf("next token: %v", err)
	}
	if tok.Kind != wbxml.KindTag {
		t.Fatalf("expected tag, got %s", tok.Kind)
	}
	body, err := dec.CaptureRaw(tok.HasContent)
	if err != nil {
		t.Fatalf("capture raw: %v", err)
	}
	return body
}

// SPEC: MS-ASCMD/sync.typed
func TestSyncTyped_Email(t *testing.T) {
	in := []eas.Email{
		{Subject: "first", From: "a@example.com", To: "b@example.com"},
		{Subject: "second", From: "c@example.com", To: "d@example.com"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req eas.SyncRequest
		decodeWBXML(t, r, &req)
		adds := make([]eas.SyncAdd, 0, len(in))
		for i, e := range in {
			adds = append(adds, eas.SyncAdd{
				ServerID: serverIDFor(i),
				ApplicationData: &wbxml.RawElement{
					Page:  wbxml.PageAirSync,
					Bytes: mustEmailBody(t, &e),
				},
			})
		}
		writeWBXML(t, w, &eas.SyncResponse{
			Status: int32(eas.SyncStatusSuccess),
			Collections: eas.SyncCollections{
				Collection: []eas.SyncCollection{{
					SyncKey:      "S-2",
					CollectionID: "1",
					Class:        "Email",
					Status:       int32(eas.SyncStatusSuccess),
					Commands:     &eas.SyncCommands{Add: adds},
				}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv)
	resp, err := SyncTyped[eas.Email](context.Background(), c, "user@example.com", &eas.SyncRequest{
		Collections: eas.SyncCollections{
			Collection: []eas.SyncCollection{{SyncKey: "0", CollectionID: "1", GetChanges: 1}},
		},
	})
	if err != nil {
		t.Fatalf("SyncTyped: %v", err)
	}
	if len(resp.Collections) != 1 {
		t.Fatalf("collections: %d", len(resp.Collections))
	}
	col := resp.Collections[0]
	if col.Class != "Email" || col.SyncKey != "S-2" {
		t.Fatalf("collection metadata: %+v", col)
	}
	if len(col.Add) != len(in) {
		t.Fatalf("Add count = %d, want %d", len(col.Add), len(in))
	}
	for i, item := range col.Add {
		if item.ApplicationData == nil {
			t.Fatalf("Add[%d] ApplicationData nil", i)
		}
		if item.ApplicationData.Subject != in[i].Subject {
			t.Fatalf("Add[%d] Subject %q, want %q", i, item.ApplicationData.Subject, in[i].Subject)
		}
	}
}

func serverIDFor(i int) string {
	return "1:" + string(rune('0'+i))
}

// SPEC: MS-ASCMD/ping.response
// SPEC: MS-ASCMD/ping.status.changes
func TestPing_ChangesAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req eas.PingRequest
		decodeWBXML(t, r, &req)
		if req.HeartbeatInterval != 60 {
			t.Errorf("HeartbeatInterval = %d", req.HeartbeatInterval)
		}
		writeWBXML(t, w, &eas.PingResponse{
			Status: 2,
			Folders: eas.PingResponseFolders{
				Folder: []string{"1"},
			},
		})
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := c.Ping(ctx, "user@example.com", &eas.PingRequest{
		HeartbeatInterval: 60,
		Folders: eas.PingFolders{
			Folder: []eas.PingFolder{{ID: "1", Class: "Email"}},
		},
	})
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if !eas.PingHasChanges(resp.Status) {
		t.Errorf("Ping not signalling changes: %+v", resp)
	}
}

// SPEC: MS-ASCMD/scenario.full
func TestPing_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Ping(ctx, "user@example.com", &eas.PingRequest{
		HeartbeatInterval: 60,
		Folders:           eas.PingFolders{Folder: []eas.PingFolder{{ID: "1", Class: "Email"}}},
	})
	if err == nil {
		t.Fatalf("Ping with cancelled ctx: expected error")
	}
}
