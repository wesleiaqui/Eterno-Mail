// Reactive preferences for the smart inbox presentation. They are local UI
// choices, so keeping them in localStorage avoids adding a migration while
// still preserving them across restarts.

export type InboxDisplayMode = 'priority' | 'categories' | 'chronological'
export type PeopleGrouping = 'per-account' | 'unified'
export type InboxCardID = 'people' | 'notifications' | 'news' | 'read'

const inboxCardIDs = ['people', 'notifications', 'news', 'read'] as const

type InboxCardPreferences = {
  grouping: PeopleGrouping
  visibleCount: number // 0 means unlimited
  hiddenAccounts: string[]
}

const STORAGE_KEY = 'eterno-mail:inbox-display'
const PEOPLE_GROUPING_KEY = 'eterno-mail:inbox-people-grouping'
const PEOPLE_VISIBLE_KEY = 'eterno-mail:inbox-people-visible'
const PEOPLE_ACCOUNTS_KEY = 'eterno-mail:inbox-people-accounts'
const CARD_PREFERENCES_KEY = 'eterno-mail:inbox-card-preferences'

let initialized = false
let displayMode = $state<InboxDisplayMode>('categories')
let peopleGrouping = $state<PeopleGrouping>('per-account')
let peopleVisibleCount = $state(0)
let hiddenPeopleAccounts = $state<string[]>([])
let cardPreferences = $state<Record<InboxCardID, InboxCardPreferences>>({
  people: { grouping: 'per-account', visibleCount: 0, hiddenAccounts: [] },
  notifications: { grouping: 'unified', visibleCount: 0, hiddenAccounts: [] },
  news: { grouping: 'unified', visibleCount: 0, hiddenAccounts: [] },
  read: { grouping: 'unified', visibleCount: 0, hiddenAccounts: [] },
})

function save(key: string, value: string): void {
  if (typeof localStorage !== 'undefined') localStorage.setItem(key, value)
}

export function initializeInboxDisplayPreferences(): void {
  if (initialized || typeof localStorage === 'undefined') return
  initialized = true

  const mode = localStorage.getItem(STORAGE_KEY)
  if (mode === 'priority' || mode === 'categories' || mode === 'chronological') displayMode = mode

  const grouping = localStorage.getItem(PEOPLE_GROUPING_KEY)
  if (grouping === 'per-account' || grouping === 'unified') peopleGrouping = grouping

  const visible = Number(localStorage.getItem(PEOPLE_VISIBLE_KEY))
  if ([3, 5, 10, 20].includes(visible)) peopleVisibleCount = visible

  try {
    const hidden = JSON.parse(localStorage.getItem(PEOPLE_ACCOUNTS_KEY) || '[]')
    if (Array.isArray(hidden) && hidden.every(item => typeof item === 'string')) hiddenPeopleAccounts = hidden
  } catch {
    // Invalid local preferences should never block the inbox.
  }

  try {
    const stored = JSON.parse(localStorage.getItem(CARD_PREFERENCES_KEY) || 'null')
    if (stored && typeof stored === 'object') {
      for (const id of ['people', 'notifications', 'news', 'read'] as InboxCardID[]) {
        const value = stored[id]
        if (!value || typeof value !== 'object') continue
        const groupingValue = value.grouping === 'per-account' || value.grouping === 'unified' ? value.grouping : cardPreferences[id].grouping
        const visibleValue = [0, 3, 5, 10, 20].includes(value.visibleCount) ? value.visibleCount : cardPreferences[id].visibleCount
        const hiddenValue = Array.isArray(value.hiddenAccounts) && value.hiddenAccounts.every((item: unknown) => typeof item === 'string') ? value.hiddenAccounts : []
        cardPreferences[id] = { grouping: groupingValue, visibleCount: visibleValue, hiddenAccounts: hiddenValue }
      }
    } else {
      // Bring existing Pessoas preferences forward to the card-level format.
      cardPreferences.people = { grouping: peopleGrouping, visibleCount: peopleVisibleCount, hiddenAccounts: hiddenPeopleAccounts }
    }
  } catch {
    // Keep defaults when an old preference cannot be parsed.
  }
}

