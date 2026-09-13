package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"easymail/internal/models"
)

// Store wraps the SQLite database for local caching
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("cannot open database: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}
	return s, nil
}

// Close shuts down the database
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	// Phase 1: Create tables if they don't exist (fresh install)
	createTables := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			server_url TEXT NOT NULL,
			device_id TEXT NOT NULL,
			device_type TEXT NOT NULL DEFAULT 'EasyMail',
			connected INTEGER NOT NULL DEFAULT 0,
			last_sync DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS folders (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			server_id TEXT NOT NULL,
			parent_id TEXT,
			name TEXT NOT NULL,
			type INTEGER NOT NULL DEFAULT 1,
			is_hidden INTEGER NOT NULL DEFAULT 0,
			unread_count INTEGER NOT NULL DEFAULT 0,
			last_sync DATETIME,
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			UNIQUE(account_id, server_id)
		)`,
		`CREATE TABLE IF NOT EXISTS emails (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			folder_id TEXT NOT NULL,
			server_id TEXT NOT NULL,
			from_name TEXT,
			from_email TEXT NOT NULL,
			to_text TEXT,
			to_emails TEXT,
			cc_text TEXT,
			cc_emails TEXT,
			subject TEXT,
			thread_topic TEXT,
			date_received DATETIME NOT NULL,
			is_read INTEGER NOT NULL DEFAULT 0,
			is_flagged INTEGER NOT NULL DEFAULT 0,
			importance INTEGER NOT NULL DEFAULT 1,
			has_attachment INTEGER NOT NULL DEFAULT 0,
			preview TEXT,
			body TEXT,
			body_type TEXT,
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			FOREIGN KEY (folder_id) REFERENCES folders(id) ON DELETE CASCADE,
			UNIQUE(account_id, folder_id, server_id)
		)`,
		`CREATE TABLE IF NOT EXISTS contacts (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			name TEXT,
			first_name TEXT,
			last_name TEXT,
			email TEXT NOT NULL,
			email2 TEXT,
			email3 TEXT,
			phone TEXT,
			mobile TEXT,
			company TEXT,
			job_title TEXT,
			avatar_url TEXT,
			server_id TEXT,
			last_email_at DATETIME,
			email_count INTEGER NOT NULL DEFAULT 0,
			is_favorite INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			UNIQUE(account_id, email)
		)`,
		`CREATE TABLE IF NOT EXISTS calendar_events (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			folder_id TEXT,
			server_id TEXT,
			subject TEXT,
			location TEXT,
			start_time DATETIME,
			end_time DATETIME,
			all_day_event INTEGER NOT NULL DEFAULT 0,
			organizer_name TEXT,
			organizer_email TEXT,
			busy_status INTEGER NOT NULL DEFAULT 0,
			sensitivity INTEGER NOT NULL DEFAULT 0,
			reminder INTEGER,
			body TEXT,
			body_type TEXT DEFAULT 'text',
			recurrence_type INTEGER DEFAULT 0,
			recurrence_until TEXT,
			attendees TEXT,
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS attachments (
			id TEXT PRIMARY KEY,
			email_id TEXT NOT NULL,
			name TEXT NOT NULL,
			mime_type TEXT,
			size INTEGER,
			is_inline INTEGER NOT NULL DEFAULT 0,
			content_id TEXT,
			file_reference TEXT,
			method INTEGER DEFAULT 1,
			FOREIGN KEY (email_id) REFERENCES emails(id) ON DELETE CASCADE
		)`,
		// Persisted ActiveSync SyncKeys per collection, so contacts and
		// calendar syncs resume incrementally instead of full-refetching with
		// the hardcoded "0" key every time.
		`CREATE TABLE IF NOT EXISTS sync_keys (
			account_id TEXT NOT NULL,
			collection_id TEXT NOT NULL,
			sync_key TEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (account_id, collection_id),
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
		)`,
		// Indexes for performance
	}
	for _, m := range createTables {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}

	// Phase 2: Migrate contacts table — add missing columns from v0.1
	// SQLite doesn't support ALTER TABLE ADD COLUMN IF NOT EXISTS, so check pragmas
	contactCols, _ := s.db.Query("PRAGMA table_info(contacts)")
	colExists := map[string]bool{}
	if contactCols != nil {
		for contactCols.Next() {
			var cid int; var name, ctype string; var notnull, dflt int; var pk bool
			contactCols.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
			colExists[name] = true
		}
		contactCols.Close()
	}
	addCols := []struct{ col, typ string }{
		{"first_name", "TEXT"},
		{"last_name", "TEXT"},
		{"email2", "TEXT"},
		{"email3", "TEXT"},
		{"phone", "TEXT"},
		{"mobile", "TEXT"},
		{"company", "TEXT"},
		{"job_title", "TEXT"},
		{"server_id", "TEXT"},
	}
	for _, ac := range addCols {
		if !colExists[ac.col] {
			if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE contacts ADD COLUMN %s %s", ac.col, ac.typ)); err != nil {
				return fmt.Errorf("migration failed adding contacts.%s: %w", ac.col, err)
			}
		}
	}

	// Phase 2.5: Migrate attachments table — add missing columns introduced in v0.4
	attCols, _ := s.db.Query("PRAGMA table_info(attachments)")
	attColExists := map[string]bool{}
	if attCols != nil {
		for attCols.Next() {
			var cid int; var name, ctype string; var notnull, dflt int; var pk bool
			attCols.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
			attColExists[name] = true
		}
		attCols.Close()
	}
	attAddCols := []struct{ col, typ string }{
		{"file_reference", "TEXT"},
		{"method", "INTEGER DEFAULT 1"},
	}
	for _, ac := range attAddCols {
		if !attColExists[ac.col] {
			if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE attachments ADD COLUMN %s %s", ac.col, ac.typ)); err != nil {
				return fmt.Errorf("migration failed adding attachments.%s: %w", ac.col, err)
			}
		}
	}

	// Phase 2.6: Migrate calendar_events table — add attendees column (v0.4.28)
	calCols, _ := s.db.Query("PRAGMA table_info(calendar_events)")
	calColExists := map[string]bool{}
	if calCols != nil {
		for calCols.Next() {
			var cid int; var name, ctype string; var notnull, dflt int; var pk bool
			calCols.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
			calColExists[name] = true
		}
		calCols.Close()
	}
	calAddCols := []struct{ col, typ string }{
		{"attendees", "TEXT"},
	}
	for _, ac := range calAddCols {
		if !calColExists[ac.col] {
			if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE calendar_events ADD COLUMN %s %s", ac.col, ac.typ)); err != nil {
				return fmt.Errorf("migration failed adding calendar_events.%s: %w", ac.col, err)
			}
		}
	}

	// Phase 3: Create indexes (safe to run IF NOT EXISTS)
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_emails_account_date ON emails(account_id, date_received DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_emails_from ON emails(account_id, from_email)`,
		`CREATE INDEX IF NOT EXISTS idx_emails_folder ON emails(folder_id, date_received DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_last_email ON contacts(account_id, last_email_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_emails_contact ON emails(account_id, from_email, date_received DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_calendar_account_time ON calendar_events(account_id, start_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_name ON contacts(account_id, name)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_email ON contacts(account_id, email)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_server ON contacts(account_id, server_id)`,
	}
	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, idx)
		}
	}
	return nil
}

