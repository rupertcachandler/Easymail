package eas

// ItemOperationsRequest is the MS-ASCMD ItemOperations command payload.
// SOGo supports the Fetch form for both attachments (FileReference) and
// untruncated email bodies (CollectionId + ServerId + BodyPreference).
//
// SPEC: MS-ASCMD §2.2.9 / MS-ASITEMOPERATIONS
type ItemOperationsRequest struct {
	XMLName struct{}               `wbxml:"ItemOperations.ItemOperations"`
	Fetch   ItemOperationsFetch    `wbxml:"ItemOperations.Fetch,omitempty"`
}

// ItemOperationsFetch is a single Fetch entry. To download an attachment,
// set FileReference (the SOGo handle `mail/FOLDER/UID/PATH`). To fetch a
// full email body, set CollectionId + ServerId and a BodyPreference.
type ItemOperationsFetch struct {
	XMLName         struct{}       `wbxml:"ItemOperations.Fetch"`
	FileReference   string         `wbxml:"AirSyncBase.FileReference,omitempty"`
	CollectionId    string         `wbxml:"AirSync.CollectionId,omitempty"`
	ServerId        string         `wbxml:"AirSync.ServerId,omitempty"`
	BodyPreference  []BodyPreference `wbxml:"AirSyncBase.BodyPreference,omitempty"`
	MIMESupport     int32          `wbxml:"AirSync.MIMESupport,omitempty"`
}

// ItemOperationsResponse is the server reply. Attachment fetches arrive in
// non-multipart form as base64 text inside Response/Fetch/Properties/Data.
type ItemOperationsResponse struct {
	XMLName  struct{}                      `wbxml:"ItemOperations.ItemOperations"`
	Status   int32                         `wbxml:"ItemOperations.Status,omitempty"`
	Response ItemOperationsResponseBlock   `wbxml:"ItemOperations.Response,omitempty"`
}

// ItemOperationsResponseBlock wraps the per-Fetch response entries.
type ItemOperationsResponseBlock struct {
	XMLName struct{}                          `wbxml:"ItemOperations.Response"`
	Fetch   []ItemOperationsFetchResult       `wbxml:"ItemOperations.Fetch,omitempty"`
}

// ItemOperationsFetchResult is one Fetch response.
type ItemOperationsFetchResult struct {
	XMLName       struct{}                      `wbxml:"ItemOperations.Fetch"`
	Status        int32                         `wbxml:"ItemOperations.Status,omitempty"`
	FileReference string                        `wbxml:"AirSyncBase.FileReference,omitempty"`
	CollectionId  string                        `wbxml:"AirSync.CollectionId,omitempty"`
	ServerId      string                        `wbxml:"AirSync.ServerId,omitempty"`
	Properties    ItemOperationsProperties      `wbxml:"ItemOperations.Properties,omitempty"`
}

// ItemOperationsProperties carries the fetched content. Data is base64
// encoded for attachment fetches.
type ItemOperationsProperties struct {
	XMLName     struct{} `wbxml:"ItemOperations.Properties"`
	ContentType string   `wbxml:"AirSyncBase.ContentType,omitempty"`
	Data        string   `wbxml:"ItemOperations.Data,omitempty"`
	Part        int32    `wbxml:"ItemOperations.Part,omitempty"`
}
