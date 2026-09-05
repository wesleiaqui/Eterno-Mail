<script lang="ts">
  import Icon from '@iconify/svelte'
  import { onMount } from 'svelte'
  import AccountSection from './AccountSection.svelte'
  import UnifiedInboxSection from './UnifiedInboxSection.svelte'
  import AccountDialog from '$lib/components/settings/AccountDialog.svelte'
  import DeleteAccountDialog from '$lib/components/settings/DeleteAccountDialog.svelte'
  import SettingsDialog from '$lib/components/settings/SettingsDialog.svelte'
  import { Button } from '$lib/components/ui/button'
  import Avatar from '$lib/components/kit/Avatar.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { contactSourcesStore } from '$lib/stores/contactSources.svelte'
  import { contactPhotos } from '$lib/stores/contactPhotos.svelte'
  import { isAccountExpanded, setAccountExpanded, isUnifiedInboxExpanded, setFolderCollapsed, getUIState, getUIStateVersion, saveUIState, setActiveExtension } from '$lib/stores/uiState.svelte'
  import { setFocusedPane } from '$lib/stores/keyboard.svelte'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { account, folder } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { formatDistanceToNow } from 'date-fns'
  import { getCurrentDateFnsLocale } from '$lib/stores/settings.svelte'
  import { getSidebarWidth } from '$lib/stores/uiState.svelte'

  // Folder item type for flat navigation list
  interface FolderNavItem {
    type: 'unified' | 'unified-account' | 'account-header' | 'folder'
    accountId?: string
    folderId?: string
    folderPath?: string
    folderName: string
    folderType?: string
  }

  // Track focused account header for keyboard navigation
  let focusedAccountId = $state<string | null>(null)

  // Ref to scrollable container for auto-scroll
  let scrollContainer: HTMLDivElement | null = null

  // Track expanded state for each account (reactive, synced with persisted state)
  let expandedAccounts = $state<Record<string, boolean>>({})

  // Initialize expanded state from persisted storage
  // Depends on both accounts list AND UI state version (so it re-runs when persisted state loads)
  $effect(() => {
    // Read version to create dependency - effect re-runs when UI state finishes loading
    const _version = getUIStateVersion()

    const newExpanded: Record<string, boolean> = {}
    for (const acc of accountStore.accounts) {
      newExpanded[acc.account.id] = isAccountExpanded(acc.account.id)
    }
    expandedAccounts = newExpanded
  })

  // Toggle account expansion
  function toggleAccountExpanded(accountId: string) {
    const newValue = !expandedAccounts[accountId]
    expandedAccounts[accountId] = newValue
    setAccountExpanded(accountId, newValue)
  }

  // Track collapsed state for folders with children (reactive, synced with persisted state)
  let collapsedFolders = $state<Record<string, boolean>>({})

  // Initialize collapsed state from persisted storage and prune stale entries
  $effect(() => {
    const _version = getUIStateVersion()

    // Collect all folder IDs that exist in current accounts
    const allFolderIds = new Set<string>()
    const collectIds = (trees: folder.FolderTree[]) => {
      for (const tree of trees) {
        if (tree.folder) allFolderIds.add(tree.folder.id)
        if (tree.children) collectIds(tree.children)
      }
    }
    for (const acc of accountStore.accounts) {
      collectIds(acc.folders || [])
    }

    // Read persisted collapsed state, keep only entries for existing folders
    const persisted = getUIState().collapsedFolders
    const newCollapsed: Record<string, boolean> = {}
    let hasStale = false
    for (const folderId of Object.keys(persisted)) {
      if (allFolderIds.has(folderId)) {
        newCollapsed[folderId] = persisted[folderId]
      } else {
        hasStale = true
      }
    }
    collapsedFolders = newCollapsed

    // Persist cleaned state if stale entries were pruned
    if (hasStale) {
      saveUIState({ collapsedFolders: newCollapsed })
    }
  })

  // Toggle folder collapse
  function toggleFolderCollapsed(folderId: string) {
    const isCurrentlyCollapsed = collapsedFolders[folderId] !== false
    const newValue = !isCurrentlyCollapsed
    collapsedFolders = { ...collapsedFolders, [folderId]: newValue }
    setFolderCollapsed(folderId, newValue)
  }

  interface Props {
    onFolderSelect?: (accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) => void
    onUnifiedFolderSelect?: (accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) => void
    onUnifiedInboxSelect?: () => void
    onCompose?: () => void
    onMessagesMoved?: () => void
    selectedAccountId?: string | null
    selectedFolderId?: string | null
    selectionSource?: 'unified' | 'account' | null
    isFocused?: boolean
    isFlashing?: boolean
    showBackButton?: boolean
    onBack?: () => void
    collapsed?: boolean
    onToggleCollapsed?: () => void
  }

  let {
    onFolderSelect,
    onUnifiedFolderSelect,
    onUnifiedInboxSelect,
    onCompose,
    onMessagesMoved,
    selectedAccountId = null,
    selectedFolderId = null,
    selectionSource = null,
    isFocused: _isFocused = false,
    isFlashing = false,
    showBackButton = false,
    onBack,
    collapsed = false,
    onToggleCollapsed,
  }: Props = $props()

  // Dialog state
  let showAccountDialog = $state(false)
  let showDeleteDialog = $state(false)
  let showSettingsDialog = $state(false)
  let showAllFolders = $state(false)
  let expandedFolderGroups = $state<Record<string, boolean>>({})
  // Presets use the saved expanded width as their durable value. This keeps
  // manual resizing and the Settings selector in one source of truth.
  const sidebarDensity = $derived(getSidebarWidth() >= 350 ? 'sidebar-density-large' : getSidebarWidth() >= 310 ? 'sidebar-density-medium' : 'sidebar-density-compact')
  const settingsAccount = $derived(accountStore.accounts[0]?.account ?? null)
  const settingsPhoto = $derived(contactPhotos.get(settingsAccount?.email ?? ''))
  let editingAccount = $state<account.Account | null>(null)
  let deletingAccount = $state<account.Account | null>(null)

  $effect(() => {
    const emails = accountStore.accounts.map(item => item.account.email).filter(Boolean)
    if (emails.length) void contactPhotos.ensure(emails)
  })

  // Load accounts and contact sources on mount
  onMount(() => {
    // Load accounts, then trigger comprehensive sync on launch
    accountStore.load().then(async () => {
      try {
        await accountStore.syncAllComplete()
      } catch (err) {
        console.error('Failed to sync on launch:', err)
      }
    })

    contactSourcesStore.load()
  })

  // Get accounts with their inbox folders for unified inbox section
  function getAccountsWithInbox() {
    return accountStore.accounts.map(acc => {
      // Find the inbox folder in the folder tree
      const findInbox = (folders: folder.FolderTree[]): folder.Folder | null => {
        for (const f of folders) {
          if (f.folder?.type === 'inbox') {
            return f.folder
          }
          if (f.children) {
            const found = findInbox(f.children)
            if (found) return found
          }
        }
        return null
      }
      return {
        account: acc.account,
        inbox: findInbox(acc.folders || [])
      }
    })
  }

  // Handle unified inbox selection (All Inboxes)
  function handleUnifiedInboxSelect() {
    onUnifiedInboxSelect?.()
  }

  // Handle individual account inbox selection from unified section
  function handleAccountInboxSelect(accountId: string, folderId: string, folderPath: string) {
    onUnifiedFolderSelect?.(accountId, folderId, folderPath, 'Inbox', 'inbox')
  }

  // Sync status: { accountName, label, percentage } — accountName + percentage are populated only when a sync is active
  let syncStatus = $derived.by<{ accountName: string | null; label: string; percentage: number | null }>(() => {
    if (accountStore.isAnySyncing) {
      const syncingAcc = accountStore.accounts.find((a) => a.syncing)
      if (syncingAcc) {
        const accountName = syncingAcc.account.name
        const progress = accountStore.getSyncProgress(syncingAcc.account.id)
        if (progress) {
          if (progress.phase === 'folders') return { accountName, label: $_('sidebar.syncingFolders'), percentage: null }
          if (progress.phase === 'messages') return { accountName, label: $_('sidebar.fetchingMessageList'), percentage: null }
          if (progress.phase === 'headers') return { accountName, label: $_('sidebar.fetchingHeaders', { values: { percentage: progress.percentage } }), percentage: progress.percentage }
          return { accountName, label: $_('sidebar.syncingContent', { values: { percentage: progress.percentage } }), percentage: progress.percentage }
        }
        return { accountName, label: $_('sidebar.syncing'), percentage: null }
      }
      return { accountName: null, label: $_('sidebar.syncing'), percentage: null }
    }
    if (!accountStore.isOnline) return { accountName: null, label: $_('sidebar.offline'), percentage: null }
    if (!accountStore.lastSyncTime) return { accountName: null, label: $_('sidebar.notSynced'), percentage: null }
    return { accountName: null, label: $_('sidebar.synced', { values: { time: formatDistanceToNow(accountStore.lastSyncTime, { addSuffix: true, locale: getCurrentDateFnsLocale() }) } }), percentage: null }
  })

  // Handle folder selection
  function handleFolderSelect(accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) {
    accountStore.selectFolder(accountId, folderId, folderPath, folderName)
    onFolderSelect?.(accountId, folderId, folderPath, folderName, folderType)
  }

  function findFolderByType(trees: folder.FolderTree[], type: string): folder.Folder | null {
    for (const tree of trees) {
      if (tree.folder?.type === type) return tree.folder
      const child = findFolderByType(tree.children || [], type)
      if (child) return child
    }
    return null
  }

  function findFolderByPath(trees: folder.FolderTree[], path: string): folder.Folder | null {
    for (const tree of trees) {
      if (tree.folder?.path === path) return tree.folder
      const child = findFolderByPath(tree.children || [], path)
      if (child) return child
    }
    return null
  }

  function findAccountFolder(item: { account: account.Account; folders: folder.FolderTree[] }, type: string): folder.Folder | null {
    const typedFolder = findFolderByType(item.folders, type)
    if (typedFolder) return typedFolder

    // Some IMAP providers do not label special folders with a type. The
    // account's configured special-folder paths are the reliable fallback.
    const configuredPath = (() => {
      switch (type) {
        case 'archive': return item.account.archiveFolderPath
        case 'spam': return item.account.spamFolderPath
        case 'all': return item.account.allMailFolderPath
        case 'starred': return item.account.starredFolderPath
        case 'sent': return item.account.sentFolderPath
        case 'drafts': return item.account.draftsFolderPath
        case 'trash': return item.account.trashFolderPath
        default: return undefined
      }
    })()

    return configuredPath ? findFolderByPath(item.folders, configuredPath) : null
  }

  function selectPrimaryFolder(type: string) {
    const preferred = accountStore.accounts.find(item => item.account.id === selectedAccountId && selectedAccountId !== 'unified')
    const orderedAccounts = preferred ? [preferred, ...accountStore.accounts.filter(item => item !== preferred)] : accountStore.accounts
    for (const item of orderedAccounts) {
      const target = findAccountFolder(item, type)
      if (!target) continue
      handleFolderSelect(item.account.id, target.id, target.path, target.name, target.type)
      return
    }
    // A folder may not exist on every provider. Opening the full folder panel
    // still gives the user a real, actionable place to choose from.
    showAllFolders = true
  }

  function isPrimaryFolderSelected(type: string): boolean {
    for (const item of accountStore.accounts) {
      const target = findAccountFolder(item, type)
      if (target?.id === selectedFolderId) return true
    }
    return false
  }

  function openAllFolders(): void {
    // The folders panel intentionally does not render over the compact rail.
    // Expand first so "More" has the same useful result in both layouts.
    if (collapsed) {
      onToggleCollapsed?.()
      showAllFolders = true
      return
    }
    showAllFolders = !showAllFolders
  }

  function toggleFolderGroup(event: MouseEvent, type: string): void {
    event.stopPropagation()
    const willExpand = !expandedFolderGroups[type]
    // The account rows are a detail panel, not another permanent navigation
    // tree. Keep one group open at a time so their avatar column cannot stack
    // underneath several unrelated folders.
    expandedFolderGroups = willExpand ? { [type]: true } : {}

    // A per-account list needs the room of the complete navigation. Opening
    // it from the rail therefore expands the sidebar as part of the action.
    if (collapsed) onToggleCollapsed?.()
  }

  function getFolderGroupAccounts(type: string) {
    // Keep the group useful even if one provider has not exposed/configured
    // that special folder yet. Resolution happens when its account is chosen.
    return accountStore.accounts.map(item => ({ account: item.account, type }))
  }

  function selectFolderGroupAccount(accountId: string, type: string): void {
    const item = accountStore.accounts.find(candidate => candidate.account.id === accountId)
    const target = item ? findAccountFolder(item, type) : null
    if (item && target) {
      handleFolderSelect(item.account.id, target.id, target.path, target.name, target.type)
      return
    }

    // A provider may not support this special folder. Give the user a useful
    // next action instead of a dead account row.
    showAllFolders = true
  }

  const primaryFolders = [
    { type: 'starred', label: 'sidebar.starred', icon: 'mdi:pin-outline' },
    { type: 'drafts', label: 'sidebar.drafts', icon: 'mdi:file-outline' },
    { type: 'sent', label: 'sidebar.sent', icon: 'mdi:send-outline' },
    { type: 'trash', label: 'sidebar.trash', icon: 'mdi:delete-outline' },
  ]

  const secondaryFolders = [
    { type: 'archive', label: 'sidebar.archived', icon: 'mdi:check' },
    { type: 'spam', label: 'sidebar.blocked', icon: 'mdi:thumb-down-outline' },
    { type: 'all', label: 'sidebar.snoozed', icon: 'mdi:clock-outline' },
  ]

  // Open add account dialog
  function openAddAccount() {
    editingAccount = null
    showAccountDialog = true
  }

  // Open edit account dialog
  function openEditAccount(acc: account.Account) {
    editingAccount = acc
    showAccountDialog = true
  }

  // Open delete confirmation
  function openDeleteAccount(acc: account.Account) {
    deletingAccount = acc
    showDeleteDialog = true
  }

  // Sync all accounts (comprehensive sync)
  export async function syncAllAccounts() {
    try {
      await accountStore.syncAllComplete()
    } catch (err) {
      console.error('Sync failed:', err)
      // Error is already stored in account store
    }
  }

  // Cancel all running syncs
  export async function cancelSync() {
    try {
      await accountStore.cancelAllSyncs()
    } catch (err) {
      console.error('Failed to cancel sync:', err)
    }
  }

  // Toggle sync (start if not running, cancel if running) - for keyboard shortcut
  export async function toggleSync() {
    if (accountStore.isAnySyncing) {
      await cancelSync()
    } else {
      await syncAllAccounts()
    }
  }

  // Build flat list of all navigable folders including Unified Inbox
  // The list matches the exact visual order in the sidebar, respecting expanded/collapsed state
  function buildFolderNavList(): FolderNavItem[] {
    const items: FolderNavItem[] = []

    // Add Unified Inbox section items if more than 1 account
    if (accountStore.accounts.length > 1) {
      // 1. Add "All Inboxes"
      items.push({
        type: 'unified',
        folderName: 'Unified Inbox',
        folderType: 'unified',
      })

      // 2. Add each account's inbox (under unified section) - only if unified section is expanded
      if (isUnifiedInboxExpanded()) {
        for (const accWithFolders of accountStore.accounts) {
          // Skip if account is not fully loaded yet (can happen during reauth)
          if (!accWithFolders.account) continue

          const findInbox = (trees: folder.FolderTree[]): folder.Folder | null => {
            for (const tree of trees) {
              if (tree.folder?.type === 'inbox') return tree.folder
              if (tree.children) {
                const found = findInbox(tree.children)
                if (found) return found
              }
            }
            return null
          }
          const inbox = findInbox(accWithFolders.folders || [])
          if (inbox) {
            items.push({
              type: 'unified-account',
              accountId: accWithFolders.account.id,
              folderId: inbox.id,
              folderPath: inbox.path,
              folderName: inbox.name,
              folderType: 'inbox',
            })
          }
        }
      }
    }

    // 3. Add account headers and their folders
    for (const accWithFolders of accountStore.accounts) {
      // Skip if account is not fully loaded yet (can happen during reauth)
      if (!accWithFolders.account) continue

      // Always add the account header (so user can navigate to it and expand)
      items.push({
        type: 'account-header',
        accountId: accWithFolders.account.id,
        folderName: accWithFolders.account.name,
      })

      // Only add folders if the account is expanded
      if (expandedAccounts[accWithFolders.account.id]) {
        const flattenFolders = (trees: folder.FolderTree[]) => {
          for (const tree of trees) {
            if (tree.folder) {
              items.push({
                type: 'folder',
                accountId: accWithFolders.account.id,
                folderId: tree.folder.id,
                folderPath: tree.folder.path,
                folderName: tree.folder.name,
                folderType: tree.folder.type,
              })
            }
            // Skip children of collapsed folders
            if (tree.children && tree.children.length > 0 && tree.folder && collapsedFolders[tree.folder.id] === false) {
              flattenFolders(tree.children)
            }
          }
        }
        flattenFolders(accWithFolders.folders || [])
      }
    }

    return items
  }

  // Get current folder index in navigation list
  function getCurrentFolderIndex(): number {
    const navList = buildFolderNavList()

    // Check if an account header is focused
    if (focusedAccountId) {
      return navList.findIndex(item =>
        item.type === 'account-header' && item.accountId === focusedAccountId
      )
    }

    // Check if Unified Inbox is selected (All Inboxes)
    if (selectedAccountId === 'unified') {
      return navList.findIndex(item => item.type === 'unified')
    }

    // Check selectionSource to find the correct item
    if (selectionSource === 'unified') {
      // Looking for unified-account item
      return navList.findIndex(item =>
        item.type === 'unified-account' && item.folderId === selectedFolderId
      )
    } else {
      // Looking for regular folder item
      return navList.findIndex(item =>
        item.type === 'folder' && item.folderId === selectedFolderId
      )
    }
  }

  // Navigate to previous folder (exposed for keyboard navigation)
  export function selectPreviousFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return

    const currentIndex = getCurrentFolderIndex()
    const newIndex = currentIndex <= 0 ? 0 : currentIndex - 1

    selectFolderByIndex(navList, newIndex)
  }

  // Navigate to next folder (exposed for keyboard navigation)
  export function selectNextFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return

    const currentIndex = getCurrentFolderIndex()
    const newIndex = currentIndex >= navList.length - 1 ? navList.length - 1 : currentIndex + 1

    selectFolderByIndex(navList, newIndex)
  }

  // Jump to the top sidebar item — All Inboxes when present (Alt+G)
  export function selectFirstFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return
    selectFolderByIndex(navList, 0)
  }

  // Jump to the last visible item: last folder of the last expanded account,
  // or the last account header when all trees are collapsed (Alt+Shift+G)
  export function selectLastFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return
    selectFolderByIndex(navList, navList.length - 1)
  }

  // Scroll an item into view
  function scrollItemIntoView(item: FolderNavItem) {
    if (!scrollContainer) return

    // Build selector based on item type
    let selector: string | null = null
    if (item.type === 'unified') {
      selector = '[data-sidebar-nav-item="unified"], [data-sidebar-item="unified"]'
    } else if (item.type === 'unified-account' && item.folderId) {
      selector = `[data-sidebar-item="unified-account"][data-folder-id="${item.folderId}"]`
    } else if (item.type === 'account-header' && item.accountId) {
      selector = `[data-sidebar-item="account-header"][data-account-id="${item.accountId}"]`
    } else if (item.type === 'folder' && item.folderId) {
      selector = `[data-sidebar-item="folder"][data-folder-id="${item.folderId}"]`
    }

    if (selector) {
      const element = scrollContainer.querySelector(selector)
      element?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    }
  }

  // Select folder by index in nav list
  function selectFolderByIndex(navList: FolderNavItem[], index: number) {
    const item = navList[index]
    if (!item) return

    // Clear account header focus when selecting a folder
    if (item.type !== 'account-header') {
      focusedAccountId = null
    }

    if (item.type === 'unified') {
      onUnifiedInboxSelect?.()
    } else if (item.type === 'unified-account' && item.accountId && item.folderId && item.folderPath) {
      // Select from unified section - uses onUnifiedFolderSelect
      onUnifiedFolderSelect?.(item.accountId, item.folderId, item.folderPath, item.folderName, item.folderType || 'inbox')
    } else if (item.type === 'account-header' && item.accountId) {
      // Focus on account header (Enter/Space will toggle expand)
      focusedAccountId = item.accountId
    } else if (item.type === 'folder' && item.accountId && item.folderId && item.folderPath) {
      // Select from account tree - uses handleFolderSelect
      handleFolderSelect(item.accountId, item.folderId, item.folderPath, item.folderName, item.folderType || 'folder')
    }

    // Scroll the selected item into view
    scrollItemIntoView(item)
  }

  // Toggle expand/collapse for the focused account (called on Enter/Space/Alt+Enter)
  export function toggleFocusedAccount() {
    if (focusedAccountId) {
      toggleAccountExpanded(focusedAccountId)
    }
  }

  // Check if an account header is focused
  export function hasFocusedAccount(): boolean {
    return focusedAccountId !== null
  }

  // Check if the currently selected folder has children
  export function hasSelectedFolderWithChildren(): boolean {
    if (!selectedFolderId || selectionSource !== 'account') return false
    return folderHasChildren(selectedFolderId)
  }

  // Toggle collapse for the currently selected folder
  export function toggleSelectedFolderCollapse(): void {
    if (!selectedFolderId || selectionSource !== 'account') return
    if (!folderHasChildren(selectedFolderId)) return
    toggleFolderCollapsed(selectedFolderId)
  }

  // Check if a folder has children by searching the account folder trees
  function folderHasChildren(folderId: string): boolean {
    for (const acc of accountStore.accounts) {
      const found = findTreeNode(acc.folders || [], folderId)
      if (found) return (found.children && found.children.length > 0) || false
    }
    return false
  }

  // Find a FolderTree node by folder ID
  function findTreeNode(trees: folder.FolderTree[], folderId: string): folder.FolderTree | null {
    for (const tree of trees) {
      if (tree.folder?.id === folderId) return tree
      if (tree.children) {
        const found = findTreeNode(tree.children, folderId)
        if (found) return found
      }
    }
    return null
  }
