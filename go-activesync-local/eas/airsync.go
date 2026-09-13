package eas

import "github.com/remdev/go-activesync/wbxml"

// SyncRequest is the MS-ASCMD Sync command request payload.
type SyncRequest struct {
	XMLName     struct{}        `wbxml:"AirSync.Sync"`
	Collections SyncCollections `wbxml:"AirSync.Collections"`
}

// SyncResponse is the MS-ASCMD Sync command response payload.
type SyncResponse struct {
	XMLName     struct{}        `wbxml:"AirSync.Sync"`
	Status      int32           `wbxml:"AirSync.Status,omitempty"`
	Collections SyncCollections `wbxml:"AirSync.Collections"`
}

// SyncCollections wraps the Collection entries in a Sync request/response.
type SyncCollections struct {
	Collection []SyncCollection `wbxml:"AirSync.Collection"`
}

// SyncCollection is a per-collection entry inside a Sync request/response.
type SyncCollection struct {
	SyncKey       string        `wbxml:"AirSync.SyncKey"`
	CollectionID  string        `wbxml:"AirSync.CollectionId"`
	Class         string        `wbxml:"AirSync.Class,omitempty"`
	GetChanges    int32         `wbxml:"AirSync.GetChanges,omitempty"`
	WindowSize    int32         `wbxml:"AirSync.WindowSize,omitempty"`
	Status        int32         `wbxml:"AirSync.Status,omitempty"`
	MoreAvailable int32         `wbxml:"AirSync.MoreAvailable,omitempty"`
	Options       *SyncOptions  `wbxml:"AirSync.Options,omitempty"`
	Commands      *SyncCommands `wbxml:"AirSync.Commands,omitempty"`
	Responses     *SyncCommands `wbxml:"AirSync.Responses,omitempty"`
}

// SyncOptions is the per-collection Options element for a Sync request.
type SyncOptions struct {
	FilterType     int32            `wbxml:"AirSync.FilterType,omitempty"`
	Class          string           `wbxml:"AirSync.Class,omitempty"`
	MIMESupport    int32            `wbxml:"AirSync.MIMESupport,omitempty"`
	MIMETruncation int32            `wbxml:"AirSync.MIMETruncation,omitempty"`
	MaxItems       int32            `wbxml:"AirSync.MaxItems,omitempty"`
	BodyPreference []BodyPreference `wbxml:"AirSyncBase.BodyPreference,omitempty"`
}

// BodyPreference is the AirSyncBase preference declaration.
type BodyPreference struct {
	Type           int32 `wbxml:"AirSyncBase.Type"`
	TruncationSize int32 `wbxml:"AirSyncBase.TruncationSize,omitempty"`
	AllOrNone      int32 `wbxml:"AirSyncBase.AllOrNone,omitempty"`
	Preview        int32 `wbxml:"AirSyncBase.Preview,omitempty"`
}

// SyncCommands wraps Add/Change/Delete/Fetch commands inside a Sync
// request/response.
type SyncCommands struct {
	Add    []SyncAdd    `wbxml:"AirSync.Add,omitempty"`
	Change []SyncChange `wbxml:"AirSync.Change,omitempty"`
	Delete []SyncDelete `wbxml:"AirSync.Delete,omitempty"`
	Fetch  []SyncFetch  `wbxml:"AirSync.Fetch,omitempty"`
}

// SyncAdd carries a server-pushed addition or a client-side new item.
//
// ApplicationData is left as a raw WBXML element because the concrete payload
// type (Email, Appointment, Contact, Task, …) depends on the collection's
// Class. Use the convenience methods on SyncAdd / SyncChange (Email,
// Appointment, Contact, Task) or UnmarshalApplicationData[T] to decode the
// payload into a typed value.
type SyncAdd struct {
	ServerID        string            `wbxml:"AirSync.ServerId,omitempty"`
	ClientID        string            `wbxml:"AirSync.ClientId,omitempty"`
	ApplicationData *wbxml.RawElement `wbxml:"AirSync.ApplicationData,omitempty,raw"`
}

// SyncChange carries an item modification.
type SyncChange struct {
	ServerID        string            `wbxml:"AirSync.ServerId"`
	ApplicationData *wbxml.RawElement `wbxml:"AirSync.ApplicationData,omitempty,raw"`
}

// SyncDelete carries an item deletion notification.
type SyncDelete struct {
	ServerID string `wbxml:"AirSync.ServerId"`
}

// SyncFetch carries an explicit Fetch request.
type SyncFetch struct {
	ServerID string `wbxml:"AirSync.ServerId"`
}
