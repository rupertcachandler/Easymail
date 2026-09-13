package models

import "time"

// Account represents an email account configuration
type Account struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Password      string    `json:"password"`
	ServerURL      string    `json:"serverUrl"`
	DeviceID      string    `json:"deviceId"`
	DeviceType    string    `json:"deviceType"`
	Connected     bool      `json:"connected"`
	OofState      string    `json:"oofState,omitempty"`    // OOF: "", "enabled", "disabled"
	OofExternal   string    `json:"oofExternal,omitempty"` // External OOF reply
	OofInternal   string    `json:"oofInternal,omitempty"` // Internal OOF reply
	LastSync      time.Time `json:"lastSync"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Folder represents an email folder (synced from server)
type Folder struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"accountId"`
	ServerID    string    `json:"serverId"`
	ParentID    string    `json:"parentId,omitempty"`
	Name        string    `json:"name"`
	Type        int       `json:"type"` // EAS folder type: 2=Inbox, 3=Drafts, 4=Trash, 5=Sent, 7=Tasks, 8=Calendar, 9=Contacts, 12=Custom
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
	HasAttachment bool              `json:"hasAttachment"`
	Attachments   []EmailAttachment `json:"attachments,omitempty"`
	Preview       string            `json:"preview"`
	Body          string            `json:"body,omitempty"`
	BodyType      string            `json:"bodyType,omitempty"` // "html" or "text"
}

// EmailAttachment is a single attachment referenced on an email. FileReference
// is the SOGo handle (mail/FOLDER/UID/PATH) used by ItemOperations Fetch to
// download the bytes.
type EmailAttachment struct {
	DisplayName       string `json:"displayName,omitempty"`
	FileReference     string `json:"fileReference,omitempty"`
	ContentId         string `json:"contentId,omitempty"`
	IsInline          bool   `json:"isInline,omitempty"`
	Method            int    `json:"method,omitempty"`
	EstimatedDataSize int    `json:"estimatedDataSize,omitempty"`
}

// Contact represents a person (synced from EAS Contacts + aggregated from email)
type Contact struct {
	ID                  string    `json:"id"`
	AccountID           string    `json:"accountId"`
	Name                string    `json:"name"`
	FirstName           string    `json:"firstName,omitempty"`
	LastName            string    `json:"lastName,omitempty"`
	Email               string    `json:"email"`
	Email2              string    `json:"email2,omitempty"`
	Email3              string    `json:"email3,omitempty"`
	Phone               string    `json:"phone,omitempty"`
	Mobile              string    `json:"mobile,omitempty"`
	Company             string    `json:"company,omitempty"`
	JobTitle            string    `json:"jobTitle,omitempty"`
	AvatarURL           string    `json:"avatarUrl,omitempty"`
	LastEmailAt         time.Time `json:"lastEmailAt"`
	EmailCount          int       `json:"emailCount"`
	IsFavorite          bool      `json:"isFavorite"`
	ServerID            string    `json:"serverId,omitempty"` // EAS server ID for synced contacts
}

// CalendarEvent represents a calendar appointment (synced from EAS)
type CalendarEvent struct {
	ID              string    `json:"id"`
	AccountID       string    `json:"accountId"`
	ServerID        string    `json:"serverId"`
	Subject         string    `json:"subject"`
	Location        string    `json:"location,omitempty"`
	StartTime       time.Time `json:"startTime"`
	EndTime         time.Time `json:"endTime"`
	AllDayEvent     bool      `json:"allDayEvent"`
	OrganizerName   string    `json:"organizerName,omitempty"`
	OrganizerEmail  string    `json:"organizerEmail,omitempty"`
	Attendees       []string  `json:"attendees,omitempty"` // invitee email addresses
	BusyStatus      int       `json:"busyStatus"` // 0=Free, 1=Tentative, 2=Busy, 3=OOF
	Sensitivity     int       `json:"sensitivity"` // 0=Normal, 1=Personal, 2=Private, 3=Confidential
	Reminder        int       `json:"reminder,omitempty"` // Minutes before
	Body            string    `json:"body,omitempty"`
	BodyType        string    `json:"bodyType,omitempty"`
	RecurrenceType  int       `json:"recurrenceType,omitempty"`
	RecurrenceUntil string    `json:"recurrenceUntil,omitempty"`
}

// OOFSettings represents Out-of-Office / Holiday message settings
type OOFSettings struct {
	State     string `json:"state"`     // "enabled", "disabled"
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
	Internal  string `json:"internal"`  // Reply for internal senders
	External  string `json:"external"`  // Reply for external senders
}

// Attachment represents an email attachment
type Attachment struct {
	ID        string `json:"id"`
	EmailID   string `json:"emailId"`
	Name      string `json:"name"`
	MimeType  string `json:"mimeType"`
	Size      int64  `json:"size"`
	IsInline  bool   `json:"isInline"`
	ContentID string `json:"contentId,omitempty"`
}