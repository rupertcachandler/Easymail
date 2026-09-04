package activesync

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/remdev/go-activesync/autodiscover"
	"github.com/remdev/go-activesync/client"
	"github.com/remdev/go-activesync/eas"

	"easymail/internal/models"
)

// GenerateDeviceID creates a stable device identifier for EAS
func GenerateDeviceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("easymail-%x", b)
}

// Client wraps the go-activesync library for EAS communication
type Client struct {
	account   *models.Account
	easClient *client.Client
	syncKeys  map[string]string // folderID -> syncKey
	connected bool
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

// Connect establishes the EAS connection (autodiscover + provision)
func (c *Client) Connect(ctx context.Context) error {
	var baseURL string
	var err error

	// Use explicit server URL or autodiscover
	if c.account.ServerURL != "" {
		baseURL = c.account.ServerURL
	} else {
		// Autodiscover
		ad := autodiscover.New(nil) // uses http.DefaultClient
		resp, err := ad.Discover(ctx, c.account.Email, &autodiscover.Credentials{
			Username: c.account.Email,
			Password: c.account.Password,
		})
		if err != nil {
			return fmt.Errorf("autodiscover failed: %w", err)
		}
		baseURL = resp.URL
	}

	// Create EAS client
	c.easClient, err = client.New(client.Config{
		BaseURL:    baseURL,
		Auth:       &client.BasicAuth{Username: c.account.Email, Password: c.account.Password},
		DeviceID:   c.account.DeviceID,
		DeviceType: c.account.DeviceType,
		UserAgent:  "EasyMail/1.0",
	})
	if err != nil {
		return fmt.Errorf("client creation failed: %w", err)
	}

	// Provision the device
	if _, err := c.easClient.Provision(ctx, c.account.Email); err != nil {
		return fmt.Errorf("provision failed: %w", err)
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
			Name:      add.DisplayName,
			Type:      int(add.Type),
		}
		folder.ID = fmt.Sprintf("%s-%s", c.account.ID, add.ServerID)
		folders = append(folders, folder)
	}

	return folders, nil
}

// SyncEmails syncs emails from a specific folder
func (c *Client) SyncEmails(ctx context.Context, folderID string) ([]*models.Email, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Initial sync (sync key "0")
	syncKey := "0"
	if sk, ok := c.syncKeys[folderID]; ok {
		syncKey = sk
	}

	resp, err := client.SyncTyped[eas.Email](ctx, c.easClient, c.account.Email, &eas.SyncRequest{
		Collections: eas.SyncCollections{
			Collection: []eas.SyncCollection{{
				SyncKey:      syncKey,
				CollectionID: folderID,
				GetChanges:   1,
				WindowSize:   25,
			}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("email sync failed: %w", err)
	}

	var emails []*models.Email
	for _, col := range resp.Collections {
		// Store the sync key for incremental syncs
		c.syncKeys[folderID] = col.SyncKey

		for _, add := range col.Add {
			if add.ApplicationData == nil {
				continue
			}
			email := easEmailToModel(add.ApplicationData, c.account.ID, folderID)
			emails = append(emails, email)
		}
	}

	return emails, nil
}

// easEmailToModel converts an EAS Email to our internal model
func easEmailToModel(e *eas.Email, accountID, folderID string) *models.Email {
	email := &models.Email{
		AccountID:     accountID,
		FolderID:      fmt.Sprintf("%s-%s", accountID, folderID),
		ServerID:      "", // Set from sync response
		From:          e.From,
		FromEmail:     ParseEmailAddress(e.From),
		To:            e.To,
		ToEmails:      ParseEmailList(e.To),
		CC:            e.Cc,
		CCEmails:      ParseEmailList(e.Cc),
		Subject:       e.Subject,
		ThreadTopic:   e.ThreadTopic,
		IsRead:        e.Read,
		IsFlagged:     false,
		Importance:    int(e.Importance),
		HasAttachment: false,
		Preview:       "",
		Body:          "",
		BodyType:      "text",
	}
	email.ID = fmt.Sprintf("%s-%s", accountID, email.ServerID)
	return email
}

// ParseEmailAddress extracts just the email part from "Name <email>" format
func ParseEmailAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if i := strings.Index(addr, "<"); i >= 0 {
		if j := strings.Index(addr, ">"); j > i {
			return addr[i+1 : j]
		}
	}
	return addr
}

// ParseEmailList splits a comma-separated list of email addresses
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

// clientWithTLS is not yet implemented - will add TLS config for self-signed certs
// func clientWithTLS() *http.Client { ... }