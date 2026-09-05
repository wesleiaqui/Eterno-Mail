package sync

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hkdb/aerion/internal/database"
	"github.com/hkdb/aerion/internal/folder"
)

func TestSyncMessagesRejectsFolderFromAnotherAccount(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}

	const accountA = "account-a"
	const accountB = "account-b"
	const folderB = "folder-b"
	for _, accountID := range []string{accountA, accountB} {
		if _, err := db.Exec(
			`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			accountID, accountID, accountID+"@example.com", "imap.example.com", "smtp.example.com", accountID,
		); err != nil {
			t.Fatalf("seed account %s: %v", accountID, err)
		}
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type)
		 VALUES (?, ?, ?, ?, ?)`,
		folderB, accountB, "INBOX", "INBOX", "inbox",
	); err != nil {
		t.Fatalf("seed folder: %v", err)
	}

	engine := &Engine{folderStore: folder.NewStore(db), folderLocks: folderLocks{locks: map[string]chan struct{}{}}}
	err = engine.SyncMessages(context.Background(), accountA, folderB, 0, false)
	if err == nil {
		t.Fatal("SyncMessages() error = nil, want account-folder mismatch")
	}
	if !strings.Contains(err.Error(), "folder-b belongs to account account-b, not account-a") {
		t.Fatalf("SyncMessages() error = %q, want account-folder mismatch", err)
	}

	var messageCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&messageCount); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("message count = %d, want 0", messageCount)
	}
}
