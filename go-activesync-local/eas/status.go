package eas

// Status numeric constants from MS-ASCMD §2.2.4 (global) and command-scoped
// sections. Codes referenced by client behavior are named explicitly; the
// remainder are surfaced as opaque integers.
const (
	StatusSuccess                   = 1
	StatusInvalidPolicy             = 142
	StatusInvalidPolicyKey          = 143
	StatusInvalidDeviceID           = 144
	StatusDeviceInformationRequired = 165
)

// Sync command Status values per MS-ASCMD §2.2.1.21.4.
const (
	SyncStatusSuccess         int32 = 1
	SyncStatusInvalidSyncKey  int32 = 3
	SyncStatusProtocolError   int32 = 4
	SyncStatusServerError     int32 = 5
	SyncStatusConversionError int32 = 6
	SyncStatusConflict        int32 = 7
	SyncStatusObjectNotFound  int32 = 8
)

// IsKnownSyncStatus reports whether code is a defined Sync status value.
func IsKnownSyncStatus(code int32) bool {
	switch code {
	case SyncStatusSuccess, SyncStatusInvalidSyncKey, SyncStatusProtocolError,
		SyncStatusServerError, SyncStatusConversionError, SyncStatusConflict,
		SyncStatusObjectNotFound:
		return true
	}
	return false
}

// ShouldReprovision reports whether the given global Status code requires a
// fresh Provision exchange before the original command can be retried
// (MS-ASCMD §2.2.4).
func ShouldReprovision(code int32) bool {
	return code == int32(StatusInvalidPolicy) || code == int32(StatusInvalidPolicyKey)
}
