<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  // @ts-ignore - wailsjs path
  import { GetAppInfo, OpenURL } from '../../../../wailsjs/go/app/App.js'
  import logo from '../../../assets/images/logo-universal.png'
  import { _ } from '$lib/i18n'
  import {
    initializeUpdateChecker,
    checkForUpdates,
    getAutoCheckUpdates,
    setAutoCheckUpdates,
    getCheckingForUpdates,
    getLastUpdateCheck,
    getAvailableUpdate,
  } from '$lib/stores/updateChecker.svelte'

  interface AppInfo {
    name: string
    version: string
    description: string
    website: string
    license: string
  }

  type CheckState = 'idle' | 'up-to-date' | 'available' | 'error'

  let appInfo = $state<AppInfo | null>(null)
  let loading = $state(true)
  let checkState = $state<CheckState>('idle')

  const autoCheckEnabled = $derived(getAutoCheckUpdates())
  const checkingUpdates = $derived(getCheckingForUpdates())
  const lastChecked = $derived(getLastUpdateCheck())
  const availableUpdate = $derived(getAvailableUpdate())

  onMount(async () => {
    try {
      const [info] = await Promise.all([
        GetAppInfo(),
        initializeUpdateChecker(),
      ])
      appInfo = info
    } catch (err) {
      console.error('Failed to load app info:', err)
    } finally {
      loading = false
    }
  })

  const PRIVACY_URL = 'https://github.com/wesleiaqui/eternomail/blob/main/docs/PRIVACY.md'
  const TERMS_URL = 'https://github.com/wesleiaqui/eternomail/blob/main/docs/TERMS.md'

  function openExternal(url: string) {
    OpenURL(url).catch((err: unknown) => console.error('Failed to open URL:', err))
  }

  function openWebsite() {
    if (appInfo?.website) openExternal(appInfo.website)
  }

  function openPrivacyPolicy() {
    openExternal(PRIVACY_URL)
  }

  function openTermsOfService() {
    openExternal(TERMS_URL)
  }

  function openRelease() {
    if (availableUpdate?.releaseUrl) openExternal(availableUpdate.releaseUrl)
  }

  async function toggleAutomaticUpdates() {
    try {
      await setAutoCheckUpdates(!autoCheckEnabled)
    } catch (err) {
      console.error('Failed to save automatic update preference:', err)
    }
  }

  async function checkNow() {
    checkState = 'idle'
    try {
      const info = await checkForUpdates(true)
      checkState = info?.available ? 'available' : 'up-to-date'
    } catch (err) {
      console.error('Manual update check failed:', err)
      checkState = 'error'
    }
  }

  function formatLastChecked(value: string): string {
    if (!value) return $_('updates.neverChecked')
    const timestamp = Date.parse(value)
    if (!Number.isFinite(timestamp)) return $_('updates.neverChecked')
    return $_('updates.lastChecked', { values: { time: new Date(timestamp).toLocaleString() } })
  }
</script>

