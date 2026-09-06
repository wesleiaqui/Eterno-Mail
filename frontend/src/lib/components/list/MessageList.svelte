<script lang="ts" module>
  // Session-only memory of how far the user paginated ("Load more"), where
  // they scrolled, and which thread was selected (the keyboard-nav anchor),
  // per folder view. Module scope so it survives folder switches and
  // component remounts for the life of the app session; gone on quit.
  // Keyed "accountId:folderId" ("unified:inbox" for the unified view).
  const sessionListState = new Map<
    string,
    { loadedCount: number; scrollTop: number; selectedThreadId: string | null }
  >()
</script>

<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import Icon from '@iconify/svelte'
  import ConversationRow from './ConversationRow.svelte'
  import { DropdownMenu } from 'bits-ui'
  import { cn } from '$lib/utils'
  import { Button } from '$lib/components/ui/button'
  // @ts-ignore - wailsjs bindings
  import { GetConversations, GetConversationCount, SyncFolder, ForceSyncFolder, CancelFolderSync, SetMessageListSortOrder, GetUnifiedFolderConversations, GetUnifiedFolderCount, SearchConversations, SearchUnifiedFolder, GetSearchCount, GetSearchCountUnifiedFolder, SyncUnifiedFolder, EmptyUnifiedTrash, GetFTSIndexStatus, IsFTSIndexing, Trash, DeletePermanently, EmptyTrash, Undo, IMAPSearchFolder, IMAPSearchUnifiedInbox, FetchServerMessage } from '../../../../wailsjs/go/app/App'
  import { toasts } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'
  import { ConfirmDialog } from '$lib/components/ui/confirm-dialog'
  // @ts-ignore - wailsjs path
  import { message } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs runtime
  import { EventsOn, EventsOff } from '../../../../wailsjs/runtime/runtime'
  import { getLanguage, getMessageListDensity, getMessageListSortOrder, setMessageListSortOrder, getShowMessageListProfilePics } from '$lib/stores/settings.svelte'
  import { getInboxDisplayMode, setInboxDisplayMode as setInboxDisplayPreference, initializeInboxDisplayPreferences, getInboxCardGrouping, getInboxCardVisibleCount, isInboxCardAccountVisible, type InboxDisplayMode } from '$lib/stores/inboxDisplay.svelte'
  import { contactPhotos } from '$lib/stores/contactPhotos.svelte'
  import { domainFromEmail, senderLogos } from '$lib/stores/senderLogos.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { getLayoutMode, hideViewer } from '$lib/stores/layout.svelte'
  import { isDialogGuardActive } from '$lib/stores/dialogGuard'

  interface Props {
    accountId?: string | null
    folderId?: string | null
    folderName?: string
    folderType?: string
    onConversationSelect?: (threadId: string, folderId: string, accountId: string) => void
    onReply?: (mode: 'reply' | 'reply-all' | 'forward', messageId: string) => void
    onRowActionComplete?: (autoSelectNext: boolean) => void
    onBulkMarkRead?: (messageIds: string[]) => void
    onBulkArchive?: (messageIds: string[]) => void
    isFocused?: boolean
    isFlashing?: boolean
    showFolderToggle?: boolean
    onToggleSidebar?: () => void
  }

  let {
    accountId = null,
    folderId = null,
    folderName = 'Inbox',
    folderType = 'inbox',
    onConversationSelect,
    onReply,
    onRowActionComplete,
    onBulkMarkRead,
    onBulkArchive,
    isFocused: _isFocused = false,
    isFlashing = false,
    showFolderToggle = false,
    onToggleSidebar,
  }: Props = $props()

  // State
  let conversations = $state<message.Conversation[]>([])
  let totalCount = $state(0)
  let loading = $state(false)
  let error = $state<string | null>(null)
  let selectedThreadId = $state<string | null>(null)
  let lastLoadedFolderId = $state<string | null>(null) // Track folder changes
  let loadGeneration = $state(0) // Invalidates stale async results when folder changes mid-load (#200)

  // Derived: check if this folder is currently syncing (from account store's progress tracking)
  const syncing = $derived(
    !!(accountId && folderId && accountStore.syncProgress[accountId]?.[folderId] !== undefined)
  )

  // Derived: get sync progress for this folder (if syncing)
  const syncProgress = $derived(
    accountId && folderId
      ? accountStore.syncProgress[accountId]?.[folderId]
      : null
  )

  // Multi-select state
  let checkedThreadIds = $state<Set<string>>(new Set())
  let lastClickedIndex = $state<number | null>(null)

  // Pagination
  const PAGE_SIZE = 50
  let offset = $state(0)

  // Debounce timer for reloading after flag changes
  let reloadTimer: ReturnType<typeof setTimeout> | null = null

  // Debounce timer for coalescing sync event reloads (fixes event flooding with 3+ accounts)
  let syncReloadTimer: ReturnType<typeof setTimeout> | null = null

  // Deferred reload: when a dialog (e.g. folder picker) is open, defer the reload
  // so the component tree isn't destroyed mid-interaction
  let pendingReload = false

  // Buffer for flag changes that arrive while loadConversations() is in-flight.
  // On notification click, loadConversations (folder change) and MarkAsRead race —
  // the flagsChanged event may fire before the new conversations array is ready.
  let pendingFlagChanges: Array<{messageIds: string[], isRead: boolean}> = []

  // Search state
  let showSearch = $state(false)
  let searchQuery = $state('')
  let searchResults = $state<any[]>([])  // ConversationSearchResult from backend
  let searchTotalCount = $state(0)
  let searchOffset = $state(0)
  let isSearching = $state(false)
  let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null

  // Filter state
  let filterMode = $state<string>('')  // '' | 'unread' | 'starred' | 'attachments'

  type InboxDisplayGroup = {
    id: string
    label: string
    icon: string
    category: string
    recipient?: string
    conversations: message.Conversation[]
  }

  // Inbox presentation is deliberately a view preference: it never changes
  // folders, messages or the server-side sort order.
  let inboxDisplayMode = $state<InboxDisplayMode>(getInboxDisplayMode())
  let showInboxDisplayPicker = $state(false)
  let collapsedInboxGroups = $state<Set<string>>(new Set())
  let expandedInboxGroups = $state<Set<string>>(new Set())
  let inboxCardPreferencesVersion = $state(0)

  const inboxDisplayOptions = $derived<Array<{ id: InboxDisplayMode; label: string; description: string; icon: string }>>([
    { id: 'priority', label: $_('inbox.priority'), description: $_('inbox.priorityDescription'), icon: 'mdi:lightning-bolt-outline' },
    { id: 'categories', label: $_('inbox.categories'), description: $_('inbox.categoriesDescription'), icon: 'mdi:shape-outline' },
    { id: 'chronological', label: $_('inbox.chronological'), description: $_('inbox.chronologicalDescription'), icon: 'mdi:format-list-bulleted' },
  ])

  const inboxDisplayLabel = $derived(inboxDisplayOptions.find(option => option.id === inboxDisplayMode)?.label ?? $_('inbox.categories'))

  onMount(() => {
    initializeInboxDisplayPreferences()
    inboxDisplayMode = getInboxDisplayMode()
    const handleInboxDisplayChange = (event: Event) => {
      const mode = (event as CustomEvent<InboxDisplayMode>).detail
      if (mode !== 'priority' && mode !== 'categories' && mode !== 'chronological') return
      inboxDisplayMode = mode
      collapsedInboxGroups = new Set()
      expandedInboxGroups = new Set()
    }
    const handleInboxCardPreferencesChange = () => {
      // Recreate the category view after an option is changed in Settings.
      // This prevents stale grouped rows when the two panels are open together.
      inboxCardPreferencesVersion += 1
      collapsedInboxGroups = new Set()
      expandedInboxGroups = new Set()
    }
    window.addEventListener('eterno-mail:inbox-display-change', handleInboxDisplayChange)
    window.addEventListener('eterno-mail:inbox-card-preferences-change', handleInboxCardPreferencesChange)
    return () => {
      window.removeEventListener('eterno-mail:inbox-display-change', handleInboxDisplayChange)
      window.removeEventListener('eterno-mail:inbox-card-preferences-change', handleInboxCardPreferencesChange)
    }
  })

  function setInboxDisplayMode(mode: InboxDisplayMode) {
    inboxDisplayMode = mode
    setInboxDisplayPreference(mode)
    collapsedInboxGroups = new Set()
    expandedInboxGroups = new Set()
    showInboxDisplayPicker = false
  }

  function toggleInboxGroup(groupID: string) {
    const next = new Set(collapsedInboxGroups)
    if (next.has(groupID)) next.delete(groupID)
    else next.add(groupID)
    collapsedInboxGroups = next
  }

  function markInboxGroupDone(group: InboxDisplayGroup, event: MouseEvent) {
    event.stopPropagation()
    const messageIds = group.conversations.flatMap((conversation: any) =>
      conversation.messageIds || conversation.messages?.map((item: any) => item.id) || []
    )
    if (messageIds.length) onBulkMarkRead?.(messageIds)
  }

  // "Pessoas" is the only inbox category that is split by destination. This
  // keeps conversations sent to separate accounts visually independent while
  // newsletters, notifications and commercial mail remain compact groups.
  function recipientForConversation(conversation: message.Conversation): string {
    const conversationAccountID = (conversation as any).accountId || accountId
    const account = accountStore.accounts.find(item => item.account.id === conversationAccountID)
    return account?.account.email || conversation.accountName || 'Conta atual'
  }

  function visibleInboxConversations(group: InboxDisplayGroup): message.Conversation[] {
    if (expandedInboxGroups.has(group.id)) return group.conversations
    const limit = getInboxCardVisibleCount(group.category)
    return limit === 0 ? group.conversations : group.conversations.slice(0, limit)
  }

  function showAllInboxConversations(groupID: string) {
    expandedInboxGroups = new Set([...expandedInboxGroups, groupID])
  }

  function categoryForConversation(conversation: message.Conversation): { id: string; label: string; icon: string } {
    // Newly synced messages carry a classification inferred from their real
    // RFC/IMAP headers. Keep the old sender heuristic only as a fallback for
    // messages that were already stored before this feature existed.
    const headerCategory = (conversation as any).inboxCategory as string | undefined
    if (headerCategory === 'news') return { id: 'news', label: $_('inbox.news'), icon: 'mdi:newspaper-variant-outline' }
    if (headerCategory === 'notifications') return { id: 'notifications', label: $_('inbox.notifications'), icon: 'mdi:bell-outline' }
    if (headerCategory === 'commercial') return { id: 'commercial', label: $_('inbox.commercial'), icon: 'mdi:storefront-outline' }

    const sender = `${conversation.participants?.[0]?.name ?? ''} ${conversation.participants?.[0]?.email ?? ''}`.toLowerCase()
    const senderEmail = `${conversation.participants?.[0]?.email ?? ''}`.toLowerCase()
    const subject = `${conversation.subject ?? ''} ${conversation.snippet ?? ''}`.toLowerCase()
    // Only inspect the mailbox name (the part before @), normalised so
    // doNotReply, do-not-reply and do_not_reply all mean the same thing.
    const mailbox = (senderEmail.split('@')[0] ?? '').replace(/[^a-z0-9]/g, '')
    const domain = senderEmail.split('@')[1] ?? ''
    const domainHas = (names: string[]) => names.some(name =>
      new RegExp(`(^|[.-])${name}([.-]|$)`).test(domain)
    )
    const automatedMailboxNames = [
      // Replies, system mail and mail transport.
      'noreply', 'donotreply', 'notreply', 'autoreply', 'automaticreply',
      'auto', 'automatic', 'automated', 'automacao', 'automatico', 'automatica',
      'system', 'sistema', 'mailer', 'mailbot', 'bot', 'robot', 'daemon',
      'postmaster', 'bounce', 'bounces', 'returnpath',
      // Notifications, lists and editorial/marketing mail.
      'notification', 'notifications', 'notificacao', 'notificacoes', 'alert',
      'alerts', 'aviso', 'avisos', 'update', 'updates', 'newsletter', 'news',
      'digest', 'mailinglist', 'broadcast', 'bulk', 'marketing', 'campaign',
      'campaigns', 'promo', 'promotions', 'offers', 'deals',
      // Commercial and customer-service addresses.
      'sales', 'vendas', 'commercial', 'comercial', 'service', 'services',
      'support', 'suporte', 'help', 'ajuda', 'atendimento', 'sac', 'contact',
      'contato', 'faleconosco', 'info', 'information', 'comunicacao',
      'relacionamento', 'financeiro', 'billing', 'invoice', 'faturamento',
      'cobranca', 'payment', 'payments', 'pagamento', 'orders', 'order',
      'pedido', 'pedidos', 'shipping', 'delivery', 'entrega', 'account',
      'accounts', 'conta', 'security', 'seguranca', 'verify', 'verification',
      'verificacao', 'transaction', 'transactions', 'transacional',
    ]
    const isAutomatedMailbox = automatedMailboxNames.some(name =>
      mailbox === name || mailbox.startsWith(name) || mailbox.endsWith(name)
    )

    // Newsletters are determined from the sender address, not from the
    // subject. A commercial subject such as an offer must not turn an order
    // confirmation into "Notícias". The mailbox rules cover names such as
    // deals@ and contato@; domain rules cover shop/news/comunicacao and their
    // common variations.
    if (/(deal|deals|oferta|ofertas|contato|contact|newsletter|news|boletim|informativo|comunicado|comunicados|novidades|promo|promocao|promocoes|offers|conteudo|editorial|imprensa|press|digest|weekly|daily|semanal|diario|community|comunidade)/.test(mailbox) ||
        /(deal|deals|oferta|ofertas|contato|contact|newsletter|news|boletim|informativo|comunicado|novidades|promo|promocao|offers|conteudo|editorial|imprensa|press|digest|community|comunidade)/.test(sender) ||
        /(^|[.-])(shop|shops|store|stores|news|newsletter|noticias|comunicacao|comunicacoes|comunica|marketing|promo|promos|promocao|promocoes|offers|ofertas|deals|blog|blogs|media|mailing|updates|update|digest|conteudo|content|editorial|press|imprensa|community|comunidade|campaign|campaigns)([.-]|$)/.test(domain)) {
      return { id: 'news', label: $_('inbox.news'), icon: 'mdi:newspaper-variant-outline' }
    }

    // These are messages that usually require attention or report a state
    // change: security, sign-in, payment, support ticket and similar events.
    if (domainHas(['alert', 'alerts', 'notify', 'notification', 'notifications', 'security', 'secure', 'account', 'accounts', 'auth', 'login', 'status', 'support']) ||
        /(google search console|discord|support|mercado pago|google)/.test(sender) ||
        /(alert|security|notification|notifica|confirm|approved|aprovad|cancel|payment|pagamento|pix|login|sign-in|access code|c[oó]digo|ticket|verification|verifica[cç][aã]o)/.test(subject)) {
      return { id: 'notifications', label: $_('inbox.notifications'), icon: 'mdi:bell-outline' }
    }

    // A company address or an automated mailbox is useful to keep separate
    // from actual people, even when its subject is neither a promotion nor an
    // alert. The remaining fallback is intentionally reserved for people.
    if (isAutomatedMailbox ||
        domainHas(['bank', 'banco', 'finance', 'financial', 'financas', 'pay', 'payment', 'payments', 'billing', 'invoice', 'cobranca', 'insurance', 'seguro', 'health', 'saude', 'travel', 'viagens', 'hotel', 'delivery', 'entrega', 'food', 'market', 'marketplace', 'commerce', 'ecommerce', 'business', 'empresa', 'corp', 'corporate', 'service', 'services']) ||
        /(deals|empresa|empresas|tower|comunica[cç][aã]o|academy|academia|platform|shop|store|business|bank|banco|giga|pandap[eé]|livelo|caixa|tns money|smiles|gopro|vivo)/.test(sender)) {
      return { id: 'commercial', label: $_('inbox.commercial'), icon: 'mdi:storefront-outline' }
    }
    // Common consumer-mail domains are a strong signal that the sender is a
    // person. Unknown domains retain the conservative Pessoas fallback below.
    if (domainHas(['gmail', 'googlemail', 'outlook', 'hotmail', 'live', 'yahoo', 'icloud', 'me', 'mac', 'protonmail', 'proton', 'zoho', 'uol', 'bol', 'terra'])) {
      return { id: 'people', label: $_('inbox.people'), icon: 'mdi:account-outline' }
    }
    return { id: 'people', label: $_('inbox.people'), icon: 'mdi:account-outline' }
  }

  function dayGroupFor(dateValue: Date | string | undefined): { id: string; label: string } {
    const date = dateValue ? new Date(dateValue) : new Date(0)
    const today = new Date()
    const startToday = new Date(today.getFullYear(), today.getMonth(), today.getDate()).getTime()
    const startDate = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
    const yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1).getTime()
    if (startDate === startToday) return { id: 'today', label: $_('inbox.today') }
    if (startDate === yesterday) return { id: 'yesterday', label: $_('inbox.yesterday') }
    const sameYear = date.getFullYear() === today.getFullYear()
    const month = new Intl.DateTimeFormat(getLanguage() || 'en', { month: 'long' }).format(date)
    const monthLabel = `${month.charAt(0).toUpperCase()}${month.slice(1)}`
    return {
      // Older conversations are grouped by month, not by an unnecessarily
      // dense daily card. The year is shown only when it differs from today.
      id: `month-${date.getFullYear()}-${date.getMonth()}`,
      label: sameYear ? monthLabel : `${monthLabel} ${date.getFullYear()}`,
    }
  }

  function inboxGroups(): InboxDisplayGroup[] {
    const groups = new Map<string, InboxDisplayGroup>()
    const add = (id: string, label: string, icon: string, category: string, conversation: message.Conversation, recipient?: string) => {
      const existing = groups.get(id)
      if (existing) existing.conversations.push(conversation)
      else groups.set(id, { id, label, icon, category, recipient, conversations: [conversation] })
    }

    for (const conversation of conversations) {
      if (inboxDisplayMode === 'categories') {
        // Read conversations always live in one "Lidos" card. This keeps the
        // action of opening an email tangible: once the last unread message in
        // its conversation is read, it leaves its source category immediately.
        const isRead = (conversation.unreadCount || 0) === 0
        const classified = categoryForConversation(conversation)
        const category = isRead
          ? { id: 'read', label: $_('inbox.read'), icon: 'mdi:eye-outline' }
          // Commercial mail is a type of notification in this compact inbox,
          // so the view intentionally has no separate commercial card.
          : classified.id === 'commercial'
            ? { id: 'notifications', label: $_('inbox.notifications'), icon: 'mdi:bell-outline' }
            : classified
        const personAccountID = (conversation as any).accountId || accountId || ''
        if (personAccountID && !isInboxCardAccountVisible(category.id, personAccountID)) continue
        const recipient = getInboxCardGrouping(category.id) === 'per-account'
          ? recipientForConversation(conversation)
          : undefined
        const groupID = recipient ? `${category.id}:${recipient.toLowerCase()}` : category.id
        add(groupID, category.label, category.icon, category.id, conversation, recipient)
      } else if (inboxDisplayMode === 'priority') {
        const isPriority = (conversation.unreadCount || 0) > 0 || conversation.isStarred
        add(isPriority ? 'priority' : 'other', isPriority ? $_('inbox.priority') : $_('inbox.other'), isPriority ? 'mdi:lightning-bolt-outline' : 'mdi:inbox-outline', isPriority ? 'priority' : 'other', conversation)
      } else {
        const day = dayGroupFor(conversation.latestDate)
        add(day.id, day.label, 'mdi:calendar-blank-outline', 'chronological', conversation)
      }
    }

    const order = inboxDisplayMode === 'categories'
      ? ['people', 'notifications', 'news', 'read']
      : inboxDisplayMode === 'priority'
        ? ['priority', 'other']
        : []
    return [...groups.values()].sort((a, b) => {
      if (order.length === 0) return 0
      return order.indexOf(a.category) - order.indexOf(b.category)
    })
  }

  const filterLabel = $derived((() => {
    switch (filterMode) {
      case 'unread': return $_('messageList.filterUnread')
      case 'starred': return $_('messageList.filterStarred')
      case 'attachments': return $_('messageList.filterAttachments')
      default: return ''
    }
  })())

  const filterOptions = $derived([
    { value: '', label: $_('messageList.filterAll') },
    { value: 'unread', label: $_('messageList.filterUnread'), separator: true },
    { value: 'starred', label: $_('messageList.filterStarred') },
    { value: 'attachments', label: $_('messageList.filterAttachments') },
  ])

  // Server search state
  let serverSearchMode = $state(false)
  let serverSearchResults = $state<any[]>([])
  let serverSearchCount = $state(0)
  let serverSearchTotalCount = $state(0)  // Total matching UIDs on server (may exceed serverSearchCount when limited)
  let isServerSearching = $state(false)
  let lastServerQuery = $state('')
  const SERVER_SEARCH_LIMIT = 200

  // FTS indexing state
  let indexProgress = $state(0)
  let indexComplete = $state(true)
  let isIndexing = $state(false)
  let searchInputRef = $state<HTMLInputElement | null>(null)

  function isUnifiedSpecialFolder(acctId: string, fldId: string): boolean {
    const acct = accountStore.accounts.find(a => a.account.id === acctId)
    if (!acct) return false
    const configuredPath = (() => {
      const account: any = acct.account
      switch (folderId) {
        case 'archive': return account.archiveFolderPath
        case 'spam': return account.spamFolderPath
        case 'all': return account.allMailFolderPath
        case 'starred': return account.starredFolderPath
        case 'sent': return account.sentFolderPath
        case 'drafts': return account.draftsFolderPath
        case 'trash': return account.trashFolderPath
        default: return undefined
      }
    })()
    for (const tree of acct.folders) {
      if (tree.folder?.id === fldId) return tree.folder.type === folderId || tree.folder.path === configuredPath
      for (const child of tree.children || []) {
        if (child.folder?.id === fldId) return child.folder.type === folderId || child.folder.path === configuredPath
      }
    }
    return false
  }

  // Schedule a debounced reload — coalesces rapid sync events from multiple accounts
  // into a single loadConversations() call after they settle (300ms).
  // Defers if a dialog guard is active (e.g. folder picker open).
  function scheduleReload() {
    if (isDialogGuardActive()) {
      pendingReload = true
      return
    }
    if (syncReloadTimer) clearTimeout(syncReloadTimer)
    syncReloadTimer = setTimeout(() => {
      syncReloadTimer = null
      if (loading) {
        pendingReload = true
        return
      }
      // Preserve the loaded window + scroll position across the sync reload
      // (#348) — same pattern as handleActionComplete. Without this, a sync
      // collapses "Load more" pagination back to the first page and jumps
      // the view to the top.
      const scrollTop = listContainerRef?.scrollTop ?? 0
      const totalLoaded = Math.max(conversations.length, PAGE_SIZE)
      offset = 0
      loadConversations(totalLoaded).then(() => {
        if (listContainerRef) {
          requestAnimationFrame(() => {
            listContainerRef!.scrollTop = scrollTop
          })
        }
      })
    }, 300)
  }

  // Listen for folder sync events from backend
  onMount(() => {
    EventsOn('folder:synced', (data: { accountId: string; folderId: string }) => {
      // Reload if this is the current folder, or unified inbox when an inbox folder synced
      if ((isUnifiedView && isUnifiedSpecialFolder(data.accountId, data.folderId)) || (!isUnifiedView && accountId && folderId && data.accountId === accountId && data.folderId === folderId)) {
        scheduleReload()
      }
    })

    // Listen for messages:updated events (e.g., from IDLE push notifications)
    EventsOn('messages:updated', (data: { accountId: string; folderId: string }) => {
      // Reload if this is the current folder, or unified inbox when an inbox folder updated
      if ((isUnifiedView && isUnifiedSpecialFolder(data.accountId, data.folderId)) || (!isUnifiedView && accountId && folderId && data.accountId === accountId && data.folderId === folderId)) {
        scheduleReload()
      }
    })

    // Listen for message read-state changes. Star changes ride their own
    // `messages:starredChanged` event and don't affect unread counts here.
    EventsOn('messages:readChanged', (data: { messageIds: string[], isRead: boolean }) => {
      // Update conversations locally instead of reloading from DB
      let anyUpdated = false
      for (const c of conversations) {
        const affectedCount = (c.messageIds || []).filter(id => data.messageIds.includes(id)).length
        if (affectedCount > 0) {
          anyUpdated = true
          const delta = data.isRead ? -affectedCount : affectedCount
          c.unreadCount = Math.max(0, (c.unreadCount || 0) + delta)
        }
      }
      if (anyUpdated) {
        conversations = conversations
        return
      }
      // loadConversations() is in-flight — the new array isn't ready yet.
      // Buffer this change so we can apply it after the load completes.
      if (loading) {
        pendingFlagChanges.push({ messageIds: data.messageIds, isRead: data.isRead })
      }
    })

    // Listen for FTS indexing progress
    EventsOn('fts:progress', (data: { folderId: string; indexed: number; total: number; percentage: number }) => {
      if (folderId && data.folderId === folderId) {
        indexProgress = data.percentage
        indexComplete = false
        isIndexing = true
      }
    })

    // Listen for FTS indexing completion
    EventsOn('fts:complete', (data: { folderId: string }) => {
      if (folderId && data.folderId === folderId) {
        indexComplete = true
        isIndexing = false
        indexProgress = 100
      }
    })

    // Listen for FTS indexing status changes
    EventsOn('fts:indexing', (data: { status: string }) => {
      switch (data.status) {
        case 'completed':
          indexComplete = true
          isIndexing = false
          break
        case 'started':
          isIndexing = true
          break
      }
    })

    // Check initial FTS index status for current folder
    checkFTSIndexStatus()

    // Flush deferred reloads once dialogs close
    dialogGuardInterval = setInterval(() => {
      if (pendingReload && !isDialogGuardActive()) {
        pendingReload = false
        scheduleReload()
      }
    }, 500)
  })

  let dialogGuardInterval: ReturnType<typeof setInterval> | null = null

  onDestroy(() => {
    EventsOff('folder:synced')
    EventsOff('messages:updated')
    EventsOff('messages:readChanged')
    EventsOff('fts:progress')
    EventsOff('fts:complete')
    EventsOff('fts:indexing')
    if (reloadTimer) clearTimeout(reloadTimer)
    if (syncReloadTimer) clearTimeout(syncReloadTimer)
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
    if (dialogGuardInterval) clearInterval(dialogGuardInterval)
  })

  // Check FTS index status for current folder
  async function checkFTSIndexStatus() {
    if (!folderId) return
    try {
      const status = await GetFTSIndexStatus(folderId)
      if (status) {
        indexComplete = status.isComplete
        if (status.totalCount > 0) {
          indexProgress = Math.round((status.indexedCount / status.totalCount) * 100)
        }
      }
      isIndexing = await IsFTSIndexing()
    } catch (err) {
      console.error('Failed to check FTS index status:', err)
    }
  }

  // Track previous folder to detect actual changes
  let prevAccountId: string | null = null
  let prevFolderId: string | null = null

  // Remember the departing folder's pagination + scroll for the session so
  // returning to it restores the view (follow-up to #348).
  function rememberListState() {
    if (prevAccountId === null || prevFolderId === null) return
    sessionListState.set(`${prevAccountId}:${prevFolderId}`, {
      loadedCount: conversations.length,
      scrollTop: listContainerRef?.scrollTop ?? 0,
      selectedThreadId,
    })
  }

  // Clear selection and search when folder changes
  $effect(() => {
    const currentAccount = isUnifiedView ? 'unified' : accountId
    const currentFolder = isUnifiedView ? folderId : folderId

    if (!isUnifiedView && (!accountId || !folderId)) {
      rememberListState()
      prevAccountId = null
      prevFolderId = null
      conversations = []
      totalCount = 0
      checkedThreadIds = new Set()
      return
    }

    // Only reset and reload if folder actually changed
    if (currentAccount === prevAccountId && currentFolder === prevFolderId) return

    rememberListState()
    prevAccountId = currentAccount
    prevFolderId = currentFolder
    loadGeneration++ // Invalidate any in-flight loads from the previous folder (#200)
    offset = 0
    checkedThreadIds = new Set()
    lastClickedIndex = null
    // Clear search state when folder changes
    showSearch = false
    searchQuery = ''
    searchResults = []
    searchTotalCount = 0
    searchOffset = 0
    serverSearchMode = false
    serverSearchResults = []
    serverSearchCount = 0
    serverSearchTotalCount = 0
    lastServerQuery = ''
    // Restore this folder's session state: reload the previously-paginated
    // window and put the scroll back where the user left it.
    const remembered = sessionListState.get(`${currentAccount}:${currentFolder}`)
    const restoreCount = remembered && remembered.loadedCount > PAGE_SIZE ? remembered.loadedCount : undefined
    loadConversations(restoreCount).then(() => {
      // Guard against rapid folder switches: only restore if we're still on
      // the folder this load was for.
      if (!remembered || prevAccountId !== currentAccount || prevFolderId !== currentFolder) {
        return
      }
      // Restore the selection (keyboard-nav anchor) so j/k continue from
      // where the user left off instead of the auto-selected first row —
      // only if the thread still exists in the reloaded window.
      if (
        remembered.selectedThreadId &&
        conversations.some((c) => c.threadId === remembered.selectedThreadId)
      ) {
        selectedThreadId = remembered.selectedThreadId
      }
      if (remembered.scrollTop > 0 && listContainerRef) {
        requestAnimationFrame(() => {
          listContainerRef!.scrollTop = remembered.scrollTop
        })
      }
    })
    checkFTSIndexStatus()
  })

  // Compute selected message IDs from all checked conversations (for multi-select context menu)
  // Check both conversations and searchResults since selections can span both
  // Use Set to deduplicate in case same conversation appears in both arrays
  const selectedMessageIds = $derived(
    [...new Set(
      [...conversations, ...searchResults]
        .filter((c) => checkedThreadIds.has(c.threadId))
        .flatMap((c: any) => c.messageIds || c.messages?.map((m: any) => m.id) || [])
    )]
  )

  // Aggregated star/read state for multi-select context menu
  // Show "Star" if any selected is unstarred, show "Mark as Read" if any selected is unread
  const selectedHasUnstarred = $derived(
    [...conversations, ...searchResults]
      .filter((c) => checkedThreadIds.has(c.threadId))
      .some((c: any) => !c.isStarred)
  )
  const selectedHasUnread = $derived(
    [...conversations, ...searchResults]
      .filter((c) => checkedThreadIds.has(c.threadId))
      .some((c: any) => (c.unreadCount || 0) > 0)
  )

  // Clear multi-select (called when right-clicking on unchecked row)
  function clearSelection() {
    checkedThreadIds = new Set()
    lastClickedIndex = null
  }

  // Check if viewing unified inbox
  const isUnifiedView = $derived(accountId === 'unified' && !!folderId)

  // A bound backend call can occasionally stall while SQLite is recovering a
  // lock after a hot reload. Never leave the message pane in a permanent
  // loading state: surface a retryable error instead.
  async function withLoadTimeout<T>(operation: Promise<T>, label: string, timeoutMs = 12_000): Promise<T> {
    let timeoutID: ReturnType<typeof setTimeout> | undefined
    try {
      return await Promise.race([
        operation,
        new Promise<T>((_resolve, reject) => {
          timeoutID = setTimeout(() => reject(new Error(`${label} timed out`)), timeoutMs)
        }),
      ])
    } finally {
      if (timeoutID !== undefined) clearTimeout(timeoutID)
    }
  }

  async function loadConversations(customLimit?: number) {
    // For unified view, we don't need accountId/folderId
    if (!isUnifiedView && (!accountId || !folderId)) return

    // Prevent concurrent loads — defer instead of dropping
    if (loading) {
      pendingReload = true
      return
    }

    loading = true
    error = null

    // Capture offset and generation at start — both may change during async operations
    const currentOffset = offset
    const limit = customLimit ?? PAGE_SIZE
    const generation = loadGeneration

    try {
      // Render conversations as soon as their query completes. Counting is
      // useful for pagination, but must never hold the whole inbox hostage.
      const convList = await withLoadTimeout(
        isUnifiedView
          ? GetUnifiedFolderConversations(folderId!, currentOffset, limit, getMessageListSortOrder(), filterMode)
          : GetConversations(accountId!, folderId!, currentOffset, limit, getMessageListSortOrder(), filterMode),
        'conversation list',
      )
      let count = convList?.length ?? 0
      try {
        count = await withLoadTimeout(
          isUnifiedView
            ? GetUnifiedFolderCount(folderId!, filterMode)
            : GetConversationCount(accountId!, folderId!, filterMode),
          'conversation count',
          4_000,
        )
      } catch (countError) {
        console.warn('Failed to count conversations; rendering loaded results:', countError)
      }

      // Discard stale result — folder was switched while this load was in-flight (#200)
      if (generation !== loadGeneration) return

      if (currentOffset !== 0) {
        conversations = [...conversations, ...(convList || [])]
        totalCount = count
        return
      }

      conversations = convList || []

      // Apply any flag changes that arrived while we were loading.
      // This fixes the race where MarkAsRead fires before the new array is ready.
      if (pendingFlagChanges.length > 0) {
        for (const change of pendingFlagChanges) {
          for (const c of conversations) {
            const affectedCount = (c.messageIds || []).filter(
              (id: string) => change.messageIds.includes(id)
            ).length
            if (affectedCount > 0) {
              const delta = change.isRead ? -affectedCount : affectedCount
              c.unreadCount = Math.max(0, (c.unreadCount || 0) + delta)
            }
          }
        }
        pendingFlagChanges = []
      }

      // Check if we switched to a different folder
      const folderChanged = lastLoadedFolderId !== folderId
      lastLoadedFolderId = folderId

      // Auto-select first message on folder navigation or initial load
      if (conversations.length === 0) {
        selectedThreadId = null
        totalCount = count
        return
      }

      if (folderChanged || !selectedThreadId) {
        selectedThreadId = conversations[0].threadId
      }
      totalCount = count
    } catch (err) {
      // Discard stale error — folder was switched while this load was in-flight (#200)
      if (generation !== loadGeneration) return
      console.error('Failed to load messages:', err)
      error = $_('viewer.failedToLoadMessages')
    } finally {
      loading = false
      // Flush any deferred reload (from sync event during load or dialog guard).
      // Only the latest-generation load should drive the flush — otherwise a
      // stale completion could fire scheduleReload redundantly.
      if (generation === loadGeneration && pendingReload && !isDialogGuardActive()) {
        pendingReload = false
        scheduleReload()
      }
    }
  }

  export async function syncFolder() {
    if (isUnifiedView) {
      await SyncUnifiedFolder(folderId!)
      await loadConversations(Math.max(conversations.length, PAGE_SIZE))
      return
    }
    // virtual:archive is derived from All Mail; it is never a valid IMAP
    // folder ID and must not reach SyncFolder.
    if (folderId === 'virtual:archive') {
      await loadConversations(Math.max(conversations.length, PAGE_SIZE))
      return
    }
    if (!accountId || !folderId) return

    error = null

    try {
      // SyncFolder returns after headers sync, but body fetch continues in background
      // The account store tracks sync:progress and folder:synced events to manage syncing state
      // Preserve the loaded window + scroll position (#348) — manual sync must
      // not collapse "Load more" pagination.
      const scrollTop = listContainerRef?.scrollTop ?? 0
      const totalLoaded = Math.max(conversations.length, PAGE_SIZE)
      await SyncFolder(accountId, folderId)
      offset = 0
      await loadConversations(totalLoaded)
      if (listContainerRef) {
        requestAnimationFrame(() => {
          listContainerRef!.scrollTop = scrollTop
        })
      }
    } catch (err) {
      console.error('Failed to sync folder:', err)
      error = $_('viewer.failedToLoadMessages')
    }
    // No need to manage syncing state - account store handles it via events
  }

  // Cancel folder sync
  export async function cancelFolderSync() {
    if (isUnifiedView || !accountId || !folderId) return

    try {
      await CancelFolderSync(accountId, folderId)
    } catch (err) {
      console.error('Failed to cancel folder sync:', err)
    }
  }

  // Toggle folder sync (start if not running, cancel if running) - for keyboard shortcut and UI
  export async function toggleFolderSync() {
    if (syncing) {
      await cancelFolderSync()
      return
    }
    await syncFolder()
  }

  // Force re-sync folder (clears bodies & attachments, then re-fetches)
  async function forceSyncFolder() {
    if (isUnifiedView || !accountId || !folderId) return

    error = null

    try {
      await ForceSyncFolder(accountId, folderId)
      offset = 0
      await loadConversations()
    } catch (err) {
      console.error('Failed to force re-sync folder:', err)
      error = $_('viewer.failedToLoadMessages')
    }
  }

  // Handle search input with debounce
  function handleSearchInput() {
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer)

    if (!searchQuery.trim()) {
      // Clear search immediately if query is empty
      searchResults = []
      searchTotalCount = 0
      serverSearchResults = []
      serverSearchCount = 0
      serverSearchTotalCount = 0
      serverSearchMode = false
      return
    }

    // In server mode, don't auto-search locally — user will press Shift+Enter
    if (serverSearchMode) return

    searchDebounceTimer = setTimeout(() => {
      performSearch()
    }, 300)
  }

  // Perform the actual search
  async function performSearch() {
    const query = searchQuery.trim()
    if (!query) {
      searchResults = []
      searchTotalCount = 0
      searchOffset = 0
      return
    }

    // Don't start a new search if one is already in progress
    if (isSearching) return

    isSearching = true
    error = null
    searchOffset = 0  // Reset offset for new search

    try {
      let results: any[] = []
      let count = 0

      if (isUnifiedView) {
        ;[results, count] = await Promise.all([
          SearchUnifiedFolder(folderId!, query, 0, PAGE_SIZE, filterMode),
          GetSearchCountUnifiedFolder(folderId!, query, filterMode),
        ])
      } else if (accountId && folderId) {
        ;[results, count] = await Promise.all([
          SearchConversations(accountId, folderId, query, 0, PAGE_SIZE, filterMode),
          GetSearchCount(accountId, folderId, query, filterMode),
        ])
      }

      searchResults = results || []
      searchTotalCount = count
      // Auto-select first search result for keyboard navigation
      if (searchResults.length > 0) {
        selectedThreadId = searchResults[0].threadId
      }
    } catch (err) {
      console.error('Search failed:', err)
      error = $_('viewer.failedToLoadMessages')
    } finally {
      isSearching = false
    }
  }

  // Load more search results (pagination)
  async function loadMoreSearchResults() {
    const query = searchQuery.trim()
    if (!query || isSearching) return

    // Cancel any pending search debounce to prevent race conditions
    if (searchDebounceTimer) {
      clearTimeout(searchDebounceTimer)
      searchDebounceTimer = null
    }

    isSearching = true
    const newOffset = searchOffset + PAGE_SIZE

    try {
      let results: any[] = []
      if (isUnifiedView) {
        results = await SearchUnifiedFolder(folderId!, query, newOffset, PAGE_SIZE, filterMode)
      } else if (accountId && folderId) {
        results = await SearchConversations(accountId, folderId, query, newOffset, PAGE_SIZE, filterMode)
      }

      if (results && results.length > 0) {
        searchResults = [...searchResults, ...results]
        searchOffset = newOffset
      }
    } catch (err) {
      console.error('Load more search results failed:', err)
      error = $_('viewer.failedToLoadMessages')
    } finally {
      isSearching = false
    }
  }

  // Clear search and return to normal view
  function clearSearch() {
    searchQuery = ''
    searchResults = []
    searchTotalCount = 0
    searchOffset = 0
    showSearch = false
    serverSearchMode = false
    serverSearchResults = []
    serverSearchCount = 0
    serverSearchTotalCount = 0
    lastServerQuery = ''
    isServerSearching = false
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  }

  // Handle keyboard events in search input
  function handleSearchKeydown(event: KeyboardEvent) {
    switch (true) {
      case event.key === 'Enter' && event.shiftKey:
        event.preventDefault()
        if (isUnifiedView) return
        handleShiftEnter()
        break
      case event.key === 'Enter':
        // Move focus from search input to message list so user can navigate with arrow keys
        event.preventDefault()
        searchInputRef?.blur()
        listContainerRef?.focus()
        break
    }
  }

  // Smart toggle/re-search for server search (Shift+Enter)
  function handleShiftEnter() {
    const query = searchQuery.trim()
    if (!query) return

    if (!serverSearchMode) {
      // Local → server
      serverSearchMode = true
      lastServerQuery = query
      performServerSearch()
      return
    }

    if (query !== lastServerQuery) {
      // Server mode, query changed → re-search
      lastServerQuery = query
      performServerSearch()
      return
    }

    // Server mode, same query → toggle back to local
    serverSearchMode = false
  }

  // Perform IMAP server-side search. limit=0 means no limit (show all).
  async function performServerSearch(limit: number = SERVER_SEARCH_LIMIT) {
    const query = searchQuery.trim()
    if (!query || (!isUnifiedView && (!accountId || !folderId))) return

    // The IMAP fallback currently has only an inbox implementation. Keep local
    // FTS search correct for every special view instead of silently searching
    // inbox while the user is viewing a different unified folder.
    if (isUnifiedView && folderId !== 'inbox') {
      toasts.error('Server search is currently available for unified Inbox only')
      return
    }

    isServerSearching = true
    error = null
    try {
      const response = isUnifiedView
        ? await IMAPSearchUnifiedInbox(query, limit)
        : await IMAPSearchFolder(accountId!, folderId!, query, limit)
      const items = (response?.results || []).map(adaptServerResult)
      serverSearchResults = items
      serverSearchCount = items.length
      serverSearchTotalCount = response?.totalCount ?? items.length
      if (items.length > 0) {
        selectedThreadId = items[0].threadId
      }
    } catch (err) {
      console.error('Server search failed:', err)
      error = $_('viewer.failedToLoadMessages')
    } finally {
      isServerSearching = false
    }
  }

  // Map IMAPSearchResult to ConversationRow-compatible shape
  function adaptServerResult(r: any): any {
    return {
      threadId: r.messageId || `server-uid-${r.uid}`,
      subject: r.subject,
      snippet: r.isLocal ? r.snippet : '',
      messageCount: 1,
      unreadCount: r.isRead ? 0 : 1,
      hasAttachments: r.hasAttachments,
      isStarred: r.isStarred,
      latestDate: r.date,
      participants: [{ name: r.fromName, email: r.fromEmail }],
      messageIds: r.messageId ? [r.messageId] : [],
      accountId: r.accountId,
      folderId: r.folderId,
      _isLocal: r.isLocal,
      _uid: r.uid,
    }
  }

  // Toggle search visibility
  function toggleSearch() {
    showSearch = !showSearch
    if (showSearch) {
      // Focus input after it appears
      setTimeout(() => searchInputRef?.focus(), 50)
      return
    }
    clearSearch()
  }

  // Check if we're in search mode with results
  const isSearchMode = $derived(showSearch && searchQuery.trim().length > 0)
  const canUseInboxDisplay = $derived(folderType === 'inbox' && !isSearchMode && !filterMode)

  // Active list - either conversations, local search results, or server search results
  const activeList = $derived(
    isSearchMode
      ? (serverSearchMode ? serverSearchResults : searchResults)
      : conversations
  )
  const activeCount = $derived(
    isSearchMode
      ? (serverSearchMode ? serverSearchTotalCount : searchTotalCount)
      : totalCount
  )

  // Resolve every visible sender through the same batch path. Contact photos are
  // optional, but brand-logo lookup must not depend on that display preference.
  $effect(() => {
    const list = activeList
    const configuredAccountEmails = new Set(
      accountStore.accounts.map(item => item.account.email.trim().toLowerCase()).filter(Boolean)
    )
    const t = setTimeout(() => {
      const emails: string[] = []
      for (const c of list) {
        const email = c?.participants?.[0]?.email
        if (email) {
          emails.push(email)
        }
      }
      const accountEmails = emails.filter(email => configuredAccountEmails.has(email.trim().toLowerCase()))
      const ensureLogos = (logoEmails: string[]) => {
        const domains = logoEmails.map(domainFromEmail).filter(Boolean)
        if (!domains.length) return
        void senderLogos.ensure(domains)
      }
      if (!getShowMessageListProfilePics()) {
        void contactPhotos.ensure(accountEmails)
        ensureLogos(emails.filter(email => !configuredAccountEmails.has(email.trim().toLowerCase())))
        return
      }
      void contactPhotos.ensure(emails).then(() => {
        ensureLogos(emails.filter(email => !configuredAccountEmails.has(email.trim().toLowerCase()) && !contactPhotos.get(email)))
      }).catch(() => {
        // A contact lookup must never suppress the independent logo fallback.
        ensureLogos(emails.filter(email => !configuredAccountEmails.has(email.trim().toLowerCase())))
      })
    }, 150)
    return () => clearTimeout(t)
  })

  function toggleSetEntry(set: Set<string>, key: string) {
    if (set.has(key)) {
      set.delete(key)
      return
    }
    set.add(key)
  }

  function selectConversation(threadId: string, index: number, event?: MouseEvent) {
    // Shift+click: range select (preserve anchor)
    if (event?.shiftKey) {
      const start = lastClickedIndex !== null ? Math.min(lastClickedIndex, index) : index
      const end = lastClickedIndex !== null ? Math.max(lastClickedIndex, index) : index
      const newChecked = new Set(checkedThreadIds)
      for (let i = start; i <= end; i++) {
        newChecked.add(activeList[i].threadId)
      }
      checkedThreadIds = newChecked
      return
    }

    // Update anchor for non-shift clicks
    lastClickedIndex = index

    // Ctrl/Cmd+click: toggle single checkbox without changing selection
    if (event?.ctrlKey || event?.metaKey) {
      const newChecked = new Set(checkedThreadIds)
      toggleSetEntry(newChecked, threadId)
      checkedThreadIds = newChecked
      return
    }

    // Normal click - select for viewing, clear checks
    checkedThreadIds = new Set()
    selectedThreadId = threadId

    // For unified view or search, use real folderId and accountId from conversation data
    const conversation = activeList[index] as any
    const realFolderId = (isUnifiedView || isSearchMode) && conversation.folderId ? conversation.folderId : folderId!
    const realAccountId = (isUnifiedView || isSearchMode) && conversation.accountId ? conversation.accountId : accountId!

    // If this is a non-local server result, fetch it first
    if (serverSearchMode && conversation._isLocal === false && conversation._uid) {
      fetchAndSelectServerResult(conversation, realFolderId, realAccountId)
      return
    }
    onConversationSelect?.(threadId, realFolderId, realAccountId)
  }

  // Fetch a non-local server result, save locally, update the result, then select
  async function fetchAndSelectServerResult(conversation: any, realFolderId: string, realAccountId: string) {
    try {
      const msg = await FetchServerMessage(realAccountId, realFolderId, conversation._uid)
      if (msg) {
        // Update the server result to be local
        const idx = serverSearchResults.findIndex(r => r._uid === conversation._uid)
        if (idx >= 0) {
          serverSearchResults[idx] = {
            ...serverSearchResults[idx],
            threadId: msg.threadId || msg.id,
            messageIds: [msg.id],
            snippet: msg.snippet || '',
            _isLocal: true,
            _uid: conversation._uid,
          }
          serverSearchResults = serverSearchResults
          selectedThreadId = serverSearchResults[idx].threadId
        }
        onConversationSelect?.(msg.threadId || msg.id, realFolderId, realAccountId)
      }
    } catch (err) {
      console.error('Failed to fetch server message:', err)
      error = $_('viewer.failedToLoadMessages')
    }
  }

  function handleCheck(threadId: string, isChecked: boolean, index: number, event?: MouseEvent) {
    if (event?.shiftKey && lastClickedIndex !== null) {
      const start = Math.min(lastClickedIndex, index)
      const end = Math.max(lastClickedIndex, index)
      const newChecked = new Set(checkedThreadIds)
      for (let i = start; i <= end; i++) {
        newChecked.add(activeList[i].threadId)
      }
      checkedThreadIds = newChecked
      return
    }

    lastClickedIndex = index
    const newChecked = new Set(checkedThreadIds)
    if (isChecked) newChecked.add(threadId)
    if (!isChecked) newChecked.delete(threadId)
    checkedThreadIds = newChecked
  }

  export function handleActionComplete(autoSelectNext: boolean = false) {
    onRowActionComplete?.(autoSelectNext)
    // Get target index BEFORE reload (for auto-select after delete/archive/spam)
    // Uses earliest checked item's index so bulk delete doesn't overshoot
    const currentIndex = getEarliestCheckedIndex()
    const scrollTop = listContainerRef?.scrollTop ?? 0

    // If in search mode, refresh search results instead of conversations
    if (isSearchMode) {
      performSearch().then(() => {
        // Restore scroll position
        if (listContainerRef) {
          requestAnimationFrame(() => {
            listContainerRef!.scrollTop = scrollTop
          })
        }

        // Auto-select next message if requested
        if (autoSelectNext) {
          const isNarrow = getLayoutMode() === 'narrow'
          if (isNarrow) {
            hideViewer()
          }
          if (currentIndex >= 0 && searchResults.length > 0) {
            const newIndex = Math.min(currentIndex, searchResults.length - 1)
            const conv = searchResults[newIndex]
            if (conv) {
              if (isNarrow) {
                selectedThreadId = conv.threadId
              }
              if (!isNarrow) {
                selectConversation(conv.threadId, newIndex)
              }
            }
          }
        }
      })
      return
    }

    // Preserve loaded messages: reload all messages that were loaded
    // Use conversations.length to track actual loaded count (offset gets reset after first action)
    const totalLoaded = Math.max(conversations.length, PAGE_SIZE)
    offset = 0

    loadConversations(totalLoaded).then(() => {
      // Restore scroll position
      if (listContainerRef) {
        requestAnimationFrame(() => {
          listContainerRef!.scrollTop = scrollTop
        })
      }

      // Auto-select next message if requested (for delete/archive/spam actions)
      // After reload, the same index now points to what was the "next" message
      if (autoSelectNext) {
        const isNarrow = getLayoutMode() === 'narrow'
        if (isNarrow) {
          hideViewer()
        }
        if (currentIndex >= 0 && conversations.length > 0) {
          const newIndex = Math.min(currentIndex, conversations.length - 1)
          const conv = conversations[newIndex]
          if (conv) {
            if (isNarrow) {
              selectedThreadId = conv.threadId
            }
            if (!isNarrow) {
              selectConversation(conv.threadId, newIndex)
            }
          }
        }
      }

    })
  }

  // Toggle sort order and persist to backend
  async function toggleSortOrder() {
    const newOrder = getMessageListSortOrder() === 'newest' ? 'oldest' : 'newest'
    try {
      await SetMessageListSortOrder(newOrder)
      setMessageListSortOrder(newOrder)
      offset = 0
      loadConversations()
    } catch (err) {
      console.error('Failed to save sort order:', err)
    }
  }

  // Set filter mode and reload
  function setFilter(mode: string) {
    filterMode = mode
    offset = 0
    if (isSearchMode) {
      searchOffset = 0
      performSearch()
      return
    }
    loadConversations()
  }

  // Calculate total unread count
  const unreadCount = $derived(
    conversations.reduce((sum, c) => sum + (c.unreadCount || 0), 0)
  )

  // Reference to the list container for scrolling
  let listContainerRef = $state<HTMLDivElement | null>(null)

  function loadNextPage() {
    if (loading || conversations.length >= totalCount) return
    // Continue at the actual end of the loaded window. This also avoids
    // duplicate rows after a reload that restored more than one page.
    offset = conversations.length
    void loadConversations()
  }

  function handleListScroll(event: Event) {
    const container = event.currentTarget as HTMLDivElement
    // Start loading shortly before the user reaches the end so scrolling is
    // continuous rather than stopping at a manual "Load more" control.
    if (container.scrollHeight - container.scrollTop - container.clientHeight < 180) {
      loadNextPage()
    }
  }

  // Reference to the "Load more" button for keyboard navigation
  let loadMoreButtonRef = $state<HTMLButtonElement | null>(null)

  // Get current selected index
  function getSelectedIndex(): number {
    if (!selectedThreadId) return -1
    return activeList.findIndex(c => c.threadId === selectedThreadId)
  }

  // Select previous message (exposed for keyboard navigation)
  // Just moves focus, doesn't clear checkboxes or open in viewer
  export function selectPrevious() {
    if (activeList.length === 0) return

    const currentIndex = getSelectedIndex()
    const newIndex = currentIndex <= 0 ? 0 : currentIndex - 1

    const conv = activeList[newIndex]
    if (conv) {
      selectedThreadId = conv.threadId
      scrollToIndex(newIndex)
      // Blur any focused element so Enter key triggers openSelected() instead of the button
      ;(document.activeElement as HTMLElement)?.blur?.()
    }
  }

  // Select next message (exposed for keyboard navigation)
  // Just moves focus, doesn't clear checkboxes or open in viewer
  export function selectNext() {
    if (activeList.length === 0) return

    const currentIndex = getSelectedIndex()

    // If at last message and more are available, focus the "Load more" button
    if (currentIndex >= activeList.length - 1 && activeList.length < activeCount) {
      loadMoreButtonRef?.focus()
      return
    }

    const newIndex = currentIndex >= activeList.length - 1 ? activeList.length - 1 : currentIndex + 1

    const conv = activeList[newIndex]
    if (conv) {
      selectedThreadId = conv.threadId
      scrollToIndex(newIndex)
      // Blur any focused element so Enter key triggers openSelected() instead of the button
      ;(document.activeElement as HTMLElement)?.blur?.()
    }
  }

  // Select + scroll to the first message (g)
  export function selectFirst() {
    if (activeList.length === 0) return
    selectedThreadId = activeList[0].threadId
    scrollToIndex(0)
    // Blur any focused element so Enter key triggers openSelected() instead of the button
    ;(document.activeElement as HTMLElement)?.blur?.()
  }

  // Select + scroll to the last LOADED message (G) — does not paginate,
  // consistent with j/k stopping at the loaded window
  export function selectLast() {
    if (activeList.length === 0) return
    const last = activeList.length - 1
    selectedThreadId = activeList[last].threadId
    scrollToIndex(last)
    // Blur any focused element so Enter key triggers openSelected() instead of the button
    ;(document.activeElement as HTMLElement)?.blur?.()
  }

  // Open the currently selected conversation (exposed for keyboard navigation)
  export function openSelected() {
    if (!selectedThreadId) return

    const index = getSelectedIndex()
    if (index >= 0) {
      const conv = activeList[index] as any
      const realFolderId = (isUnifiedView || isSearchMode) && conv.folderId ? conv.folderId : folderId!
      const realAccountId = (isUnifiedView || isSearchMode) && conv.accountId ? conv.accountId : accountId!
      onConversationSelect?.(selectedThreadId, realFolderId, realAccountId)
    }
  }

  // Select a specific thread by ID (exposed for notification clicks)
  export function selectThread(threadId: string) {
    selectedThreadId = threadId
    const index = activeList.findIndex(c => c.threadId === threadId)
    if (index >= 0) {
      scrollToIndex(index)
    }
  }

  // Toggle search focus (exposed for keyboard navigation via Ctrl+S)
  // Three-state: closed → open, open but unfocused → focus, open and focused → close
  export function toggleSearchFocus() {
    switch (true) {
      case !showSearch:
        showSearch = true
        setTimeout(() => searchInputRef?.focus(), 50)
        break
      case document.activeElement !== searchInputRef:
        searchInputRef?.focus()
        break
      default:
        clearSearch()
    }
  }

  // Get the currently selected thread ID (exposed for parent access)
  export function getSelectedThreadId(): string | null {
    return selectedThreadId
  }

  // Get message IDs for the keyboard-focused thread (for delete without checking)
  export function getSelectedMessageIds(): string[] {
    if (!selectedThreadId) return []
    const conv = activeList.find(c => c.threadId === selectedThreadId) as any
    if (!conv) return []
    return conv.messageIds || conv.messages?.map((m: any) => m.id) || []
  }

  // Get account and folder info for the keyboard-focused thread (for unified inbox)
  export function getSelectedConversationInfo(): { accountId: string; folderId: string } | null {
    if (!selectedThreadId) return null
    const conv = activeList.find(c => c.threadId === selectedThreadId) as any
    if (!conv) return null

    const realAccountId = (isUnifiedView || isSearchMode) && conv.accountId ? conv.accountId : accountId
    const realFolderId = (isUnifiedView || isSearchMode) && conv.folderId ? conv.folderId : folderId

    if (!realAccountId || !realFolderId) return null
    return { accountId: realAccountId, folderId: realFolderId }
  }

  // Check if the keyboard-focused thread is starred
  export function isSelectedStarred(): boolean {
    if (!selectedThreadId) return false
    const conv = activeList.find(c => c.threadId === selectedThreadId) as any
    return conv?.isStarred ?? false
  }

  // Toggle checkbox for focused message (Space key)
  export function toggleCheck() {
    if (!selectedThreadId) return
    const newChecked = new Set(checkedThreadIds)
    toggleSetEntry(newChecked, selectedThreadId)
    checkedThreadIds = newChecked
    lastClickedIndex = getSelectedIndex()
  }

  // Select previous message AND check both current and previous (Shift+Up/k)
  export function selectPreviousWithCheck() {
    if (activeList.length === 0) return

    const currentIndex = getSelectedIndex()
    if (currentIndex <= 0) return  // Already at top or no selection

    const newIndex = currentIndex - 1
    const conv = activeList[newIndex]
    if (!conv) return

    // Check both current and new message
    const newChecked = new Set(checkedThreadIds)
    newChecked.add(activeList[currentIndex].threadId)
    newChecked.add(conv.threadId)
    checkedThreadIds = newChecked

    // Move focus (but don't open in viewer)
    selectedThreadId = conv.threadId
    scrollToIndex(newIndex)
    // Blur any focused element so Enter key triggers openSelected() instead of the button
    ;(document.activeElement as HTMLElement)?.blur?.()
  }

  // Select next message AND check both current and next (Shift+Down/j)
  export function selectNextWithCheck() {
    if (activeList.length === 0) return

    const currentIndex = getSelectedIndex()
    if (currentIndex < 0 || currentIndex >= activeList.length - 1) return  // No selection or already at bottom

    const newIndex = currentIndex + 1
    const conv = activeList[newIndex]
    if (!conv) return

    // Check both current and new message
    const newChecked = new Set(checkedThreadIds)
    newChecked.add(activeList[currentIndex].threadId)
    newChecked.add(conv.threadId)
    checkedThreadIds = newChecked

    // Move focus (but don't open in viewer)
    selectedThreadId = conv.threadId
    scrollToIndex(newIndex)
    // Blur any focused element so Enter key triggers openSelected() instead of the button
    ;(document.activeElement as HTMLElement)?.blur?.()
  }

  // Get all checked message IDs for bulk operations
  export function getCheckedMessageIds(): string[] {
    return selectedMessageIds
  }

  // Check if any messages are checked
  export function hasCheckedMessages(): boolean {
    return checkedThreadIds.size > 0
  }

  // Get aggregated star state (true if any unstarred)
  export function getCheckedHasUnstarred(): boolean {
    return selectedHasUnstarred
  }

  // Get aggregated read state (true if any unread)
  export function getCheckedHasUnread(): boolean {
    return selectedHasUnread
  }

  // Get index of earliest checked item (for post-delete focus)
  function getEarliestCheckedIndex(): number {
    if (checkedThreadIds.size === 0) return getSelectedIndex()
    for (let i = 0; i < activeList.length; i++) {
      if (checkedThreadIds.has(activeList[i].threadId)) return i
    }
    return getSelectedIndex()
  }

  // Clear all checkboxes
  export function clearChecked() {
    checkedThreadIds = new Set()
    lastClickedIndex = null
  }

  export function selectAll() {
    // Ctrl/Cmd+A toggles the complete selection so it can also dismiss it.
    if (checkedThreadIds.size > 0) {
      clearSelection()
      return
    }
    checkedThreadIds = new Set(activeList.map(c => c.threadId))
  }

  // Open context menu for the currently selected conversation row
  export function openContextMenu() {
    if (!selectedThreadId || !listContainerRef) return
    const index = activeList.findIndex(c => c.threadId === selectedThreadId)
    if (index < 0) return
    const rows = listContainerRef.querySelectorAll('[data-conversation-row]')
    const row = rows[index] as HTMLElement | undefined
    if (!row) return
    const rect = row.getBoundingClientRect()
    row.dispatchEvent(new MouseEvent('contextmenu', {
      bubbles: true,
      clientX: rect.right,
      clientY: rect.top + rect.height / 2,
    }))
  }

  // Row component refs keyed by threadId — lets keyboard shortcuts reach the
  // focused row's context-menu folder picker (Alt+M / Alt+C)
  let rowRefs: Record<string, ConversationRow | null> = {}

  function getFocusedRowRef(): ConversationRow | null {
    if (!selectedThreadId) return null
    return rowRefs[selectedThreadId] ?? null
  }

  export function isFolderPickerOpen(): boolean {
    return getFocusedRowRef()?.isFolderPickerOpen() ?? false
  }

  export function toggleMoveToDialog() {
    getFocusedRowRef()?.toggleFolderPicker('move')
  }

  export function toggleCopyToDialog() {
    getFocusedRowRef()?.toggleFolderPicker('copy')
  }

  // Permanent delete confirmation state
  let showDeleteConfirm = $state(false)
  let pendingDeleteIds = $state<string[]>([])

  // Empty trash confirmation state
  let showEmptyTrashConfirm = $state(false)

  async function handleUndo() {
    try {
      const description = await Undo()
      toasts.success($_('toast.undone', { values: { description } }))
    } catch (err) {
      console.error('Undo failed:', err)
      toasts.error($_('toast.undoFailed'))
    }
  }

  async function handleConfirmPermanentDelete() {
    try {
      await DeletePermanently(pendingDeleteIds)
      toasts.success($_('toast.permanentlyDeleted'))
      handleActionComplete(true)
      clearChecked()
    } catch (err) {
      console.error('Permanent delete failed:', err)
      toasts.error($_('toast.failedToDelete'))
    }
    showDeleteConfirm = false
    pendingDeleteIds = []
  }

  async function handleEmptyTrash() {
    try {
      if (isUnifiedView) {
        await EmptyUnifiedTrash()
      } else {
        if (!accountId || !folderId) return
        await EmptyTrash(accountId, folderId)
      }
      toasts.success($_('toast.trashEmptied'))
      handleActionComplete(true)
      clearChecked()
    } catch (err) {
      console.error('Empty trash failed:', err)
      toasts.error($_('toast.failedToEmptyTrash'))
    }
    showEmptyTrashConfirm = false
  }

  // Shared delete handler — same flow as context menu "Delete" action
  // Set permanent=true to force permanent delete (e.g. Shift+Delete)
  export function requestDelete(messageIds: string[], permanent: boolean = false) {
    if (permanent || folderType === 'trash') {
      pendingDeleteIds = messageIds
      showDeleteConfirm = true
      return
    }
    Trash(messageIds)
      .then((movedToTrash) => {
        const toastMsg = movedToTrash ? $_('toast.movedToTrash') : $_('toast.deletedFromFolder')
        const actions = movedToTrash ? [{ label: $_('common.undo'), onClick: handleUndo }] : []
        toasts.success(toastMsg, actions)
        handleActionComplete(true)
        clearChecked()
      })
      .catch((err) => {
        console.error('Delete failed:', err)
        toasts.error($_('toast.failedToDelete'))
      })
  }

  // Scroll to a specific index in the list
  function scrollToIndex(index: number) {
    if (!listContainerRef) return

    const rows = listContainerRef.querySelectorAll('[data-conversation-row]')
    const row = rows[index] as HTMLElement | undefined
    if (row) {
      row.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    }
  }
