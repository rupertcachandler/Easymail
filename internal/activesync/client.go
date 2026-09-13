package activesync

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/remdev/go-activesync/client"
	"github.com/remdev/go-activesync/eas"
	"github.com/remdev/go-activesync/wbxml"

	"easymail/internal/models"
)

// GenerateDeviceID creates a stable device identifier for EAS
func GenerateDeviceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("easymail-%x", b)
}

// stubPolicyStore skips EAS provisioning (Z-Push returns 501 for it)
type stubPolicyStore struct{}

func (s *stubPolicyStore) Get(_ context.Context) (string, error) { return "0", nil }
func (s *stubPolicyStore) Set(_ context.Context, _ string) error  { return nil }

// Client wraps the go-activesync library for EAS communication
type Client struct {
	account   *models.Account
	easClient *client.Client
	syncKeys  map[string]string // collectionID -> syncKey (in-memory cache)
	connected bool

	// syncMu serializes per-collection wall-clock syncs. SOGo keeps one sync
	// state per collection: two racing syncs (startup + UI refresh) each
	// restarting from "0" invalidate the other's SyncKey (status 3/13), which
	// forces repeated full backfills and wipes the persisted key. One sync at
	// a time per client removes the race.
	syncMu sync.Mutex

	// loadSyncKey/saveSyncKey persist per-collection SyncKeys so contacts and
	// calendar syncs resume incrementally instead of full-refetching with the
	// hardcoded "0" key every time. Wired in by the app layer (connectAccount).
	loadSyncKey func(collectionID string) (string, error)
	saveSyncKey func(collectionID, syncKey string) error
}

// NewClient creates a new EAS client for an account
func NewClient(acc *models.Account) (*Client, error) {
	if acc.Email == "" || acc.Password == "" {
		return nil, fmt.Errorf("account must have email and password")
	}
	return &Client{
		account:  acc,
		syncKeys: make(map[string]string),
	}, nil
}

// SetSyncKeyStore wires persisted SyncKey storage so contacts/calendar syncs
// resume incrementally instead of restoring "0" (full refetch) every time.
// load/save are keyed by collectionID and account-bound by the caller.
func (c *Client) SetSyncKeyStore(load func(collectionID string) (string, error), save func(collectionID, syncKey string) error) {
	c.loadSyncKey = load
	c.saveSyncKey = save
}

// Connect establishes the EAS connection
func (c *Client) Connect(ctx context.Context) error {
	baseURL := c.account.ServerURL
	if baseURL == "" {
		return fmt.Errorf("account must have a server URL configured")
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		// Without a timeout a hung SOGo sync (stale collection lock from a
		// previous killed instance, dead keep-alive, etc.) blocks forever with
		// nothing logged. 60s makes the hang surface as a transient error so
		// the paging loop can retry instead of freezing the UI on old mail.
		Timeout: 60 * time.Second,
	}

	var err error
	c.easClient, err = client.New(client.Config{
		BaseURL:       baseURL,
		Auth:          &client.BasicAuth{Username: c.account.Email, Password: c.account.Password},
		DeviceID:     c.account.DeviceID,
		DeviceType:    c.account.DeviceType,
		UserAgent:    "EasyMail/1.0",
		HTTPClient:    httpClient,
		PolicyStore:  &stubPolicyStore{},
		QueryEncoding: client.QueryEncodingPlain,
	})
	if err != nil {
		return fmt.Errorf("client creation failed: %w", err)
	}

	c.connected = true
	return nil
}

// Close shuts down the client
func (c *Client) Close() {
	c.connected = false
}

// IsConnected returns connection state
func (c *Client) IsConnected() bool {
	return c.connected
}

// SyncFolders performs an initial folder sync
func (c *Client) SyncFolders(ctx context.Context) ([]*models.Folder, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}

	resp, err := c.easClient.FolderSync(ctx, c.account.Email, "0")
	if err != nil {
		return nil, fmt.Errorf("folder sync failed: %w", err)
	}

	var folders []*models.Folder
	for _, add := range resp.Changes.Add {
		folder := &models.Folder{
			AccountID: c.account.ID,
			ServerID:  add.ServerID,
			ParentID:  add.ParentID, // preserve hierarchy so subfolders nest correctly
			Name:      add.DisplayName,
			Type:      int(add.Type),
		}
		folder.ID = fmt.Sprintf("%s-%s", c.account.ID, add.ServerID)
		folders = append(folders, folder)
	}

	return folders, nil
}

// syncCollection performs a two-phase email sync with BodyPreference fallback.
//
// SOGo ground truth (from Alinto/SOGo ActiveSync source):
//  1. Options/FilterType/BodyPreference are read from EVERY Sync request — SOGo
//     does NOT require them only on SyncKey=0, and phase 2 is the request that
//     actually returns items. processSyncGetChanges reads FilterType from the
//     current document (dateFromFilterType), so omitting Options in phase 2
//     means NO date filter and full-history sync.
//  2. BodyPreference: SOGo uses [[...BodyPreference] lastObject] — the LAST
//     BodyPreference in the list wins. Sending [HTML, Plain] makes SOGo choose
//     Plain; send a single HTML entry.
//  3. Phase 1 (SyncKey=0) returns a SyncKey and cached FolderOptions but NO
//     items (first_sync). Only phase 2 returns items.
// syncEmailResult carries both the mail items to save and the ServerIds the
// server reports as deleted (EAS Sync Delete commands), so the app can mirror
// server-side deletions locally instead of keeping deleted mail forever.
type syncEmailResult struct {
	items   []eas.TypedItem[eas.Email]
	deleted []string
}

func (c *Client) syncCollection(ctx context.Context, collectionID string, opts *eas.SyncOptions) (*syncEmailResult, error) {
	return c.syncCollectionRetry(ctx, collectionID, opts, 1)
}

