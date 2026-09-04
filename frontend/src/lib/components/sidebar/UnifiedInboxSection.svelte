<script lang="ts">
  import Icon from '@iconify/svelte'
  // @ts-ignore - wailsjs path
  import type { account, folder } from '../../../../wailsjs/go/models'
  import { isUnifiedInboxExpanded, setUnifiedInboxExpanded } from '$lib/stores/uiState.svelte'
  import FolderContextMenu from './FolderContextMenu.svelte'
  import Avatar from '$lib/components/kit/Avatar.svelte'
  import { contactPhotos } from '$lib/stores/contactPhotos.svelte'
  import { _ } from '$lib/i18n'

  interface AccountWithInbox {
    account: account.Account
    inbox: folder.Folder | null
  }

  interface Props {
    accounts: AccountWithInbox[]
    selectedAccountId: string | null
    selectedFolderId: string | null
    selectionSource: 'unified' | 'account' | null
    onSelectUnified: () => void
    onSelectAccountInbox: (accountId: string, folderId: string, folderPath: string) => void
    collapsed?: boolean
  }

  let {
    accounts,
    selectedAccountId,
    selectedFolderId,
    selectionSource,
    onSelectUnified,
    onSelectAccountInbox,
    collapsed = false,
  }: Props = $props()

  // Initialize from persisted state (defaults to true)
  let expanded = $state(isUnifiedInboxExpanded())

  // Check if unified inbox is selected (All Inboxes)
  const isUnifiedSelected = $derived(selectedAccountId === 'unified' && selectedFolderId === 'inbox')

  // The sidebar has its own account rows, so make their locally-synced contact
  // photos available just like the message list does.
  $effect(() => {
    const emails = accounts.map(item => item.account.email).filter(Boolean)
    if (emails.length) void contactPhotos.ensure(emails)
  })

  // Check if a specific account inbox is selected IN THE UNIFIED SECTION
  // Only highlight if selectionSource is 'unified'
  function isAccountInboxSelected(accountId: string, inboxId: string): boolean {
    return selectionSource === 'unified' && selectedAccountId === accountId && selectedFolderId === inboxId
  }

  // Toggle expand/collapse and persist
  function toggleExpanded() {
    expanded = !expanded
    setUnifiedInboxExpanded(expanded)
  }

  function handleUnifiedClick() {
    onSelectUnified()
  }

  function handleAccountInboxClick(acc: AccountWithInbox) {
    if (acc.inbox) {
      onSelectAccountInbox(acc.account.id, acc.inbox.id, acc.inbox.path)
    }
  }

</script>

<div class="sidebar-unified-section">
  {#if collapsed}
    <!-- A separate rail button avoids squeezing the expanded inbox controls
         (chevron, label and count) into a single icon cell. -->
    <button
      type="button"
      class="sidebar-inbox-rail-button {isUnifiedSelected ? 'sidebar-inbox-rail-button-selected' : ''}"
      data-sidebar-nav-item="unified"
      onclick={handleUnifiedClick}
    >
      <Icon icon="mdi:inbox-multiple" class="w-4 h-4" />
    </button>
  {:else}
    <!-- Unified Inbox Header -->
    <div
      class="sidebar-inbox-entry w-full flex items-center gap-2 rounded-xl transition-colors cursor-pointer {isUnifiedSelected
        ? 'bg-primary/10 text-primary'
        : 'hover:bg-muted/50'}"
      data-sidebar-item="unified"
    >
      <!-- Expand/Collapse Toggle -->
      <button
        class="sidebar-inbox-toggle p-0.5 -ml-0.5 rounded transition-colors"
        onclick={(e) => { e.stopPropagation(); toggleExpanded() }}
      >
        <Icon
          icon={expanded ? 'mdi:chevron-down' : 'mdi:chevron-right'}
          class="w-4 h-4 text-muted-foreground"
        />
      </button>

      <!-- Clickable area for selecting unified inbox -->
      <button
        class="flex-1 flex items-center gap-2"
        onclick={handleUnifiedClick}
      >
        <Icon icon="mdi:inbox-multiple" class="w-4 h-4 flex-shrink-0" />
        <span data-sidebar-label class="flex-1 text-left text-sm font-medium truncate">{$_('sidebar.inbox')}</span>
      </button>
    </div>

    <!-- Individual Account Inboxes -->
    {#if expanded}
      <div class="sidebar-inbox-accounts">
        {#each accounts as acc (acc.account.id)}
          {#if acc.inbox}
            {@const avatarPhoto = contactPhotos.get(acc.account.email)}
            <FolderContextMenu folderId={acc.inbox.id}>
              <button
                class="sidebar-inbox-account w-full flex items-center gap-2 rounded-lg text-sm transition-colors {isAccountInboxSelected(acc.account.id, acc.inbox.id)
                  ? 'bg-primary/10 text-primary'
                  : 'hover:bg-muted/50 text-muted-foreground hover:text-foreground'}"
                data-sidebar-item="unified-account"
                data-folder-id={acc.inbox.id}
                onclick={() => handleAccountInboxClick(acc)}
              >
                <Avatar
                  email={acc.account.email}
                  name={acc.account.name}
                  size={16}
                  photoData={avatarPhoto?.data}
                  photoMediaType={avatarPhoto?.mediaType}
                />
                <span data-sidebar-label class="flex-1 text-left truncate">{acc.account.email || acc.account.name}</span>
                {#if acc.inbox.unreadCount > 0}
                  <span data-sidebar-badge class="px-1.5 py-0.5 text-xs font-medium bg-muted text-muted-foreground rounded-full">
                    {acc.inbox.unreadCount}
                  </span>
                {/if}
              </button>
            </FolderContextMenu>
          {/if}
        {/each}
      </div>
    {/if}
  {/if}
</div>