</script>

<div class="spark-sidebar spark-sidebar-rebuilt {sidebarDensity} {collapsed ? 'spark-sidebar-collapsed' : ''} relative flex flex-col h-full {isFlashing ? 'pane-focus-flash' : ''}">
  <div class="sidebar-rebuilt-toolbar">
    <button type="button" class="sidebar-compose-button" onclick={onCompose} title={$_('sidebar.compose')} aria-label={$_('sidebar.compose')}>
      <Icon icon="mdi:pencil-plus-outline" class="h-5 w-5" />
      <span data-sidebar-label>{$_('sidebar.compose')}</span>
    </button>
    {#if showBackButton}
      <button type="button" class="sidebar-toolbar-button" title={$_('responsive.back')} aria-label={$_('aria.closeSidebar')} onclick={onBack}>
        <Icon icon="mdi:close" class="h-5 w-5" />
      </button>
    {/if}
  </div>

  <div class="sidebar-content sidebar-rebuilt-content flex-1 overflow-y-auto scrollbar-thin" bind:this={scrollContainer}>
    {#if accountStore.loading}
      <div class="flex items-center justify-center py-8">
        <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    {:else if accountStore.accounts.length === 0}
      <!-- Empty State -->
      <div class="flex flex-col items-center justify-center py-8 px-4 text-center">
        <Icon icon="mdi:email-plus-outline" class="w-12 h-12 text-muted-foreground mb-3" />
        <h3 class="text-sm font-medium mb-1">{$_('sidebar.noAccountsYet')}</h3>
        <p class="text-xs text-muted-foreground mb-4">
          {$_('sidebar.addFirstAccount')}
        </p>
        <Button size="sm" onclick={openAddAccount}>
          <Icon icon="mdi:plus" class="w-4 h-4 mr-1" />
          {$_('sidebar.addAccount')}
        </Button>
      </div>
    {:else}
      <nav class="sidebar-primary-navigation" aria-label={$_('sidebar.navigation')}>
        <button type="button" class="sidebar-primary-link sidebar-home-link" onclick={handleUnifiedInboxSelect}>
          <Icon icon="mdi:home-outline" class="h-6 w-6" />
          <span data-sidebar-label>{$_('sidebar.home')}</span>
        </button>

        <UnifiedInboxSection
          accounts={getAccountsWithInbox()}
          {selectedAccountId}
          {selectedFolderId}
          {selectionSource}
          {collapsed}
          onSelectUnified={handleUnifiedInboxSelect}
          onSelectAccountInbox={handleAccountInboxSelect}
        />

        <button type="button" class="sidebar-primary-link" onclick={() => setActiveExtension('calendar')}>
          <Icon icon="mdi:calendar-blank-outline" class="h-6 w-6" />
          <span data-sidebar-label>{$_('sidebar.calendar')}</span>
        </button>

        <div class="sidebar-primary-separator"></div>
        {#each primaryFolders as item (item.type)}
          <div class="sidebar-folder-group">
            <div class="sidebar-folder-group-row">
              <button type="button" class="sidebar-folder-toggle" class:sidebar-folder-toggle-expanded={expandedFolderGroups[item.type]} onclick={(event) => toggleFolderGroup(event, item.type)} aria-label={$_(item.label)} aria-expanded={expandedFolderGroups[item.type] ?? false}>
                <Icon icon={expandedFolderGroups[item.type] ? 'mdi:chevron-down' : 'mdi:chevron-right'} class="h-4 w-4" />
              </button>
              <button type="button" class="sidebar-primary-link sidebar-folder-group-link" class:sidebar-primary-link-selected={isPrimaryFolderSelected(item.type)} onclick={() => selectPrimaryFolder(item.type)}>
                <Icon icon={item.icon} class="h-6 w-6" />
                <span data-sidebar-label>{$_(item.label)}</span>
              </button>
            </div>
            {#if expandedFolderGroups[item.type]}
              <div class="sidebar-folder-account-list">
                {#each getFolderGroupAccounts(item.type) as entry (entry.account.id)}
                  {@const avatarPhoto = contactPhotos.get(entry.account.email)}
                  <button type="button" class="sidebar-folder-account" onclick={() => selectFolderGroupAccount(entry.account.id, item.type)}>
                    <Avatar email={entry.account.email} name={entry.account.name} size={16} photoData={avatarPhoto?.data} photoMediaType={avatarPhoto?.mediaType} />
                    <span class="truncate">{entry.account.email || entry.account.name}</span>
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        {/each}

        {#if !collapsed}
          <p class="sidebar-primary-heading" data-sidebar-label>{$_('sidebar.folders')}</p>
          {#each secondaryFolders as item (item.type)}
            <div class="sidebar-folder-group sidebar-secondary-folder-group">
              <div class="sidebar-folder-group-row">
                <button type="button" class="sidebar-folder-toggle" class:sidebar-folder-toggle-expanded={expandedFolderGroups[item.type]} onclick={(event) => toggleFolderGroup(event, item.type)} aria-label={$_(item.label)} aria-expanded={expandedFolderGroups[item.type] ?? false}>
                  <Icon icon={expandedFolderGroups[item.type] ? 'mdi:chevron-down' : 'mdi:chevron-right'} class="h-4 w-4" />
                </button>
                <button type="button" class="sidebar-primary-link sidebar-folder-group-link" class:sidebar-primary-link-selected={isPrimaryFolderSelected(item.type)} onclick={() => selectPrimaryFolder(item.type)}>
                  <Icon icon={item.icon} class="h-6 w-6" />
                  <span data-sidebar-label>{$_(item.label)}</span>
                </button>
              </div>
              {#if expandedFolderGroups[item.type]}
                <div class="sidebar-folder-account-list">
                  {#each getFolderGroupAccounts(item.type) as entry (entry.account.id)}
                    {@const avatarPhoto = contactPhotos.get(entry.account.email)}
                    <button type="button" class="sidebar-folder-account" onclick={() => selectFolderGroupAccount(entry.account.id, item.type)}>
                      <Avatar email={entry.account.email} name={entry.account.name} size={16} photoData={avatarPhoto?.data} photoMediaType={avatarPhoto?.mediaType} />
                      <span class="truncate">{entry.account.email || entry.account.name}</span>
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          {/each}
        {/if}

        <button type="button" class="sidebar-more-button" onclick={openAllFolders}>
          <Icon icon="mdi:dots-horizontal" class="h-6 w-6" />
          <span data-sidebar-label>{$_('sidebar.more')}</span>
        </button>
        <button type="button" class="sidebar-collapse-nav-button" onclick={onToggleCollapsed} title={collapsed ? $_('sidebar.expand') : $_('sidebar.collapse')} aria-label={collapsed ? $_('sidebar.expand') : $_('sidebar.collapse')}>
          <Icon icon={collapsed ? 'mdi:chevron-double-right' : 'mdi:chevron-double-left'} class="h-5 w-5" />
          <span data-sidebar-label>{collapsed ? $_('sidebar.expand') : $_('sidebar.collapse')}</span>
        </button>
      </nav>
    {/if}
  </div>

  <div class="sidebar-bottom-actions">
    <button type="button" class="sidebar-refresh-entry" onclick={toggleSync} title={accountStore.isAnySyncing ? $_('sidebar.clickToCancel') : $_('sidebar.syncAllAccounts')} aria-label={accountStore.isAnySyncing ? $_('sidebar.clickToCancel') : $_('sidebar.syncAllAccounts')}>
      <Icon icon={accountStore.isAnySyncing ? 'mdi:sync' : 'mdi:refresh'} class="h-5 w-5 {accountStore.isAnySyncing ? 'animate-spin' : ''}" />
      <span data-sidebar-label>{accountStore.isAnySyncing ? $_('sidebar.syncing') : $_('sidebar.syncAllAccounts')}</span>
    </button>

    <button class="sidebar-settings-entry" type="button" data-sidebar-settings onclick={() => (showSettingsDialog = true)} title={$_('sidebar.settings')}>
      {#if settingsAccount}
        <Avatar email={settingsAccount.email} name={settingsAccount.name} size={28} photoData={settingsPhoto?.data} photoMediaType={settingsPhoto?.mediaType} />
      {:else}
        <Icon icon="mdi:cog-outline" class="h-6 w-6" />
      {/if}
      <span data-sidebar-label>{$_('sidebar.settings')}</span>
      {#if contactSourcesStore.hasErrors}<span class="sidebar-settings-error"></span>{/if}
    </button>
  </div>

  {#if showAllFolders}
    <aside class="sidebar-all-folders-panel" aria-label={$_('sidebar.allFolders')}>
      <header class="sidebar-all-folders-header">
        <h2>{$_('sidebar.allFolders')}</h2>
        <button type="button" class="sidebar-toolbar-button" onclick={() => (showAllFolders = false)} aria-label={$_('aria.dismiss')}>
          <Icon icon="mdi:close" class="h-5 w-5" />
        </button>
      </header>
      <div class="sidebar-all-folders-scroll scrollbar-thin">
        {#each accountStore.accounts as accWithFolders (accWithFolders.account.id)}
          <AccountSection
            account={accWithFolders.account}
            folders={accWithFolders.folders}
            loading={accWithFolders.loading}
            syncing={accWithFolders.syncing}
            error={accWithFolders.error}
            selectedFolderId={accountStore.selectedFolder?.folderId ?? ''}
            {selectionSource}
            isHeaderFocused={focusedAccountId === accWithFolders.account.id}
            isExpanded={expandedAccounts[accWithFolders.account.id] ?? true}
            syncError={accountStore.getSyncError(accWithFolders.account.id)}
            {collapsedFolders}
            {onMessagesMoved}
            onFolderSelect={handleFolderSelect}
            onToggleExpanded={() => toggleAccountExpanded(accWithFolders.account.id)}
            onToggleFolderCollapse={toggleFolderCollapsed}
            onEdit={() => openEditAccount(accWithFolders.account)}
            onDelete={() => openDeleteAccount(accWithFolders.account)}
            onSync={() => accountStore.syncAccount(accWithFolders.account.id)}
          />
        {/each}
        <button type="button" class="sidebar-all-folders-add" onclick={openAddAccount}>
          <Icon icon="mdi:plus" class="h-5 w-5" /> {$_('sidebar.addAccount')}
        </button>
      </div>
    </aside>
  {/if}
</div>

<!-- Account Dialog -->
<AccountDialog
  bind:open={showAccountDialog}
  editAccount={editingAccount}
  onClose={() => {
    showAccountDialog = false
    editingAccount = null
    setFocusedPane('messageList')
  }}
/>

<!-- Delete Confirmation Dialog -->
<DeleteAccountDialog
  bind:open={showDeleteDialog}
  account={deletingAccount}
  onClose={() => {
    showDeleteDialog = false
    deletingAccount = null
    setFocusedPane('messageList')
  }}
/>

<!-- Settings Dialog -->
<SettingsDialog
  bind:open={showSettingsDialog}
  onClose={() => {
    showSettingsDialog = false
    setFocusedPane('messageList')
  }}
/>
