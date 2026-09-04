package store

import (
	"database/sql"
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
	migrations := []string{
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
			email TEXT NOT NULL,
			avatar_url TEXT,
			last_email_at DATETIME,
			email_count INTEGER NOT NULL DEFAULT 0,
			is_favorite INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			UNIQUE(account_id, email)
		)`,
		`CREATE TABLE IF NOT EXISTS attachments (
			id TEXT PRIMARY KEY,
			email_id TEXT NOT NULL,
			name TEXT NOT NULL,
			mime_type TEXT,
			size INTEGER,
			is_inline INTEGER NOT NULL DEFAULT 0,
			content_id TEXT,
			FOREIGN KEY (email_id) REFERENCES emails(id) ON DELETE CASCADE
		)`,
		// Indexes for performance
		`CREATE INDEX IF NOT EXISTS idx_emails_account_date ON emails(account_id, date_received DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_emails_from ON emails(account_id, from_email)`,
		`CREATE INDEX IF NOT EXISTS idx_emails_folder ON emails(folder_id, date_received DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_last_email ON contacts(account_id, last_email_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_emails_contact ON emails(account_id, from_email, date_received DESC)`,
	}
	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}
	return nil
}

// === Account Operations ===

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
	// Update contact aggregation
	return s.upsertContact(e.AccountID, e.From, e.FromEmail, e.DateReceived)
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
	return emails, rows.Err()
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