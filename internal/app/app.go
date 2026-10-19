package app

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"log"
	"net/mail"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"easymail/internal/activesync"
	"easymail/internal/ai"
	"easymail/internal/models"
	"easymail/internal/store"
)

// version is set at build time via -ldflags "-X easymail/internal/app.version=...".
// (The old -X main.version=... was a silent no-op because main has no version var.)
var version = "0.5.3"

// GetVersion returns the current build number so the UI can show it.
func (a *App) GetVersion() string {
	return version
}

// App is the main application context, bound to the frontend
type App struct {
	ctx     context.Context
	store   *store.Store
	clients map[string]*activesync.Client // account ID -> client
	win     *application.WebviewWindow    // main window handle (SetWindow, for badge title)

	// syncMu serializes every mail sync (frontend-triggered, the daily
	// reconcile, and the fast poll) so SOGo never sees two concurrent drains
	// on the same folder — concurrent windows make it return 405 and the
	// loser aborts mid-window, leaving the store on stale mail.
	syncMu sync.Mutex

	// activityLog is the human-readable activity log file (received/sent mail
	// + EAS/SMTP server comms); lazily opened on first write.
	activityLog  *os.File
	activityPath string
	activityMu   sync.Mutex

	// Self-healing state: healMu single-flights a repair, healing blocks
	// re-entry while one is running (a poll round can trip several corrupt
	// writes before the heal finishes).
	healMu  sync.Mutex
	healing atomic.Bool
}

// New creates a new App instance
func New() (*App, error) {
	configDir, err := dataDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find config dir: %w", err)
	}
	dbPath := filepath.Join(configDir, "boi", "boi.db")
	log.Printf("BOI: config dir=%s, db path=%s", configDir, dbPath)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, fmt.Errorf("cannot create config dir %s: %w", filepath.Dir(dbPath), err)
	}
	// Mobile: remember last-good server IPs so syncs survive DNS blips
	// (phone doze / VPN sleep make lookups fail intermittently).
	activesync.SetDNSCachePath(filepath.Join(filepath.Dir(dbPath), "dns-cache.json"))

	// Remove stale WAL/SHM files from previous runs
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")

	s, err := store.New(dbPath)
	if err != nil {
		log.Printf("BOI: database open failed for %s: %v", dbPath, err)
		return nil, fmt.Errorf("cannot open database: %w", err)
	}

	a := &App{
		store:   s,
		clients: make(map[string]*activesync.Client),
	}

	// Integrity gate: SQLite happily opens a corrupt file, and sync writes
	// into a damaged btree then fail silently for days (seen 2026-10-02 and
	// 2026-10-15). quick_check before anything else touches the DB; heal now
	// if it fails — mail is a cache, the server re-backfills whatever a
	// recovery cannot salvage.
	var healNote string
	if err := s.IntegrityCheck(); err != nil {
		log.Printf("BOI: database integrity check FAILED: %v", err)
		how, rerr := s.RecoverNow()
		if rerr != nil {
			log.Printf("BOI: database self-heal failed: %v (continuing with fresh DB)", rerr)
		} else {
			log.Printf("BOI: database self-heal done via %s", how)
			healNote = how
		}
		if err := s.IntegrityCheck(); err != nil {
			log.Printf("BOI: database STILL unhealthy after self-heal: %v", err)
		}
	}

	// Snapshot the freshly-opened (and now known-good) DB so a later
	// corruption can't cost more than this session's mail. Taken BEFORE any
	// account connects or sync writes, so the copy is a clean point-in-time.
	// We keep the newest N (pruneBackups).
	if err := backupDatabase(s.Path()); err != nil {
		log.Printf("BOI: startup db backup failed (non-fatal): %v", err)
	}

	// Activity log lives next to the DB — same dir, human-readable file.
	// Kept separate from stdout/journald so the user can tail/review
	// received/sent mail + server comms without app logs mixed in.
	a.activityPath = filepath.Join(filepath.Dir(dbPath), "activity.log")
	if err := a.logActivity("BOI startup"); err != nil {
		log.Printf("BOI: activity log init failed (non-fatal): %v", err)
	}
	if healNote != "" {
		a.logActivity("DB CORRUPTION found at startup — self-healed via " + healNote)
	}

	return a, nil
}

// backupDatabase snapshots the SQLite DB to a timestamped sibling file
// (boi.db.bak-YYYYMMDD-HHMMSS) so a future corruption can't cost more than
// the last snapshot. Best-effort WAL checkpoint first so the copy includes
// recent commits that still live in the -wal file. Runs at startup (on a
// known-good DB, before any sync writes) and daily before the reconcile; the
// newest few are kept by pruneBackups.
func backupDatabase(dbPath string) error {
	if db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000"); err == nil {
		_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		db.Close()
	}
	data, err := os.ReadFile(dbPath)
	if err != nil {
		return fmt.Errorf("read db for backup: %w", err)
	}
	ts := time.Now().Format("20060102-150405")
	backup := fmt.Sprintf("%s.bak-%s", dbPath, ts)
	if err := os.WriteFile(backup, data, 0o600); err != nil {
		return fmt.Errorf("write backup %s: %w", backup, err)
	}
	log.Printf("BOI: db backup written -> %s (%d bytes)", backup, len(data))
	pruneBackups(dbPath)
	return nil
}

// pruneBackups keeps the newest 3 startup backups and the newest 3 corrupt
// quarantines (each including any -wal/-shm sidecars) and deletes older ones
// so a ~260MB DB directory can't grow unbounded.
func pruneBackups(dbPath string) {
	const keep = 3
	for _, prefix := range []string{dbPath + ".bak-", dbPath + ".corrupt-"} {
		groups := map[string]time.Time{}
		entries, _ := filepath.Glob(prefix + "*")
		for _, e := range entries {
			base := strings.TrimSuffix(strings.TrimSuffix(e, "-wal"), "-shm")
			if fi, err := os.Stat(base); err == nil && fi.ModTime().After(groups[base]) {
				groups[base] = fi.ModTime()
			}
		}
		bases := make([]string, 0, len(groups))
		for b := range groups {
			bases = append(bases, b)
		}
		sort.Slice(bases, func(i, j int) bool { return groups[bases[i]].After(groups[bases[j]]) })
		if len(bases) > keep {
			for _, stale := range bases[keep:] {
				for _, suffix := range []string{"", "-wal", "-shm"} {
					os.Remove(stale + suffix)
				}
				if stale != "" {
					log.Printf("BOI: pruned old backup/quarantine %s", filepath.Base(stale))
				}
			}
		}
	}
}

// selfHeal repairs a corrupted local database in place and swaps the
// store's underlying handle so the app keeps running without a restart.
// Called wherever SQLite reports corruption (poll round, sync saves).
// Single-flighted via healing/healMu so concurrent callers don't double-heal.
func (a *App) selfHeal(trigger string) {
	if !a.healing.CompareAndSwap(false, true) {
		return // a heal is already running
	}
	defer a.healing.Store(false)
	a.healMu.Lock()
	defer a.healMu.Unlock()

	log.Printf("selfHeal: database corruption detected (via %s) — repairing", trigger)
	a.logActivity("DB CORRUPTION detected (via " + trigger + ") — self-healing")
	how, err := a.store.RecoverNow()
	if err != nil {
		log.Printf("selfHeal: FAILED: %v", err)
		a.logActivity("DB self-heal FAILED: " + err.Error())
		return
	}
	log.Printf("selfHeal: done via %s", how)
	a.logActivity("DB self-heal complete via " + how + " — resyncing from server")
}

// emitEvent publishes a UI event on the v3 application event bus. Safe to
// call at any time — a no-op before the application instance exists (e.g.
// during startup sync races).
func (a *App) emitEvent(name string, data ...any) {
	if w := application.Get(); w != nil {
		w.Event.Emit(name, data...)
	}
}

// saveFileDialog asks the user where to save a file. v3 builder API;
// returning an empty string (with nil error) means the user cancelled,
// which callers treat as a clean skip.
func (a *App) saveFileDialog(title, directory, filename string) (string, error) {
	w := application.Get()
	if w == nil {
		return "", fmt.Errorf("app not running")
	}
	d := w.Dialog.SaveFile()
	d.SetOptions(&application.SaveFileDialogOptions{
		Title:                title,
		Directory:            directory,
		Filename:             filename,
		CanCreateDirectories: true,
	})
	return d.PromptForSingleSelection()
}

// Startup is called when the app starts
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("BOI starting up")
	go startUpdateWatcher(a)

	accounts, err := a.store.ListAccounts()
	if err != nil {
		log.Printf("Warning: cannot load accounts: %v", err)
		return
	}
	for _, acc := range accounts {
		go a.connectAccount(acc)
	}
	// Daily full-window reconcile: catches deletions older than the 24h
	// window that the normal fast sync uses.
	go a.startDailyReconcile()
	// Fast background poll: keeps new mail flowing in ~every 60s without
	// the user touching anything.
	go a.startMailPolling()
}

// ServiceStartup implements the Wails v3 service lifecycle (was v2's
// OnStartup hook).
func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.Startup(ctx)
	return nil
}