func (c *Client) syncCollectionRetry(ctx context.Context, collectionID string, opts *eas.SyncOptions, retries int) (*syncEmailResult, error) {
	const windowSize = int32(100)

	// Persist the SyncKey per (collection, filter-window) so restarts resume
	// incrementally instead of re-running the full 180-day backfill over a
	// slow link — without this the app re-downloaded every body on every
	// launch (the "5 minute" wait). Status 3/13 resets fall back to "0".
	// ALWAYS start from "0". Empirically SOGo rejects persisted email sync
	// keys on resume (status 3) whenever the request carries Options — every
	// restart got "phase1 status 3 (stale key)", wiped the key, and re-ran
	// the whole 180-day backfill: the 5-minute wait. So email syncs are
	// intentionally stateless: full-filter sync on an empty folder once,
	// short-window (24h) sync on every restart. SOGo returns the latest
	// window quickly; cached history is never re-fetched because the app only
	// issues the wide-window sync when the store has no mail for the folder.
	const phase1Key = "0"

	// Phase 1: SyncKey=0 (or persisted key) WITH Options (SOGo caches FolderOptions from this)
	phase1Col := eas.SyncCollection{
		SyncKey:      phase1Key,
		CollectionID: collectionID,
		GetChanges:   1,
		WindowSize:   windowSize,
	}
	if opts != nil {
		phase1Col.Options = opts
	}

	resp1, err := client.SyncTyped[eas.Email](ctx, c.easClient, c.account.Email, &eas.SyncRequest{
		Collections: eas.SyncCollections{Collection: []eas.SyncCollection{phase1Col}},
	})
	if err != nil {
		// Status 3/13 can surface here as a transport StatusError (stale key).
		// Forget the key and retry from "0" rather than failing the sync.
		var se *client.StatusError
		if retries > 0 && errors.As(err, &se) && (se.Status == 3 || se.Status == 13) {
			log.Printf("sync %s: phase1 transport status %d (stale key) — restarting from SyncKey=0 (retries left %d)", collectionID, se.Status, retries-1)
			return c.syncCollectionRetry(ctx, collectionID, opts, retries-1)
		}
		// Server rejected the request — if we had BodyPreference, retry without it
		if opts != nil && opts.BodyPreference != nil {
			log.Printf("sync %s: phase1 with Options failed (%v) — retrying without BodyPreference", collectionID, err)
			return c.syncCollection(ctx, collectionID, stripBodyPref(opts))
		}
		return nil, fmt.Errorf("sync phase 1 failed: %w", err)
	}

	// Check collection-level status. Status 3 (invalid SyncKey) and 13 (server
	// reset) mean SOGo discarded the sync state tied to our persisted key —
	// typically because another sync (e.g. the wide window pass) restarted the
	// collection from "0". Forget the key and retry from "0"; without this the
	// whole email sync fails on restart and new mail never arrives.
	for _, col := range resp1.Collections {
		if col.Status == 3 || col.Status == 13 {
			if retries > 0 {
				log.Printf("sync %s: phase1 status %d (stale key) — restarting from SyncKey=0 (retries left %d)", collectionID, col.Status, retries - 1)
				return c.syncCollectionRetry(ctx, collectionID, opts, retries - 1)
			}
			log.Printf("sync %s: phase1 status %d — no retries left, continuing with empty result", collectionID, col.Status)
			return &syncEmailResult{}, nil
		}
		if col.Status != 0 && col.Status != 1 {
			if opts != nil && opts.BodyPreference != nil {
				log.Printf("sync %s: phase1 status %d — retrying without BodyPreference", collectionID, col.Status)
				return c.syncCollection(ctx, collectionID, stripBodyPref(opts))
			}
			return nil, fmt.Errorf("sync phase 1 status %d for %s", col.Status, collectionID)
		}
	}

	var syncKey string
	var items []eas.TypedItem[eas.Email]
	var deleted []string
	for _, col := range resp1.Collections {
		syncKey = col.SyncKey
		items = append(items, col.Add...)
		for _, ch := range col.Change {
			items = append(items, ch)
		}
		deleted = append(deleted, col.Delete...)
	}

	// Phase 2+: paging loop. SOGo returns mail changes in windows ordered
	// OLDEST-FIRST from the filter boundary (a fresh filter-7 sync delivered
	// only the 180-day cutoff emails in a single request). We must keep pulling
	// with the returned SyncKey until the server returns no more items or the
	// window comes back empty, otherwise we never reach today's mail. Caps at
	// maxPages to avoid an infinite loop if SOGo keeps returning duplicate
	// pages.
	const maxPages = 200
	for page := 0; syncKey != "" && page < maxPages; page++ {
		if page > 0 && len(items) == 0 {
			break
		}
		if page > 0 {
			// Dedup: if the page returned nothing new, stop.
		}
		pageCol := eas.SyncCollection{
			SyncKey:      syncKey,
			CollectionID: collectionID,
			GetChanges:   1,
			WindowSize:   windowSize,
		}
		if opts != nil {
			pageCol.Options = opts
		}

		resp, err := client.SyncTyped[eas.Email](ctx, c.easClient, c.account.Email, &eas.SyncRequest{
			Collections: eas.SyncCollections{Collection: []eas.SyncCollection{pageCol}},
		})
		if err != nil {
			// Status 3 (invalid SyncKey) / 13 (server reset) surfaces here as a
			// transport StatusError. SOGo discarded the sync state — restart
			// from SyncKey=0, bounded by retries.
			var se *client.StatusError
			if retries > 0 && errors.As(err, &se) && (se.Status == 3 || se.Status == 13) {
				log.Printf("sync %s: page %d failed with status %d — restarting from SyncKey=0 (retries left %d)", collectionID, page, se.Status, retries - 1)
				return c.syncCollectionRetry(ctx, collectionID, opts, retries - 1)
			}
			// Transient HTTP errors (405 Not Allowed from a concurrent/racing
			// sync, 5xx, timeouts) MUST NOT abort the drain — SOGo windows are
			// oldest-first, so dying here means today's mail is never reached
			// and the store stays stuck on stale data. Retry the same page a
			// few times with backoff before giving up.
			if isTransientSyncErr(err) {
				const pageAttempts = 5
				attempt := 0
				for {
					attempt++
					log.Printf("sync %s: page %d attempt %d/%d failed (%v) — retrying", collectionID, page, attempt, pageAttempts, err)
					time.Sleep(time.Duration(attempt) * 800 * time.Millisecond)
					resp, err = client.SyncTyped[eas.Email](ctx, c.easClient, c.account.Email, &eas.SyncRequest{
						Collections: eas.SyncCollections{Collection: []eas.SyncCollection{pageCol}},
					})
					if err == nil {
						break
					}
					if attempt >= pageAttempts {
						log.Printf("sync %s: page %d giving up after %d attempts: %v", collectionID, page, attempt, err)
						return nil, fmt.Errorf("sync phase 2 page %d failed after %d attempts: %w", page, attempt, err)
					}
				}
			} else {
				log.Printf("sync %s: page %d failed (%v)", collectionID, page, err)
				return nil, fmt.Errorf("sync phase 2 page %d failed: %w", page, err)
			}
		}

		batch := 0
		for _, col := range resp.Collections {
			if col.Status != 0 && col.Status != 1 {
				if col.Status == 3 || col.Status == 13 {
					if retries > 0 {
						log.Printf("sync %s: page %d status %d — restarting from SyncKey=0 (retries left %d)", collectionID, page, col.Status, retries - 1)
						return c.syncCollectionRetry(ctx, collectionID, opts, retries - 1)
					}
				}
				return nil, fmt.Errorf("sync phase 2 page %d status %d for %s", page, col.Status, collectionID)
			}
			syncKey = col.SyncKey
			for _, it := range col.Add {
				items = append(items, it)
				batch++
			}
			for _, ch := range col.Change {
				items = append(items, ch)
				batch++
			}
			deleted = append(deleted, col.Delete...)
		}
		if batch == 0 {
			// No more items in the window — done.
			break
		}
	}



	bpTag := "no-BP"
	if opts != nil && opts.BodyPreference != nil {
		bpTag = "with-BP"
	}
	sinceTag := "all-time"
	if opts != nil && opts.FilterType > 0 {
		sinceTag = fmt.Sprintf("filter-%d", opts.FilterType)
		// FilterType values: 0=all, 1=1day, 2=3days, 3=1week, 4=2weeks, 5=1month, 6=3months, 7=6months, 8=1year
	}
	log.Printf("sync %s %s %s: got %d items, %d deleted", collectionID, bpTag, sinceTag, len(items), len(deleted))
	return &syncEmailResult{items: items, deleted: deleted}, nil
}

