package eas

// MoveItemsRequest is the MS-ASCMD MoveItems command payload. It moves one
// or more items from their source folder into a destination folder on the
// server. Each entry identifies a single message by its server ID and its
// current (source) folder, plus the destination folder.
type MoveItemsRequest struct {
	XMLName struct{}        `wbxml:"Move.MoveItems"`
	Moves   []MoveItemEntry `wbxml:"Move.Move"`
}

// MoveItemEntry identifies a single item to move.
type MoveItemEntry struct {
	SrcMsgID string `wbxml:"Move.SrcMsgId"`
	SrcFldID string `wbxml:"Move.SrcFldId"`
	DstFldID string `wbxml:"Move.DstFldId"`
}

// MoveItemsResponse is the server reply to MoveItems. Status 1 (Success) is
// expected; per-item responses appear under Response when the whole request
// succeeds or a per-item status is returned.
type MoveItemsResponse struct {
	XMLName   struct{}             `wbxml:"Move.MoveItems"`
	Responses []MoveItemsResponse_ `wbxml:"Move.Response"`
}

// MoveItemsResponse_ is the per-item (or whole-request) reply.
type MoveItemsResponse_ struct {
	SrcMsgID string `wbxml:"Move.SrcMsgId"`
	Status   int32  `wbxml:"Move.Status"`
	DstMsgID string `wbxml:"Move.DstMsgId,omitempty"`
}

// FolderCreateRequest is the MS-ASCMD FolderCreate command payload. It
// creates a user-visible folder (type 12 = custom/mail subfolder) under the
// given parent (0 = the iOS mailbox root). SOGo honours ParentId + Type.
type FolderCreateRequest struct {
	XMLName     struct{} `wbxml:"FolderHierarchy.FolderCreate"`
	ParentID    string   `wbxml:"FolderHierarchy.ParentId"`
	DisplayName string   `wbxml:"FolderHierarchy.DisplayName"`
	Type        int32    `wbxml:"FolderHierarchy.Type"` // 12 = user-mail folder
}

// FolderCreateResponse is the server reply to FolderCreate.
type FolderCreateResponse struct {
	XMLName     struct{} `wbxml:"FolderHierarchy.FolderCreate"`
	Status      int32    `wbxml:"FolderHierarchy.Status"`
	ServerID    string   `wbxml:"FolderHierarchy.ServerId,omitempty"`
	DisplayName string   `wbxml:"FolderHierarchy.DisplayName,omitempty"`
}

// FolderDeleteRequest is the MS-ASCMD FolderDelete command payload.
type FolderDeleteRequest struct {
	XMLName  struct{} `wbxml:"FolderHierarchy.FolderDelete"`
	ServerID string   `wbxml:"FolderHierarchy.ServerId"`
}

// FolderDeleteResponse is the server reply to FolderDelete.
type FolderDeleteResponse struct {
	XMLName struct{} `wbxml:"FolderHierarchy.FolderDelete"`
	Status  int32    `wbxml:"FolderHierarchy.Status"`
}
