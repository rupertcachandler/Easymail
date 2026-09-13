package eas

// Importance enum values per MS-ASEMAIL §2.2.2.40.
const (
	ImportanceLow    int32 = 0
	ImportanceNormal int32 = 1
	ImportanceHigh   int32 = 2
)

// BodyType values per MS-ASAIRSYNCBASE
const (
	BodyTypePlain int32 = 1
	BodyTypeHTML  int32 = 2
	BodyTypeRTF   int32 = 3
	BodyTypeMIME  int32 = 4
)

// AirSyncBaseBody represents the nested Body element from the AirSyncBase
// code page. Z-Push sends Type+Data inside a Body wrapper element.
type AirSyncBaseBody struct {
	XMLName struct{} `wbxml:"AirSyncBase.Body"`
	Type    int32    `wbxml:"AirSyncBase.Type,omitempty"`
	Data    string   `wbxml:"AirSyncBase.Data,omitempty"`
}

// Attachment represents an Email attachment from the AirSyncBase.Attachments
// block (MS-ASBASE 2.2.2.6). SOGo emits this block under the AirSyncBase
// namespace with a FileReference of the form `mail/FOLDER/UID/PATH`, which is
// the handle ItemOperations Fetch uses to download the bytes.
type Attachment struct {
	XMLName             struct{} `wbxml:"AirSyncBase.Attachment"`
	DisplayName         string   `wbxml:"AirSyncBase.DisplayName,omitempty"`
	FileReference       string   `wbxml:"AirSyncBase.FileReference,omitempty"`
	ContentId           string   `wbxml:"AirSyncBase.ContentId,omitempty"`
	ContentLocation     string   `wbxml:"AirSyncBase.ContentLocation,omitempty"`
	IsInline            int32    `wbxml:"AirSyncBase.IsInline,omitempty"`
	Method              int32    `wbxml:"AirSyncBase.Method,omitempty"`
	EstimatedDataSize   int32    `wbxml:"AirSyncBase.EstimatedDataSize,omitempty"`
}

// AirSyncBaseAttachments is the container wrapper around the Attachment list.
type AirSyncBaseAttachments struct {
	XMLName     struct{}     `wbxml:"AirSyncBase.Attachments"`
	Attachment  []Attachment `wbxml:"AirSyncBase.Attachment,omitempty"`
}

// Email is the sync representation of an e-mail message (MS-ASEMAIL 14.1).
// It marshals as the AirSync.ApplicationData wrapper used inside Sync
// commands, with Email-page child elements.
type Email struct {
	XMLName      struct{}        `wbxml:"AirSync.ApplicationData"`
	DateReceived string          `wbxml:"Email.DateReceived,omitempty"`
	Subject      string          `wbxml:"Email.Subject,omitempty"`
	From         string          `wbxml:"Email.From,omitempty"`
	To           string          `wbxml:"Email.To,omitempty"`
	Cc           string          `wbxml:"Email.Cc,omitempty"`
	ReplyTo      string          `wbxml:"Email.ReplyTo,omitempty"`
	DisplayTo    string          `wbxml:"Email.DisplayTo,omitempty"`
	ThreadTopic  string          `wbxml:"Email.ThreadTopic,omitempty"`
	Importance   int32           `wbxml:"Email.Importance,omitempty"`
	Read         bool            `wbxml:"Email.Read,omitempty"`
	FlagStatus   int32           `wbxml:"Email.FlagStatus,omitempty"` // 0=not flagged, 1=completed, 2=flagged
	HasAttach    bool                  `wbxml:"Email2.HasAttachment,omitempty"`
	MessageClass string                `wbxml:"Email.MessageClass,omitempty"`
	ContentClass string                `wbxml:"Email.ContentClass,omitempty"`
	Body         AirSyncBaseBody       `wbxml:"AirSyncBase.Body,omitempty"`
	Preview      string                `wbxml:"AirSyncBase.Preview,omitempty"`
	Attachments  AirSyncBaseAttachments `wbxml:"AirSyncBase.Attachments,omitempty"`
}