// isTransientSyncErr reports whether a Sync transport error is worth retrying
// the same page: HTTP 405 (SOGo says "not allowed" when a concurrent/racing
// sync holds the collection — harmless, transient), 5xx, and network errors.
func isTransientSyncErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, token := range []string{"405", "500", "502", "503", "504", "connection", "timeout", "EOF"} {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
}

// stripBodyPref returns a copy of opts without BodyPreference (keeps Class/FilterType/MIMESupport)
func stripBodyPref(opts *eas.SyncOptions) *eas.SyncOptions {
	if opts == nil {
		return nil
	}
	return &eas.SyncOptions{
		FilterType:   opts.FilterType,
		Class:        opts.Class,
		MIMESupport:  opts.MIMESupport,
	}
}

// SyncEmails syncs emails from a folder.
// Single HTML BodyPreference; filter 7 (180 days); Options sent in BOTH
// phases because SOGo reads FilterType/BodyPreference from the request that
// returns items (phase 2), not from SyncKey=0.
// Class="Email" is required by Z-Push for BodyPreference to work.
func (c *Client) SyncEmails(ctx context.Context, collectionID string) ([]*models.Email, error) {
	return c.SyncEmailsFilter(ctx, collectionID, 7)}

// SyncEmailsFilter syncs emails from a folder with an explicit SOGo FilterType
// (0/8=no limit, 1=1d, 2=3d, 3=7d, 4=14d, 5=30d, 6=90d, 7=180d). Exposed for
// diagnostics/probing but safe to use for windowed syncs.
func (c *Client) SyncEmailsFilter(ctx context.Context, collectionID string, filter int32) ([]*models.Email, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}
	// One EAS sync at a time per client — see syncMu. Holding the mutex across
	// the whole drain prevents a racing syncAll from restarting the collection
	// from "0" mid-flight and invalidating our SyncKey.
	c.syncMu.Lock()
	defer c.syncMu.Unlock()

	emails, _, err := c.syncEmailsResolved(ctx, collectionID, filter)
	return emails, err
}

// SyncEmailsWithDeletes returns the emails to save AND the ServerIds the
// server reported as deleted, so the app can mirror server-side deletions
// locally. SyncEmailsFilter (above) drops the deletions for probe/diagnostic
// callers that only want the item list.
func (c *Client) SyncEmailsWithDeletes(ctx context.Context, collectionID string, filter int32) ([]*models.Email, []string, error) {
	return c.syncEmailsResolved(ctx, collectionID, filter)
}

// syncEmailsResolved runs one serialized email sync (see syncMu) and returns
// the emails to save plus the ServerIds the server reported as deleted.
func (c *Client) syncEmailsResolved(ctx context.Context, collectionID string, filter int32) ([]*models.Email, []string, error) {
	if !c.connected {
		return nil, nil, fmt.Errorf("not connected")
	}
	// One EAS sync at a time per client — see syncMu. Holding the mutex across
	// the whole drain prevents a racing syncAll from restarting the collection
	// from "0" mid-flight and invalidating our SyncKey.
	c.syncMu.Lock()
	defer c.syncMu.Unlock()

	// SOGo maps FilterType: 0/8=no limit, 1=1d, 2=3d, 3=7d, 4=14d, 5=30d,
	// 6=90d, 7=180d (NSCalendarDate+ActiveSync dateFromFilterType).
	// Use 7 (last 6 months) so we don't drag years of history down.
	// MIMESupport=2 asks SOGo to preserve the native content type; without it
	// SOGo coerces raw-MIME (type 4) requests down and S/MIME mail behaves oddly.
	emailOpts := &eas.SyncOptions{
		Class:       "Email", // CRITICAL: Z-Push needs this to honour BodyPreference
		FilterType:  filter,  // Last 6 months (SOGo: 7=180 days)
		MIMESupport: 2,
		// SOGo honours the LAST BodyPreference entry ([[...BodyPreference]
		// lastObject]); [HTML, Plain] made it pick Plain. Send a single HTML
		// entry — SOGo's native-type logic downgrades to plain (Type=1) when a
		// message has no HTML part.
		BodyPreference: []eas.BodyPreference{
			{Type: eas.BodyTypeHTML, TruncationSize: 200000, AllOrNone: 1},
		},
	}

	res, err := c.syncCollection(ctx, collectionID, emailOpts)
	if err != nil {
		return nil, nil, err
	}

	log.Printf("SyncEmails: got %d items (%d deleted) for folder %s", len(res.items), len(res.deleted), collectionID)

	var emails []*models.Email
	var nilData int
	for _, item := range res.items {
		if item.ApplicationData == nil {
			nilData++
			continue
		}
		email := easEmailToModel(item.ApplicationData, c.account.ID, collectionID)
		email.ServerID = item.ServerID
		email.ID = fmt.Sprintf("%s-%s", c.account.ID, item.ServerID)
		log.Printf("SyncEmails: id=%s from=%q subj=%q bodyLen=%d bodyType=%s(bodyInt=%d) hasAttach=%v date=%s",
			item.ServerID, email.From, email.Subject, len(email.Body), email.BodyType,
			item.ApplicationData.Body.Type, email.HasAttachment, email.DateReceived.Format("2006-01-02"))
		emails = append(emails, email)
	}
	if nilData > 0 {
		log.Printf("SyncEmails: %d items had nil ApplicationData (skipped)", nilData)
	}
	log.Printf("SyncEmails: returning %d emails for folder %s", len(emails), collectionID)
	return emails, res.deleted, nil
}