// ServiceShutdown implements the Wails v3 service lifecycle (was v2's
// OnShutdown hook).
func (a *App) ServiceShutdown() {
	log.Println("BOI shutting down")
	a.Shutdown()
}

// Shutdown is called on exit
func (a *App) Shutdown() {
	if a.activityLog != nil {
		a.activityLog.Close()
		a.activityLog = nil
	}
	if a.store != nil {
		a.store.Close()
	}
}

// logActivity appends one timestamped human-readable line to the activity
// log (next to the DB at ~/.config/boi/activity.log). The file is opened
// lazily and appends under a mutex so concurrent poll/sync/send can't
// interleave lines. Failures are logged to stderr but never fatal — an
// unwritable activity log must not break mail.
func (a *App) logActivity(line string) error {
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	if a.activityPath == "" {
		return nil
	}
	if a.activityLog == nil {
		f, err := os.OpenFile(a.activityPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
		a.activityLog = f
	}
	// RFC3339 with local offset; no microseconds — keeps it readable.
	_, err := fmt.Fprintf(a.activityLog, "%s  %s\n", time.Now().Format("2006-01-02 15:04:05"), line)
	if err != nil {
		return err
	}
	return a.activityLog.Sync()
}

// GetActivityLog returns the tail of the activity log (newest first) for the
// in-app Activity viewer. It reads the file back so it also works on a fresh
// app start where `lines` weren't all logged this session.
func (a *App) GetActivityLog(limit int) ([]string, error) {
	if limit <= 0 {
		limit = 200
	}
	if a.activityPath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(a.activityPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{"No activity yet."}, nil
		}
		return nil, err
	}
	all := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	// Newest first, capped.
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all, nil
}

// === Account Management ===

// AddAccount adds a new email account
// sanitizeServerURL cleans up a user-typed EAS endpoint URL. Mobile keyboards
// routinely autocorrect '.' to ',' in hostnames and drop the scheme; both
// break DNS at first connect with a confusing "no such host". Trim, default
// the scheme to https, and fix comma typos in the host portion.
func sanitizeServerURL(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return s
	}
	rest := s
	i := strings.Index(s, "://")
	if i >= 0 {
		rest = s[i+3:]
	} else {
		rest = s
		s = "https://" + s
	}
	// Host runs to the first '/'; EAS hostnames never contain commas.
	if j := strings.Index(rest, "/"); j >= 0 {
		s = s[:len(s)-len(rest)] + strings.ReplaceAll(rest[:j], ",", ".") + rest[j:]
	} else {
		s = s[:len(s)-len(rest)] + strings.ReplaceAll(rest, ",", ".")
	}
	return s
}

func (a *App) AddAccount(name, email, password, serverURL string) (*models.Account, error) {
	serverURL = sanitizeServerURL(serverURL)
	log.Printf("AddAccount called: name=%q email=%q server=%q", name, email, serverURL)
	acc := &models.Account{
		Name:       name,
		Email:      email,
		Password:   password,
		ServerURL:  serverURL,
		DeviceID:   activesync.GenerateDeviceID(),
		DeviceType: "BOI",
	}
	// Generate a stable ID
	if acc.ID == "" {
		acc.ID = fmt.Sprintf("acc-%d", time.Now().UnixMilli())
	}
	log.Printf("AddAccount: created account ID=%s DeviceID=%s", acc.ID, acc.DeviceID)
	if err := a.store.SaveAccount(acc); err != nil {
		log.Printf("AddAccount: save failed: %v", err)
		return nil, fmt.Errorf("cannot save account: %w", err)
	}
	log.Printf("AddAccount: saved, starting connectAccount goroutine")
	go a.connectAccount(acc)
	return acc, nil
}

// UpdateAccount edits an existing account (name/email/password/serverURL),
// preserving its ID and device identity, then reconnects.
func (a *App) UpdateAccount(id, name, email, password, serverURL string) (*models.Account, error) {
	serverURL = sanitizeServerURL(serverURL)
	log.Printf("UpdateAccount called: id=%q email=%q server=%q", id, email, serverURL)
	accounts, err := a.store.ListAccounts()
	if err != nil {
		return nil, fmt.Errorf("cannot load accounts: %w", err)
	}
	var acc *models.Account
	for _, ac := range accounts {
		if ac.ID == id {
			acc = ac
			break
		}
	}
	if acc == nil {
		return nil, fmt.Errorf("account not found: %s", id)
	}
	// Keep existing client/device identity; refresh the editable fields.
	if old, ok := a.clients[id]; ok {
		old.Close()
		delete(a.clients, id)
	}
	acc.Name = name
	acc.Email = email
	// The settings form can't read back stored passwords, so it sends ""
	// when the user leaves the field untouched. An unconditional overwrite
	// would blank the stored password on any unrelated account edit (name,
	// server) and silently kill the account's connection. Empty = keep.
	if password != "" {
		acc.Password = password
	} else {
		log.Printf("UpdateAccount: kept existing password for %s (field empty)", id)
	}
	if serverURL != "" {
		acc.ServerURL = serverURL
	}
	acc.Connected = false
	if err := a.store.SaveAccount(acc); err != nil {
		return nil, fmt.Errorf("cannot save account: %w", err)
	}
	log.Printf("UpdateAccount: saved account %s, reconnecting", acc.ID)
	go a.connectAccount(acc)
	return acc, nil
}

// RemoveAccount removes an account
func (a *App) RemoveAccount(id string) error {
	if client, ok := a.clients[id]; ok {
		client.Close()
		delete(a.clients, id)
	}
	return a.store.DeleteAccount(id)
}

// ListAccounts returns all configured accounts
func (a *App) ListAccounts() ([]*models.Account, error) {
	return a.store.ListAccounts()
}

// === Folder Operations ===

// GetFolders returns all synced folders for an account
func (a *App) GetFolders(accountID string) ([]*models.Folder, error) {
	result, err := a.store.ListFolders(accountID)
	log.Printf("GetFolders: account=%s returned %d folders", accountID, len(result))
	return result, err
}

// SyncFolders triggers a folder sync for an account
func (a *App) SyncFolders(accountID string) error {
	log.Printf("SyncFolders: account=%s", accountID)
	client, ok := a.clients[accountID]
	if !ok {
		log.Printf("SyncFolders: account %s not connected", accountID)
		return fmt.Errorf("account not connected")
	}
	folders, err := client.SyncFolders(a.ctx)
	if err != nil {
		log.Printf("SyncFolders: sync failed: %v", err)
		return fmt.Errorf("folder sync failed: %w", err)
	}
	log.Printf("SyncFolders: got %d folders", len(folders))
	for _, f := range folders {
		log.Printf("  folder: type=%d name=%q serverId=%q id=%q", f.Type, f.Name, f.ServerID, f.ID)
		if err := a.store.SaveFolder(f); err != nil {
			log.Printf("Warning: cannot save folder %s: %v", f.ServerID, err)
		}
	}
	a.emitEvent("folders-updated", accountID)
	return nil
}

// === Email Operations ===

// GetEmails returns emails for a folder
func (a *App) GetEmails(accountID, folderID string, offset, limit int) ([]*models.Email, error) {
	log.Printf("GetEmails: account=%s folderID=%s offset=%d limit=%d", accountID, folderID, offset, limit)
	result, err := a.store.ListEmails(accountID, folderID, offset, limit)
	log.Printf("GetEmails: returned %d emails", len(result))
	return result, err
}

// GetEmailBody returns body and body_type for a single email (lists carry
// preview only to keep bridge payloads small).
func (a *App) GetEmailBody(accountID, emailID string) (*models.Email, error) {
	return a.store.GetEmailBody(accountID, emailID)
}

// GetEmailsByContact returns all emails exchanged with a contact
func (a *App) GetEmailsByContact(accountID, contactEmail string, offset, limit int) ([]*models.Email, error) {
	return a.store.ListEmailsByContact(accountID, contactEmail, offset, limit)
}

// SyncEmails triggers an email sync for a folder.
//
// Two-pass strategy (SOGo behaviour we've observed live):
//   - Tight window (FilterType 1, last 24h) returns EVERYTHING fresh — the
//     13 messages that arrived today are only visible here. The 405 we saw
//     on unbounded syncs and the stale-until-9/11 results on the 180-day
//     window make it the ONLY reliable way to catch new mail.
//   - Wide window (FilterType 7, last 180 days) backfills history. SOGo's
//     cached snapshot can lag (stopped at 9/11 while tight showed 9/12), so
//     the tight result is saved first and the wide pass only ADDS older
//     items. SaveEmail is INSERT OR REPLACE on server_id, so overlapping
//     windows never duplicate rows.
func (a *App) SyncEmails(accountID, serverID string) error {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	log.Printf("SyncEmails: account=%s serverID=%s", accountID, serverID)
	client, ok := a.clients[accountID]
	if !ok {
		log.Printf("SyncEmails: account %s not connected, clients map has %d entries", accountID, len(a.clients))
		return fmt.Errorf("account not connected")
	}

	// Stateless windowed sync. SOGo rejects resumed (persisted-key) email
	// syncs with status 3 whenever Options are attached, so we do NOT persist
	// email SyncKeys — instead we pick the filter window by what we already
	// have locally: empty folder ⇒ wide 180-day backfill (first run, slow but
	// one-off); folder with cached mail ⇒ narrow 24h window, which SOGo
	// serves in seconds even from SyncKey "0". Cached history stays, new and
	// deleted mail is caught in the 24h window, and no stale key ever forces
	// a repeated 5-minute re-drain on restart.
	filter := int32(1) // 1 = last 24h
	folderID := accountID + "-" + serverID
	count, err := a.store.CountEmails(folderID)
	if err == nil && count == 0 {
		filter = 7 // empty store: pull the 180-day window once
	}
	emails, deletedIDs, err := client.SyncEmailsWithDeletes(a.ctx, serverID, filter)
	if err != nil {
		log.Printf("SyncEmails: sync failed for %s: %v", serverID, err)
		return fmt.Errorf("email sync failed: %w", err)
	}
	log.Printf("SyncEmails: filter=%d got %d emails (%d deleted) for folder %s", filter, len(emails), len(deletedIDs), serverID)
	a.applySyncResults(accountID, serverID, "manual", emails, deletedIDs)
	a.emitEvent("emails-updated", accountID, serverID)
	return nil
}

