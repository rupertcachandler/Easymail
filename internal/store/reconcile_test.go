package store

import (
	"path/filepath"
	"testing"
	"time"

	"easymail/internal/models"
)

// TestReconcileEmails verifies the ghost-sweep logic that backs the poll and
// reconcile reconciliation: rows whose server_id is absent from the
// authoritative response AND fall inside [lo, hi) are removed; rows outside
// the window are left alone (the response doesn't cover them, so their
// absence proves nothing).
func TestReconcileEmails(t *testing.T) {
	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	accID := "acc-1"
	folderID := accID + "-mail%2Finbox"
	now := time.Now().UTC()

	save := func(sid string, received time.Time) {
		e := &models.Email{
			ID:           folderID + "-" + sid,
			AccountID:    accID,
			FolderID:     folderID,
			ServerID:     sid,
			DateReceived: received,
			FromEmail:    "t@example.com",
		}
		if err := s.SaveEmail(e); err != nil {
			t.Fatalf("save %s: %v", sid, err)
		}
	}
	// Three rows: 200 (recent, gone from server → ghost), 201 (~4 days old →
	// outside the poll window but inside the wide one, absent from the wide
	// authoritative set → ghost of the wide pass), 202 (recent, still on
	// server → survives).
	save("200", now.Add(-3*time.Hour))
	save("201", now.Add(-100*time.Hour))
	save("202", now.Add(-time.Hour))
	save("203", now.AddDate(0, 0, -200)) // ancient, outside any window

	// Poll pass: lo=now-24h, hi=now. Authoritative set: 202 (and a server id
	// we never cached). Only 200 is a swept ghost.
	ghosts, err := s.ReconcileEmails(folderID, []string{"202", "999"}, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("reconcile poll pass: %v", err)
	}
	if len(ghosts) != 1 || ghosts[0] != "200" {
		t.Fatalf("expected only ghost 200 swept in poll pass, got %v", ghosts)
	}
	for _, sid := range []string{"201", "202", "203"} {
		ok, err := s.EmailExists(folderID, sid)
		if err != nil {
			t.Fatalf("exists %s: %v", sid, err)
		}
		if !ok {
			t.Fatalf("row %s must survive the poll sweep", sid)
		}
	}

	// Wide pass (reconcile path): lo=now-180d, hi=now-72h. Authoritative set
	// only has 202. 201 (48h old) is absent from the set and inside the wide
	// window → ghost. 202 is in the set → survives. 203 is older than lo →
	// untouched (response doesn't cover it).
	ghosts, err = s.ReconcileEmails(folderID, []string{"202"}, now.AddDate(0, 0, -180), now.Add(-72*time.Hour))
	if err != nil {
		t.Fatalf("reconcile wide pass: %v", err)
	}
	if len(ghosts) != 1 || ghosts[0] != "201" {
		t.Fatalf("expected only ghost 201 swept in wide pass, got %v", ghosts)
	}
	for _, sid := range []string{"202", "203"} {
		ok, err := s.EmailExists(folderID, sid)
		if err != nil || !ok {
			t.Fatalf("row %s must survive wide pass, ok=%v err=%v", sid, ok, err)
		}
	}
}