// FetchAttachment downloads an attachment from SOGo via ItemOperations Fetch.
// fileReference is the SOGo handle `mail/FOLDER/UID/PATH` returned in the
// AirSyncBase.Attachments block during Sync. Returns the raw bytes and the
// MIME content type reported by SOGo.
func (c *Client) FetchAttachment(ctx context.Context, fileReference string) ([]byte, string, error) {
	if !c.connected || c.easClient == nil {
		return nil, "", fmt.Errorf("not connected")
	}

	resp, err := c.easClient.ItemOperations(ctx, c.account.Email, &eas.ItemOperationsRequest{
		Fetch: eas.ItemOperationsFetch{
			FileReference: fileReference,
		},
	})
	if err != nil {
		return nil, "", fmt.Errorf("itemoperations fetch %q: %w", fileReference, err)
	}
	if resp.Status != 0 && resp.Status != 1 {
		return nil, "", fmt.Errorf("itemoperations status %d", resp.Status)
	}
	for _, f := range resp.Response.Fetch {
		if f.Status != 0 && f.Status != 1 {
			return nil, "", fmt.Errorf("fetch %q status %d", fileReference, f.Status)
		}
		if f.Properties.Data == "" {
			return nil, "", fmt.Errorf("fetch %q returned empty data", fileReference)
		}
		data, err := base64.StdEncoding.DecodeString(f.Properties.Data)
		if err != nil {
			return nil, "", fmt.Errorf("fetch %q: bad base64: %w", fileReference, err)
		}
		return data, f.Properties.ContentType, nil
	}
	return nil, "", fmt.Errorf("fetch %q: no response entry", fileReference)
}

// FetchMessageSource fetches the raw MIME source of a message from the server
// via ItemOperations (CollectionId + ServerId + MIMESupport=2). SOGo returns
// the message as base64 text in Response/Fetch/Properties — the part of Data
// that is the real source is the encoded MIME bytes, which we decode and
// return. This lets the UI show and explain the ACTUAL headers, not a local
// reconstruction.
func (c *Client) FetchMessageSource(ctx context.Context, collectionID, serverID string) (string, error) {
	if !c.connected || c.easClient == nil {
		return "", fmt.Errorf("not connected")
	}
	resp, err := c.easClient.ItemOperations(ctx, c.account.Email, &eas.ItemOperationsRequest{
		Fetch: eas.ItemOperationsFetch{
			CollectionId: collectionID,
			ServerId:     serverID,
			MIMESupport:  2,
		},
	})
	if err != nil {
		return "", fmt.Errorf("itemoperations fetch source: %w", err)
	}
	if resp.Status != 0 && resp.Status != 1 {
		return "", fmt.Errorf("itemoperations status %d", resp.Status)
	}
	for _, f := range resp.Response.Fetch {
		if f.Status != 0 && f.Status != 1 {
			return "", fmt.Errorf("fetch source status %d", f.Status)
		}
		// SOGo may return text directly or base64-encoded MIME bytes.
		if f.Properties.Data != "" {
			if decoded, err := base64.StdEncoding.DecodeString(f.Properties.Data); err == nil {
				return string(decoded), nil
			}
			return f.Properties.Data, nil
		}
	}
	return "", fmt.Errorf("fetch source: no data in response")
}

// SyncContacts syncs contacts from the contacts folder (two-phase, no BodyPreference)
func (c *Client) SyncContacts(ctx context.Context, collectionID string) ([]*models.Contact, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}

	items, _, err := twoPhaseSync[eas.Contact](ctx, c, collectionID, 100)
	if err != nil {
		return nil, err
	}
	var contacts []*models.Contact
	for _, item := range items {
		if item.ApplicationData == nil {
			continue
		}
		contact := easContactToModel(item.ApplicationData, c.account.ID, item.ServerID)
		contact.ID = fmt.Sprintf("%s-%s", c.account.ID, item.ServerID)
		contacts = append(contacts, contact)
	}
	return contacts, nil
}

// SyncCalendar syncs calendar events from the calendar folder (two-phase, no BodyPreference)
func (c *Client) SyncCalendar(ctx context.Context, collectionID string) ([]*models.CalendarEvent, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}

	// forceFull: the calendar read refresh always pulls the complete
	// collection so the store's replace-reconcile never wipes events that an
	// incremental (resuming-key) sync would omit.
	items, _, err := twoPhaseSyncMode[eas.Appointment](ctx, c, collectionID, 50, 0, true)
	if err != nil {
		return nil, err
	}
	var events []*models.CalendarEvent
	for _, item := range items {
		if item.ApplicationData == nil {
			continue
		}
		event := easCalendarToModel(item.ApplicationData, c.account.ID, item.ServerID)
		event.ID = fmt.Sprintf("%s-%s", c.account.ID, item.ServerID)
		events = append(events, event)
	}
	return events, nil
}

