<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { _ } from '$lib/i18n'
  import {
    getInboxDisplayMode,
    getInboxCardGrouping,
    getInboxCardVisibleCount,
    initializeInboxDisplayPreferences,
    isInboxCardAccountVisible,
    setInboxDisplayMode,
    setInboxCardAccountVisible,
    setInboxCardGrouping,
    setInboxCardVisibleCount,
    type InboxCardID,
    type InboxDisplayMode,
    type PeopleGrouping,
  } from '$lib/stores/inboxDisplay.svelte'

  type CardDefinition = { id: InboxCardID; label: string; description: string; icon: string }

  const cards = $derived<CardDefinition[]>([
    { id: 'people', label: $_('inbox.people'), description: $_('inbox.peopleDescription'), icon: 'mdi:account-outline' },
    { id: 'notifications', label: $_('inbox.notifications'), description: $_('inbox.notificationsDescription'), icon: 'mdi:bell-outline' },
    { id: 'news', label: $_('inbox.news'), description: $_('inbox.newsDescription'), icon: 'mdi:newspaper-variant-outline' },
    { id: 'read', label: $_('inbox.read'), description: $_('inbox.readDescription'), icon: 'mdi:eye-outline' },
  ])

  let selectedCard = $state<InboxCardID | null>(null)
  let displayMode = $state<InboxDisplayMode>(getInboxDisplayMode())
  let grouping = $state<PeopleGrouping>('per-account')
  let visibleCount = $state(0)

  const currentCard = $derived(cards.find(card => card.id === selectedCard) ?? null)

  onMount(() => {
    initializeInboxDisplayPreferences()
    displayMode = getInboxDisplayMode()
    if (selectedCard) loadCard(selectedCard)
  })

  function openCard(cardID: InboxCardID) {
    selectedCard = cardID
    loadCard(cardID)
  }

  function loadCard(cardID: InboxCardID) {
    grouping = getInboxCardGrouping(cardID)
    visibleCount = getInboxCardVisibleCount(cardID)
  }

  function updateDisplayMode(mode: InboxDisplayMode) {
    displayMode = mode
    setInboxDisplayMode(mode)
  }

  function updateGrouping(event: Event) {
    if (!selectedCard) return
    grouping = (event.currentTarget as HTMLSelectElement).value as PeopleGrouping
    setInboxCardGrouping(selectedCard, grouping)
  }

  function updateVisibleCount(event: Event) {
    if (!selectedCard) return
    visibleCount = Number((event.currentTarget as HTMLSelectElement).value)
    setInboxCardVisibleCount(selectedCard, visibleCount)
  }

  function updateAccountVisibility(accountID: string, visible: boolean) {
    // The detail panel can be closed while a Switch finishes its event. Do not
    // let that short-lived state turn into a null preference access.
    if (!selectedCard) return
    setInboxCardAccountVisible(selectedCard, accountID, visible)
  }
</script>