// === Account Operations ===

func (s *Store) GetAccount(id string) (*models.Account, error) {
	row := s.db.QueryRow(`SELECT id, name, email, password, server_url, device_id, device_type, connected, last_sync, created_at FROM accounts WHERE id = ?`, id)
	var a models.Account
	err := row.Scan(&a.ID, &a.Name, &a.Email, &a.Password, &a.ServerURL, &a.DeviceID, &a.DeviceType, &a.Connected, &a.LastSync, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) SaveAccount(a *models.Account) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO accounts (id, name, email, password, server_url, device_id, device_type, connected, last_sync, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Name, a.Email, a.Password, a.ServerURL, a.DeviceID, a.DeviceType, a.Connected, a.LastSync, a.CreatedAt)
	return err
}

func (s *Store) DeleteAccount(id string) error {
	return s.transact(func(tx *sql.Tx) error {
		// Cascading deletes will handle emails, folders, contacts
		if _, err := tx.Exec(`DELETE FROM accounts WHERE id = ?`, id); err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) ListAccounts() ([]*models.Account, error) {
	rows, err := s.db.Query(`SELECT id, name, email, password, server_url, device_id, device_type, connected, last_sync, created_at FROM accounts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []*models.Account
	for rows.Next() {
		a := &models.Account{}
		var lastSync, createdAt sql.NullTime
		if err := rows.Scan(&a.ID, &a.Name, &a.Email, &a.Password, &a.ServerURL, &a.DeviceID, &a.DeviceType, &a.Connected, &lastSync, &createdAt); err != nil {
			return nil, err
		}
		if lastSync.Valid {
			a.LastSync = lastSync.Time
		}
		if createdAt.Valid {
			a.CreatedAt = createdAt.Time
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// === Folder Operations ===

func (s *Store) SaveFolder(f *models.Folder) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO folders (id, account_id, server_id, parent_id, name, type, is_hidden, unread_count, last_sync)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.AccountID, f.ServerID, f.ParentID, f.Name, f.Type, f.IsHidden, f.UnreadCount, f.LastSync)
	return err
}

func (s *Store) ListFolders(accountID string) ([]*models.Folder, error) {
	rows, err := s.db.Query(`SELECT id, account_id, server_id, parent_id, name, type, is_hidden, unread_count, last_sync
		FROM folders WHERE account_id = ? ORDER BY type, name`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var folders []*models.Folder
	for rows.Next() {
		f := &models.Folder{}
		var parentID sql.NullString
		var lastSync sql.NullTime
		if err := rows.Scan(&f.ID, &f.AccountID, &f.ServerID, &parentID, &f.Name, &f.Type, &f.IsHidden, &f.UnreadCount, &lastSync); err != nil {
			return nil, err
		}
		f.ParentID = parentID.String
		if lastSync.Valid {
			f.LastSync = lastSync.Time
		}
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

// === Email Operations ===

// DeleteEmail removes emails from a folder by their server ids (EAS Sync
// Delete commands). Used when the server reports an email deleted so the
// local store mirrors it instead of keeping deleted mail forever.
func (s *Store) DeleteEmails(folderID string, serverIDs []string) error {
	if len(serverIDs) == 0 {
		return nil
	}
	return s.transact(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`DELETE FROM emails WHERE folder_id = ? AND server_id = ?`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, sid := range serverIDs {
			if _, err := stmt.Exec(folderID, sid); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) SaveEmail(e *models.Email) error {
	toEmails := strings.Join(e.ToEmails, ",")
	ccEmails := strings.Join(e.CCEmails, ",")
	_, err := s.db.Exec(`INSERT OR REPLACE INTO emails (id, account_id, folder_id, server_id, from_name, from_email,
		to_text, to_emails, cc_text, cc_emails, subject, thread_topic, date_received,
		is_read, is_flagged, importance, has_attachment, preview, body, body_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.AccountID, e.FolderID, e.ServerID, e.From, e.FromEmail,
		e.To, toEmails, e.CC, ccEmails, e.Subject, e.ThreadTopic, e.DateReceived,
		e.IsRead, e.IsFlagged, e.Importance, e.HasAttachment, e.Preview, e.Body, e.BodyType)
	if err != nil {
		return err
	}
	// Replace this email's attachments (authoritative from the sync response).
	if _, err := s.db.Exec(`DELETE FROM attachments WHERE email_id = ?`, e.ID); err != nil {
		return err
	}
	for _, a := range e.Attachments {
		if _, err := s.db.Exec(`INSERT INTO attachments
			(id, email_id, name, mime_type, size, is_inline, content_id, file_reference, method)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.ID+"/"+a.FileReference, e.ID, a.DisplayName, "", int64(a.EstimatedDataSize),
			boolToInt(a.IsInline), a.ContentId, a.FileReference, int64(a.Method)); err != nil {
			return err
		}
	}
	// Update contact aggregation. Remember the OTHER party, not yourself:
	//  - received mail → the sender;
	//  - sent mail → the recipient(s) (a sent item's From is your own address,
	//    which we skip so your own address never becomes a remembered contact).
	// EAS-synced contacts and remembered addresses share the same table, so
	// this one call feeds both the Contacts view and compose autocomplete.
	if err := s.rememberEmailParties(e); err != nil {
		return err
	}
	return nil
}

// PurgeOwnAddress removes rows for the account's own email from the contacts
// table. Older builds (before rememberEmailParties learned to skip self)
// recorded your own address as a contact when sent-mail From was upserted;
// this cleans that legacy pollution so your own address never shows in the
// Contacts view or compose autocomplete.
func (s *Store) PurgeOwnAddress(accountID string) error {
	acc, err := s.GetAccount(accountID)
	if err != nil || acc == nil {
		return err
	}
	own := strings.ToLower(strings.TrimSpace(acc.Email))
	if own == "" {
		return nil
	}
	_, err = s.db.Exec(`DELETE FROM contacts WHERE account_id = ? AND (
		lower(email) = ? OR lower(email2) = ? OR lower(email3) = ?
	)`, accountID, own, own, own)
	return err
}

func (s *Store) rememberEmailParties(e *models.Email) error {
	acc, err := s.GetAccount(e.AccountID)
	if err != nil {
		// No account row (edge case) — fall back to remembering the sender.
		return s.upsertContact(e.AccountID, e.From, e.FromEmail, e.DateReceived)
	}
	own := strings.ToLower(strings.TrimSpace(acc.Email))
	from := strings.ToLower(strings.TrimSpace(e.FromEmail))
	self := own != "" && (from == own || strings.HasPrefix(from, strings.Split(own, "@")[0]+"@"))

	if self {
		// Sent mail: remember each recipient.
		for _, to := range e.ToEmails {
			t := strings.ToLower(strings.TrimSpace(to))
			if t == "" || t == own || t == from {
				continue
			}
			if err := s.upsertContact(e.AccountID, e.To, t, e.DateReceived); err != nil {
				return err
			}
		}
		return nil
	}
	// Received mail: remember the sender (skip empty).
	if from == "" || from == own {
		return nil
	}
	return s.upsertContact(e.AccountID, e.From, e.FromEmail, e.DateReceived)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CountEmails counts cached emails in a folder. The sync layer uses it to
// pick the EAS filter window: empty store ⇒ full 180-day backfill; cached
// mail ⇒ narrow 24h window so restarts stay fast without relying on SOGo's
// flaky resumed-key syncs.
func (s *Store) CountEmails(folderID string) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM emails WHERE folder_id = ?`, folderID).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return n, err
}

func (s *Store) ListEmails(accountID, folderID string, offset, limit int) ([]*models.Email, error) {
	rows, err := s.db.Query(`SELECT id, account_id, folder_id, server_id, from_name, from_email,
		to_text, to_emails, cc_text, cc_emails, subject, thread_topic, date_received,
		is_read, is_flagged, importance, has_attachment, preview, body, body_type
		FROM emails WHERE account_id = ? AND folder_id = ?
		ORDER BY date_received DESC LIMIT ? OFFSET ?`, accountID, folderID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanEmails(rows)
}

func (s *Store) ListEmailsByContact(accountID, contactEmail string, offset, limit int) ([]*models.Email, error) {
	rows, err := s.db.Query(`SELECT id, account_id, folder_id, server_id, from_name, from_email,
		to_text, to_emails, cc_text, cc_emails, subject, thread_topic, date_received,
		is_read, is_flagged, importance, has_attachment, preview, body, body_type
		FROM emails WHERE account_id = ? AND (from_email = ? OR to_emails LIKE ?)
		ORDER BY date_received DESC LIMIT ? OFFSET ?`,
		accountID, contactEmail, "%"+contactEmail+"%", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanEmails(rows)
}

func (s *Store) scanEmails(rows *sql.Rows) ([]*models.Email, error) {
	var emails []*models.Email
	for rows.Next() {
		e := &models.Email{}
		var toEmails, ccEmails sql.NullString
		var threadTopic, body, bodyType sql.NullString
		if err := rows.Scan(&e.ID, &e.AccountID, &e.FolderID, &e.ServerID, &e.From, &e.FromEmail,
			&e.To, &toEmails, &e.CC, &ccEmails, &e.Subject, &threadTopic, &e.DateReceived,
			&e.IsRead, &e.IsFlagged, &e.Importance, &e.HasAttachment, &e.Preview, &body, &bodyType); err != nil {
			return nil, err
		}
		if toEmails.Valid {
			e.ToEmails = strings.Split(toEmails.String, ",")
		}
		if ccEmails.Valid {
			e.CCEmails = strings.Split(ccEmails.String, ",")
		}
		e.ThreadTopic = threadTopic.String
		e.Body = body.String
		e.BodyType = bodyType.String
		emails = append(emails, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.loadAttachments(emails), nil
}

// loadAttachments batch-loads attachment rows for the given emails and
// attaches them. file_reference is the SOGo handle used by ItemOperations
// Fetch to download the bytes.
func (s *Store) loadAttachments(emails []*models.Email) []*models.Email {
	if len(emails) == 0 {
		return emails
	}
	ids := make([]string, len(emails))
	byID := make(map[string]*models.Email, len(emails))
	for i, e := range emails {
		ids[i] = e.ID
		byID[e.ID] = e
	}
	q := `SELECT email_id, name, mime_type, size, is_inline, content_id, file_reference, method
		FROM attachments WHERE email_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + `)
		ORDER BY rowid`
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return emails
	}
	defer rows.Close()
	for rows.Next() {
		var emailID, name, fileRef string
		var mimeType, contentID sql.NullString
		var size, isInline int64
		var method sql.NullInt64
		if err := rows.Scan(&emailID, &name, &mimeType, &size, &isInline, &contentID, &fileRef, &method); err != nil {
			continue
		}
		e, ok := byID[emailID]
		if !ok {
			continue
		}
		m := 1
		if method.Valid {
			m = int(method.Int64)
		}
		e.Attachments = append(e.Attachments, models.EmailAttachment{
			DisplayName:       name,
			FileReference:     fileRef,
			ContentId:         contentID.String,
			IsInline:          isInline == 1,
			Method:            m,
			EstimatedDataSize: int(size),
		})
	}
	return emails
}

// === Contact Operations ===

func (s *Store) upsertContact(accountID, name, email string, lastEmail time.Time) error {
	_, err := s.db.Exec(`INSERT INTO contacts (id, account_id, name, email, last_email_at, email_count)
		VALUES (?, ?, ?, ?, ?, 1)
		ON CONFLICT(account_id, email) DO UPDATE SET
			name = COALESCE(?, name),
			last_email_at = CASE WHEN ? > last_email_at THEN ? ELSE last_email_at END,
			email_count = email_count + 1`,
		email+"@"+accountID, accountID, name, email, lastEmail, name, lastEmail, lastEmail)
	return err
}

func (s *Store) ListContactsByLastEmail(accountID string) ([]*models.Contact, error) {
	rows, err := s.db.Query(`SELECT id, account_id, name, email, avatar_url, last_email_at, email_count, is_favorite
		FROM contacts WHERE account_id = ?
		ORDER BY is_favorite DESC, last_email_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var contacts []*models.Contact
	for rows.Next() {
		c := &models.Contact{}
		var avatarURL sql.NullString
		var lastEmail sql.NullTime
		if err := rows.Scan(&c.ID, &c.AccountID, &c.Name, &c.Email, &avatarURL, &lastEmail, &c.EmailCount, &c.IsFavorite); err != nil {
			return nil, err
		}
		c.AvatarURL = avatarURL.String
		if lastEmail.Valid {
			c.LastEmailAt = lastEmail.Time
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

func (s *Store) SearchContacts(accountID, query string) ([]*models.Contact, error) {
	rows, err := s.db.Query(`SELECT id, account_id, name, email, avatar_url, last_email_at, email_count, is_favorite
		FROM contacts WHERE account_id = ? AND (name LIKE ? OR email LIKE ?)
		ORDER BY last_email_at DESC`, accountID, "%"+query+"%", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var contacts []*models.Contact
	for rows.Next() {
		c := &models.Contact{}
		var avatarURL sql.NullString
		var lastEmail sql.NullTime
		if err := rows.Scan(&c.ID, &c.AccountID, &c.Name, &c.Email, &avatarURL, &lastEmail, &c.EmailCount, &c.IsFavorite); err != nil {
			return nil, err
		}
		c.AvatarURL = avatarURL.String
		if lastEmail.Valid {
			c.LastEmailAt = lastEmail.Time
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

func (s *Store) SaveContact(c *models.Contact) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO contacts (id, account_id, name, first_name, last_name, email, email2, email3, phone, mobile, company, job_title, avatar_url, server_id, last_email_at, email_count, is_favorite)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.AccountID, c.Name, c.FirstName, c.LastName, c.Email, c.Email2, c.Email3, c.Phone, c.Mobile, c.Company, c.JobTitle, c.AvatarURL, c.ServerID, c.LastEmailAt, c.EmailCount, c.IsFavorite)
	return err
}

func (s *Store) ListCalendarEvents(accountID, folderID string) ([]*models.CalendarEvent, error) {
	rows, err := s.db.Query(`SELECT id, account_id, server_id, subject, location, start_time, end_time,
		all_day_event, organizer_name, organizer_email, busy_status, sensitivity, reminder, body, body_type, recurrence_type, attendees
		FROM calendar_events WHERE account_id = ?
		ORDER BY start_time DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []*models.CalendarEvent
	for rows.Next() {
		e := &models.CalendarEvent{}
		var location, orgName, orgEmail, body, bodyType, attendees sql.NullString
		var reminder sql.NullInt64
		var recurrence sql.NullInt64
		if err := rows.Scan(&e.ID, &e.AccountID, &e.ServerID, &e.Subject, &location, &e.StartTime, &e.EndTime,
			&e.AllDayEvent, &orgName, &orgEmail, &e.BusyStatus, &e.Sensitivity, &reminder, &body, &bodyType, &recurrence, &attendees); err != nil {
			return nil, err
		}
		e.Location = location.String
		e.OrganizerName = orgName.String
		e.OrganizerEmail = orgEmail.String
		e.Body = body.String
		e.BodyType = bodyType.String
		if reminder.Valid {
			e.Reminder = int(reminder.Int64)
		}
		if recurrence.Valid {
			e.RecurrenceType = int(recurrence.Int64)
		}
		e.Attendees = parseAttendees(attendees.String)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) SaveCalendarEvent(e *models.CalendarEvent) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO calendar_events (id, account_id, folder_id, server_id, subject, location, start_time, end_time,
		all_day_event, organizer_name, organizer_email, busy_status, sensitivity, reminder, body, body_type, recurrence_type, attendees)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.AccountID, e.AccountID+"-cal", e.ServerID, e.Subject, e.Location, e.StartTime, e.EndTime,
		e.AllDayEvent, e.OrganizerName, e.OrganizerEmail, e.BusyStatus, e.Sensitivity, e.Reminder, e.Body, e.BodyType, e.RecurrenceType, attendeesJSON(e.Attendees))
	return err
}

func attendeesJSON(list []string) string {
	if len(list) == 0 {
		return ""
	}
	b, _ := json.Marshal(list)
	return string(b)
}

func parseAttendees(s string) []string {
	if s == "" {
		return nil
	}
	var list []string
	if err := json.Unmarshal([]byte(s), &list); err != nil {
		return nil
	}
	return list
}

// ReplaceCalendarEvents reconciles an account's calendar with the fresh batch
// the server just returned. SOGo hands out a fresh ServerID for the same event
// on every sync, so upserting by ServerID-derived id duplicates every entry per
// restart. The server is the source of truth for the calendar folder, so we
// delete the account's events and re-insert the returned set atomically.
func (s *Store) ReplaceCalendarEvents(accountID, folderID string, events []*models.CalendarEvent) error {
	return s.transact(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM calendar_events WHERE account_id = ? AND folder_id = ?`, accountID, folderID); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO calendar_events (id, account_id, folder_id, server_id, subject, location, start_time, end_time,
			all_day_event, organizer_name, organizer_email, busy_status, sensitivity, reminder, body, body_type, recurrence_type, attendees)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, e := range events {
			if _, err := stmt.Exec(e.ID, e.AccountID, e.AccountID+"-cal", e.ServerID, e.Subject, e.Location, e.StartTime, e.EndTime,
				e.AllDayEvent, e.OrganizerName, e.OrganizerEmail, e.BusyStatus, e.Sensitivity, e.Reminder, e.Body, e.BodyType, e.RecurrenceType, attendeesJSON(e.Attendees)); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteCalendarEvent removes a single event from the store by its primary id.
func (s *Store) DeleteCalendarEvent(id string) error {
	_, err := s.db.Exec(`DELETE FROM calendar_events WHERE id = ?`, id)
	return err
}

// GetSyncKey returns the persisted ActiveSync SyncKey for a collection, or
// "" if none is stored (first sync). With no stored key, callers fall back
// to "0" and do a full initial sync.
func (s *Store) GetSyncKey(accountID, collectionID string) (string, error) {
	var k string
	err := s.db.QueryRow(`SELECT sync_key FROM sync_keys WHERE account_id = ? AND collection_id = ?`, accountID, collectionID).Scan(&k)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return k, err
}

// SetSyncKey persists a collection's SyncKey so the next sync resumes
// incrementally rather than restarting from "0".
func (s *Store) SetSyncKey(accountID, collectionID, syncKey string) error {
	_, err := s.db.Exec(`INSERT INTO sync_keys (account_id, collection_id, sync_key) VALUES (?, ?, ?)
		ON CONFLICT(account_id, collection_id) DO UPDATE SET sync_key = excluded.sync_key, updated_at = CURRENT_TIMESTAMP`,
		accountID, collectionID, syncKey)
	return err
}

// MetaGet/MetaSet store small app-level key/value rows (e.g. the last-run
// timestamp of the daily reconcile) so background jobs catch up when the app
// is next opened, even if it was closed across the scheduled window. Rows are
// namespaced under account_id='__meta__' to reuse the sync_keys table.
func (s *Store) MetaGet(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT sync_key FROM sync_keys WHERE account_id = '__meta__' AND collection_id = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (s *Store) MetaSet(key, value string) error {
	return s.SetSyncKey("__meta__", key, value)
}

// === Helpers ===

func (s *Store) transact(fn func(*sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}