// applySyncResults saves newly-synced emails and mirrors server-side
// deletions (EAS Sync Delete commands) so mail removed on the server
// disappears from the local store instead of lingering forever. It also logs
// genuinely-new arrivals to the activity log (a resync rewrites existing rows
// via INSERT OR REPLACE, so EmailExists distinguishes real arrivals from
// refresh noise) — trigger describes where the sync came from (poll/backfill/
// manual) for the activity record.
func (a *App) applySyncResults(accountID, serverID string, trigger string, emails []*models.Email, deletedIDs []string) {
	folderID := accountID + "-" + serverID
	fname := a.folderName(accountID, serverID)
	var n, newN int
	var firstSaveErr error
	for _, e := range emails {
		isNew := false
		if ok, err := a.store.EmailExists(folderID, e.ServerID); err != nil {
			// A corrupt DB used to make this error read as "not new",
			// silently dropping the RECV audit trail — treat corruption as
			// the emergency it is.
			if store.IsCorruption(err) {
				a.selfHeal("exists-check")
			}
		} else {
			isNew = !ok
		}
		if err := a.store.SaveEmail(e); err != nil {
			if store.IsCorruption(err) {
				a.selfHeal("save-email")
			}
			log.Printf("Warning: cannot save email %s: %v", e.ServerID, err)
			if firstSaveErr == nil {
				firstSaveErr = err
				a.logActivity(fmt.Sprintf("SYNC-ERROR folder=%s email=%s: %v", fname, e.ServerID, err))
			}
		} else {
			n++
			if isNew {
				newN++
				who := e.FromEmail
				if who == "" {
					who = e.From
				}
				if err := a.logActivity(fmt.Sprintf("RECV folder=%s from=%s subject=%q via=%s", fname, who, e.Subject, trigger)); err != nil {
					log.Printf("Warning: activity log write failed: %v", err)
				}
			}
		}
	}
	if n > 0 {
		log.Printf("applySyncResults: saved %d emails (%d new) for folder %s", n, newN, folderID)
	}
	if len(deletedIDs) > 0 {
		if err := a.store.DeleteEmails(folderID, deletedIDs); err != nil {
			log.Printf("applySyncResults: failed to apply %d deletions for %s: %v", len(deletedIDs), folderID, err)
		}
	}
	// New-mail alert: UI event (frontend chime) + platform signal (Android
	// notification, macOS dock bounce). Gated to background poll arrivals:
	// manual syncs already refresh the visible list, and a first-connect
	// backfill can drop hundreds of mails at once — no siren for those.
	if trigger == "poll" && newN > 0 {
		a.emitEvent("new-mail", newN)
		a.notifyNewMail(newN)
	}
}

// folderName resolves a folder's display name from its ServerID for use in
// readable activity-log lines.
func (a *App) folderName(accountID, serverID string) string {
	folders, err := a.store.ListFolders(accountID)
	if err != nil {
		return serverID
	}
	for _, f := range folders {
		if f.ServerID == serverID {
			return f.Name
		}
	}
	return serverID
}

// FetchAttachment downloads an attachment via ItemOperations Fetch and
// returns the bytes (base64-encoded for transport), the MIME content type,
// and a locally-generated filename derived from the FileReference.
func (a *App) FetchAttachment(accountID, fileReference string) (map[string]string, error) {
	log.Printf("FetchAttachment: account=%s ref=%s", accountID, fileReference)
	c, ok := a.clients[accountID]
	if !ok {
		return nil, fmt.Errorf("account not connected")
	}
	data, mime, err := c.FetchAttachment(a.ctx, fileReference)
	if err != nil {
		log.Printf("FetchAttachment: failed: %v", err)
		return nil, fmt.Errorf("attachment fetch failed: %w", err)
	}
	_, filename := filepath.Split(fileReference)
	if filename == "" {
		filename = "attachment"
	}
	return map[string]string{
		"data":        base64.StdEncoding.EncodeToString(data),
		"contentType": mime,
		"filename":    filename,
	}, nil
}