<div class="mx-auto max-w-2xl px-1 pb-5">
  {#if !selectedCard}
    <header class="flex items-start gap-3 border-b border-border pb-5">
      <span class="inbox-settings-hero-icon"><Icon icon="mdi:inbox-outline" class="h-5 w-5" /></span>
      <div>
        <h2 class="text-lg font-semibold text-foreground">{$_('sidebar.inbox')}</h2>
        <p class="mt-1 text-sm text-muted-foreground">{$_('inbox.settingsDescription')}</p>
      </div>
    </header>

    <section class="mt-5" aria-labelledby="inbox-view-title">
      <div class="mb-3 flex items-center gap-2">
        <Icon icon="mdi:view-dashboard-outline" class="h-4 w-4 text-primary" />
        <h3 id="inbox-view-title" class="text-sm font-semibold text-foreground">{$_('inbox.displayTitle')}</h3>
      </div>
      <div class="grid grid-cols-3 gap-2">
        {#each [
          { id: 'priority', label: $_('inbox.priority'), icon: 'mdi:lightning-bolt-outline' },
          { id: 'categories', label: $_('inbox.categories'), icon: 'mdi:shape-outline' },
          { id: 'chronological', label: $_('inbox.chronological'), icon: 'mdi:format-list-bulleted' },
        ] as option (option.id)}
          <button type="button" class="inbox-view-option" class:inbox-view-option-active={displayMode === option.id} onclick={() => updateDisplayMode(option.id as InboxDisplayMode)}>
            <Icon icon={option.icon} class="h-5 w-5" />
            <span>{option.label}</span>
          </button>
        {/each}
      </div>
    </section>

    <section class="mt-7 border-t border-border pt-5" aria-labelledby="inbox-card-settings-title">
      <p id="inbox-card-settings-title" class="mb-3 text-sm text-muted-foreground">{$_('inbox.cardSettings')}</p>
      <div class="space-y-1">
        {#each cards as card (card.id)}
          <button type="button" class="inbox-card-link" data-card={card.id} onclick={() => openCard(card.id)}>
            <span class="inbox-settings-icon"><Icon icon={card.icon} class="h-5 w-5" /></span>
            <span class="min-w-0 flex-1 text-left">
              <span class="block text-sm font-semibold text-foreground">{card.label}</span>
              <span class="mt-0.5 block truncate text-xs text-muted-foreground">{card.description}</span>
            </span>
            <Icon icon="mdi:chevron-right" class="h-5 w-5 text-primary" />
          </button>
        {/each}
      </div>
    </section>
  {:else if currentCard}
    <header class="flex items-center gap-3 border-b border-border pb-5">
      <button type="button" class="inbox-settings-back" onclick={() => (selectedCard = null)} aria-label={$_('inbox.backToCardSettings')}>
        <Icon icon="mdi:arrow-left" class="h-5 w-5" />
      </button>
      <span class="inbox-settings-icon"><Icon icon={currentCard.icon} class="h-5 w-5" /></span>
      <div>
        <h2 class="text-lg font-semibold text-foreground">{currentCard.label}</h2>
        <p class="mt-0.5 text-sm text-muted-foreground">{currentCard.description}</p>
      </div>
    </header>

    <section class="inbox-card-detail" aria-label={`Configurações de ${currentCard.label}`}>
      <div class="inbox-setting-row">
        <div>
          <h3>{$_('inbox.grouping')}</h3>
          <p>{$_('inbox.groupingHelp')}</p>
        </div>
        <select value={grouping} onchange={updateGrouping} aria-label="Agrupamento de e-mails">
          <option value="unified">{$_('inbox.unified')}</option>
          <option value="per-account">{$_('inbox.perAccount')}</option>
        </select>
      </div>

      <div class="inbox-setting-row border-t border-border pt-5">
        <div>
          <h3>{$_('inbox.visibleEmails')}</h3>
          <p>{$_('inbox.visibleEmailsHelp')}</p>
        </div>
        <select value={String(visibleCount)} onchange={updateVisibleCount} aria-label="Quantidade de e-mails visíveis">
          <option value="0">{$_('inbox.unlimited')}</option>
          <option value="3">{$_('inbox.emailCount', { values: { count: 3 } })}</option>
          <option value="5">{$_('inbox.emailCount', { values: { count: 5 } })}</option>
          <option value="10">{$_('inbox.emailCount', { values: { count: 10 } })}</option>
          <option value="20">{$_('inbox.emailCount', { values: { count: 20 } })}</option>
        </select>
      </div>

      {#if accountStore.accounts.length > 0}
        <div class="mt-6 border-t border-border pt-5">
          <div class="mb-3 flex items-center gap-2">
            <Icon icon="mdi:account-multiple-outline" class="h-4 w-4 text-primary" />
            <h3 class="text-sm font-semibold text-foreground">{$_('inbox.showAccounts')}</h3>
          </div>
          <div class="space-y-1">
            {#each accountStore.accounts as item (item.account.id)}
              <div class="inbox-account-row">
                <span class="flex h-8 w-8 items-center justify-center rounded-full bg-primary/15 text-sm font-semibold text-primary">
                  {(item.account.name || item.account.email || '?').slice(0, 1).toUpperCase()}
                </span>
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-sm font-medium text-foreground">{item.account.name || item.account.email}</span>
                  {#if item.account.name}<span class="block truncate text-xs text-muted-foreground">{item.account.email}</span>{/if}
                </span>
                <Switch checked={isInboxCardAccountVisible(selectedCard ?? '', item.account.id)} onCheckedChange={(checked) => updateAccountVisibility(item.account.id, checked)} />
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </section>
  {/if}
</div>
