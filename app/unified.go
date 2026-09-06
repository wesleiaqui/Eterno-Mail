package app

import (
	"fmt"
	"sort"

	"github.com/hkdb/aerion/internal/folder"
	"github.com/hkdb/aerion/internal/message"
)

// unifiedSpecialFolders resolves the real folders represented by a synthetic
// unified navigation type. GetSpecialFolder is deliberately the single source
// of truth: it honors an account's configured path before folder_type.
func (a *App) unifiedSpecialFolders(folderType string) ([]*folder.Folder, error) {
	typeValue := folder.Type(folderType)
	switch typeValue {
	case folder.TypeInbox, folder.TypeDrafts, folder.TypeSent, folder.TypeTrash, folder.TypeStarred, folder.TypeArchive, folder.TypeSpam, folder.TypeAll:
	default:
		return nil, fmt.Errorf("unsupported unified folder type: %s", folderType)
	}
	accounts, err := a.accountStore.List()
	if err != nil {
		return nil, err
	}
	resolved := make([]*folder.Folder, 0, len(accounts))
	for _, account := range accounts {
		if !account.Enabled {
			continue
		}
		item, err := a.GetSpecialFolder(account.ID, typeValue)
		if err != nil {
			return nil, err
		}
		if item != nil {
			resolved = append(resolved, item)
		}
	}
	return resolved, nil
}

func (a *App) unifiedSpecialFolderIDs(folderType string) ([]string, error) {
	folders, err := a.unifiedSpecialFolders(folderType)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(folders))
	for _, item := range folders {
		ids = append(ids, item.ID)
	}
	return ids, nil
}

func (a *App) GetUnifiedFolderConversations(folderType string, offset, limit int, sortOrder, filterValue string) ([]*message.Conversation, error) {
	if folderType == string(folder.TypeArchive) {
		return a.listUnifiedArchived(offset, limit, sortOrder, filterValue)
	}
	ids, err := a.unifiedSpecialFolderIDs(folderType)
	if err != nil {
		return nil, err
	}
	return a.messageStore.ListConversationsByFolderIDs(ids, offset, limit, sortOrder, filterValue)
}

func (a *App) listUnifiedArchived(offset, limit int, sortOrder, filterValue string) ([]*message.Conversation, error) {
	accounts, err := a.accountStore.List()
	if err != nil {
		return nil, err
	}
	var physicalIDs, allMailIDs []string
	for _, account := range accounts {
		if !account.Enabled {
			continue
		}
		physical, err := a.GetSpecialFolder(account.ID, folder.TypeArchive)
		if err != nil {
			return nil, err
		}
		if physical != nil {
			physicalIDs = append(physicalIDs, physical.ID)
			continue
		}
		allMail, err := a.GetSpecialFolder(account.ID, folder.TypeAll)
		if err != nil {
			return nil, err
		}
		if allMail != nil {
			allMailIDs = append(allMailIDs, allMail.ID)
		}
	}
	// Fetch enough rows to merge physical and virtual sources globally before
	// applying pagination. Archive sources are normally small header lists.
	physical, err := a.messageStore.ListConversationsByFolderIDs(physicalIDs, 0, offset+limit, sortOrder, filterValue)
	if err != nil {
		return nil, err
	}
	virtual, err := a.messageStore.ListVirtualArchivedConversations(allMailIDs, 0, offset+limit, sortOrder, filterValue)
	if err != nil {
		return nil, err
	}
	combined := append(physical, virtual...)
	sort.SliceStable(combined, func(i, j int) bool {
		if sortOrder == "oldest" {
			return combined[i].LatestDate.Before(combined[j].LatestDate)
		}
		return combined[i].LatestDate.After(combined[j].LatestDate)
	})
	if offset >= len(combined) {
		return []*message.Conversation{}, nil
	}
	end := offset + limit
	if end > len(combined) {
		end = len(combined)
	}
	return combined[offset:end], nil
}

func (a *App) GetUnifiedFolderCount(folderType, filterValue string) (int, error) {
	if folderType == string(folder.TypeArchive) {
		// The list path is authoritative for mixed physical/virtual archive
		// sources; count it without exposing synthetic folder IDs.
		items, err := a.listUnifiedArchived(0, 100000, "newest", filterValue)
		return len(items), err
	}
	ids, err := a.unifiedSpecialFolderIDs(folderType)
	if err != nil {
		return 0, err
	}
	return a.messageStore.CountConversationsByFolderIDs(ids, filterValue)
}

func (a *App) SearchUnifiedFolder(folderType, query string, offset, limit int, filterValue string) ([]*message.ConversationSearchResult, error) {
	ids, err := a.unifiedSpecialFolderIDs(folderType)
	if err != nil {
		return nil, err
	}
	results, _, err := a.messageStore.SearchConversationsByFolderIDs(ids, query, offset, limit, filterValue)
	return results, err
}

func (a *App) GetSearchCountUnifiedFolder(folderType, query, filterValue string) (int, error) {
	ids, err := a.unifiedSpecialFolderIDs(folderType)
	if err != nil {
		return 0, err
	}
	_, count, err := a.messageStore.SearchConversationsByFolderIDs(ids, query, 0, 0, filterValue)
	return count, err
}

// SyncUnifiedFolder syncs each represented real folder. It never passes a
// synthetic navigation ID to the sync engine.
func (a *App) SyncUnifiedFolder(folderType string) error {
	folders, err := a.unifiedSpecialFolders(folderType)
	if err != nil {
		return err
	}
	for _, item := range folders {
		if err := a.SyncFolder(item.AccountID, item.ID); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) EmptyUnifiedTrash() error {
	folders, err := a.unifiedSpecialFolders(string(folder.TypeTrash))
	if err != nil {
		return err
	}
	for _, item := range folders {
		if err := a.EmptyTrash(item.AccountID, item.ID); err != nil {
			return err
		}
	}
	return nil
}
