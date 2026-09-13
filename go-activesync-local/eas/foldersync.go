package eas

// FolderSyncRequest is the MS-ASCMD FolderSync command payload sent by the
// client. The initial SyncKey is "0".
type FolderSyncRequest struct {
	XMLName struct{} `wbxml:"FolderHierarchy.FolderSync"`
	SyncKey string   `wbxml:"FolderHierarchy.SyncKey"`
}

// NewFolderSyncRequest builds a FolderSync request with the given SyncKey.
// Use "0" for the initial sync.
func NewFolderSyncRequest(syncKey string) FolderSyncRequest {
	return FolderSyncRequest{SyncKey: syncKey}
}

// FolderSyncResponse is the server reply to FolderSync.
type FolderSyncResponse struct {
	XMLName struct{}      `wbxml:"FolderHierarchy.FolderSync"`
	Status  int32         `wbxml:"FolderHierarchy.Status"`
	SyncKey string        `wbxml:"FolderHierarchy.SyncKey,omitempty"`
	Changes FolderChanges `wbxml:"FolderHierarchy.Changes"`
}

// FolderChanges wraps Add/Update/Delete entries returned by FolderSync.
type FolderChanges struct {
	Count  int32          `wbxml:"FolderHierarchy.Count,omitempty"`
	Add    []FolderAdd    `wbxml:"FolderHierarchy.Add"`
	Update []FolderUpdate `wbxml:"FolderHierarchy.Update"`
	Delete []FolderDelete `wbxml:"FolderHierarchy.Delete"`
}

// FolderAdd describes a newly-created folder.
type FolderAdd struct {
	ServerID    string `wbxml:"FolderHierarchy.ServerId"`
	ParentID    string `wbxml:"FolderHierarchy.ParentId,omitempty"`
	DisplayName string `wbxml:"FolderHierarchy.DisplayName"`
	Type        int32  `wbxml:"FolderHierarchy.Type"`
}

// FolderUpdate describes an updated folder.
type FolderUpdate struct {
	ServerID    string `wbxml:"FolderHierarchy.ServerId"`
	ParentID    string `wbxml:"FolderHierarchy.ParentId,omitempty"`
	DisplayName string `wbxml:"FolderHierarchy.DisplayName"`
	Type        int32  `wbxml:"FolderHierarchy.Type"`
}

// FolderDelete describes a deleted folder.
type FolderDelete struct {
	ServerID string `wbxml:"FolderHierarchy.ServerId"`
}
