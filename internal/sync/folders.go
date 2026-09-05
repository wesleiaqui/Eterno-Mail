package sync

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	gosync "sync"
	"time"

	"github.com/hkdb/aerion/internal/folder"
	imapPkg "github.com/hkdb/aerion/internal/imap"
)

// folderStatusResult holds the result of a parallel STATUS fetch
type folderStatusResult struct {
	mailbox *imapPkg.Mailbox
	status  *imapPkg.Mailbox // STATUS returns same type as LIST but with counts populated
	err     error
}

// folderStatusWorkers must not exceed the per-account IMAP pool maximum
// (currently 3). Each worker holds one connection and reuses it for multiple
// STATUS commands, avoiding an Acquire/Release cycle per mailbox.
const folderStatusWorkers = 3

type folderStatusMetrics struct {
	requested    int
	completed    int
	failed       int
	workers      int
	acquireTotal time.Duration
}

type folderStatusJob struct {
	index   int
	mailbox *imapPkg.Mailbox
}

var errFolderStatusUnprocessed = errors.New("mailbox status was not processed")

// SyncFolders synchronizes the folder list for an account
func (e *Engine) SyncFolders(ctx context.Context, accountID string) error {
	startedAt := time.Now()
	acquireStarted := time.Now()
	e.log.Debug().Str("account", accountID).Msg("Syncing folders")

	// Get a connection from the pool for LIST
	conn, err := e.pool.GetConnection(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	listAcquireDuration := time.Since(acquireStarted)
	defer func() { e.pool.Release(conn) }()

	// List mailboxes from server
	listStarted := time.Now()
	mailboxes, err := conn.Client().ListMailboxes()
	if err != nil {
		return fmt.Errorf("failed to list mailboxes: %w", err)
	}
	listDuration := time.Since(listStarted)

	// Fetch subscribed mailbox set (for caching subscription state locally)
	subscribedStarted := time.Now()
	subscribedSet, subErr := conn.Client().ListSubscribedMailboxes()
	subscribedDuration := time.Since(subscribedStarted)
	if subErr != nil {
		e.log.Debug().Err(subErr).Str("account", accountID).Msg("Failed to list subscribed mailboxes, subscription state will not be cached")
	}
	// LIST and STATUS do not need the same connection. Return it before starting
	// parallel STATUS work so all regular pool slots remain available.
	e.pool.Release(conn)
	conn = nil

	totalFolders := len(mailboxes)
	if totalFolders == 0 {
		e.log.Info().Str("account", accountID).Msg("No folders found")
		return nil
	}

	// Emit initial "folders" phase progress
	e.emitProgress(accountID, "", 0, totalFolders, "folders")

	// Get existing local folders
	localFolders, err := e.folderStore.List(accountID)
	if err != nil {
		return fmt.Errorf("failed to list local folders: %w", err)
	}

	// Build a map of local folders by path
	localByPath := make(map[string]*folder.Folder)
	for _, f := range localFolders {
		localByPath[f.Path] = f
	}

	// Build path→type override map from account folder mappings
	// This ensures user-configured folder mappings take precedence over IMAP auto-detection
	pathTypeOverrides := make(map[string]folder.Type)
	if acc, err := e.accountStore.Get(accountID); err == nil && acc != nil {
		mappings := map[string]folder.Type{
			acc.SentFolderPath:    folder.TypeSent,
			acc.DraftsFolderPath:  folder.TypeDrafts,
			acc.TrashFolderPath:   folder.TypeTrash,
			acc.SpamFolderPath:    folder.TypeSpam,
			acc.ArchiveFolderPath: folder.TypeArchive,
			acc.AllMailFolderPath: folder.TypeAll,
			acc.StarredFolderPath: folder.TypeStarred,
		}
		for path, fType := range mappings {
			if path != "" {
				pathTypeOverrides[path] = fType
			}
		}
	}

	// Fetch STATUS for all folders in parallel
	statusStarted := time.Now()
	results, statusMetrics := e.fetchFolderStatusWorkers(ctx, accountID, mailboxes)
	statusDuration := time.Since(statusStarted)

	// Sort results by path depth so parents are processed before children.
	// IMAP LIST does not guarantee ordering, so a child may appear before its parent.
	sort.SliceStable(results, func(i, j int) bool {
		iDelim := results[i].mailbox.Delimiter
		jDelim := results[j].mailbox.Delimiter
		iDepth := 1
		jDepth := 1
		if iDelim != "" {
			iDepth = strings.Count(results[i].mailbox.Name, iDelim) + 1
		}
		if jDelim != "" {
			jDepth = strings.Count(results[j].mailbox.Name, jDelim) + 1
		}
		return iDepth < jDepth
	})

	// Track which paths we've seen and process results
	processStarted := time.Now()
	seenPaths := make(map[string]bool)
	processed := 0

	for _, result := range results {
		mb := result.mailbox
		seenPaths[mb.Name] = true
		processed++

		// Emit progress
		e.emitProgress(accountID, "", processed, totalFolders, "folders")

		// Convert IMAP folder type to our folder type
		folderType := convertFolderType(mb.Type)

		// Override with account folder mapping if configured
		if override, ok := pathTypeOverrides[mb.Name]; ok {
			folderType = override
		}

		// Skip folders where STATUS failed entirely
		if result.err != nil && result.status == nil {
			e.log.Debug().Err(result.err).Str("mailbox", mb.Name).Msg("Failed to get mailbox status, skipping folder")
			continue
		}

		status := result.status

		// Update subscription state from server
		isSubscribed := subscribedSet != nil && subscribedSet[mb.Name]

		// Check if folder exists locally
		if existing, ok := localByPath[mb.Name]; ok {
			// Update existing folder
			existing.Name = extractFolderName(mb.Name, mb.Delimiter)
			existing.Type = folderType
			// Only update subscription state when the server returned data.
			// Some servers (Microsoft 365) reject LIST-EXTENDED (SUBSCRIBED);
			// when subscribedSet is nil, preserve local state set by the user
			// via SubscribeFolder/UnsubscribeFolder rather than wiping it.
			if subscribedSet != nil {
				existing.Subscribed = isSubscribed
			}

			// Recompute parent (self-healing for broken parent links)
			if mb.Delimiter != "" {
				parts := strings.Split(mb.Name, mb.Delimiter)
				if len(parts) > 1 {
					parentPath := strings.Join(parts[:len(parts)-1], mb.Delimiter)
					if parent, ok := localByPath[parentPath]; ok {
						existing.ParentID = parent.ID
					}
				}
			}

			if status != nil {
				// A MODSEQ belongs to one UIDValidity generation. Folder discovery
				// may observe a UIDValidity change before SyncMessages selects this
				// folder, so clear the independent flag watermark here as well.
				// Otherwise the next CHANGEDSINCE could use a baseline from the old
				// UID universe.
				if existing.UIDValidity != 0 && status.UIDValidity != 0 && existing.UIDValidity != status.UIDValidity {
					e.log.Info().
						Str("folder", mb.Name).
						Uint32("previous_uidvalidity", existing.UIDValidity).
						Uint32("current_uidvalidity", status.UIDValidity).
						Uint64("flags_sync_modseq_before", existing.FlagsSyncModSeq).
						Msg("Folder discovery: UIDValidity changed; resetting flag-sync watermark")
					existing.FlagsSyncModSeq = 0
				}
				existing.UIDValidity = status.UIDValidity
				existing.UIDNext = status.UIDNext
				existing.HighestModSeq = status.HighestModSeq
				existing.TotalCount = int(status.Messages)
				existing.UnreadCount = int(status.Unseen)
			}

			if err := e.folderStore.Update(existing); err != nil {
				e.log.Warn().Err(err).Str("path", mb.Name).Msg("Failed to update folder")
			}
		} else {
			// Create new folder
			f := &folder.Folder{
				AccountID:  accountID,
				Name:       extractFolderName(mb.Name, mb.Delimiter),
				Path:       mb.Name,
				Type:       folderType,
				Subscribed: isSubscribed,
			}
			if status != nil {
				f.UIDValidity = status.UIDValidity
				f.UIDNext = status.UIDNext
				f.HighestModSeq = status.HighestModSeq
				f.TotalCount = int(status.Messages)
				f.UnreadCount = int(status.Unseen)
			}

			// Handle parent folder
			if mb.Delimiter != "" {
				parts := strings.Split(mb.Name, mb.Delimiter)
				if len(parts) > 1 {
					parentPath := strings.Join(parts[:len(parts)-1], mb.Delimiter)
					if parent, ok := localByPath[parentPath]; ok {
						f.ParentID = parent.ID
					}
				}
			}

			if err := e.folderStore.Create(f); err != nil {
				e.log.Warn().Err(err).Str("path", mb.Name).Msg("Failed to create folder")
			} else {
				// Add to local map so child folders can find their parent
				localByPath[f.Path] = f
			}
		}
	}

	// Delete folders that no longer exist on server
	for path, f := range localByPath {
		if !seenPaths[path] {
			e.log.Debug().Str("path", path).Msg("Deleting removed folder")
			if err := e.folderStore.Delete(f.ID); err != nil {
				e.log.Warn().Err(err).Str("path", path).Msg("Failed to delete folder")
			}
		}
	}

	e.log.Info().
		Str("account", accountID).
		Int("folders", len(mailboxes)).
		Int("status_requested", statusMetrics.requested).
		Int("status_completed", statusMetrics.completed).
		Int("status_failed", statusMetrics.failed).
		Int("status_workers", statusMetrics.workers).
		Dur("status_acquire_total", statusMetrics.acquireTotal).
		Dur("list_acquire", listAcquireDuration).
		Dur("list", listDuration).
		Dur("subscribed_list", subscribedDuration).
		Dur("status", statusDuration).
		Dur("process", time.Since(processStarted)).
		Dur("duration", time.Since(startedAt)).
		Msg("Folder sync complete")

	// Persist auto-detected folder mappings if not already set.
	// This ensures getSpecialFolder always finds the correct folder via the
	// account mapping, even when multiple folders match the same type by name
	// (e.g., iCloud's "Sent" and "Sent Messages" both match TypeSent).
	e.persistAutoDetectedMappings(accountID)

	return nil
}

// persistAutoDetectedMappings checks if the account's folder path mappings are
// empty and fills them from auto-detected folder types. Only writes to DB when
// at least one mapping was empty and a detected folder was found.
func (e *Engine) persistAutoDetectedMappings(accountID string) {
	acc, err := e.accountStore.Get(accountID)
	if err != nil || acc == nil {
		return
	}

	type mapping struct {
		current string
		fType   folder.Type
	}

	mappings := []mapping{
		{acc.SentFolderPath, folder.TypeSent},
		{acc.DraftsFolderPath, folder.TypeDrafts},
		{acc.TrashFolderPath, folder.TypeTrash},
		{acc.SpamFolderPath, folder.TypeSpam},
		{acc.ArchiveFolderPath, folder.TypeArchive},
		{acc.AllMailFolderPath, folder.TypeAll},
		{acc.StarredFolderPath, folder.TypeStarred},
	}

	results := make([]string, len(mappings))
	updated := false

	for i, m := range mappings {
		results[i] = m.current
		if m.current != "" {
			continue
		}
		detected, _ := e.folderStore.GetByType(accountID, m.fType)
		if detected != nil {
			results[i] = detected.Path
			updated = true
		}
	}

	if !updated {
		return
	}

	if err := e.accountStore.UpdateFolderMappings(
		accountID,
		results[0], results[1], results[2], results[3], results[4], results[5], results[6],
	); err != nil {
		e.log.Warn().Err(err).Str("account", accountID).Msg("Failed to persist auto-detected folder mappings")
		return
	}

	e.log.Debug().Str("account", accountID).Msg("Auto-detected folder mappings persisted")
}

// fetchFolderStatusParallel is retained for focused callers/tests. SyncFolders
// uses fetchFolderStatusWorkers to include aggregate metrics in its final log.
func (e *Engine) fetchFolderStatusParallel(ctx context.Context, accountID string, mailboxes []*imapPkg.Mailbox) []folderStatusResult {
	results, _ := e.fetchFolderStatusWorkers(ctx, accountID, mailboxes)
	return results
}

// fetchFolderStatusWorkers sends exactly one STATUS command for every
// selectable mailbox. It uses at most folderStatusWorkers persistent pool
// connections; each worker acquires once, drains several jobs sequentially,
// then releases once. Non-selectable LIST entries retain their position in the
// result list but never receive STATUS.
func (e *Engine) fetchFolderStatusWorkers(ctx context.Context, accountID string, mailboxes []*imapPkg.Mailbox) ([]folderStatusResult, folderStatusMetrics) {
	results := make([]folderStatusResult, len(mailboxes))
	jobs := make([]folderStatusJob, 0, len(mailboxes))
	for i, mailbox := range mailboxes {
		results[i] = folderStatusResult{mailbox: mailbox}
		if mailbox == nil || !mailbox.IsSelectable() {
			continue
		}
		// This is replaced by the single worker that consumes this job. If every
		// worker fails to acquire a connection, it preserves the old behavior of
		// treating this mailbox status as failed rather than silently using zeroes.
		results[i].err = errFolderStatusUnprocessed
		jobs = append(jobs, folderStatusJob{index: i, mailbox: mailbox})
	}

	metrics := folderStatusMetrics{requested: len(jobs)}
	if len(jobs) == 0 {
		return results, metrics
	}

	metrics.workers = folderStatusWorkers
	if len(jobs) < metrics.workers {
		metrics.workers = len(jobs)
	}

	// A buffered, pre-filled channel means an acquisition failure cannot leave a
	// producer blocked with no remaining worker. Other acquired workers continue
	// draining the same queue; any unconsumed jobs retain the controlled error
	// above and are counted as failures below.
	jobCh := make(chan folderStatusJob, len(jobs))
	for _, job := range jobs {
		jobCh <- job
	}
	close(jobCh)

	var wg gosync.WaitGroup
	var mu gosync.Mutex
	var acquireErr error

	for i := 0; i < metrics.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			acquireStarted := time.Now()
			conn, err := e.pool.GetConnection(ctx, accountID)
			mu.Lock()
			metrics.acquireTotal += time.Since(acquireStarted)
			mu.Unlock()
			if err != nil {
				mu.Lock()
				acquireErr = err
				mu.Unlock()
				return
			}
			defer e.pool.Release(conn)

			for job := range jobCh {
				status, statusErr := conn.Client().GetMailboxStatus(ctx, job.mailbox.Name)
				mu.Lock()
				results[job.index] = folderStatusResult{
					mailbox: job.mailbox,
					status:  status,
					err:     statusErr,
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	for _, job := range jobs {
		result := results[job.index]
		if result.err == errFolderStatusUnprocessed {
			if acquireErr != nil {
				result.err = acquireErr
			} else if ctx.Err() != nil {
				result.err = ctx.Err()
			}
			results[job.index] = result
		}
		if result.err == nil && result.status != nil {
			metrics.completed++
		} else {
			metrics.failed++
		}
	}
	return results, metrics
}