// DownloadAttachment fetches an attachment and writes it to the user's
// Downloads directory, returning the absolute path it was saved to.
//
// The webview cannot save files via a synthetic <a download> click (WebKitGTK
// ignores it), so the actual write happens here in Go. The frontend then
// surfaces the saved path to the user.
func (a *App) DownloadAttachment(accountID, fileReference, suggestedName string) (string, error) {
	log.Printf("DownloadAttachment: account=%s ref=%s name=%q", accountID, fileReference, suggestedName)
	c, ok := a.clients[accountID]
	if !ok {
		return "", fmt.Errorf("account not connected")
	}
	data, mime, err := c.FetchAttachment(a.ctx, fileReference)
	if err != nil {
		log.Printf("DownloadAttachment: failed: %v", err)
		return "", fmt.Errorf("attachment fetch failed: %w", err)
	}

	// Sanitise the suggested name (avoid path traversal / odd chars, keep the
	// extension). Decide the extension from the MIME type if the name has none.
	filename := sanitizeFilename(suggestedName)
	if filename == "" {
		filename = "attachment"
	}
	if filepath.Ext(filename) == "" {
		if ext := extForMIME(mime); ext != "" {
			filename += ext
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	defaultDir := filepath.Join(home, "Downloads")

	// Ask the user where to save — native save dialog. Returning an empty
	// string means the user cancelled, which we treat as a clean "skip"
	// (empty path, nil error) rather than a failure.
	path, err := a.saveFileDialog("Save attachment", defaultDir, filename)
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return "", nil // cancelled
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// GetMessageSource fetches the REAL raw MIME source of an email from the
// server (ItemOperations, MIMESupport=2) so the UI can break down and
// translate the actual headers rather than a locally-reconstructed block.
// folderServerId is the EAS CollectionId; serverId is the message ServerId.
func (a *App) GetMessageSource(accountID, folderServerId, serverId string) (string, error) {
	client, ok := a.clients[accountID]
	if !ok {
		return "", fmt.Errorf("account not connected")
	}
	return client.FetchMessageSource(a.ctx, folderServerId, serverId)
}

// SaveHTMLAs writes arbitrary text content (e.g. a message body that
// references external/inline sources) to a user-chosen file via the native
// save dialog. Returns the saved path, or "" if the user cancelled.
func (a *App) SaveHTMLAs(content, suggestedName string) (string, error) {
	log.Printf("SaveHTMLAs: name=%q len=%d", suggestedName, len(content))
	filename := sanitizeFilename(suggestedName)
	if filename == "" {
		filename = "message.html"
	}
	if filepath.Ext(filename) == "" {
		filename += ".html"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	defaultDir := filepath.Join(home, "Downloads")
	path, err := a.saveFileDialog("Save message source", defaultDir, filename)
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return "", nil // cancelled
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// sanitizeFilename keeps only safe characters; returns "" if nothing usable.
func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	for _, r := range "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\v\f\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f" {
		name = strings.ReplaceAll(name, string(r), "")
	}
	name = strings.Trim(name, ". ") // no trailing dot/space, no ".."
	if name == ".." || name == "." {
		return ""
	}
	return name
}

// extForMIME returns a file extension for a MIME content type, or "" if
// unknown. Only the common cases needed for attachments are covered.
func extForMIME(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "application/pdf":
		return ".pdf"
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "application/zip":
		return ".zip"
	case "text/plain":
		return ".txt"
	case "text/html":
		return ".html"
	case "application/msword":
		return ".doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.ms-excel":
		return ".xls"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	default:
		return ""
	}
}

// === Contact Operations ===

// GetContacts returns all contacts sorted by last email date
func (a *App) GetContacts(accountID string) ([]*models.Contact, error) {
	return a.store.ListContactsByLastEmail(accountID)
}

// SearchContacts searches contacts by name or email
func (a *App) SearchContacts(accountID, query string) ([]*models.Contact, error) {
	return a.store.SearchContacts(accountID, query)
}

// SyncContacts syncs contacts from the EAS contacts folder
func (a *App) SyncContacts(accountID, serverID string) error {
	client, ok := a.clients[accountID]
	if !ok {
		return fmt.Errorf("account not connected")
	}
	contacts, err := client.SyncContacts(a.ctx, serverID)
	if err != nil {
		log.Printf("SyncContacts: failed for %s: %v", serverID, err)
		return fmt.Errorf("contact sync failed: %w", err)
	}
	log.Printf("SyncContacts: got %d contacts for folder %s", len(contacts), serverID)
	for _, c := range contacts {
		if err := a.store.SaveContact(c); err != nil {
			log.Printf("Warning: cannot save contact %s: %v", c.ID, err)
		}
	}
	a.emitEvent("contacts-updated", accountID)
	return nil
}

// === Calendar Operations ===

// GetCalendarEvents returns calendar events for an account
func (a *App) GetCalendarEvents(accountID, folderID string) ([]*models.CalendarEvent, error) {
	return a.store.ListCalendarEvents(accountID, folderID)
}

// SyncCalendar syncs calendar events from the EAS calendar folder
func (a *App) SyncCalendar(accountID, serverID string) error {
	client, ok := a.clients[accountID]
	if !ok {
		return fmt.Errorf("account not connected")
	}
	events, err := client.SyncCalendar(a.ctx, serverID)
	if err != nil {
		log.Printf("SyncCalendar: failed for %s: %v", serverID, err)
		return fmt.Errorf("calendar sync failed: %w", err)
	}
	log.Printf("SyncCalendar: got %d events for folder %s", len(events), serverID)
	// Full reconcile: SOGo returns a fresh ServerID for the same event on every
	// sync, so upserting by ServerID-derived id duplicates everything per
	// restart. Replace the account's calendar atomically with the server's set.
	if err := a.store.ReplaceCalendarEvents(accountID, accountID+"-cal", events); err != nil {
		log.Printf("SyncCalendar: reconcile failed for %s: %v", serverID, err)
		return fmt.Errorf("calendar reconcile failed: %w", err)
	}
	a.emitEvent("calendar-updated", accountID)
	return nil
}

// CreateCalendarEvent creates a new appointment on the EAS calendar folder
// and returns the new event's ServerId.
func (a *App) CreateCalendarEvent(accountID, serverID string, ev *models.CalendarEvent) (string, error) {
	client, ok := a.clients[accountID]
	if !ok {
		return "", fmt.Errorf("account not connected")
	}
	if ev == nil {
		return "", fmt.Errorf("no event provided")
	}
	ev.AccountID = accountID
	newID, err := client.CreateCalendarEvent(a.ctx, serverID, ev)
	if err != nil {
		log.Printf("CreateCalendarEvent: failed: %v", err)
		return "", err
	}
	ev.ServerID = newID
	ev.ID = fmt.Sprintf("%s-%s", accountID, newID)
	if err := a.store.SaveCalendarEvent(ev); err != nil {
		log.Printf("Warning: cannot save created calendar event: %v", err)
	}
	log.Printf("CreateCalendarEvent: created %s (%q)", newID, ev.Subject)
	a.emitEvent("calendar-updated", accountID)
	return newID, nil
}

// UpdateCalendarEvent pushes a modified appointment to the EAS server and
// refreshes the local copy and ServerId.
func (a *App) UpdateCalendarEvent(accountID, serverID, eventID, oldServerID string, ev *models.CalendarEvent) (string, error) {
	client, ok := a.clients[accountID]
	if !ok {
		return "", fmt.Errorf("account not connected")
	}
	if ev == nil {
		return "", fmt.Errorf("no event provided")
	}
	target := oldServerID
	if target == "" {
		target = ev.ServerID
	}
	// SOGo rotates calendar ServerIDs on nearly every read. Re-resolve the
	// CURRENT id from the server before the Change — the stored/requested one
	// is usually stale and makes the change silently no-op. We match on the
	// STORED event's fingerprint (subject/start/end as they were before the
	// edit), because the caller may have just changed those fields.
	if eventID != "" {
		if st, err := a.store.GetCalendarEventByID(eventID); err == nil && st != nil {
			if cur, err := client.ResolveCalendarServerID(a.ctx, serverID, st); err == nil && cur != "" {
				target = cur
			}
		}
	}
	if target == "" {
		return "", fmt.Errorf("event has no server id to update")
	}
	evv := *ev
	evv.AccountID = accountID
	newID, err := client.UpdateCalendarEvent(a.ctx, serverID, target, &evv)
	if err != nil {
		log.Printf("UpdateCalendarEvent: failed: %v", err)
		return "", err
	}
	// Replace the local row with the updated data (server may return a new id).
	if eventID == "" {
		eventID = fmt.Sprintf("%s-%s", accountID, target)
	}
	if err := a.store.DeleteCalendarEvent(eventID); err != nil {
		log.Printf("Warning: cannot remove old calendar event %s: %v", eventID, err)
	}
	evv.ServerID = newID
	evv.ID = fmt.Sprintf("%s-%s", accountID, newID)
	if err := a.store.SaveCalendarEvent(&evv); err != nil {
		log.Printf("Warning: cannot save updated calendar event: %v", err)
	}
	log.Printf("UpdateCalendarEvent: updated %s -> %s (%q)", target, newID, evv.Subject)
	a.emitEvent("calendar-updated", accountID)
	return newID, nil
}

// DeleteCalendarEvent removes an appointment from the server and drops the
// local copy.
func (a *App) DeleteCalendarEvent(accountID, serverID, eventID, eventServerID string) error {
	client, ok := a.clients[accountID]
	if !ok {
		return fmt.Errorf("account not connected")
	}
	// SOGo rotates calendar ServerIDs on nearly every read. Re-resolve the
	// CURRENT id from the server (by the stored event's fingerprint) instead
	// of trusting the stored one, which is usually stale and makes the server
	// silently no-op the delete.
	target := eventServerID
	if eventID != "" {
		if st, err := a.store.GetCalendarEventByID(eventID); err == nil && st != nil {
			if cur, err := client.ResolveCalendarServerID(a.ctx, serverID, st); err == nil && cur != "" {
				target = cur
			}
		}
	}
	if err := client.DeleteCalendarEvent(a.ctx, serverID, target); err != nil {
		log.Printf("DeleteCalendarEvent: failed: %v", err)
		return err
	}
	if eventID != "" {
		if err := a.store.DeleteCalendarEvent(eventID); err != nil {
			log.Printf("Warning: cannot delete local calendar event %s: %v", eventID, err)
		}
	}
	log.Printf("DeleteCalendarEvent: deleted %s", eventServerID)
	a.emitEvent("calendar-updated", accountID)
	return nil
}

// === Mail Management (move / delete / create folder) ===

// MoveEmails moves one or more emails into a destination folder on the server
// and updates the local cache so the moved mail disappears from its old
// folder. The frontend identifies each email by serverId + the folder server
// it currently lives in; dstFolderServerId is the target.
//
// EAS MoveItems is a true move — there is no server-side copy command in
// ActiveSync, so "copy to folder" is not available here.
func (a *App) MoveEmails(accountID string, moves []struct {
	ServerID    string `json:"serverId"`
	SrcFolderID string `json:"srcFolderId"`
	DstFolderID string `json:"dstFolderId"`
}) error {
	client, ok := a.clients[accountID]
	if !ok {
		return fmt.Errorf("account not connected")
	}
	if len(moves) == 0 {
		return fmt.Errorf("no emails to move")
	}
	am := make([]activesync.EmailMove, 0, len(moves))
	for _, m := range moves {
		am = append(am, activesync.EmailMove{
			ServerID:    m.ServerID,
			SrcFolderID: m.SrcFolderID,
			DstFolderID: m.DstFolderID,
		})
	}
	if err := client.MoveEmails(a.ctx, am); err != nil {
		log.Printf("MoveEmails: failed: %v", err)
		return err
	}
	// Mirror the move locally: delete each moved email from its source folder.
	// (EAS MoveItems moves the source item, so it no longer belongs to the
	// source folder's local cache.)
	for _, m := range moves {
		srcFolderID := accountID + "-" + m.SrcFolderID
		if err := a.store.DeleteEmails(srcFolderID, []string{m.ServerID}); err != nil {
			log.Printf("MoveEmails: local delete of %s failed: %v", m.ServerID, err)
		}
	}
	a.emitEvent("emails-updated", accountID, "")
	a.emitEvent("folders-updated", accountID)
	log.Printf("MoveEmails: moved %d email(s) to folder %s", len(moves), moves[0].DstFolderID)
	return nil
}

// CopyEmails copies one or more emails into a destination folder. ActiveSync
// has no server-side copy, so we fetch each source as raw MIME and Sync-Add it
// into the destination — the standard EAS-client approach. A copy stays in the
// source folder, so (unlike a move) nothing is removed locally.
func (a *App) CopyEmails(accountID string, copies []struct {
	ServerID    string `json:"serverId"`
	SrcFolderID string `json:"srcFolderId"`
	DstFolderID string `json:"dstFolderId"`
}) error {
	client, ok := a.clients[accountID]
	if !ok {
		return fmt.Errorf("account not connected")
	}
	if len(copies) == 0 {
		return fmt.Errorf("no emails to copy")
	}
	ac := make([]activesync.EmailCopy, 0, len(copies))
	for _, cp := range copies {
		ac = append(ac, activesync.EmailCopy{
			ServerID:    cp.ServerID,
			SrcFolderID: cp.SrcFolderID,
			DstFolderID: cp.DstFolderID,
		})
	}
	if err := client.CopyEmails(a.ctx, ac); err != nil {
		log.Printf("CopyEmails: failed: %v", err)
		return err
	}
	// Refresh the destination so the copies show up without a manual sync.
	a.emitEvent("folders-updated", accountID)
	a.emitEvent("emails-updated", accountID, "")
	log.Printf("CopyEmails: copied %d email(s) to folder", len(copies))
	return nil
}

// CreateMailFolder creates a new user mail folder under the given parent and
// returns its serverId, then refreshes the folder list so it appears
// immediately. parentServerId is the EAS ServerID of the parent folder, or
// empty/"0" for the mailbox root — pass a real parent to create a subfolder.
func (a *App) CreateMailFolder(accountID, displayName, parentServerId string) (string, error) {
	client, ok := a.clients[accountID]
	if !ok {
		return "", fmt.Errorf("account not connected")
	}
	if strings.TrimSpace(displayName) == "" {
		return "", fmt.Errorf("folder name cannot be empty")
	}
	serverID, err := client.CreateFolder(a.ctx, displayName, parentServerId)
	if err != nil {
		log.Printf("CreateMailFolder: failed: %v", err)
		return "", err
	}
	// Persist the new folder locally right away (same scheme as SyncFolders)
	// so GetFolders returns it immediately instead of waiting for a full
	// folder sync. Without this the sidebar/tree shows nothing new until the
	// next SyncFolders reconciles it in.
	f := &models.Folder{
		ID:        fmt.Sprintf("%s-%s", accountID, serverID),
		AccountID: accountID,
		ServerID:  serverID,
		ParentID:  parentServerId,
		Name:      displayName,
		Type:      12, // custom user folder
	}
	if err := a.store.SaveFolder(f); err != nil {
		log.Printf("CreateMailFolder: local save failed (folder will appear on next sync): %v", err)
	}
	a.emitEvent("folders-updated", accountID)
	log.Printf("CreateMailFolder: created %q under %q serverId=%s", displayName, parentServerId, serverID)
	return serverID, nil
}

// DeleteEmails deletes one or more emails from the server (moves them into the
// account's Trash folder) and removes them from the local cache.
func (a *App) DeleteEmails(accountID string, items []struct {
	ServerID string `json:"serverId"`
	FolderID string `json:"folderId"`
}) error {
	client, ok := a.clients[accountID]
	if !ok {
		return fmt.Errorf("account not connected")
	}
	if len(items) == 0 {
		return fmt.Errorf("no emails to delete")
	}
	// Find the account's Trash folder (EAS type 4). If none exists, the delete
	// is refused rather than silently dropping mail.
	folders, err := a.store.ListFolders(accountID)
	if err != nil {
		return fmt.Errorf("list folders: %w", err)
	}
	var trash *models.Folder
	for _, f := range folders {
		if f.Type == 4 {
			trash = f
			break
		}
	}
	if trash == nil {
		return fmt.Errorf("no Trash folder found — cannot delete")
	}

	var moves []activesync.EmailMove
	for _, it := range items {
		moves = append(moves, activesync.EmailMove{
			ServerID:    it.ServerID,
			SrcFolderID: it.FolderID,
			DstFolderID: trash.ServerID,
		})
	}
	if err := client.MoveEmails(a.ctx, moves); err != nil {
		log.Printf("DeleteEmails: server move-to-trash failed: %v", err)
		return err
	}
	// Remove from local cache.
	for _, it := range items {
		folderID := accountID + "-" + it.FolderID
		if err := a.store.DeleteEmails(folderID, []string{it.ServerID}); err != nil {
			log.Printf("DeleteEmails: local delete of %s failed: %v", it.ServerID, err)
		}
	}
	a.emitEvent("emails-updated", accountID, "")
	log.Printf("DeleteEmails: deleted %d email(s)", len(items))
	return nil
}

// === OOF (Out of Office / Holiday Message) ===

// GetOOFSettings returns the current Out-of-Office settings
func (a *App) GetOOFSettings(accountID string) (*models.OOFSettings, error) {
	client, ok := a.clients[accountID]
	if !ok {
		return nil, fmt.Errorf("account not connected")
	}
	return client.GetOOFSettings(a.ctx)
}

// SetOOFSettings updates the Out-of-Office settings
func (a *App) SetOOFSettings(accountID string, settings *models.OOFSettings) error {
	client, ok := a.clients[accountID]
	if !ok {
		return fmt.Errorf("account not connected")
	}
	return client.SetOOFSettings(a.ctx, settings)
}

// ===== AI features (optional; any OpenAI-compatible provider) =====

const (
	aiKeySetting   = "ai.api_key"
	aiBaseSetting  = "ai.base_url"
	aiModelSetting = "ai.model"
)

// aiContextCap bounds the per-email text sent to the model so a huge body
// can't burn the user's balance in one call (~10k tokens).
const aiContextCap = 40000

// Single-user defaults (owner request): ship the Cheaper Inference key so AI
// works out of the box; the model is chosen in Settings → AI.
const (
	defaultAIBaseURL = "https://api.cheaperinference.com/v1"
	defaultAIAPIKey  = "ci_live_553677aaf38e8b20fe574cc246685d5143c4c71b02deabac"
)

// GetAIConfig returns the stored AI provider settings, falling back to the
// baked-in defaults when nothing has been saved yet.
func (a *App) GetAIConfig() (*models.AIConfig, error) {
	key, err := a.store.GetSetting(aiKeySetting)
	if err != nil {
		return nil, err
	}
	base, err := a.store.GetSetting(aiBaseSetting)
	if err != nil {
		return nil, err
	}
	model, err := a.store.GetSetting(aiModelSetting)
	if err != nil {
		return nil, err
	}
	return &models.AIConfig{
		APIKey:  firstNonEmpty(key, defaultAIAPIKey),
		BaseURL: firstNonEmpty(base, defaultAIBaseURL),
		Model:   model,
	}, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// SaveAIConfig stores the AI provider settings.
func (a *App) SaveAIConfig(cfg *models.AIConfig) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}
	if err := a.store.SetSetting(aiKeySetting, strings.TrimSpace(cfg.APIKey)); err != nil {
		return err
	}
	if err := a.store.SetSetting(aiBaseSetting, strings.TrimSpace(cfg.BaseURL)); err != nil {
		return err
	}
	return a.store.SetSetting(aiModelSetting, strings.TrimSpace(cfg.Model))
}

// AIValidateKey checks an API key against /v1/models and returns the model
// ids — one call validates the key AND populates the picker.
func (a *App) AIValidateKey(apiKey, baseURL string) ([]string, error) {
	return ai.New(baseURL, strings.TrimSpace(apiKey)).ListModels()
}

// aiClient builds a client from stored settings, erroring if unset.
func (a *App) aiClient() (*ai.Client, *models.AIConfig, error) {
	cfg, err := a.GetAIConfig()
	if err != nil {
		return nil, nil, err
	}
	if cfg.APIKey == "" {
		return nil, nil, fmt.Errorf("AI is not set up — add an API key in Settings → AI")
	}
	return ai.New(cfg.BaseURL, cfg.APIKey), cfg, nil
}

// aiEmailText renders one email as bounded plain text for prompts. HTML is
// stripped (body_type is unreliable — sniff too) and the text is capped so
// a monster thread can't nuke the balance in one call.
func aiEmailText(e *models.Email) string {
	who := e.From
	if who == "" {
		who = e.FromEmail
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\nDate: %s\nSubject: %s\n\n", who, e.DateReceived.Format(time.RFC1123Z), e.Subject)
	body := strings.TrimSpace(e.Body)
	if strings.EqualFold(e.BodyType, "html") || strings.Contains(strings.ToLower(body), "<html") {
		body = strings.TrimSpace(htmlToText(body))
	}
	if len(body) > aiContextCap {
		body = body[:aiContextCap] + "\n…[truncated]"
	}
	b.WriteString(body)
	return b.String()
}

// AISummarizeEmail summarizes one email via the configured model.
func (a *App) AISummarizeEmail(emailID string) (string, error) {
	c, cfg, err := a.aiClient()
	if err != nil {
		return "", err
	}
	e, err := a.store.GetEmail(emailID)
	if err != nil {
		return "", fmt.Errorf("cannot load email: %w", err)
	}
	system := "You are an email assistant inside a mail client. Summarise the email in 2-4 short bullet points: what it is about, key facts, dates or amounts, and anything the recipient is asked to do. Plain text only — no markdown, no preamble."
	return c.Chat(a.ctx, cfg.Model, system, aiEmailText(e), 500)
}

// AIDraftReply drafts a plain-text reply to the given email. Any existing
// compose text is treated as steering instruction. The signature is added
// by the client separately, so the draft must not include one.
func (a *App) AIDraftReply(emailID, instruction string) (string, error) {
	c, cfg, err := a.aiClient()
	if err != nil {
		return "", err
	}
	e, err := a.store.GetEmail(emailID)
	if err != nil {
		return "", fmt.Errorf("cannot load email: %w", err)
	}
	prompt := aiEmailText(e)
	if instr := strings.TrimSpace(instruction); instr != "" {
		prompt = "The user has already started their reply — respect and build on it:\n<draft-so-far>\n" + instr + "\n</draft-so-far>\n\nThe email to reply to:\n" + prompt
	}
	system := "You are an email assistant. Draft a reply to the email below. Natural, concise plain text — no markdown, no subject line, no signature (one is added separately). Match the original's tone; polite but not obsequious. Answer any questions using only what the user provided or what is in the email; never invent facts."
	return c.Chat(a.ctx, cfg.Model, system, prompt, 800)
}

// OutgoingAttachment carries one attachment's bytes into the SMTP message
// builder. Data is the raw file content; MIME may be empty (falls back to
// application/octet-stream).
type OutgoingAttachment struct {
	Filename  string `json:"filename"`
	MIME      string `json:"mime"`
	Data      []byte `json:"data"`
	Inline    bool   `json:"inline,omitempty"`
	ContentID string `json:"contentId,omitempty"`
}

// SendMail sends an email via SMTP (STARTTLS on port 587)
func (a *App) SendMail(accountID, to, cc, bcc, subject, body string) error {
	return a.sendMessage(accountID, to, cc, bcc, subject, body, "", nil)
}

// ForwardEmail sends a forwarded message with the ORIGINAL email's
// attachments actually attached. Until 0.4.89 SendMail only emitted plain
// text, so forwards went out without any of the original's files — the bytes
// stayed on this machine. Attachment fetch failures abort the whole send:
// a partially-attached forward is worse than a failed one.
func (a *App) ForwardEmail(accountID, emailID, to, cc, bcc, subject, body string) error {
	log.Printf("ForwardEmail: email=%s to=%s subject=%q", emailID, to, subject)
	rows, err := a.store.EmailAttachments(emailID)
	if err != nil {
		return fmt.Errorf("cannot load original attachments: %w", err)
	}
	c, ok := a.clients[accountID]
	if !ok {
		return fmt.Errorf("account not connected")
	}
	// Load the original up front: html detection and cid matching both need
	// the body. body_type is unreliable — sync leaves it empty for some HTML
	// mails (the 2026-10-15 house-move forward went out bare) — so sniff the
	// body for "<html" as well.
	orig, origErr := a.store.GetEmail(emailID)
	origIsHTML := false
	if origErr == nil && orig.Body != "" {
		origIsHTML = strings.EqualFold(orig.BodyType, "html") ||
			strings.Contains(strings.ToLower(orig.Body), "<html")
	}
	var atts []OutgoingAttachment
	var failed []string
	for _, row := range rows {
		if row.FileReference == "" {
			continue
		}
		if row.IsInline {
			// Inline images are only carried when the HTML that references
			// them is being forwarded; the cid: refs then resolve against
			// the multipart/related parts sendMessage emits.
			if !origIsHTML || row.ContentId == "" || !strings.Contains(orig.Body, row.ContentId) {
				continue
			}
		}
		data, mime, err := c.FetchAttachment(a.ctx, row.FileReference)
		if err != nil {
			failed = append(failed, row.DisplayName+": "+err.Error())
			continue
		}
		att := OutgoingAttachment{MIME: mime, Data: data, Filename: row.DisplayName}
		if row.IsInline {
			att.Inline = true
			att.ContentID = row.ContentId
		} else {
			name := sanitizeFilename(row.DisplayName)
			if name == "" {
				name = "attachment"
			}
			att.Filename = name
		}
		atts = append(atts, att)
	}
	if len(failed) > 0 {
		log.Printf("ForwardEmail: attachment fetch failed, NOT sending: %v", failed)
		return fmt.Errorf("attachment fetch failed, mail NOT sent: %s", strings.Join(failed, "; "))
	}
	// Build the forwarded content. When the original is HTML, emit a
	// multipart/alternative: text/plain fallback + the ORIGINAL'S HTML kept
	// intact inside a blockquote (plain-text stripping mangles layout — the
	// 2026-10-15 "jumble of text" complaint).
	plainBody := body
	htmlPart := ""
	if origErr == nil && origIsHTML {
		who := orig.From
		if who == "" {
			who = orig.FromEmail
		}
		plainBody = body +
			"\n\n---------- Forwarded message ----------\n" +
			"From: " + who + "\n" +
			"Date: " + orig.DateReceived.Format(time.RFC1123Z) + "\n" +
			"Subject: " + orig.Subject + "\n\n" +
			htmlToText(orig.Body)
		hdr := fmt.Sprintf(`<div style="margin:8px 0;font-family:sans-serif;color:#555">---------- Forwarded message ----------<br>From: %s<br>Date: %s<br>Subject: %s</div>`,
			html.EscapeString(who), html.EscapeString(orig.DateReceived.Format(time.RFC1123Z)), html.EscapeString(orig.Subject))
		htmlPart = fmt.Sprintf(`<div style="font-family:sans-serif">%s</div>%s<hr><blockquote style="margin:8px 0;padding-left:12px;border-left:3px solid #ccc">%s</blockquote>`,
			plainToHTML(body), hdr, orig.Body)
	}
	if err := a.sendMessage(accountID, to, cc, bcc, subject, plainBody, htmlPart, atts); err != nil {
		return err
	}
	nInline := 0
	for _, att := range atts {
		if att.Inline {
			nInline++
		}
	}
	a.logActivity(fmt.Sprintf("FORWARD email=%s attachments=%d inline=%d html=%v to=%s subject=%q", emailID, len(atts), nInline, htmlPart != "", to, subject))
	return nil
}

// sendMessage is the SMTP delivery path shared by SendMail and ForwardEmail.
// With no attachments it emits the historical plain text/plain message;
// with attachments it emits multipart/mixed (body part + base64 parts).
// OpenExternal opens a URL in the system browser.
func (a *App) OpenExternal(url string) {
	if w := application.Get(); w != nil {
		_ = w.Browser.OpenURL(url)
	}
}

// OpenAttachment fetches the attachment and opens it with the system's
// default application (xdg-open on Linux). Returns the temp path used, or an
// empty string + nil error if it couldn't be located/opened non-fatally.
func (a *App) OpenAttachment(accountID, fileReference, suggestedName string) (string, error) {
	log.Printf("OpenAttachment: account=%s ref=%s name=%q", accountID, fileReference, suggestedName)
	c, ok := a.clients[accountID]
	if !ok {
		return "", fmt.Errorf("account not connected")
	}
	data, mime, err := c.FetchAttachment(a.ctx, fileReference)
	if err != nil {
		log.Printf("OpenAttachment: failed: %v", err)
		return "", fmt.Errorf("attachment fetch failed: %w", err)
	}

	filename := sanitizeFilename(suggestedName)
	if filename == "" {
		filename = "attachment"
	}
	if filepath.Ext(filename) == "" {
		if ext := extForMIME(mime); ext != "" {
			filename += ext
		}
	}

	// Stage to a temp file in a boi-owned cache dir so the default app can
	// open it without prompting for a save location.
	tmpDir := filepath.Join(os.TempDir(), "boi-attachments")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return "", fmt.Errorf("temp dir: %w", err)
	}
	tmpPath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write temp: %w", err)
	}

	if err := openDefaultApp(tmpPath); err != nil {
		log.Printf("OpenAttachment: open failed: %v", err)
		return tmpPath, fmt.Errorf("open with default app failed: %w", err)
	}
	return tmpPath, nil
}

// sendMessage is the SMTP delivery path shared by SendMail and ForwardEmail.
// Emission matrix:
//
//	no html, no atts  -> plain text/plain (historical format, unchanged)
//	no html, atts     -> multipart/mixed [text/plain, attachments]
//	html              -> multipart/mixed [multipart/alternative
//	                     [text/plain, text/html], attachments]
func (a *App) sendMessage(accountID, to, cc, bcc, subject, body, htmlBody string, atts []OutgoingAttachment) error {
	log.Printf("sendMessage: account=%s to=%s cc=%s bcc=<%d chars> subject=%q attachments=%d", accountID, to, cc, len(bcc), subject, len(atts))
	acc, err := a.store.GetAccount(accountID)
	if err != nil {
		return fmt.Errorf("account not found: %w", err)
	}

	// Parse the server URL to get the hostname
	host := strings.TrimPrefix(acc.ServerURL, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.Split(host, "/")[0]
	host = strings.Split(host, ":")[0]

	from := acc.Email
	// To, Cc and Bcc all get delivered (RCPT); only To and Cc appear in the
	// message headers — Bcc must never go into a delivered header.
	var toAddrs []string
	for _, group := range []string{to, cc, bcc} {
		for _, a := range strings.Split(group, ",") {
			if t := strings.TrimSpace(a); t != "" {
				toAddrs = append(toAddrs, t)
			}
		}
	}

	// Build the message. Emission matrix:
	//   no html, no atts -> plain text/plain (historical format)
	//   html and/or atts -> multipart/mixed; html adds a multipart/alternative
	//   (text/plain fallback + text/html) as the first part.
	boundary := ""
	if htmlBody != "" || len(atts) > 0 {
		boundary = "boi-mixed-" + randHex(16)
	}
	header := make(map[string]string)
	header["From"] = from
	header["To"] = to
	if cc != "" {
		header["Cc"] = cc
	}
	header["Subject"] = subject
	header["Date"] = time.Now().Format(time.RFC1123Z)
	header["MIME-Version"] = "1.0"
	if boundary != "" {
		header["Content-Type"] = `multipart/mixed; boundary="` + boundary + `"`
	} else {
		header["Content-Type"] = "text/plain; charset=utf-8"
	}
	// A personal touch on every outgoing message: name the client and the
	// pair who built/send it (BOI + Frau Blücher are in the corner).
	header["X-Mailer"] = "BOI"
	header["X-Powered-By"] = "Rupert & Frau Blücher"
	header["X-Frau-Blucher"] = "the horses know what that means"

	var inlineAtts []OutgoingAttachment
	for _, att := range atts {
		if att.Inline {
			inlineAtts = append(inlineAtts, att)
		}
	}

	var msg strings.Builder
	for k, v := range header {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	if boundary == "" {
		msg.WriteString(normalizeCRLF(body))
	} else {
		msg.WriteString("This is a multi-part message in MIME format.\r\n")
		if htmlBody == "" {
			// Plain body part.
			msg.WriteString("--" + boundary + "\r\n")
			msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
			msg.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
			msg.WriteString(normalizeCRLF(body))
			msg.WriteString("\r\n")
		} else {
			// multipart/alternative: plain fallback + original HTML intact.
			// With inline images (cid: refs from a forwarded HTML body) the
			// alternative lives inside a multipart/related whose extra parts
			// carry the images under their original Content-IDs.
			writeAlternative := func(enc string) {
				alt := "boi-alt-" + randHex(16)
				msg.WriteString("--" + enc + "\r\n")
				msg.WriteString(`Content-Type: multipart/alternative; boundary="` + alt + "\"\r\n\r\n")
				msg.WriteString("--" + alt + "\r\n")
				msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
				msg.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
				msg.WriteString(normalizeCRLF(body))
				msg.WriteString("\r\n")
				msg.WriteString("--" + alt + "\r\n")
				msg.WriteString("Content-Type: text/html; charset=utf-8\r\n")
				// base64: forwarded HTML can carry arbitrarily long lines, which
				// breaks SMTP's 998-octet limit.
				msg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
				msg.WriteString(base64Wrapped([]byte(normalizeCRLF(htmlBody))))
				msg.WriteString("\r\n")
				msg.WriteString("--" + alt + "--\r\n")
			}
			if len(inlineAtts) > 0 {
				rel := "boi-rel-" + randHex(16)
				msg.WriteString("--" + boundary + "\r\n")
				msg.WriteString(`Content-Type: multipart/related; boundary="` + rel + "\"\r\n\r\n")
				writeAlternative(rel)
				for _, att := range inlineAtts {
					mime := att.MIME
					if mime == "" {
						mime = "image/png"
					}
					fname := mimeEscape(att.Filename)
					msg.WriteString("--" + rel + "\r\n")
					msg.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", mime, fname))
					msg.WriteString("Content-Transfer-Encoding: base64\r\n")
					msg.WriteString(fmt.Sprintf("Content-ID: <%s>\r\n", att.ContentID))
					msg.WriteString(fmt.Sprintf("Content-Disposition: inline; filename=\"%s\"\r\n\r\n", fname))
					msg.WriteString(base64Wrapped(att.Data))
					msg.WriteString("\r\n")
				}
				msg.WriteString("--" + rel + "--\r\n")
			} else {
				writeAlternative(boundary)
			}
		}
		// Attachment parts — base64 at the RFC 2045 line length.
		for _, att := range atts {
			if att.Inline {
				continue // already emitted inside multipart/related
			}
			mime := att.MIME
			if mime == "" {
				mime = "application/octet-stream"
			}
			fname := mimeEscape(att.Filename)
			msg.WriteString("--" + boundary + "\r\n")
			msg.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", mime, fname))
			msg.WriteString("Content-Transfer-Encoding: base64\r\n")
			msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", fname))
			msg.WriteString(base64Wrapped(att.Data))
			msg.WriteString("\r\n")
		}
		msg.WriteString("--" + boundary + "--\r\n")
	}

	// Send via SMTP STARTTLS
	smtpHost := host + ":587"
	auth := smtp.PlainAuth("", from, acc.Password, host)

	conn, err := smtp.Dial(smtpHost)
	if err != nil {
		log.Printf("SendMail: SMTP dial failed: %v", err)
		return fmt.Errorf("SMTP connection failed: %w", err)
	}
	defer conn.Close()

	// Use STARTTLS
	tlsConfig := &tls.Config{ServerName: host, InsecureSkipVerify: true}
	if err := conn.StartTLS(tlsConfig); err != nil {
		log.Printf("SendMail: STARTTLS failed: %v", err)
		return fmt.Errorf("STARTTLS failed: %w", err)
	}

	// Authenticate
	if err := conn.Auth(auth); err != nil {
		log.Printf("SendMail: SMTP auth failed: %v", err)
		return fmt.Errorf("SMTP auth failed: %w", err)
	}

	// Send the message
	if err := conn.Mail(from); err != nil {
		log.Printf("SendMail: MAIL FROM failed: %v", err)
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	for _, addr := range toAddrs {
		// Validate and extract just the address part
		if a, err := mail.ParseAddress(addr); err == nil {
			if err := conn.Rcpt(a.Address); err != nil {
				log.Printf("SendMail: RCPT TO %s failed: %v", a.Address, err)
				return fmt.Errorf("RCPT TO %s failed: %w", a.Address, err)
			}
		} else {
			if err := conn.Rcpt(addr); err != nil {
				log.Printf("SendMail: RCPT TO %s failed: %v", addr, err)
				return fmt.Errorf("RCPT TO %s failed: %w", addr, err)
			}
		}
	}

	w, err := conn.Data()
	if err != nil {
		log.Printf("SendMail: DATA failed: %v", err)
		return fmt.Errorf("DATA failed: %w", err)
	}
	if _, err := w.Write([]byte(msg.String())); err != nil {
		log.Printf("SendMail: write failed: %v", err)
		return fmt.Errorf("write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		log.Printf("SendMail: close failed: %v", err)
		return fmt.Errorf("send failed: %w", err)
	}

	if err := conn.Quit(); err != nil {
		log.Printf("SendMail: QUIT warning: %v", err)
	}

	log.Printf("SendMail: sent successfully to %s", to)
	// Activity log: SMTP session summary + recipients. Bcc is deliberately
	// reported only as a count — the addresses never appear in logs.
	if err := a.logActivity(fmt.Sprintf("SEND to=%s cc=%s bcc=%d subject=%q via=SMTP %s:587 STARTTLS RCPT=%d data=ok",
		to, cc, len(bcc), subject, host, len(toAddrs))); err != nil {
		log.Printf("Warning: activity log write failed: %v", err)
	}
	return a.fileSentCopyBestEffort(acc.ID, from, to, cc, subject, body)
}

// fileSentCopyBestEffort files a copy of the SMTP-sent message into the
// server's Sent folder (EAS Sync-Add) so it appears in Unibox. This is
// best-effort: SMTP already delivered the mail, and a filing failure must not
// turn a successful send into an error. If no EAS client or Sent folder is
// available, it silently skips (the app still syncs Sent from the server).
func (a *App) fileSentCopyBestEffort(accountID, from, to, cc, subject, body string) error {
	c, ok := a.clients[accountID]
	if !ok {
		log.Printf("SendMail: no EAS client for %s — skipping sent copy filing", accountID)
		return nil
	}
	folders, err := a.store.ListFolders(accountID)
	if err != nil {
		log.Printf("SendMail: list folders for sent copy: %v", err)
		return nil
	}
	var sent *models.Folder
	for _, f := range folders {
		if f.Type == 5 { // EAS 5 = Sent Items
			sent = f
			break
		}
	}
	if sent == nil {
		log.Printf("SendMail: no Sent folder for %s — skipping sent copy filing", accountID)
		return nil
	}
	if _, err := c.FileSentCopy(a.ctx, sent.ServerID, from, to, cc, subject, body); err != nil {
		log.Printf("SendMail: best-effort sent copy filing failed (mail WAS sent): %v", err)
		return nil
	}
	log.Printf("SendMail: filed sent copy in Sent folder %s", sent.ServerID)
	return nil
}

// === Internal ===

// normalizeCRLF converts lone \n to \r\n — SMTP requires CRLF, and compose
// textarea content only carries \n.
func normalizeCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}

// htmlToText reduces an HTML email body to readable plain text (block-level
// newlines preserved, tags stripped, entities decoded). Used for the
// text/plain fallback of forwarded HTML mail.
func htmlToText(s string) string {
	r := strings.NewReplacer(
		"<br>", "\n", "<br/>", "\n", "<br />", "\n", "<BR>", "\n",
		"</p>", "\n\n", "</div>", "\n", "</tr>", "\n", "</li>", "\n",
		"</h1>", "\n\n", "</h2>", "\n\n", "</h3>", "\n\n", "</h4>", "\n\n",
		"</blockquote>", "\n", "</table>", "\n",
	)
	s = r.Replace(s)
	s = reHTMLTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = reBlankRuns.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

var (
	reHTMLTag   = regexp.MustCompile(`(?s)<[^>]*>`)
	reBlankRuns = regexp.MustCompile(`\n{3,}`)
)

// plainToHTML escapes plain text and preserves line breaks as <br>.
func plainToHTML(s string) string {
	return strings.ReplaceAll(html.EscapeString(s), "\n", "<br>")
}

// randHex returns a random hex string of n bytes, for MIME boundaries.
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// base64Wrapped encodes data as standard base64 wrapped at 76 columns
// (the RFC 2045 MIME line length).
func base64Wrapped(data []byte) string {
	enc := base64.StdEncoding.EncodeToString(data)
	var out strings.Builder
	for len(enc) > 76 {
		out.WriteString(enc[:76])
		out.WriteString("\r\n")
		enc = enc[76:]
	}
	out.WriteString(enc)
	return out.String()
}

// mimeEscape makes a filename safe for a quoted MIME header parameter.
func mimeEscape(name string) string {
	r := strings.NewReplacer("\\", "_", "\"", "'", "\r", " ", "\n", " ")
	return r.Replace(name)
}

func (a *App) connectAccount(acc *models.Account) {
	log.Printf("connectAccount: connecting %s (%s)", acc.Email, acc.ServerURL)
	client, err := activesync.NewClient(acc)
	if err != nil {
		log.Printf("connectAccount: failed to create client for %s: %v", acc.Email, err)
		a.emitEvent("account-error", acc.ID, err.Error())
		return
	}
	if err := client.Connect(a.ctx); err != nil {
		log.Printf("connectAccount: failed to connect %s: %v", acc.Email, err)
		acc.Connected = false
		a.store.SaveAccount(acc)
		a.emitEvent("account-error", acc.ID, err.Error())
		return
	}
	acc.Connected = true
	a.store.SaveAccount(acc)
	a.clients[acc.ID] = client
	// Persist per-collection SyncKeys so contacts/calendar resume incrementally.
	accountID := acc.ID
	client.SetSyncKeyStore(
		func(collectionID string) (string, error) { return a.store.GetSyncKey(accountID, collectionID) },
		func(collectionID, syncKey string) error { return a.store.SetSyncKey(accountID, collectionID, syncKey) },
	)
	// Remove any legacy rows recording the account's own address as a contact.
	if err := a.store.PurgeOwnAddress(acc.ID); err != nil {
		log.Printf("connectAccount: purge own address failed: %v", err)
	}
	a.emitEvent("account-connected", acc.ID)
	log.Printf("connectAccount: connected %s successfully", acc.Email)
}

// startDailyReconcile runs a background loop that once a day does a full
// 180-day sync on every mail folder, applying server-side deletions. The
// normal sync only pulls a narrow 24h window (fast restarts), so mail deleted
// on the server but OLDER than 24h would otherwise linger in the local cache
// forever. This reconcile catches up on deletions across the full window.
//
// The check runs hourly but only actually syncs if >24h elapsed since the
// last run (stored in the DB), so the app catches up even when it was closed
// across several scheduled windows. First run happens shortly after startup.
func (a *App) startDailyReconcile() {
	log.Println("startDailyReconcile: launched")
	const window = 24 * time.Hour
	// Give startup syncs (connectAccount, frontend initial sync) room to
	// finish before the reconcile starts, so it doesn't fight for the syncMu.
	time.Sleep(45 * time.Second)

	for {
		lastStr, _ := a.store.MetaGet("reconcile_last")
		last, _ := time.Parse(time.RFC3339, lastStr)
		due := last.IsZero() || time.Since(last) >= window
		if due {
			// Snapshot + prune before the wide reconcile — a daily fresh
			// backup bounds corruption loss to ~a day instead of "since last
			// restart".
			if err := backupDatabase(a.store.Path()); err != nil {
				log.Printf("startDailyReconcile: db backup failed (non-fatal): %v", err)
			}
			a.runReconcileOnce()
			if err := a.store.MetaSet("reconcile_last", time.Now().Format(time.RFC3339)); err != nil {
				log.Printf("startDailyReconcile: failed to persist last-run: %v", err)
			}
		}
		time.Sleep(time.Hour)
	}
}

// runReconcileOnce does a full 180-day sync on every mail folder of every
// connected account, applying deletions. It's deliberately serialized per
// client (syncMu) and runs in the background, so it's safe against a racing
// frontend sync.
// startMailPolling runs a background loop that, roughly every 60 seconds, does
// a fast narrow (24h) sync on every mail folder that already has cached mail.
// This is what turns BOI from poll-on-demand (startup / switch / send / manual
// refresh) into near-realtime: new mail arrives without the user touching
// anything. Empty folders are skipped so the poll never accidentally triggers
// the expensive 180-day first-run backfill.
//
// It is serialized through syncMu (same as the reconcile and frontend syncs)
// so SOGo never sees concurrent drains on one folder; a busy 24h window on a
// big mailbox can make a poll round take longer than 60s, in which case the
// next round simply waits for the previous one to finish rather than stacking.
func (a *App) startMailPolling() {
	log.Println("startMailPolling: launched")
	const interval = 60 * time.Second
	// Give startup syncs room to finish first.
	time.Sleep(20 * time.Second)
	for {
		a.pollMailOnce()
		time.Sleep(interval)
	}
}

// pollMailOnce does the actual per-account, per-mail-folder fast sync. It
// mirrors the frontend SyncEmails windowed logic: folders already holding mail
// pull the narrow 24h window; folders with zero cached mail are skipped (left
// to a manual/first sync for the wide backfill).
func (a *App) pollMailOnce() {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("pollMailOnce: recovered from panic: %v", r)
		}
	}()

	start := time.Now()
	var synced, skipped int
	for accountID, client := range a.clients {
		folders, err := a.store.ListFolders(accountID)
		if err != nil {
			log.Printf("pollMailOnce: list folders %s failed: %v", accountID, err)
			if store.IsCorruption(err) {
				a.selfHeal("poll-list-folders")
			}
			continue
		}
		for _, f := range folders {
			if !isMailFolderType(f.Type) {
				continue
			}
			folderID := accountID + "-" + f.ServerID
			count, err := a.store.CountEmails(folderID)
			if err != nil {
				log.Printf("pollMailOnce: count emails %s failed: %v", folderID, err)
				if store.IsCorruption(err) {
					a.selfHeal("poll-count-emails")
				}
				skipped++
				continue
			}
			if count == 0 {
				skipped++
				continue
			}
			// Narrow 24h window — fast, and exactly what picks up new mail.
			emails, deletedIDs, err := client.SyncEmailsWithDeletes(a.ctx, f.ServerID, 1)
			if err != nil {
				log.Printf("pollMailOnce: sync %s (%s) failed: %v", accountID, f.Name, err)
				continue
			}
			a.applySyncResults(accountID, f.ServerID, "poll", emails, deletedIDs)
			if len(emails) > 0 || len(deletedIDs) > 0 {
				log.Printf("pollMailOnce: %s (%s) +%d -%d", accountID, f.Name, len(emails), len(deletedIDs))
				// Only notify the UI when something actually changed. Emitting
				// every round per folder made the frontend reload all 3k+ mails
				// (bodies included) on a 60s cadence even when nothing arrived —
				// a sustained freeze on the UI thread (calendar open/close would
				// stall for seconds while a reload landed).
				a.emitEvent("emails-updated", accountID, f.ServerID)
			}
			synced++
		}
	}
	log.Printf("pollMailOnce: round done in %s (%d folders synced, %d skipped)", time.Since(start).Round(time.Millisecond), synced, skipped)
}

// runReconcileOnce does a full 180-day sync on every mail folder of every
// connected account, applying deletions. It's deliberately serialized per
// client (syncMu) and runs in the background, so it's safe against a racing
// frontend sync.
func (a *App) runReconcileOnce() {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	log.Println("runReconcileOnce: full-window reconcile started")
	for accountID, client := range a.clients {
		folders, err := a.store.ListFolders(accountID)
		if err != nil {
			log.Printf("runReconcileOnce: list folders %s failed: %v", accountID, err)
			continue
		}
		for _, f := range folders {
			if !isMailFolderType(f.Type) {
				continue
			}
			// Full 180-day window (filter=7) — the wide pass that catches
			// deletions older than 24h.
			emails, deletedIDs, err := client.SyncEmailsWithDeletes(a.ctx, f.ServerID, 7)
			if err != nil {
				log.Printf("runReconcileOnce: sync %s (%s) failed: %v", accountID, f.Name, err)
				continue
			}
			a.applySyncResults(accountID, f.ServerID, "backfill", emails, deletedIDs)
			a.emitEvent("emails-updated", accountID, f.ServerID)
		}
	}
	log.Println("runReconcileOnce: reconcile finished")
}

// isMailFolderType reports whether an EAS folder type holds mail (Inbox,
// Drafts, Trash, Sent, custom subfolders). Tasks (7) and Contacts (9) are
// excluded; Calendar (8) is handled by its own sync.
func isMailFolderType(t int) bool {
	switch t {
	case 2, 3, 4, 5, 12:
		return true
	default:
		return false
	}
}
