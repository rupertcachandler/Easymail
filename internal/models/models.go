package models

import "time"

// Account represents an email account configuration
type Account struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Password    string    `json:"password"`
	ServerURL   string    `json:"serverUrl"`
	DeviceID    string    `json:"deviceId"`
	DeviceType  string    `json:"deviceType"`
	Connected   bool      `json:"connected"`
	LastSync    time.Time `json:"lastSync"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Folder represents an email folder (synced from server)
type Folder struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"accountId"`
	ServerID    string    `json:"serverId"`
	ParentID    string    `json:"parentId,omitempty"`
	Name        string    `json:"name"`
	Type        int       `json:"type"` // EAS folder type: 1=Generic, 2=Inbox, etc.
	IsHidden    bool      `json:"isHidden"`
	UnreadCount int       `json:"unreadCount"`
	LastSync    time.Time `json:"lastSync"`
}

// Email represents a cached email message
type Email struct {
	ID            string    `json:"id"`
	AccountID     string    `json:"accountId"`
	FolderID      string    `json:"folderId"`
	ServerID      string    `json:"serverId"`
	From          string    `json:"from"`
	FromEmail     string    `json:"fromEmail"`
	To            string    `json:"to"`
	ToEmails      []string  `json:"toEmails"`
	CC            string    `json:"cc,omitempty"`
	CCEmails      []string  `json:"ccEmails,omitempty"`
	Subject       string    `json:"subject"`
	ThreadTopic   string    `json:"threadTopic,omitempty"`
	DateReceived  time.Time `json:"dateReceived"`
	IsRead        bool      `json:"isRead"`
	IsFlagged     bool      `json:"isFlagged"`
	Importance    int       `json:"importance"` // 0=Low, 1=Normal, 2=High
	HasAttachment bool      `json:"hasAttachment"`
	Preview       string    `json:"preview"` // First ~200 chars of body
	Body          string    `json:"body,omitempty"`
	BodyType      string    `json:"bodyType,omitempty"` // "html" or "text"
}

// Contact represents a person (aggregated from email from/to fields)
type Contact struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"accountId"`
	Name        string    `json:"name"`
	Email        string    `json:"email"`
	AvatarURL   string    `json:"avatarUrl,omitempty"`
	LastEmailAt time.Time `json:"lastEmailAt"`
	EmailCount  int       `json:"emailCount"`
	IsFavorite  bool      `json:"isFavorite"`
}

// Attachment represents an email attachment
type Attachment struct {
	ID        string `json:"id"`
	EmailID   string `json:"emailId"`
	Name      string `json:"name"`
	MimeType  string `json:"mimeType"`
	Size      int64  `json:"size"`
	IsInline  bool   `json:"isInline"`
	ContentID string `json:"contentId,omitempty"` // For inline images
}