package activesync

import (
	"context"
	"log"
	"os"
	"strings"
	"testing"

	"easymail/internal/models"
)

// TestListTrashContents is a manual ground-truth harness (skipped unless creds
// staged): connects with the real account, syncs the Trash folder and lists
// every item's server id + subject, so we can see what the SERVER actually
// holds (vs local ghost rows) after the hard-deletes.
func TestListTrashContents(t *testing.T) {
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
	emails, deleted, err := c.SyncEmailsWithDeletes(context.Background(), trash, 0)
	if err != nil {
		t.Fatalf("sync trash: %v", err)
	}
	log.Printf("=== SERVER Trash ground truth: %d items, %d reported-deleted ===", len(emails), len(deleted))
	for _, e := range emails {
		subj := e.Subject
		if len(subj) > 60 {
			subj = subj[:60]
		}
		log.Printf("  id=%s date=%s subj=%q", e.ServerID, e.DateReceived.Format("2006-01-02 15:04"), subj)
	}
	for _, d := range deleted {
		log.Printf("  (server-reported deleted) id=%s", d)
	}
	log.Printf("=== END ===")
}
