package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hkdb/aerion/internal/folder"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/hkdb/aerion/internal/message"
	"github.com/hkdb/aerion/internal/sync"
)

// ============================================================================
// Search API - Exposed to frontend via Wails bindings
// ============================================================================

// SearchConversations searches for conversations in a folder using full-text search
// Returns matching conversations with highlighted text
func (a *App) SearchConversations(accountID, folderID, query string, offset, limit int, filter string) ([]*message.ConversationSearchResult, error) {
	results, _, err := a.messageStore.SearchConversations(folderID, query, offset, limit, filter)
	return results, err
}

// GetSearchCount returns the total count of search results in a folder
func (a *App) GetSearchCount(accountID, folderID, query, filter string) (int, error) {
	_, count, err := a.messageStore.SearchConversations(folderID, query, 0, 0, filter)
	return count, err
}

// SearchUnifiedInbox searches across all inbox folders for all accounts
func (a *App) SearchUnifiedInbox(query string, offset, limit int, filter string) ([]*message.ConversationSearchResult, error) {
	results, _, err := a.messageStore.SearchConversationsUnifiedInbox(query, offset, limit, filter)
	return results, err
}

// GetSearchCountUnifiedInbox returns the total count of search results across all inboxes
func (a *App) GetSearchCountUnifiedInbox(query, filter string) (int, error) {
	_, count, err := a.messageStore.SearchConversationsUnifiedInbox(query, 0, 0, filter)
	return count, err
}

// GetFTSIndexStatus returns the indexing status for a specific folder
func (a *App) GetFTSIndexStatus(folderID string) (*message.FTSIndexStatus, error) {
	return a.ftsIndexer.GetIndexStatus(folderID)
}

// GetFTSIndexStatusAll returns the indexing status for all folders
func (a *App) GetFTSIndexStatusAll() (map[string]*message.FTSIndexStatus, error) {
	return a.ftsIndexer.GetAllIndexStatuses()
}

// IsFTSIndexComplete checks if a folder is fully indexed
func (a *App) IsFTSIndexComplete(folderID string) bool {
	return a.ftsIndexer.IsIndexComplete(folderID)
}

// IsFTSIndexing returns true if any folder is currently being indexed
func (a *App) IsFTSIndexing() bool {
	return a.ftsIndexer.IsAnyIndexing()
}

// RebuildFTSIndex forces a rebuild of the FTS index for a folder
func (a *App) RebuildFTSIndex(folderID string) error {
	return a.ftsIndexer.RebuildIndex(a.ctx, folderID)
}

// IMAPSearchFolder performs a server-side IMAP SEARCH query on a specific folder.
// Returns results with local message data where available, envelope data for non-local messages.
// When limit > 0, only the newest `limit` results are returned but totalCount reflects all matches.
func (a *App) IMAPSearchFolder(accountID, folderID, query string, limit int) (*sync.IMAPSearchResponse, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()
	return a.syncEngine.IMAPSearch(ctx, accountID, folderID, query, limit)
}

// IMAPSearchUnifiedInbox searches the Inbox of every enabled account on its
// server. This is the fallback for unified search when a message is outside
// the locally synced/indexed date range.
func (a *App) IMAPSearchUnifiedInbox(query string, limit int) (*sync.IMAPSearchResponse, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()
	log := logging.WithComponent("app.search")

	accounts, err := a.accountStore.List()
	if err != nil {
		return nil, fmt.Errorf("list accounts for unified IMAP search: %w", err)
	}

	response := &sync.IMAPSearchResponse{}
	for _, account := range accounts {
		if !account.Enabled {
			continue
		}

		inbox, err := a.folderStore.GetByType(account.ID, folder.TypeInbox)
		if err != nil {
			return nil, fmt.Errorf("get inbox for account %s: %w", account.ID, err)
		}
		if inbox == nil {
			continue
		}

		result, err := a.syncEngine.IMAPSearch(ctx, account.ID, inbox.ID, query, limit)
		if err != nil {
			// A single unavailable account should not hide results from the other
			// inboxes. The normal per-folder search still reports these errors.
			log.Warn().Err(err).Str("accountID", account.ID).Msg("Unified IMAP search skipped account")
			continue
		}
		response.Results = append(response.Results, result.Results...)
		response.TotalCount += result.TotalCount
	}

	sort.Slice(response.Results, func(i, j int) bool {
		return response.Results[i].Date.After(response.Results[j].Date)
	})
	if limit > 0 && len(response.Results) > limit {
		response.Results = response.Results[:limit]
	}

	return response, nil
}

// FetchServerMessage fetches a full message by UID from the IMAP server, saves it locally,
// and returns it. Used when interacting with non-local server search results.
func (a *App) FetchServerMessage(accountID, folderID string, uid int) (*message.Message, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	return a.syncEngine.FetchServerMessage(ctx, accountID, folderID, uint32(uid))
}