</script>

<div class="spark-message-list relative flex flex-col h-full {isFlashing ? 'pane-focus-flash' : ''}">
  <!-- Header -->
  <div class="spark-list-header flex items-center justify-between px-4 py-3 border-b border-border">
    <div class="flex items-center gap-2">
      {#if showFolderToggle}
        <button
          class="p-1.5 -ml-1 rounded-md hover:bg-muted transition-colors"
          title={$_('responsive.folders')}
          aria-label={$_('aria.toggleSidebar')}
          onclick={onToggleSidebar}
        >
          <Icon icon="mdi:dock-left" class="w-5 h-5 text-muted-foreground" />
        </button>
      {/if}
      {#if showSearch}
        <!-- Search input -->
        <div class="flex items-center gap-1 bg-muted rounded-md px-2 flex-1">
          <Icon icon="mdi:magnify" class="w-4 h-4 text-muted-foreground flex-shrink-0" />
          <input
            bind:this={searchInputRef}
            type="text"
            placeholder={$_('messageList.searchMessages')}
            class="bg-transparent border-none outline-none text-sm py-1.5 w-full min-w-[200px]"
            bind:value={searchQuery}
            oninput={handleSearchInput}
            onkeydown={handleSearchKeydown}
          />
          {#if serverSearchMode}
            <button
              onclick={() => { serverSearchMode = false }}
              class="px-1.5 py-0.5 text-[10px] font-medium bg-primary/20 text-primary rounded-full flex-shrink-0 hover:bg-primary/30 transition-colors"
              title={$_('search.localSearch')}
            >
              {$_('search.server')}
            </button>
          {/if}
          {#if searchQuery || isSearching || isServerSearching}
            <button
              onclick={clearSearch}
              class="p-0.5 hover:bg-muted-foreground/20 rounded"
              title={$_('messageList.clearSearch')}
            >
              {#if isSearching || isServerSearching}
                <Icon icon="mdi:loading" class="w-4 h-4 animate-spin text-muted-foreground" />
              {:else}
                <Icon icon="mdi:close" class="w-4 h-4 text-muted-foreground" />
              {/if}
            </button>
          {/if}
        </div>
      {:else}
        {#if folderType === 'inbox'}
          <button
            class="group -ml-2 rounded-lg px-2 py-1 text-left transition-colors hover:bg-muted/70"
            onclick={() => (showInboxDisplayPicker = true)}
            aria-haspopup="dialog"
            aria-label="Escolher exibição da caixa de entrada"
          >
            <span class="flex items-center gap-1 text-base font-semibold text-foreground whitespace-nowrap">
              {$_('sidebar.inbox')}
              <Icon icon="mdi:chevron-down" class="h-4 w-4 text-muted-foreground transition-transform group-hover:translate-y-px" />
            </span>
            <span class="block text-xs text-muted-foreground">{inboxDisplayLabel}</span>
          </button>
        {:else}
          <div class="min-w-0">
            <h2 class="font-semibold text-foreground whitespace-nowrap">{folderName}</h2>
          </div>
        {/if}
        <span class="spark-list-unread text-sm text-muted-foreground whitespace-nowrap">
          {$_('messageList.unread', { values: { count: unreadCount } })}
        </span>
      {/if}
    </div>
    <div class="flex items-center gap-1">
      {#if syncing}
        <!-- While syncing, show spinning icon that cancels on click -->
        <button
          class="p-2 rounded-md hover:bg-muted transition-colors"
          title={syncProgress ? `${$_('sidebar.syncing')} ${syncProgress.phase}: ${syncProgress.percentage}% - ${$_('sidebar.clickToCancel')}` : `${$_('sidebar.syncing')} ${$_('sidebar.clickToCancel')}`}
          onclick={cancelFolderSync}
        >
          <Icon
            icon="mdi:refresh"
            class="w-5 h-5 text-muted-foreground animate-spin"
          />
        </button>
      {:else}
        <!-- Dropdown menu for sync options -->
        <DropdownMenu.Root>
          <DropdownMenu.Trigger
            class="p-2 rounded-md hover:bg-muted transition-colors disabled:opacity-50"
            disabled={loading}
          >
            <Icon
              icon="mdi:refresh"
              class="w-5 h-5 text-muted-foreground"
            />
          </DropdownMenu.Trigger>
          <DropdownMenu.Portal>
            <DropdownMenu.Content
              side="bottom"
              align="end"
              sideOffset={4}
              class={cn(
                'z-50 min-w-[180px] rounded-md border bg-popover p-1 text-popover-foreground shadow-md',
                'data-[state=open]:animate-in data-[state=closed]:animate-out',
                'data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
                'data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
                'data-[side=bottom]:slide-in-from-top-2'
              )}
            >
              <DropdownMenu.Item
                onSelect={syncFolder}
                class="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none focus:bg-accent focus:text-accent-foreground"
              >
                <Icon icon="mdi:refresh" class="w-4 h-4 mr-2" />
                {$_('messageList.syncFolder')}
              </DropdownMenu.Item>
              <DropdownMenu.Separator class="-mx-1 my-1 h-px bg-border" />
              <DropdownMenu.Item
                onSelect={forceSyncFolder}
                class="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none focus:bg-accent focus:text-accent-foreground"
              >
                <Icon icon="mdi:refresh-auto" class="w-4 h-4 mr-2" />
                {$_('messageList.forceResync')}
              </DropdownMenu.Item>
            </DropdownMenu.Content>
          </DropdownMenu.Portal>
        </DropdownMenu.Root>
      {/if}
      <button
        class="p-2 rounded-md hover:bg-muted transition-colors {showSearch ? 'bg-muted' : ''}"
        title={showSearch ? $_('common.close') : $_('common.search')}
        onclick={toggleSearch}
      >
        <Icon icon={showSearch ? 'mdi:close' : 'mdi:magnify'} class="w-5 h-5 text-muted-foreground" />
      </button>
      <DropdownMenu.Root>
        <DropdownMenu.Trigger
          class="p-2 rounded-md hover:bg-muted transition-colors {filterMode ? 'bg-muted' : ''}"
          title={$_('messageList.filter')}
        >
          <Icon
            icon={filterMode ? 'mdi:filter' : 'mdi:filter-outline'}
            class="w-5 h-5 {filterMode ? 'text-primary' : 'text-muted-foreground'}"
          />
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            side="bottom"
            align="end"
            sideOffset={4}
            class={cn(
              'z-50 min-w-[180px] rounded-md border bg-popover p-1 text-popover-foreground shadow-md',
              'data-[state=open]:animate-in data-[state=closed]:animate-out',
              'data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
              'data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
              'data-[side=bottom]:slide-in-from-top-2'
            )}
          >
            {#each filterOptions as opt (opt.value ?? opt.label)}
              {#if opt.separator}
                <DropdownMenu.Separator class="-mx-1 my-1 h-px bg-border" />
              {/if}
              <DropdownMenu.Item
                onSelect={() => setFilter(opt.value)}
                class="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none focus:bg-accent focus:text-accent-foreground"
              >
                <Icon icon="mdi:check" class="w-4 h-4 mr-2 {filterMode === opt.value ? '' : 'invisible'}" />
                {opt.label}
              </DropdownMenu.Item>
            {/each}
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
      <button
        class="p-2 rounded-md hover:bg-muted transition-colors"
        title={getMessageListSortOrder() === 'newest' ? $_('messageList.showingNewest') : $_('messageList.showingOldest')}
        onclick={toggleSortOrder}
      >
        <Icon
          icon={getMessageListSortOrder() === 'newest' ? 'mdi:sort-descending' : 'mdi:sort-ascending'}
          class="w-5 h-5 text-muted-foreground"
        />
      </button>
    </div>
  </div>

  {#if showInboxDisplayPicker}
    <div class="absolute inset-0 z-40 flex items-start justify-center p-4 pt-20">
      <button
        type="button"
        class="absolute inset-0 cursor-default bg-background/70 backdrop-blur-[2px]"
        aria-label={$_('aria.dismiss')}
        onclick={() => (showInboxDisplayPicker = false)}
      ></button>
      <dialog
        open
        class="relative z-10 m-0 w-full max-w-[620px] rounded-2xl border border-border bg-card p-5 text-left shadow-2xl"
        aria-labelledby="inbox-display-title"
      >
        <div class="mb-5 flex items-center justify-between">
          <div>
            <h3 id="inbox-display-title" class="text-lg font-semibold text-foreground">{$_('inbox.displayTitle')}</h3>
            <p class="mt-1 text-sm text-muted-foreground">{$_('inbox.displayDescription')}</p>
          </div>
          <button class="rounded-lg p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground" onclick={() => (showInboxDisplayPicker = false)} aria-label={$_('aria.dismiss')}>
            <Icon icon="mdi:close" class="h-5 w-5" />
          </button>
        </div>
        <div class="grid grid-cols-3 gap-3">
          {#each inboxDisplayOptions as option (option.id)}
            <button
              class="group rounded-xl border p-3 text-left transition-all {inboxDisplayMode === option.id ? 'border-primary bg-primary/10 ring-1 ring-primary/40' : 'border-border bg-muted/25 hover:border-primary/45 hover:bg-muted/60'}"
              onclick={() => setInboxDisplayMode(option.id)}
            >
              <span class="mb-4 flex h-20 items-center justify-center rounded-lg bg-background/60 text-muted-foreground group-hover:text-primary {inboxDisplayMode === option.id ? 'text-primary' : ''}">
                <Icon icon={option.icon} class="h-10 w-10" />
              </span>
              <span class="flex items-center gap-1.5 font-semibold text-foreground">
                {#if inboxDisplayMode === option.id}<Icon icon="mdi:check-circle" class="h-4 w-4 text-primary" />{/if}
                {option.label}
              </span>
              <span class="mt-1 block text-xs leading-4 text-muted-foreground">{option.description}</span>
            </button>
          {/each}
        </div>
      </dialog>
    </div>
  {/if}

  <!-- Active filter chip -->
  {#if filterMode}
    <div class="flex items-center gap-2 px-4 py-1.5 border-b border-border bg-muted/30">
      <span class="text-xs text-muted-foreground">{$_('messageList.filterLabel')}:</span>
      <button
        class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-primary/10 text-primary hover:bg-primary/20 transition-colors"
        onclick={() => setFilter('')}
      >
        {filterLabel}
        <Icon icon="mdi:close" class="w-3 h-3" />
      </button>
    </div>
  {/if}

  {#if checkedThreadIds.size > 0}
    <div class="flex items-center gap-2 border-b border-border bg-muted/40 px-4 py-2" role="toolbar" aria-label="Ações para mensagens selecionadas">
      <span class="mr-auto text-sm font-medium text-foreground">{checkedThreadIds.size} selecionada{checkedThreadIds.size === 1 ? '' : 's'}</span>
      <button class="inline-flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground" onclick={() => onBulkMarkRead?.(selectedMessageIds)}>
        <Icon icon="mdi:check-circle-outline" class="h-4 w-4" /> {$_('common.done')}
      </button>
      <button class="inline-flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground" onclick={() => onBulkArchive?.(selectedMessageIds)}>
        <Icon icon="mdi:archive-outline" class="h-4 w-4" /> {$_('viewer.archive')}
      </button>
      <button class="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground" title={$_('common.close')} aria-label={$_('common.close')} onclick={clearSelection}>
        <Icon icon="mdi:close" class="h-4 w-4" />
      </button>
    </div>
  {/if}

  <!-- Empty Trash bar (only shown when viewing trash folder with messages, not in search mode) -->
  {#if folderType === 'trash' && totalCount > 0 && !isSearchMode}
    <div class="flex items-center justify-end px-4 py-2 bg-muted/50 border-b border-border">
      <Button
        size="sm"
        variant="outline"
        class="text-destructive hover:text-destructive hover:bg-destructive/10 border-destructive/50 bg-muted/50"
        onclick={() => { showEmptyTrashConfirm = true }}
      >
        <Icon icon="mdi:delete-sweep-outline" class="w-4 h-4 mr-1.5" />
        {$_('messageList.emptyTrash')}
      </Button>
    </div>
  {/if}

  <!-- FTS Indexing indicator (only shown when searching and index is incomplete) -->
  {#if showSearch && !indexComplete && isIndexing}
    <div class="px-4 py-2 bg-muted/50 border-b border-border">
      <div class="flex items-center gap-2 text-sm text-muted-foreground">
        <Icon icon="mdi:database-sync" class="w-4 h-4 animate-pulse" />
        <span>{$_('messageList.ftsBuilding', { values: { percentage: indexProgress } })}</span>
      </div>
      <div class="h-1 bg-muted rounded-full mt-1.5 overflow-hidden">
        <div
          class="h-full bg-primary transition-all duration-300"
          style="width: {indexProgress}%"
        ></div>
      </div>
    </div>
  {/if}

  <!-- Conversation List -->
  <div bind:this={listContainerRef} class="message-list-scroll flex-1 min-h-0 overflow-y-auto scrollbar-thin" onscroll={handleListScroll}>
    <div class="message-list-card" class:inbox-category-list={canUseInboxDisplay && inboxDisplayMode === 'categories'} class:inbox-chronological-list={canUseInboxDisplay && inboxDisplayMode === 'chronological'}>
    {#if loading && conversations.length === 0 && !isSearchMode}
      <div class="flex items-center justify-center h-32">
        <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    {:else if error}
      <div class="flex flex-col items-center justify-center h-32 text-center px-4">
        <Icon icon="mdi:alert-circle-outline" class="w-8 h-8 text-destructive mb-2" />
        <p class="text-sm text-destructive">{error}</p>
        <button
          class="mt-2 text-sm text-primary hover:underline"
          onclick={() => isSearchMode ? performSearch() : loadConversations()}
        >
          {$_('messageList.tryAgain')}
        </button>
      </div>
    {:else if !isUnifiedView && (!accountId || !folderId)}
      <div class="flex flex-col items-center justify-center h-full text-muted-foreground">
        <Icon icon="mdi:email-outline" class="w-12 h-12 mb-2" />
        <p>{$_('messageList.selectFolder')}</p>
      </div>
    {:else if isSearchMode}
      <!-- Search Results -->
      {#if isSearching || isServerSearching}
        <div class="flex flex-col items-center justify-center h-32 gap-2">
          <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
          {#if isServerSearching}
            <span class="text-xs text-muted-foreground">{$_('search.serverSearching')}</span>
          {/if}
        </div>
      {:else if serverSearchMode}
        <!-- Server search results -->
        {#if serverSearchResults.length === 0}
          <div class="flex flex-col items-center justify-center h-full text-muted-foreground">
            <Icon icon="mdi:magnify" class="w-12 h-12 mb-2" />
            <p>{$_('messageList.noResults', { values: { query: searchQuery } })}</p>
          </div>
        {:else}
          <!-- Server results header -->
          <div class="flex items-center justify-between px-4 py-2 bg-muted/30 border-b border-border text-sm text-muted-foreground">
            <span>
              {#if serverSearchCount < serverSearchTotalCount}
                {$_('search.serverResultsCapped', { values: { shown: serverSearchCount, total: serverSearchTotalCount, query: searchQuery } })}
              {:else}
                {$_('search.serverResults', { values: { count: serverSearchCount, query: searchQuery } })}
              {/if}
            </span>
            <button
              class="text-xs text-primary hover:underline"
              onclick={() => { serverSearchMode = false }}
            >
              {$_('search.localSearch')}
            </button>
          </div>
          {#each serverSearchResults as result, index (result.threadId + '-' + index)}
            {@const resultAccountId = result.accountId || accountId}
            {@const resultFolderId = result.folderId || folderId}
            <ConversationRow
              bind:this={rowRefs[result.threadId]}
              conversation={result}
              density={getMessageListDensity()}
              selected={selectedThreadId === result.threadId}
              checked={checkedThreadIds.has(result.threadId)}
              accountId={resultAccountId}
              folderId={resultFolderId}
              {folderType}
              {selectedMessageIds}
              selectedIsStarred={!selectedHasUnstarred}
              selectedIsRead={!selectedHasUnread}
              isNonLocal={result._isLocal === false}
              searchFolderName={result.folderName}
              searchFolderType={result.folderType}
              onSelect={(e) => selectConversation(result.threadId, index, e)}
              onCheck={(checked, e) => handleCheck(result.threadId, checked, index, e)}
              onClearSelection={clearSelection}
              onActionComplete={handleActionComplete}
              {onReply}
              onDelete={(ids) => requestDelete(ids)}
            />
          {/each}

          <!-- Show all results button (when results are capped) -->
          {#if serverSearchCount < serverSearchTotalCount}
            <div class="flex justify-center py-4">
              <button
                bind:this={loadMoreButtonRef}
                class="text-sm text-primary hover:underline focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 rounded px-2 py-1"
                onclick={() => performServerSearch(0)}
                disabled={isServerSearching}
              >
                {isServerSearching ? $_('common.loading') : $_('search.showAllResults', { values: { total: serverSearchTotalCount } })}
              </button>
            </div>
          {/if}
        {/if}
      {:else if searchResults.length === 0}
        <div class="flex flex-col items-center justify-center h-full text-muted-foreground">
          <Icon icon="mdi:magnify" class="w-12 h-12 mb-2" />
          <p>{$_('messageList.noResults', { values: { query: searchQuery } })}</p>
          {#if !indexComplete}
            <p class="text-xs mt-1">{$_('messageList.indexBuilding')}</p>
          {/if}
          {#if isUnifiedView || (accountId && folderId)}
            <button
              class="mt-2 text-sm text-primary hover:underline"
              onclick={() => { serverSearchMode = true; lastServerQuery = searchQuery.trim(); performServerSearch() }}
            >
              {$_('search.searchOnServer')}
            </button>
          {/if}
        </div>
      {:else}
        <!-- Local search results header -->
        <div class="flex items-center justify-between px-4 py-2 bg-muted/30 border-b border-border text-sm text-muted-foreground">
          <span>{$_('messageList.foundResults', { values: { count: searchTotalCount, query: searchQuery } })}</span>
          {#if isUnifiedView || (accountId && folderId)}
            <button
              class="text-xs text-primary hover:underline"
              onclick={() => { serverSearchMode = true; lastServerQuery = searchQuery.trim(); performServerSearch() }}
            >
              {$_('search.serverSearch')}
            </button>
          {/if}
        </div>
        {#each searchResults as result, index (result.threadId + '-' + index)}
          {@const resultAccountId = result.accountId || accountId}
          {@const resultFolderId = result.folderId || folderId}
          <ConversationRow
            bind:this={rowRefs[result.threadId]}
            conversation={result}
            density={getMessageListDensity()}
            selected={selectedThreadId === result.threadId}
            checked={checkedThreadIds.has(result.threadId)}
            accountId={isUnifiedView ? resultAccountId : accountId!}
            folderId={isUnifiedView ? resultFolderId : folderId!}
            {folderType}
            {selectedMessageIds}
            selectedIsStarred={!selectedHasUnstarred}
            selectedIsRead={!selectedHasUnread}
            highlightedSubject={result.highlightedSubject}
            highlightedSnippet={result.highlightedSnippet}
            highlightedFromName={result.highlightedFromName}
            searchFolderName={result.folderName}
            searchFolderType={result.folderType}
            onSelect={(e) => selectConversation(result.threadId, index, e)}
            onCheck={(checked, e) => handleCheck(result.threadId, checked, index, e)}
            onClearSelection={clearSelection}
            onActionComplete={handleActionComplete}
            {onReply}
            onDelete={(ids) => requestDelete(ids)}
          />
        {/each}

        <!-- Load more search results -->
        {#if searchResults.length < searchTotalCount}
          <div class="flex justify-center py-4">
            <button
              bind:this={loadMoreButtonRef}
              class="text-sm text-primary hover:underline focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 rounded px-2 py-1"
              onclick={() => loadMoreSearchResults()}
              disabled={isSearching}
            >
              {isSearching ? $_('common.loading') : $_('messageList.loadMore', { values: { remaining: searchTotalCount - searchResults.length } })}
            </button>
          </div>
        {/if}
      {/if}
    {:else if conversations.length === 0 && filterMode}
      <div class="flex flex-col items-center justify-center h-full text-muted-foreground">
        <Icon icon="mdi:filter-off-outline" class="w-12 h-12 mb-2" />
        <p>{$_('messageList.noFilteredMessages')}</p>
        <button
          class="mt-2 text-sm text-primary hover:underline"
          onclick={() => setFilter('')}
        >
          {$_('messageList.filterAll')}
        </button>
      </div>
    {:else if conversations.length === 0}
      <div class="flex flex-col items-center justify-center h-full text-muted-foreground">
        <Icon icon="mdi:inbox-outline" class="w-12 h-12 mb-2" />
        <p>{$_('messageList.noMessages')}</p>
        <button
          class="mt-2 text-sm text-primary hover:underline"
          onclick={syncFolder}
          disabled={syncing}
        >
          {$_('messageList.syncNow')}
        </button>
      </div>
    {:else if canUseInboxDisplay}
      <!-- A keyed branch makes a display-mode change rebuild the group layout
           instead of reusing a previous category card with stale children. -->
      {#key `${inboxDisplayMode}:${inboxCardPreferencesVersion}`}
      {#each inboxGroups() as group (group.id)}
        <section
          class="inbox-display-group"
          class:inbox-display-category={inboxDisplayMode === 'categories'}
          class:inbox-display-chronological={inboxDisplayMode === 'chronological'}
          data-category={group.category}
          aria-label={group.recipient ? `${group.label}: ${group.recipient}` : group.label}
        >
          {#if inboxDisplayMode === 'chronological'}
            <h3 class="inbox-chronological-heading">{group.label}</h3>
          {:else}
            <div class="inbox-category-header">
              <button class="flex min-w-0 flex-1 items-center gap-2 text-left" onclick={() => toggleInboxGroup(group.id)} aria-expanded={!collapsedInboxGroups.has(group.id)}>
                <span class="inbox-category-icon"><Icon icon={group.icon} class="h-4 w-4" /></span>
                <span class="min-w-0 flex-1">
                  <span class="block text-sm font-semibold text-foreground">{group.label}</span>
                  {#if group.recipient}<span class="mt-0.5 block truncate text-xs text-muted-foreground">{group.recipient}</span>{/if}
                </span>
              </button>
              <button type="button" class="inbox-category-toggle" title={$_('common.done')} aria-label={$_('common.done')} onclick={(event) => markInboxGroupDone(group, event)}>
                <Icon icon="mdi:check" class="h-4 w-4" />
              </button>
            </div>
          {/if}
          {#if inboxDisplayMode === 'chronological' || !collapsedInboxGroups.has(group.id)}
            <div class="inbox-category-items" class:inbox-chronological-items={inboxDisplayMode === 'chronological'}>
              {#each visibleInboxConversations(group) as conv (conv.threadId + '-' + ((conv as any).accountId || accountId || ''))}
                {@const convAccountId = (conv as any).accountId || accountId}
                {@const convFolderId = (conv as any).folderId || folderId}
                {@const conversationIndex = conversations.findIndex(item => item.threadId === conv.threadId && ((item as any).accountId || accountId) === convAccountId)}
                <ConversationRow
                  bind:this={rowRefs[conv.threadId]}
                  conversation={conv}
                  density={getMessageListDensity()}
                  selected={selectedThreadId === conv.threadId}
                  checked={checkedThreadIds.has(conv.threadId)}
                  accountId={isUnifiedView ? convAccountId : accountId!}
                  folderId={isUnifiedView ? convFolderId : folderId!}
                  {folderType}
                  {selectedMessageIds}
                  selectedIsStarred={!selectedHasUnstarred}
                  selectedIsRead={!selectedHasUnread}
                  onSelect={(e) => selectConversation(conv.threadId, conversationIndex, e)}
                  onCheck={(checked, e) => handleCheck(conv.threadId, checked, conversationIndex, e)}
                  onClearSelection={clearSelection}
                  onActionComplete={handleActionComplete}
                  {onReply}
                  onDelete={(ids) => requestDelete(ids)}
                />
              {/each}
              {#if !expandedInboxGroups.has(group.id) && getInboxCardVisibleCount(group.category) > 0 && group.conversations.length > getInboxCardVisibleCount(group.category)}
                <button
                  type="button"
                  class="inbox-category-show-all"
                  onclick={() => showAllInboxConversations(group.id)}
                >
                  {$_('inbox.showAll', { values: { count: group.conversations.length } })}
                </button>
              {/if}
            </div>
          {/if}
        </section>
      {/each}
      {/key}

      {#if conversations.length < totalCount}
        {#if loading}<div class="flex justify-center py-4 text-sm text-muted-foreground"><Icon icon="mdi:loading" class="mr-2 h-4 w-4 animate-spin" />{$_('common.loading')}</div>{/if}
      {/if}
    {:else}
      {#each conversations as conv, index (conv.threadId + '-' + (conv.accountId || accountId || ''))}
        {@const convAccountId = (conv as any).accountId || accountId}
        {@const convFolderId = (conv as any).folderId || folderId}
        <ConversationRow
          bind:this={rowRefs[conv.threadId]}
          conversation={conv}
          density={getMessageListDensity()}
          selected={selectedThreadId === conv.threadId}
          checked={checkedThreadIds.has(conv.threadId)}
          accountId={isUnifiedView ? convAccountId : accountId!}
          folderId={isUnifiedView ? convFolderId : folderId!}
          {folderType}
          {selectedMessageIds}
          selectedIsStarred={!selectedHasUnstarred}
          selectedIsRead={!selectedHasUnread}
          onSelect={(e) => selectConversation(conv.threadId, index, e)}
          onCheck={(checked, e) => handleCheck(conv.threadId, checked, index, e)}
          onClearSelection={clearSelection}
          onActionComplete={handleActionComplete}
          {onReply}
          onDelete={(ids) => requestDelete(ids)}
        />
      {/each}

      <!-- Load more button for pagination -->
      {#if conversations.length < totalCount}
        {#if loading}<div class="flex justify-center py-4 text-sm text-muted-foreground"><Icon icon="mdi:loading" class="mr-2 h-4 w-4 animate-spin" />{$_('common.loading')}</div>{/if}
      {/if}
    {/if}
    </div>
  </div>
</div>

<!-- Permanent Delete Confirmation Dialog -->
<ConfirmDialog
  bind:open={showDeleteConfirm}
  title={$_('dialog.deletePermanently')}
  description={$_('dialog.deleteDescription')}
  confirmLabel={$_('dialog.confirmDeletePermanently')}
  variant="destructive"
  onConfirm={handleConfirmPermanentDelete}
  onCancel={() => { showDeleteConfirm = false; pendingDeleteIds = [] }}
/>

<!-- Empty Trash Confirmation Dialog -->
<ConfirmDialog
  bind:open={showEmptyTrashConfirm}
  title={$_('dialog.emptyTrash')}
  description={$_('dialog.emptyTrashDescription')}
  confirmLabel={$_('dialog.confirmEmptyTrash')}
  variant="destructive"
  onConfirm={handleEmptyTrash}
  onCancel={() => { showEmptyTrashConfirm = false }}
/>
