package message

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hkdb/aerion/internal/database"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/rs/zerolog"
)

type ftsTestFixture struct {
	indexer *FTSIndexer
	db      *database.DB
}

func newFTSTestFixture(t *testing.T) ftsTestFixture {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "fts.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username) VALUES ('acct', 'Test', 'test@example.com', 'imap.example.com', 'smtp.example.com', 'test@example.com')`); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return ftsTestFixture{indexer: NewFTSIndexer(db.DB), db: db}
}

func (f ftsTestFixture) createFolder(t *testing.T, id string) {
	t.Helper()
	if _, err := f.db.Exec(`INSERT INTO folders (id, account_id, name, path) VALUES (?, 'acct', ?, ?)`, id, id, id); err != nil {
		t.Fatalf("create folder: %v", err)
	}
}

func (f ftsTestFixture) createMessage(t *testing.T, id, folderID, subject string) {
	t.Helper()
	f.createMessageWithUID(t, id, folderID, subject, 1)
}

func (f ftsTestFixture) createMessageWithUID(t *testing.T, id, folderID, subject string, uid int) {
	t.Helper()
	if _, err := f.db.Exec(`INSERT INTO messages (id, account_id, folder_id, uid, subject, snippet, body_text, body_html) VALUES (?, 'acct', ?, ?, ?, ?, ?, ?)`, id, folderID, uid, subject, subject+" snippet", subject+" body", ""); err != nil {
		t.Fatalf("create message: %v", err)
	}
}

func (f ftsTestFixture) ftsRowCount(t *testing.T) int {
	t.Helper()
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM messages_fts`).Scan(&count); err != nil {
		t.Fatalf("count FTS rows: %v", err)
	}
	return count
}

func (f ftsTestFixture) status(t *testing.T, folderID string) *FTSIndexStatus {
	t.Helper()
	status, err := f.indexer.GetIndexStatus(folderID)
	if err != nil {
		t.Fatalf("GetIndexStatus: %v", err)
	}
	return status
}

func TestIndexFolderAlreadyCompleteIsIdempotent(t *testing.T) {
	fixture := newFTSTestFixture(t)
	fixture.createFolder(t, "folder")
	fixture.createMessage(t, "message", "folder", "Quarterly report")

	if err := fixture.indexer.IndexFolder(context.Background(), "folder"); err != nil {
		t.Fatalf("first IndexFolder: %v", err)
	}
	first := fixture.status(t, "folder")
	if first == nil || !first.IsComplete || first.IndexedCount != 1 || first.TotalCount != 1 {
		t.Fatalf("first status = %#v, want one complete message", first)
	}
	if got := fixture.ftsRowCount(t); got != 1 {
		t.Fatalf("first FTS row count = %d, want 1", got)
	}

	if err := fixture.indexer.IndexFolder(context.Background(), "folder"); err != nil {
		t.Fatalf("second IndexFolder: %v", err)
	}
	second := fixture.status(t, "folder")
	if second == nil || !second.IsComplete || second.IndexedCount != 1 || second.TotalCount != 1 {
		t.Fatalf("second status = %#v, want unchanged complete status", second)
	}
	if got := fixture.ftsRowCount(t); got != 1 {
		t.Fatalf("second FTS row count = %d, want no duplicate reindex", got)
	}
}

func TestIndexFolderRepairsIncompleteStatus(t *testing.T) {
	fixture := newFTSTestFixture(t)
	fixture.createFolder(t, "folder")
	fixture.createMessage(t, "message", "folder", "Quarterly report")
	if _, err := fixture.db.Exec(`DELETE FROM messages_fts`); err != nil {
		t.Fatalf("clear FTS: %v", err)
	}
	if err := fixture.indexer.updateIndexStatus(context.Background(), "folder", 0, 0, true); err != nil {
		t.Fatalf("seed incomplete status: %v", err)
	}

	if err := fixture.indexer.IndexFolder(context.Background(), "folder"); err != nil {
		t.Fatalf("IndexFolder: %v", err)
	}
	status := fixture.status(t, "folder")
	if status == nil || !status.IsComplete || status.IndexedCount != 1 || status.TotalCount != 1 {
		t.Fatalf("status after repair = %#v, want one complete message", status)
	}
	if got := fixture.ftsRowCount(t); got != 1 {
		t.Fatalf("FTS row count after repair = %d, want 1", got)
	}
}

func TestFTSTriggerIndexesNewMessageWithoutBackgroundRepair(t *testing.T) {
	fixture := newFTSTestFixture(t)
	fixture.createFolder(t, "folder")
	fixture.createMessage(t, "first", "folder", "First quarterly report")
	if err := fixture.indexer.IndexFolder(context.Background(), "folder"); err != nil {
		t.Fatalf("IndexFolder: %v", err)
	}

	fixture.createMessageWithUID(t, "second", "folder", "Second quarterly report", 2)
	if got := fixture.ftsRowCount(t); got != 2 {
		t.Fatalf("FTS row count after insert = %d, want trigger to add second row", got)
	}
	status := fixture.status(t, "folder")
	if status == nil || !status.IsComplete || status.TotalCount != 1 {
		t.Fatalf("status = %#v, want previous snapshot left untouched by trigger", status)
	}
}

func TestIndexAllFoldersSummaryLogging(t *testing.T) {
	fixture := newFTSTestFixture(t)
	const folderCount = 28
	for index := 0; index < folderCount; index++ {
		folderID := fmt.Sprintf("folder-%02d", index)
		fixture.createFolder(t, folderID)
		if err := fixture.indexer.updateIndexStatus(context.Background(), folderID, 0, 0, true); err != nil {
			t.Fatalf("seed complete status: %v", err)
		}
	}

	previous := logging.Logger
	var output bytes.Buffer
	logging.Logger = zerolog.New(&output).Level(zerolog.DebugLevel)
	t.Cleanup(func() { logging.Logger = previous })

	if err := fixture.indexer.IndexAllFolders(context.Background()); err != nil {
		t.Fatalf("IndexAllFolders: %v", err)
	}
	logs := output.String()
	if strings.Contains(logs, "Folder already fully indexed") {
		t.Fatalf("per-folder complete logs remain: %s", logs)
	}
	for _, field := range []string{`"folders_total":28`, `"already_indexed":28`, `"indexed_folders":0`, `"failed_folders":0`, `"skipped_folders":0`} {
		if !strings.Contains(logs, field) {
			t.Fatalf("summary logs missing %q: %s", field, logs)
		}
	}
}
