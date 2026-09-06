// Runes-based release update checker.
// Automatic checks are throttled to once every six hours. A lightweight
// 30-minute timer only asks whether the throttle has expired; it does not
// hit GitHub on every tick.

// @ts-ignore - generated Wails bindings
import {
  CheckForUpdates as CheckForUpdatesBackend,
  GetAutoCheckUpdates,
  SetAutoCheckUpdates as PersistAutoCheckUpdates,
  GetSkippedUpdateVersion,
  SetSkippedUpdateVersion,
  GetLastUpdateCheck,
  SetLastUpdateCheck,
} from '../../../wailsjs/go/app/App'

export interface UpdateInfo {
  currentVersion: string
  latestVersion: string
  available: boolean
  releaseUrl: string
  releaseName: string
  publishedAt: string
}

const AUTO_CHECK_INTERVAL_MS = 6 * 60 * 60 * 1000
const POLL_INTERVAL_MS = 30 * 60 * 1000
const STARTUP_DELAY_MS = 8_000

let initialized = false
let checking = $state(false)
let autoCheckUpdates = $state(true)
let lastChecked = $state('')
let skippedUpdateVersion = $state('')
let availableUpdate = $state<UpdateInfo | null>(null)
let sessionDismissedVersion = ''

function isValidUpdateInfo(value: unknown): value is UpdateInfo {
  if (!value || typeof value !== 'object') return false
  const info = value as Record<string, unknown>
  return (
    typeof info.currentVersion === 'string' &&
    typeof info.latestVersion === 'string' &&
    typeof info.available === 'boolean' &&
    typeof info.releaseUrl === 'string' &&
    typeof info.releaseName === 'string' &&
    typeof info.publishedAt === 'string'
  )
}

function automaticCheckDue(): boolean {
  if (!lastChecked) return true
  const timestamp = Date.parse(lastChecked)
  if (!Number.isFinite(timestamp)) return true
  return Date.now() - timestamp >= AUTO_CHECK_INTERVAL_MS
}

export function getCheckingForUpdates(): boolean {
  return checking
}

export function getAutoCheckUpdates(): boolean {
  return autoCheckUpdates
}

export function getLastUpdateCheck(): string {
  return lastChecked
}

export function getAvailableUpdate(): UpdateInfo | null {
  return availableUpdate
}

export async function initializeUpdateChecker(): Promise<void> {
  if (initialized) return
  initialized = true

  try {
    const [enabled, skipped, checkedAt] = await Promise.all([
      GetAutoCheckUpdates(),
      GetSkippedUpdateVersion(),
      GetLastUpdateCheck(),
    ])
    autoCheckUpdates = enabled
    skippedUpdateVersion = skipped || ''
    lastChecked = checkedAt || ''
  } catch (err) {
    console.debug('Failed to load update-check preferences:', err)
  }

  window.setTimeout(() => {
    void checkForUpdates(false)
  }, STARTUP_DELAY_MS)

  window.setInterval(() => {
    void checkForUpdates(false)
  }, POLL_INTERVAL_MS)
}

export async function checkForUpdates(manual = false): Promise<UpdateInfo | null> {
  if (checking) return availableUpdate
  if (!manual && (!autoCheckUpdates || !automaticCheckDue())) return null

  checking = true
  try {
    const raw = await CheckForUpdatesBackend()
    const parsed = JSON.parse(raw) as unknown
    if (!isValidUpdateInfo(parsed)) {
      throw new Error('Invalid update-check response')
    }

    const checkedAt = new Date().toISOString()
    lastChecked = checkedAt
    void SetLastUpdateCheck(checkedAt).catch((err: unknown) => {
      console.debug('Failed to persist update-check timestamp:', err)
    })

    if (!parsed.available) {
      availableUpdate = null
      return parsed
    }

    const suppressed =
      parsed.latestVersion === skippedUpdateVersion ||
      parsed.latestVersion === sessionDismissedVersion

    if (manual || !suppressed) {
      availableUpdate = parsed
    }

    return parsed
  } catch (err) {
    if (manual) throw err
    console.debug('Automatic update check failed:', err)
    return null
  } finally {
    checking = false
  }
}

export async function setAutoCheckUpdates(enabled: boolean): Promise<void> {
  autoCheckUpdates = enabled
  await PersistAutoCheckUpdates(enabled)
  if (enabled) {
    void checkForUpdates(false)
  }
}

export function dismissCurrentUpdate(): void {
  if (availableUpdate) {
    sessionDismissedVersion = availableUpdate.latestVersion
  }
  availableUpdate = null
}

export async function skipCurrentUpdate(): Promise<void> {
  if (!availableUpdate) return
  const version = availableUpdate.latestVersion
  skippedUpdateVersion = version
  await SetSkippedUpdateVersion(version)
  availableUpdate = null
}