// CreateCalendarEvent creates a new appointment on the server and returns the
// newly-assigned ServerId. It establishes a fresh SyncKey for the collection
// (the local cache may be empty on first run), then issues a Sync Add command
// carrying the encoded Appointment, and reads the server's response for the
// new item id.
func (c *Client) CreateCalendarEvent(ctx context.Context, collectionID string, ev *models.CalendarEvent) (string, error) {
	if !c.connected {
		return "", fmt.Errorf("not connected")
	}

	// Build the Appointment payload for the client-side Add. Wrap times the
	// same way easCalendarToModel expects them back.
	appt := &eas.Appointment{
		Subject:        ev.Subject,
		Location:       ev.Location,
		BusyStatus:     int32(ev.BusyStatus),
		Sensitivity:    int32(ev.Sensitivity),
		AllDayEvent:    0,
		OrganizerEmail: c.account.Email,
	}
	if !ev.StartTime.IsZero() {
		appt.StartTime = ev.StartTime.UTC().Format("20060102T150405Z")
	}
	if !ev.EndTime.IsZero() {
		appt.EndTime = ev.EndTime.UTC().Format("20060102T150405Z")
	}
	if ev.AllDayEvent {
		appt.AllDayEvent = 1
	}
	encodeAttendees(appt, ev.Attendees)

	body, err := eas.CaptureApplicationDataBody(appt)
	if err != nil {
		return "", fmt.Errorf("encode appointment: %w", err)
	}

	// Establish a valid SyncKey for this collection.
	_, sk, err := twoPhaseSync[eas.Appointment](ctx, c, collectionID, 50)
	if err != nil {
		return "", fmt.Errorf("establish calendar sync key: %w", err)
	}

	newSid := fmt.Sprintf("new-%d", time.Now().UnixNano())
	add := eas.SyncAdd{
		ClientID:        newSid,
		ApplicationData: &wbxml.RawElement{Page: wbxml.PageAirSync, Bytes: body},
	}
	col := eas.SyncCollection{
		SyncKey:      sk,
		CollectionID: collectionID,
		GetChanges:   0,
		WindowSize:   50,
		Commands:     &eas.SyncCommands{Add: []eas.SyncAdd{add}},
	}
	req := &eas.SyncRequest{
		Collections: eas.SyncCollections{Collection: []eas.SyncCollection{col}},
	}
	resp, err := c.easClient.Sync(ctx, c.account.Email, req)
	if err != nil {
		return "", fmt.Errorf("create calendar event: %w", err)
	}
	for _, col := range resp.Collections.Collection {
		if col.Status != 0 && col.Status != 1 {
			return "", fmt.Errorf("calendar create status %d", col.Status)
		}
		if col.SyncKey != "" {
			c.syncKeys[collectionID] = col.SyncKey
		}
		if col.Responses != nil && len(col.Responses.Add) > 0 {
			if sid := col.Responses.Add[0].ServerID; sid != "" {
				return sid, nil
			}
		}
	}
	return "", fmt.Errorf("calendar create: no server id returned")
}

// UpdateCalendarEvent pushes a modified Appointment to the EAS calendar
// collection using a Sync Change command, returning the (possibly updated)
// ServerId on success.
func (c *Client) UpdateCalendarEvent(ctx context.Context, collectionID, serverID string, ev *models.CalendarEvent) (string, error) {
	if !c.connected {
		return "", fmt.Errorf("not connected")
	}
	appt := &eas.Appointment{
		Subject:        ev.Subject,
		Location:       ev.Location,
		BusyStatus:     int32(ev.BusyStatus),
		Sensitivity:    int32(ev.Sensitivity),
		AllDayEvent:    0,
		OrganizerEmail: c.account.Email,
	}
	if !ev.StartTime.IsZero() {
		appt.StartTime = ev.StartTime.UTC().Format("20060102T150405Z")
	}
	if !ev.EndTime.IsZero() {
		appt.EndTime = ev.EndTime.UTC().Format("20060102T150405Z")
	}
	if ev.AllDayEvent {
		appt.AllDayEvent = 1
	}
	encodeAttendees(appt, ev.Attendees)
	body, err := eas.CaptureApplicationDataBody(appt)
	if err != nil {
		return "", fmt.Errorf("encode appointment: %w", err)
	}

	_, sk, err := twoPhaseSync[eas.Appointment](ctx, c, collectionID, 50)
	if err != nil {
		return "", fmt.Errorf("establish calendar sync key: %w", err)
	}

	change := eas.SyncChange{
		ServerID:        serverID,
		ApplicationData: &wbxml.RawElement{Page: wbxml.PageAirSync, Bytes: body},
	}
	col := eas.SyncCollection{
		SyncKey:      sk,
		CollectionID: collectionID,
		GetChanges:   0,
		WindowSize:   50,
		Commands:     &eas.SyncCommands{Change: []eas.SyncChange{change}},
	}
	req := &eas.SyncRequest{
		Collections: eas.SyncCollections{Collection: []eas.SyncCollection{col}},
	}
	resp, err := c.easClient.Sync(ctx, c.account.Email, req)
	if err != nil {
		return "", fmt.Errorf("update calendar event: %w", err)
	}
	for _, col := range resp.Collections.Collection {
		if col.Status != 0 && col.Status != 1 {
			return "", fmt.Errorf("calendar update status %d", col.Status)
		}
		if col.SyncKey != "" {
			c.syncKeys[collectionID] = col.SyncKey
		}
		if col.Responses != nil && len(col.Responses.Change) > 0 {
			if sid := col.Responses.Change[0].ServerID; sid != "" {
				return sid, nil
			}
		}
	}
	return serverID, nil
}

// DeleteCalendarEvent removes an Appointment from the EAS calendar collection
// via a Sync Delete command.
func (c *Client) DeleteCalendarEvent(ctx context.Context, collectionID, serverID string) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	_, sk, err := twoPhaseSync[eas.Appointment](ctx, c, collectionID, 50)
	if err != nil {
		return fmt.Errorf("establish calendar sync key: %w", err)
	}

	del := eas.SyncDelete{ServerID: serverID}
	col := eas.SyncCollection{
		SyncKey:      sk,
		CollectionID: collectionID,
		GetChanges:   0,
		WindowSize:   50,
		Commands:     &eas.SyncCommands{Delete: []eas.SyncDelete{del}},
	}
	req := &eas.SyncRequest{
		Collections: eas.SyncCollections{Collection: []eas.SyncCollection{col}},
	}
	resp, err := c.easClient.Sync(ctx, c.account.Email, req)
	if err != nil {
		return fmt.Errorf("delete calendar event: %w", err)
	}
	for _, col := range resp.Collections.Collection {
		if col.Status != 0 && col.Status != 1 {
			return fmt.Errorf("calendar delete status %d", col.Status)
		}
		if col.SyncKey != "" {
			c.syncKeys[collectionID] = col.SyncKey
		}
	}
	return nil
}