function saveCardPreferences(): void {
  save(CARD_PREFERENCES_KEY, JSON.stringify(cardPreferences))
  // The inbox and the settings panel are separate component trees. Notify the
  // inbox explicitly so a changed card preference is reflected immediately,
  // including after the panel has been opened from a dialog.
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event('eterno-mail:inbox-card-preferences-change'))
  }
}

function isInboxCardID(cardID: string): cardID is InboxCardID {
  return (inboxCardIDs as readonly string[]).includes(cardID)
}

function getCardPreferences(cardID: string): InboxCardPreferences | undefined {
  return isInboxCardID(cardID) ? cardPreferences[cardID] : undefined
}

export function getInboxCardGrouping(cardID: string): PeopleGrouping {
  return getCardPreferences(cardID)?.grouping ?? 'unified'
}

export function setInboxCardGrouping(cardID: InboxCardID, grouping: PeopleGrouping): void {
  const preferences = getCardPreferences(cardID)
  if (!preferences) return
  cardPreferences[cardID] = { ...preferences, grouping }
  saveCardPreferences()
}

export function getInboxCardVisibleCount(cardID: string): number {
  // Priority and chronological views use transient group IDs (for example
  // "priority" and "today"), not inbox-card IDs. Their rows are unlimited.
  return getCardPreferences(cardID)?.visibleCount ?? 0
}

export function setInboxCardVisibleCount(cardID: InboxCardID, count: number): void {
  if (![0, 3, 5, 10, 20].includes(count)) return
  const preferences = getCardPreferences(cardID)
  if (!preferences) return
  cardPreferences[cardID] = { ...preferences, visibleCount: count }
  saveCardPreferences()
}

export function isInboxCardAccountVisible(cardID: string, accountID: string): boolean {
  return !getCardPreferences(cardID)?.hiddenAccounts.includes(accountID)
}

export function setInboxCardAccountVisible(cardID: InboxCardID, accountID: string, visible: boolean): void {
  const preferences = getCardPreferences(cardID)
  if (!preferences) return
  const hiddenAccounts = visible
    ? preferences.hiddenAccounts.filter(id => id !== accountID)
    : [...new Set([...preferences.hiddenAccounts, accountID])]
  cardPreferences[cardID] = { ...preferences, hiddenAccounts }
  saveCardPreferences()
}

export function getInboxDisplayMode(): InboxDisplayMode {
  return displayMode
}

export function setInboxDisplayMode(mode: InboxDisplayMode): void {
  displayMode = mode
  save(STORAGE_KEY, mode)
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent<InboxDisplayMode>('eterno-mail:inbox-display-change', { detail: mode }))
  }
}

export function getPeopleGrouping(): PeopleGrouping {
  return peopleGrouping
}

export function setPeopleGrouping(grouping: PeopleGrouping): void {
  peopleGrouping = grouping
  save(PEOPLE_GROUPING_KEY, grouping)
}

export function getPeopleVisibleCount(): number {
  return peopleVisibleCount
}

export function setPeopleVisibleCount(count: number): void {
  if (![0, 3, 5, 10, 20].includes(count)) return
  peopleVisibleCount = count
  save(PEOPLE_VISIBLE_KEY, String(count))
}

export function isPeopleAccountVisible(accountID: string): boolean {
  return !hiddenPeopleAccounts.includes(accountID)
}

export function setPeopleAccountVisible(accountID: string, visible: boolean): void {
  hiddenPeopleAccounts = visible
    ? hiddenPeopleAccounts.filter(id => id !== accountID)
    : [...new Set([...hiddenPeopleAccounts, accountID])]
  save(PEOPLE_ACCOUNTS_KEY, JSON.stringify(hiddenPeopleAccounts))
}
