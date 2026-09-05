package message

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hkdb/aerion/internal/database"
)

func TestUpsertWithValidParentsAndHeaderFields(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}

	const accountID = "account-id"
	const folderID = "folder-id"
	if _, err := db.Exec(
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		accountID, "Test Account", "test@example.com", "imap.example.com", "smtp.example.com", "test-user",
	); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type)
		 VALUES (?, ?, ?, ?, ?)`,
		folderID, accountID, "INBOX", "INBOX", "inbox",
	); err != nil {
		t.Fatalf("seed folder: %v", err)
	}

	store := NewStore(db)
	message := &Message{
		ID:            "message-id",
		AccountID:     accountID,
		FolderID:      folderID,
		UID:           62245,
		MessageID:     "<62245@example.com>",
		InReplyTo:     "<parent@example.com>",
		References:    `["<parent@example.com>"]`,
		ThreadID:      "thread-id",
		Subject:       "Header subject",
		FromName:      "Sender",
		FromEmail:     "sender@example.com",
		ToList:        `[{"email":"test@example.com"}]`,
		CcList:        `[]`,
		BccList:       `[]`,
		ReplyTo:       "reply@example.com",
		InboxCategory: "promotions",
		Date:          time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC),
		Snippet:       "Header-only message",
		Size:          1234,
		BodyFetched:   false,
		ReceivedAt:    time.Date(2026, 9, 4, 12, 1, 0, 0, time.UTC),
	}
	if err := store.Upsert(message); err != nil {
		t.Fatalf("Upsert insert: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO attachments (id, message_id, filename, content_type, size, is_inline)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		"attachment-id", message.ID, "attachment.txt", "text/plain", 1, false,
	); err != nil {
		t.Fatalf("seed attachment: %v", err)
	}

	message.ID = "message-id-updated"
	message.Subject = "Updated header subject"
	if err := store.Upsert(message); err != nil {
		t.Fatalf("Upsert conflict update: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages WHERE folder_id = ? AND uid = ?`, folderID, message.UID).Scan(&count); err != nil {
		t.Fatalf("count upserted message: %v", err)
	}
	if count != 1 {
		t.Fatalf("upserted message count = %d, want 1", count)
	}

	var storedID, attachmentMessageID string
	if err := db.QueryRow(`SELECT id FROM messages WHERE folder_id = ? AND uid = ?`, folderID, message.UID).Scan(&storedID); err != nil {
		t.Fatalf("get upserted message ID: %v", err)
	}
	if err := db.QueryRow(`SELECT message_id FROM attachments WHERE id = ?`, "attachment-id").Scan(&attachmentMessageID); err != nil {
		t.Fatalf("get attachment message ID: %v", err)
	}
	if storedID != "message-id" || attachmentMessageID != "message-id" {
		t.Fatalf("conflict update changed referenced message ID: message=%q attachment=%q", storedID, attachmentMessageID)
	}

	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign_key_check returned a violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("foreign_key_check rows: %v", err)
	}
}
