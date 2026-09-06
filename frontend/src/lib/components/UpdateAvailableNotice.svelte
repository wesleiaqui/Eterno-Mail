<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import {
    getAvailableUpdate,
    dismissCurrentUpdate,
    skipCurrentUpdate,
  } from '$lib/stores/updateChecker.svelte'
  // @ts-ignore - generated Wails binding
  import { OpenURL } from '../../../wailsjs/go/app/App.js'

  interface Props {
    hidden?: boolean
  }

  let { hidden = false }: Props = $props()
  const update = $derived(getAvailableUpdate())

  function viewRelease() {
    if (!update?.releaseUrl) return
    OpenURL(update.releaseUrl).catch((err: unknown) => {
      console.error('Failed to open release URL:', err)
    })
  }

  function skipRelease() {
    void skipCurrentUpdate().catch((err: unknown) => {
      console.error('Failed to skip update version:', err)
    })
  }
</script>

{#if update && !hidden}
  <aside
    class="fixed top-16 right-4 z-[60] w-[min(360px,calc(100vw-2rem))] rounded-2xl border border-border/80 bg-card/95 p-4 text-card-foreground shadow-[0_18px_60px_rgba(0,0,0,0.32)] backdrop-blur-xl"
    aria-live="polite"
    aria-label={$_('updates.availableTitle')}
  >
    <div class="flex items-start gap-3">
      <span class="flex h-9 w-9 flex-none items-center justify-center rounded-xl bg-primary/10 text-primary">
        <Icon icon="mdi:update" class="h-5 w-5" />
      </span>

      <div class="min-w-0 flex-1">
        <div class="flex items-start justify-between gap-3">
          <div>
            <h3 class="text-sm font-semibold leading-5 text-foreground">
              {$_('updates.availableTitle')}
            </h3>
            <p class="mt-0.5 text-xs leading-4 text-muted-foreground">
              {$_('updates.availableDescription', { values: { version: update.latestVersion } })}
            </p>
          </div>
          <button
            class="-mr-1 -mt-1 rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            onclick={dismissCurrentUpdate}
            aria-label={$_('updates.later')}
            title={$_('updates.later')}
          >
            <Icon icon="mdi:close" class="h-4 w-4" />
          </button>
        </div>

        <div class="mt-2 inline-flex items-center rounded-full bg-muted/60 px-2 py-1 text-[11px] font-medium tabular-nums text-muted-foreground">
          {$_('updates.versionTransition', {
            values: { current: update.currentVersion, latest: update.latestVersion },
          })}
        </div>

        <div class="mt-3 flex flex-wrap items-center gap-2">
          <button
            class="rounded-lg bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground shadow-sm transition-opacity hover:opacity-90"
            onclick={viewRelease}
          >
            {$_('updates.viewRelease')}
          </button>
          <button
            class="rounded-lg px-2.5 py-1.5 text-xs font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            onclick={dismissCurrentUpdate}
          >
            {$_('updates.later')}
          </button>
          <button
            class="ml-auto px-1 py-1.5 text-[11px] text-muted-foreground/80 transition-colors hover:text-foreground"
            onclick={skipRelease}
          >
            {$_('updates.skipVersion')}
          </button>
        </div>
      </div>
    </div>
  </aside>
{/if}