// MoveEmails moves one or more emails into a destination folder on the
// server (EAS MoveItems — a true move, the source copy is removed). Each
// EmailMove identifies an item by server ID and its current folder. Returns
// an error if the server reports a per-item status other than Success.
func (c *Client) MoveEmails(ctx context.Context, moves []EmailMove) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}
	if len(moves) == 0 {
		return fmt.Errorf("no emails to move")
	}
	entries := make([]eas.MoveItemEntry, 0, len(moves))
	for _, m := range moves {
		entries = append(entries, eas.MoveItemEntry{
			SrcMsgID: m.ServerID,
			SrcFldID: m.SrcFolderID,
			DstFldID: m.DstFolderID,
		})
	}
	resp, err := c.easClient.MoveItems(ctx, c.account.Email, entries)
	if err != nil {
		return fmt.Errorf("move items: %w", err)
	}
	for _, r := range resp.Responses {
		if r.Status != int32(eas.StatusSuccess) && r.Status != 0 {
			return fmt.Errorf("move item %s failed with EAS status %d", r.SrcMsgID, r.Status)
		}
	}
	return nil
}

// CreateFolder creates a new user-visible mail folder (type 12). parentID is
// the EAS ServerID of the parent folder, or "0" for the mailbox root — pass
// a real parent to create a subfolder. Returns the new folder's server ID.
func (c *Client) CreateFolder(ctx context.Context, displayName, parentID string) (string, error) {
	if !c.connected {
		return "", fmt.Errorf("not connected")
	}
	if parentID == "" {
		parentID = "0"
	}
	resp, err := c.easClient.FolderCreate(ctx, c.account.Email, parentID, displayName, 12)
	if err != nil {
		return "", fmt.Errorf("create folder: %w", err)
	}
	if resp.ServerID == "" {
		return "", fmt.Errorf("folder create returned no server id")
	}
	return resp.ServerID, nil
}

// FileSentCopy files a copy of an SMTP-sent message into the server's Sent
// folder via an EAS Sync-Add — the same proven pattern CreateCalendarEvent
// uses. SendMail keeps raw SMTP for actual delivery (reliable, and the copy
// of record), then calls this so the message also exists in EAS Sent and
// shows up in Unibox. Returns the new server id when SOGo echoes one.
func (c *Client) FileSentCopy(ctx context.Context, sentCollectionID, fromAddr, to, subject, body string) (string, error) {
	if !c.connected {
		return "", fmt.Errorf("not connected")
	}

	email := &eas.Email{
		From:          fromAddr,
		To:            to,
		Subject:       subject,
		DateReceived:  time.Now().UTC().Format("20060102T150405Z"),
		Read:          true,
		Importance:    eas.ImportanceNormal,
		Body:          eas.AirSyncBaseBody{Type: eas.BodyTypePlain, Data: body},
		MessageClass:  "Note",
		ContentClass:  "urn:content-classes:message",
	}
	payload, err := eas.CaptureApplicationDataBody(email)
	if err != nil {
		return "", fmt.Errorf("encode sent copy: %w", err)
	}

	// Establish a valid SyncKey for the Sent collection (email keys are
	// stateless here — same "0"-resume as the read syncs).
	_, sk, err := twoPhaseSync[eas.Email](ctx, c, sentCollectionID, 50)
	if err != nil {
		return "", fmt.Errorf("establish sent sync key: %w", err)
	}

	newSid := fmt.Sprintf("new-%d", time.Now().UnixNano())
	add := eas.SyncAdd{
		ClientID:        newSid,
		ApplicationData: &wbxml.RawElement{Page: wbxml.PageAirSync, Bytes: payload},
	}
	col := eas.SyncCollection{
		SyncKey:      sk,
		CollectionID: sentCollectionID,
		GetChanges:   0,
		WindowSize:   50,
		Commands:     &eas.SyncCommands{Add: []eas.SyncAdd{add}},
	}
	resp, err := c.easClient.Sync(ctx, c.account.Email, &eas.SyncRequest{
		Collections: eas.SyncCollections{Collection: []eas.SyncCollection{col}},
	})
	if err != nil {
		return "", fmt.Errorf("file sent copy: %w", err)
	}
	for _, rc := range resp.Collections.Collection {
		if rc.Status != 0 && rc.Status != 1 {
			return "", fmt.Errorf("sent copy status %d", rc.Status)
		}
		if rc.SyncKey != "" {
			c.syncKeys[sentCollectionID] = rc.SyncKey
		}
		if rc.Responses != nil && len(rc.Responses.Add) > 0 {
			if sid := rc.Responses.Add[0].ServerID; sid != "" {
				return sid, nil
			}
		}
	}
	return "", nil
}

// EmailMove identifies a single email to move between folders.
type EmailMove struct {
	ServerID    string
	SrcFolderID string
	DstFolderID string
}

// EmailCopy identifies a single email to copy into a destination folder.
type EmailCopy struct {
	ServerID    string
	SrcFolderID string
	DstFolderID string
}

// CopyEmails copies one or more emails into a destination folder. ActiveSync
// has NO server-side CopyItems command (MoveItems moves, it does not copy), so
// the standard EAS-client approach is: fetch the source message's raw MIME and
// re-add it (Sync-Add) into the destination folder. Fetching as MIME preserves
// the original headers and body intact.
func (c *Client) CopyEmails(ctx context.Context, copies []EmailCopy) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}
	if len(copies) == 0 {
		return fmt.Errorf("no emails to copy")
	}
	for _, cp := range copies {
		source, err := c.FetchMessageSource(ctx, cp.SrcFolderID, cp.ServerID)
		if err != nil {
			return fmt.Errorf("copy fetch source of %s: %w", cp.ServerID, err)
		}
		if err := c.syncAddMessage(ctx, cp.DstFolderID, source); err != nil {
			return fmt.Errorf("copy %s to folder %s: %w", cp.ServerID, cp.DstFolderID, err)
		}
	}
	return nil
}

