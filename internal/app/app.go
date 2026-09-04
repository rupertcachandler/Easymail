package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"easymail/internal/activesync"
	"easymail/internal/models"
	"easymail/internal/store"
)

// App is the main application context, bound to the frontend
type App struct {
	ctx     context.Context
	store   *store.Store
	clients map[string]*activesync.Client // account ID -> client
}

// New creates a new App instance
func New() (*App, error) {
	// Find config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find config dir: %w", err)
	}
	dbPath := filepath.Join(configDir, "easymail", "easymail.db")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, fmt.Errorf("cannot create config dir: %w", err)
	}

	s, err := store.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open database: %w", err)
	}

	return &App{
		store:   s,
		clients: make(map[string]*activesync.Client),
	}, nil
}

// Startup is called when the app starts
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("EasyMail starting up")

	// Load saved accounts and reconnect
	accounts, err := a.store.ListAccounts()
	if err != nil {
		log.Printf("Warning: cannot load accounts: %v", err)
		return
	}
	for _, acc := range accounts {
		go a.connectAccount(acc)
	}
}

// OnShutdown is called when the app is shutting down
func (a *App) OnShutdown(ctx context.Context) {
	log.Println("EasyMail shutting down")
}

// OnBeforeClose asks the user before closing
func (a *App) OnBeforeClose(ctx context.Context) bool {
	return false // Allow closing
}

// Shutdown is called on exit
func (a *App) Shutdown() {
	if a.store != nil {
		a.store.Close()
	}
}

// === Account Management ===

// AddAccount adds a new email account
func (a *App) AddAccount(name, email, password, serverURL string) (*models.Account, error) {
	acc := &models.Account{
		Name:       name,
		Email:      email,
		Password:   password,
		ServerURL:  serverURL,
		DeviceID:   activesync.GenerateDeviceID(),
		DeviceType: "EasyMail",
	}
	if err := a.store.SaveAccount(acc); err != nil {
		return nil, fmt.Errorf("cannot save account: %w", err)
	}
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
	return a.store.ListFolders(accountID)
}

// SyncFolders triggers a folder sync for an account
func (a *App) SyncFolders(accountID string) error {
	client, ok := a.clients[accountID]
	if !ok {
		return fmt.Errorf("account not connected")
	}
	folders, err := client.SyncFolders(a.ctx)
	if err != nil {
		return fmt.Errorf("folder sync failed: %w", err)
	}
	for _, f := range folders {
		if err := a.store.SaveFolder(f); err != nil {
			log.Printf("Warning: cannot save folder %s: %v", f.ServerID, err)
		}
	}
	runtime.EventsEmit(a.ctx, "folders-updated", accountID)
	return nil
}

// === Email Operations ===

// GetEmails returns emails for a folder
func (a *App) GetEmails(accountID, folderID string, offset, limit int) ([]*models.Email, error) {
	return a.store.ListEmails(accountID, folderID, offset, limit)
}

// GetEmailsByContact returns all emails exchanged with a contact
func (a *App) GetEmailsByContact(accountID, contactEmail string, offset, limit int) ([]*models.Email, error) {
	return a.store.ListEmailsByContact(accountID, contactEmail, offset, limit)
}

// SyncEmails triggers an email sync for a folder
func (a *App) SyncEmails(accountID, folderID string) error {
	client, ok := a.clients[accountID]
	if !ok {
		return fmt.Errorf("account not connected")
	}
	emails, err := client.SyncEmails(a.ctx, folderID)
	if err != nil {
		return fmt.Errorf("email sync failed: %w", err)
	}
	for _, e := range emails {
		if err := a.store.SaveEmail(e); err != nil {
			log.Printf("Warning: cannot save email %s: %v", e.ServerID, err)
		}
	}
	runtime.EventsEmit(a.ctx, "emails-updated", accountID, folderID)
	return nil
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

// === Internal ===

func (a *App) connectAccount(acc *models.Account) {
	client, err := activesync.NewClient(acc)
	if err != nil {
		log.Printf("Failed to create client for %s: %v", acc.Email, err)
		runtime.EventsEmit(a.ctx, "account-error", acc.ID, err.Error())
		return
	}
	if err := client.Connect(a.ctx); err != nil {
		log.Printf("Failed to connect %s: %v", acc.Email, err)
		runtime.EventsEmit(a.ctx, "account-error", acc.ID, err.Error())
		return
	}
	a.clients[acc.ID] = client
	runtime.EventsEmit(a.ctx, "account-connected", acc.ID)
	log.Printf("Connected: %s", acc.Email)
}