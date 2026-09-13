package eas

// PingRequest is the MS-ASCMD Ping command request payload.
type PingRequest struct {
	XMLName           struct{}    `wbxml:"Ping.Ping"`
	HeartbeatInterval int32       `wbxml:"Ping.HeartbeatInterval,omitempty"`
	Folders           PingFolders `wbxml:"Ping.Folders"`
}

// PingFolders wraps the Folder entries in a Ping request.
type PingFolders struct {
	Folder []PingFolder `wbxml:"Ping.Folder"`
}

// PingFolder is a single folder entry being monitored by Ping.
type PingFolder struct {
	ID    string `wbxml:"Ping.Id"`
	Class string `wbxml:"Ping.Class"`
}

// PingResponse is the MS-ASCMD Ping command response payload.
type PingResponse struct {
	XMLName struct{}            `wbxml:"Ping.Ping"`
	Status  int32               `wbxml:"Ping.Status"`
	Folders PingResponseFolders `wbxml:"Ping.Folders,omitempty"`
}

// PingResponseFolders wraps the list of folder identifiers carrying changes.
type PingResponseFolders struct {
	Folder []string `wbxml:"Ping.Folder"`
}

// PingHasChanges reports whether the given Ping Status indicates changes
// (MS-ASCMD §2.2.1.13.6).
func PingHasChanges(status int32) bool {
	return status == 2
}
