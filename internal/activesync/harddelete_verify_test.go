package activesync

import (
	"context"
	"log"
	"os"
	"strings"
	"testing"

	"easymail/internal/models"
)

// TestHardDeleteVerification is a manual harness (skipped unless creds staged):
// it loads the real account from /tmp/boi-account.txt (email|password|url|device),
// hard-deletes TWO polltests from the Trash folder with DeletesAsMoves=0, then
// re-syncs Trash to confirm they are actually gone — the thing 0.5.6 failed to do.
func TestHardDeleteVerification(t *testing.T) {
	raw, err := os.ReadFile("/tmp/boi-account.txt")
	if err != nil {
		t.Skipf("no account creds staged: %v", err)
	}
	parts := strings.Split(strings.TrimSpace(string(raw)), "|")
	if len(parts) < 4 {
		t.Skip("staged creds malformed")
	}
	acc := &models.Account{
		Email:      parts[0],
		Password:   parts[1],
		ServerURL:  parts[2],
		DeviceID:   parts[3],
		DeviceType: "EasyMail",
	}

	c, err := NewClient(acc)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	trash := "mail%2Fd85e002d301c7c61fc080000b7901a80"

	// Hard-delete the remaining BOI test mails with DeletesAsMoves=0.
	err = c.HardDeleteEmails(context.Background(), []struct{ ServerID, FolderID string }{
		{ServerID: "40308", FolderID: trash},
		{ServerID: "40309", FolderID: trash},
		{ServerID: "40310", FolderID: trash},
	})
	if err != nil {
		t.Fatalf("hard delete: %v", err)
	}

	// Re-sync the Trash folder and confirm none of them come back.
	emails, _, err := c.SyncEmailsWithDeletes(context.Background(), trash, 0)
	if err != nil {
		t.Fatalf("resync: %v", err)
	}
	found := map[string]bool{}
	for _, e := range emails {
		found[e.ServerID] = true
	}
	log.Printf("after hard-delete, Trash has %d items; 40308=%v, 40309=%v, 40310=%v", len(emails), found["40308"], found["40309"], found["40310"])
	if found["40308"] || found["40309"] || found["40310"] {
		t.Fatalf("hard-deleted ids STILL PRESENT in Trash — DeletesAsMoves=0 not honoured")
	}
	log.Printf("VERIFIED: BOI test mails permanently deleted from Trash")
}
