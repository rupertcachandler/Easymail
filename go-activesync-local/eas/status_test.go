package eas

import "testing"

// SPEC: MS-ASCMD/global.status.codes
func TestStatus_KnownCodes(t *testing.T) {
	if StatusSuccess != 1 {
		t.Errorf("StatusSuccess = %d", StatusSuccess)
	}
	if StatusInvalidPolicy != 142 {
		t.Errorf("StatusInvalidPolicy = %d", StatusInvalidPolicy)
	}
	if StatusInvalidPolicyKey != 143 {
		t.Errorf("StatusInvalidPolicyKey = %d", StatusInvalidPolicyKey)
	}
	if StatusInvalidDeviceID != 144 {
		t.Errorf("StatusInvalidDeviceID = %d", StatusInvalidDeviceID)
	}
}

// SPEC: MS-ASCMD/retry.142
func TestStatus_Reprovision(t *testing.T) {
	if !ShouldReprovision(int32(StatusInvalidPolicy)) {
		t.Errorf("ShouldReprovision(142) = false, want true")
	}
	if !ShouldReprovision(int32(StatusInvalidPolicyKey)) {
		t.Errorf("ShouldReprovision(143) = false, want true")
	}
	if ShouldReprovision(1) {
		t.Errorf("ShouldReprovision(1) = true, want false")
	}
}

// SPEC: MS-ASCMD/sync.status
func TestStatus_SyncEnum(t *testing.T) {
	want := map[int32]bool{
		1: true, 3: true, 4: true, 5: true, 6: true, 7: true, 8: true,
	}
	for code := range want {
		if !IsKnownSyncStatus(code) {
			t.Errorf("IsKnownSyncStatus(%d) = false", code)
		}
	}
	if IsKnownSyncStatus(99) {
		t.Errorf("IsKnownSyncStatus(99) = true, want false")
	}
}