// syncAddMessage adds a raw MIME message into a folder via Sync-Add, the same
// proven pattern FileSentCopy/CreateCalendarEvent use. The MIME body is added
// verbatim so the message keeps its original headers and structure.
func (c *Client) syncAddMessage(ctx context.Context, collectionID, mimeSource string) error {
	email := &eas.Email{
		Body:          eas.AirSyncBaseBody{Type: eas.BodyTypeMIME, Data: mimeSource},
		MessageClass:  "IPM.Note",
		ContentClass:  "urn:content-classes:message",
	}
	payload, err := eas.CaptureApplicationDataBody(email)
	if err != nil {
		return fmt.Errorf("encode copied message: %w", err)
	}
	_, sk, err := twoPhaseSync[eas.Email](ctx, c, collectionID, 50)
	if err != nil {
		return fmt.Errorf("establish sync key for %s: %w", collectionID, err)
	}
	newSid := fmt.Sprintf("new-%d", time.Now().UnixNano())
	add := eas.SyncAdd{
		ClientID:        newSid,
		ApplicationData: &wbxml.RawElement{Page: wbxml.PageAirSync, Bytes: payload},
	}
	col := eas.SyncCollection{
		SyncKey:      sk,
		CollectionID: collectionID,
		GetChanges:   0,
		WindowSize:   50,
		Commands:     &eas.SyncCommands{Add: []eas.SyncAdd{add}},
	}
	resp, err := c.easClient.Sync(ctx, c.account.Email, &eas.SyncRequest{
		Collections: eas.SyncCollections{Collection: []eas.SyncCollection{col}},
	})
	if err != nil {
		return fmt.Errorf("sync-add copy: %w", err)
	}
	for _, rc := range resp.Collections.Collection {
		if rc.Status != 0 && rc.Status != 1 {
			return fmt.Errorf("sync-add copy status %d", rc.Status)
		}
		if rc.SyncKey != "" {
			c.syncKeys[collectionID] = rc.SyncKey
		}
	}
	return nil
}

// GetOOFSettings retrieves the Out-of-Office settings (placeholder)
func (c *Client) GetOOFSettings(ctx context.Context) (*models.OOFSettings, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}
	return &models.OOFSettings{State: "unknown"}, nil
}

// SetOOFSettings updates the Out-of-Office settings (not yet implemented)
func (c *Client) SetOOFSettings(ctx context.Context, settings *models.OOFSettings) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}
	return fmt.Errorf("OOF settings not yet implemented")
}

// === Generic two-phase sync for Contacts/Calendar (no BodyPreference) ===

func twoPhaseSync[T any](ctx context.Context, c *Client, collectionID string, windowSize int) ([]eas.TypedItem[T], string, error) {
	return twoPhaseSyncMode[T](ctx, c, collectionID, windowSize, 0, false)
}

func twoPhaseSyncMode[T any](ctx context.Context, c *Client, collectionID string, windowSize, retryCount int, forceFull bool) ([]eas.TypedItem[T], string, error) {
	if retryCount > 1 {
		return nil, "", fmt.Errorf("sync: too many retries for %s", collectionID)
	}

	// Resume from the persisted SyncKey when available; fall back to "0" (full
	// initial sync) only on first run or after a server-enforced reset. The
	// calendar/contacts read syncs pass forceFull so the replace-reconcile
	// always sees the complete collection (a resuming incremental sync would
	// return only changes and the reconcile would wipe everything else).
	phase1Key := "0"
	if !forceFull && c.loadSyncKey != nil {
		if stored, err := c.loadSyncKey(collectionID); err == nil && stored != "" {
			phase1Key = stored
		}
	}

	phase1 := eas.SyncCollection{
		SyncKey:      phase1Key,
		CollectionID: collectionID,
		GetChanges:   1,
		WindowSize:   int32(windowSize),
	}
	resp1, err := client.SyncTyped[T](ctx, c.easClient, c.account.Email, &eas.SyncRequest{
		Collections: eas.SyncCollections{Collection: []eas.SyncCollection{phase1}},
	})
	if err != nil {
		return nil, "", fmt.Errorf("sync phase 1 failed: %w", err)
	}
	for _, col := range resp1.Collections {
		if col.Status == 3 || col.Status == 13 {
			// Sync key was invalid/reset — forget it and retry from scratch
			if c.saveSyncKey != nil {
				c.saveSyncKey(collectionID, "")
			}
			return twoPhaseSyncMode[T](ctx, c, collectionID, windowSize, retryCount+1, forceFull)
		}
	}

	var syncKey string
	var items []eas.TypedItem[T]
	for _, col := range resp1.Collections {
		syncKey = col.SyncKey
		items = append(items, col.Add...)
		for _, ch := range col.Change {
			items = append(items, ch)
		}
	}

	// Phase 2 if phase 1 returned no data
	if len(items) == 0 && syncKey != "" {
		phase2 := eas.SyncCollection{
			SyncKey:      syncKey,
			CollectionID: collectionID,
			GetChanges:   1,
			WindowSize:   int32(windowSize),
		}
		resp2, err := client.SyncTyped[T](ctx, c.easClient, c.account.Email, &eas.SyncRequest{
			Collections: eas.SyncCollections{Collection: []eas.SyncCollection{phase2}},
		})
		if err != nil {
			return nil, syncKey, fmt.Errorf("sync phase 2 failed: %w", err)
		}
		for _, col := range resp2.Collections {
			if col.Status == 3 || col.Status == 13 {
				if c.saveSyncKey != nil {
					c.saveSyncKey(collectionID, "")
				}
				return twoPhaseSyncMode[T](ctx, c, collectionID, windowSize, retryCount+1, forceFull)
			}
			syncKey = col.SyncKey
			items = append(items, col.Add...)
			for _, ch := range col.Change {
				items = append(items, ch)
			}
		}
	}

	c.syncKeys[collectionID] = syncKey
	if syncKey != "" && c.saveSyncKey != nil {
		c.saveSyncKey(collectionID, syncKey)
	}
	return items, syncKey, nil
}

// === Conversion helpers ===

