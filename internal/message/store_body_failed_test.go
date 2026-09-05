package message

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hkdb/aerion/internal/database"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/rs/zerolog"
)

// newBodyFailedTestStore opens a migrated temp DB, seeds one (account, folder)
// row, and returns a message Store ready to attach test messages to. Mirrors
// the test-scaffolding patterns in internal/database/database_test.go.
func newBodyFailedTestStore(t *testing.T) (*Store, string, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	const accountID = "acct-1"
	const folderID = "folder-1"

	if _, err := db.Exec(
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		accountID, "Test", "test@example.com", "imap.example.com", "smtp.example.com", "test@example.com",
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

	return NewStore(db), accountID, folderID
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func TestGetConversationLogsRedactMessageMetadata(t *testing.T) {
	store, accountID, folderID := newBodyFailedTestStore(t)
	const threadID = "sensitive-thread@notifications.example.com"
	const messageID = "<sensitive-message@notifications.example.com>"
	const subject = "Security alert for a private account"
	message := &Message{
		ID:        "local-message-id",
		AccountID: accountID,
		FolderID:  folderID,
		UID:       1,
		MessageID: messageID,
		ThreadID:  threadID,
		Subject:   subject,
		Date:      time.Now().UTC(),
		BodyText:  "private body",
		BodyHTML:  "<p>private body</p>",
	}
	if err := store.Create(message); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var output bytes.Buffer
	store.log = zerolog.New(&output).Level(zerolog.DebugLevel)
	if _, err := store.GetConversation(threadID, folderID); err != nil {
		t.Fatalf("GetConversation: %v", err)
	}

	logs := output.String()
	for _, sensitive := range []string{threadID, messageID, subject} {
		if strings.Contains(logs, sensitive) {
			t.Fatalf("GetConversation logs contain sensitive value %q: %s", sensitive, logs)
		}
	}
	for _, field := range []string{"thread_ref", "normalized_thread_ref", "message_ref"} {
		if !strings.Contains(logs, field) {
			t.Fatalf("GetConversation logs missing %q: %s", field, logs)
		}
	}
	if !strings.Contains(logs, logging.ShortHash(threadID)) || !strings.Contains(logs, logging.ShortHash(messageID)) {
		t.Fatalf("GetConversation logs missing stable references: %s", logs)
	}
}

// TestMarkBodyFailed_ExcludesFromQueue confirms the persistent flag actually
// drops a message from GetMessagesWithoutBody on subsequent calls — the whole
// point of v39. Without the flag, an unparseable message costs an IMAP FETCH
// per sync cycle forever.
func TestMarkBodyFailed_ExcludesFromQueue(t *testing.T) {
	s, accountID, folderID := newBodyFailedTestStore(t)
	now := time.Now().UTC()

	// Fetched-but-empty, non-encrypted → matches the self-healing branch of
	// GetMessagesWithoutBody, so it should be queued for re-fetch until we
	// flag it.
	stuck := &Message{
		ID:          "stuck",
		AccountID:   accountID,
		FolderID:    folderID,
		UID:         1,
		Date:        now,
		BodyFetched: true,
	}
	if err := s.Create(stuck); err != nil {
		t.Fatalf("Create stuck message: %v", err)
	}

	// Sanity: queued before flagging.
	ids, err := s.GetMessagesWithoutBody(folderID, 100, time.Time{})
	if err != nil {
		t.Fatalf("GetMessagesWithoutBody (pre-mark): %v", err)
	}
	if !containsID(ids, "stuck") {
		t.Fatalf("pre-mark: expected stuck message in queue, got %v", ids)
	}

	// Flag it.
	if err := s.MarkBodyFailed([]string{"stuck"}); err != nil {
		t.Fatalf("MarkBodyFailed: %v", err)
	}

	// After flagging, the message must no longer be queued — that's the only
	// behavior this fix has to guarantee.
	ids, err = s.GetMessagesWithoutBody(folderID, 100, time.Time{})
	if err != nil {
		t.Fatalf("GetMessagesWithoutBody (post-mark): %v", err)
	}
	if containsID(ids, "stuck") {
		t.Errorf("post-mark: expected stuck message out of queue, got %v", ids)
	}

	// Idempotent: re-flagging an already-flagged row must not error.
	if err := s.MarkBodyFailed([]string{"stuck"}); err != nil {
		t.Fatalf("MarkBodyFailed (re-flag): %v", err)
	}

	// Empty input must be a no-op.
	if err := s.MarkBodyFailed(nil); err != nil {
		t.Errorf("MarkBodyFailed(nil): %v", err)
	}
	if err := s.MarkBodyFailed([]string{}); err != nil {
		t.Errorf("MarkBodyFailed([]): %v", err)
	}
}

func TestGetIDsByMessageIDs_IncludesPendingMove(t *testing.T) {
	s, accountID, sourceFolderID := newBodyFailedTestStore(t)
	const destinationFolderID = "folder-2"

	if _, err := s.db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type)
		 VALUES (?, ?, ?, ?, ?)`,
		destinationFolderID, accountID, "Archive", "Archive", "archive",
	); err != nil {
		t.Fatalf("seed destination folder: %v", err)
	}

	msg := &Message{
		ID:        "message-1",
		AccountID: accountID,
		FolderID:  sourceFolderID,
		MessageID: "<message-1@example.com>",
		UID:       1,
		Date:      time.Now().UTC(),
	}
	if err := s.Create(msg); err != nil {
		t.Fatalf("Create message: %v", err)
	}
	if err := s.MoveMessages([]string{msg.ID}, destinationFolderID); err != nil {
		t.Fatalf("MoveMessages: %v", err)
	}

	ids, err := s.GetIDsByMessageIDs(accountID, destinationFolderID, []string{msg.MessageID})
	if err != nil {
		t.Fatalf("GetIDsByMessageIDs: %v", err)
	}
	if !containsID(ids, msg.ID) {
		t.Fatalf("expected pending moved message %q, got %v", msg.ID, ids)
	}
}
