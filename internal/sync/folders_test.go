package sync

import (
	"context"
	"testing"

	imapPkg "github.com/hkdb/aerion/internal/imap"
)

func TestFetchFolderStatusParallelSkipsNonSelectableMailbox(t *testing.T) {
	engine := &Engine{}
	container := &imapPkg.Mailbox{
		Name:       "[Gmail]",
		Attributes: []string{"\\HasChildren", "\\Noselect"},
	}

	results := engine.fetchFolderStatusParallel(context.Background(), "account-id", []*imapPkg.Mailbox{container})
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].mailbox != container || results[0].status != nil || results[0].err != nil {
		t.Fatalf("non-selectable mailbox result = %+v, want unchanged mailbox without STATUS result", results[0])
	}
}
