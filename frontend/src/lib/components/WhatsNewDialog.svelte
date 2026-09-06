<script lang="ts">
  // Per-version release announcement. Shown once after the user upgrades
  // to a new Eterno Mail version. The runtime version is supplied by App.svelte
  // from GetAppInfo(), whose source of truth is the repository VERSION file.
  // Only the explicit OK click records acknowledgement.
  import { Dialog as DialogPrimitive } from 'bits-ui'
  import { cn } from '$lib/utils'
  import { Button } from '$lib/components/ui/button'
  // @ts-ignore - wailsjs path
  import { OpenURL } from '../../../wailsjs/go/app/App.js'
  import { _ } from '$lib/i18n'

  interface Props {
    open: boolean
    version: string
    onAcknowledge: () => void
  }

  let { open = $bindable(false), version, onAcknowledge }: Props = $props()

  const CHANGELOG_URL = 'https://github.com/wesleiaqui/eternomail/blob/main/CHANGELOG.md'

  // Open external links via the backend OpenURL: on Linux it tries the OpenURI
  // portal first, so links work inside the Flatpak sandbox (where xdg-open —
  // and thus Wails' BrowserOpenURL — can't reach the host browser).
  function openExternal(url: string) {
    OpenURL(url).catch((err: unknown) => console.error('Failed to open URL:', err))
  }
</script>

<DialogPrimitive.Root bind:open>
  <DialogPrimitive.Portal>
    <DialogPrimitive.Overlay
      class="fixed inset-0 z-50 bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0"
    />
    <DialogPrimitive.Content
      class={cn(
        'fixed left-[50%] top-[50%] z-[51] grid w-full max-w-lg translate-x-[-50%] translate-y-[-50%] gap-6 border bg-background p-8 shadow-lg duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:slide-out-to-top-[48%] data-[state=open]:slide-in-from-left-1/2 data-[state=open]:slide-in-from-top-[48%] sm:rounded-lg'
      )}
    >
      <div class="flex flex-col space-y-1.5 text-center sm:text-left">
        <h2 class="text-lg font-semibold leading-none tracking-tight">
          What's new?
        </h2>
      </div>

      <div class="space-y-4 max-h-[60vh] overflow-y-auto text-sm">
        <p>🚀 Welcome to Eterno Mail v{version}!</p>

        <p>
          This version is installed and ready to use. Release-specific details
          are kept in the project change log so this dialog cannot drift from
          the actual runtime version.
        </p>

        <p>🏷 See the full change log here:</p>

        <p>
          <button
            type="button"
            class="text-primary hover:underline break-all focus:outline-none focus-visible:outline-none focus:ring-0"
            onclick={() => openExternal(CHANGELOG_URL)}
          >
            https://github.com/wesleiaqui/eternomail/blob/main/CHANGELOG.md
          </button>
        </p>
      </div>

      <div class="flex flex-col-reverse sm:flex-row sm:justify-end sm:space-x-2">
        <Button onclick={onAcknowledge}>{$_('common.ok')}</Button>
      </div>
    </DialogPrimitive.Content>
  </DialogPrimitive.Portal>
</DialogPrimitive.Root>