// encodeAttendees populates an Appointment's Attendees element from the event's
// invitee email list (each as a required attendee).
func encodeAttendees(appt *eas.Appointment, emails []string) {
	if len(emails) == 0 {
		return
	}
	set := map[string]bool{}
	var out []eas.Attendee
	for _, e := range emails {
		e = strings.TrimSpace(e)
		if e == "" || set[e] {
			continue
		}
		set[e] = true
		out = append(out, eas.Attendee{Email: e, AttendeeStatus: 0, AttendeeType: 1})
	}
	if len(out) > 0 {
		appt.Attendees = &eas.Attendees{Attendee: out}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func bodyTypeStr(t int32) string {
	switch t {
	case eas.BodyTypeHTML:
		return "html"
	case eas.BodyTypeRTF:
		return "html"
	case eas.BodyTypePlain:
		return "text"
	case eas.BodyTypeMIME:
		// SOGo returns raw MIME (type 4) for S/MIME/PGP-signed mail and when
		// MIMESupport=2. Label it so the UI can render/offer it distinctly.
		return "mime"
	default:
		return "text"
	}
}

func easEmailToModel(e *eas.Email, accountID, folderID string) *models.Email {
	email := &models.Email{
		AccountID:     accountID,
		FolderID:      fmt.Sprintf("%s-%s", accountID, folderID),
		From:          e.From,
		FromEmail:     ParseEmailAddress(e.From),
		To:            e.To,
		ToEmails:      ParseEmailList(e.To),
		CC:            e.Cc,
		CCEmails:      ParseEmailList(e.Cc),
		Subject:       e.Subject,
		ThreadTopic:   e.ThreadTopic,
		IsRead:        e.Read,
		IsFlagged:     e.FlagStatus == 2,
		Importance:    int(e.Importance),
		HasAttachment: e.HasAttach || len(e.Attachments.Attachment) > 0,
		Preview:       e.Preview,
		Body:          e.Body.Data,
		BodyType:      bodyTypeStr(e.Body.Type),
	}
	// Map SOGo's AirSyncBase.Attachments block (the authoritative source;
	// SOGo does NOT send Email2.HasAttachment).
	for _, a := range e.Attachments.Attachment {
		email.Attachments = append(email.Attachments, models.EmailAttachment{
			DisplayName:       a.DisplayName,
			FileReference:     a.FileReference,
			ContentId:         a.ContentId,
			IsInline:          a.IsInline == 1,
			Method:            int(a.Method),
			EstimatedDataSize: int(a.EstimatedDataSize),
		})
	}
	// Parse DateReceived
	if e.DateReceived != "" {
		if t, err := parseEASTime(e.DateReceived); err == nil {
			email.DateReceived = t
		}
	}
	// If no body but we have preview, use preview as fallback
	if email.Body == "" && email.Preview != "" {
		email.Body = email.Preview
		email.BodyType = "text"
	}
	// If body type is "text" but content looks like HTML, promote it
	if email.BodyType == "text" && strings.Contains(email.Body, "<") && strings.Contains(email.Body, ">") {
		email.BodyType = "html"
	}
	// Strip HTML tags from preview
	if email.Preview != "" && strings.Contains(email.Preview, "<") {
		email.Preview = stripHTML(email.Preview)
	}
	if email.Subject == "" {
		email.Subject = "(no subject)"
	}
	return email
}

func easContactToModel(c *eas.Contact, accountID, serverID string) *models.Contact {
	name := strings.TrimSpace(fmt.Sprintf("%s %s", c.FirstName, c.LastName))
	if name == "" {
		name = c.Email1Address
	}
	email := c.Email1Address
	if email == "" {
		email = c.Email2Address
	}
	if email == "" {
		email = c.Email3Address
	}

	contact := &models.Contact{
		AccountID:  accountID,
		Name:       name,
		FirstName:  c.FirstName,
		LastName:   c.LastName,
		Email:      email,
		Email2:     c.Email2Address,
		Email3:     c.Email3Address,
		Phone:      c.HomePhoneNumber,
		Mobile:     c.MobilePhoneNumber,
		Company:    c.CompanyName,
		JobTitle:   c.JobTitle,
		ServerID:   serverID,
		AvatarURL:  GravatarURL(email),
	}
	contact.ID = fmt.Sprintf("%s-%s", accountID, serverID)
	return contact
}

func easCalendarToModel(a *eas.Appointment, accountID, serverID string) *models.CalendarEvent {
	event := &models.CalendarEvent{
		AccountID:      accountID,
		ServerID:       serverID,
		Subject:        a.Subject,
		Location:       a.Location,
		AllDayEvent:    a.AllDayEvent == 1,
		OrganizerName:  a.OrganizerName,
		OrganizerEmail: a.OrganizerEmail,
		BusyStatus:     int(a.BusyStatus),
		Sensitivity:    int(a.Sensitivity),
		Reminder:       int(a.Reminder),
		Body:           "",
		BodyType:       "text",
		RecurrenceType: 0,
	}
	event.ID = fmt.Sprintf("%s-%s", accountID, serverID)

	if a.StartTime != "" {
		event.StartTime, _ = parseEASTime(a.StartTime)
	}
	if a.EndTime != "" {
		event.EndTime, _ = parseEASTime(a.EndTime)
	}
	if a.Attendees != nil {
		for _, at := range a.Attendees.Attendee {
			if at.Email != "" {
				event.Attendees = append(event.Attendees, at.Email)
			}
		}
	}
	return event
}

func GravatarURL(email string) string {
	if email == "" {
		return ""
	}
	return fmt.Sprintf("https://seccdn.libravatar.org/avatar/%x?s=128&d=identicon",
		md5.Sum([]byte(strings.TrimSpace(strings.ToLower(email)))))
}

func ParseEmailAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if i := strings.Index(addr, "<"); i >= 0 {
		if j := strings.Index(addr, ">"); j > i {
			return addr[i+1 : j]
		}
	}
	return addr
}

func ParseEmailList(list string) []string {
	if list == "" {
		return nil
	}
	parts := strings.Split(list, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		email := ParseEmailAddress(strings.TrimSpace(p))
		if email != "" {
			result = append(result, email)
		}
	}
	return result
}

func stripHTML(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

func parseEASTime(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"20060102T150405Z",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}