<div class="flex flex-col items-center justify-center space-y-6 py-6">
  {#if loading}
    <Icon icon="mdi:loading" class="h-8 w-8 animate-spin text-muted-foreground" />
  {:else if appInfo}
    <div class="flex flex-col items-center space-y-2">
      <img src={logo} alt="{appInfo.name} Logo" class="h-24 w-24" />
      <div class="space-y-1 text-center">
        <h2 class="text-2xl font-bold text-foreground">{appInfo.name}</h2>
        <p class="text-sm text-muted-foreground">
          {$_('settingsAbout.version', { values: { version: appInfo.version } })}
        </p>
      </div>
    </div>

    <p class="max-w-xs text-center text-sm text-muted-foreground">
      {appInfo.description}
    </p>

    <section class="w-full max-w-md rounded-2xl border border-border bg-card/60 p-4">
      <div class="flex items-start gap-3">
        <span class="flex h-9 w-9 flex-none items-center justify-center rounded-xl bg-primary/10 text-primary">
          <Icon icon="mdi:update" class="h-5 w-5" />
        </span>
        <div class="min-w-0 flex-1">
          <h3 class="text-sm font-semibold text-foreground">{$_('updates.sectionTitle')}</h3>
          <p class="mt-0.5 text-xs leading-4 text-muted-foreground">
            {$_('updates.sectionDescription')}
          </p>
        </div>
      </div>

      <div class="mt-4 flex items-center justify-between gap-4 rounded-xl bg-muted/35 px-3 py-3">
        <div class="min-w-0">
          <p class="text-sm font-medium text-foreground">{$_('updates.automatic')}</p>
          <p class="mt-0.5 text-xs leading-4 text-muted-foreground">{$_('updates.automaticHelp')}</p>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={autoCheckEnabled}
          class="relative h-6 w-11 flex-none rounded-full transition-colors {autoCheckEnabled ? 'bg-primary' : 'bg-muted-foreground/30'}"
          onclick={toggleAutomaticUpdates}
          title={$_('updates.automatic')}
        >
          <span class="pointer-events-none absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow-sm transition-transform duration-200 {autoCheckEnabled ? 'translate-x-5' : 'translate-x-0'}"></span>
        </button>
      </div>

      <div class="mt-3 flex items-center justify-between gap-3">
        <div class="min-w-0">
          {#if checkState === 'up-to-date'}
            <p class="text-xs font-medium text-foreground">{$_('updates.upToDate')}</p>
          {:else if checkState === 'available' && availableUpdate}
            <p class="text-xs font-medium text-primary">
              {$_('updates.availableInSettings', { values: { version: availableUpdate.latestVersion } })}
            </p>
          {:else if checkState === 'error'}
            <p class="text-xs font-medium text-destructive">{$_('updates.checkFailed')}</p>
          {:else if availableUpdate}
            <p class="text-xs font-medium text-primary">
              {$_('updates.availableInSettings', { values: { version: availableUpdate.latestVersion } })}
            </p>
          {:else}
            <p class="text-xs text-muted-foreground">{formatLastChecked(lastChecked)}</p>
          {/if}
          {#if checkState !== 'idle' && lastChecked}
            <p class="mt-0.5 text-[11px] text-muted-foreground">{formatLastChecked(lastChecked)}</p>
          {/if}
        </div>

        <Button variant="outline" size="sm" disabled={checkingUpdates} onclick={checkNow}>
          {#if checkingUpdates}
            <Icon icon="mdi:loading" class="mr-1.5 h-4 w-4 animate-spin" />
            {$_('updates.checking')}
          {:else}
            <Icon icon="mdi:refresh" class="mr-1.5 h-4 w-4" />
            {$_('updates.checkNow')}
          {/if}
        </Button>
      </div>

      {#if availableUpdate}
        <div class="mt-3 flex items-center justify-between gap-3 rounded-xl border border-primary/20 bg-primary/[0.06] px-3 py-2.5">
          <div>
            <p class="text-xs font-semibold text-foreground">{$_('updates.availableTitle')}</p>
            <p class="mt-0.5 text-[11px] tabular-nums text-muted-foreground">
              {$_('updates.versionTransition', {
                values: { current: availableUpdate.currentVersion, latest: availableUpdate.latestVersion },
              })}
            </p>
          </div>
          <Button size="sm" onclick={openRelease}>{$_('updates.viewRelease')}</Button>
        </div>
      {/if}
    </section>

    <div class="flex flex-col items-center gap-2">
      <button
        onclick={openWebsite}
        class="flex items-center gap-2 text-sm text-primary transition-colors hover:underline"
      >
        <Icon icon="mdi:github" class="h-5 w-5" />
        <span>{$_('settingsAbout.github')}</span>
      </button>
      <button
        onclick={openPrivacyPolicy}
        class="flex items-center gap-2 text-sm text-primary transition-colors hover:underline"
      >
        <Icon icon="mdi:shield-account" class="h-5 w-5" />
        <span>{$_('settingsAbout.privacyPolicy')}</span>
      </button>
      <button
        onclick={openTermsOfService}
        class="flex items-center gap-2 text-sm text-primary transition-colors hover:underline"
      >
        <Icon icon="mdi:file-document" class="h-5 w-5" />
        <span>{$_('settingsAbout.termsOfUse')}</span>
      </button>
    </div>
  {:else}
    <p class="text-muted-foreground">{$_('settingsAbout.failedToLoad')}</p>
  {/if}
</div>
