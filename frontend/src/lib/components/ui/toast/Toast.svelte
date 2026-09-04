<script lang="ts">
  import Icon from '@iconify/svelte'
  import type { Toast } from '$lib/stores/toast'

  interface Props {
    toast: Toast
  }

  let { toast }: Props = $props()

  const icons = {
    success: 'mdi:check-circle',
    error: 'mdi:alert-circle',
    info: 'mdi:information',
    warning: 'mdi:alert'
  }

  const colors = {
    success: 'text-[#64d93c]',
    error: 'text-[#ff6b6b]',
    info: 'text-[#3da7ff]',
    warning: 'text-[#f5bd47]'
  }

</script>

<div
  class="toast-card flex min-h-11 items-center gap-2.5 rounded-lg px-3.5 py-2 animate-slide-in"
  role="alert"
>
  <Icon icon={icons[toast.type]} class="h-[18px] w-[18px] flex-shrink-0 {colors[toast.type]}" />

  <p class="min-w-0 flex-1 truncate text-sm font-medium tracking-[-0.01em] text-white/85">{toast.message}</p>

  {#if toast.actions && toast.actions.length > 0}
    <div class="flex shrink-0 items-center border-l border-white/[0.12] pl-3">
      {#each toast.actions as action (action.label)}
        <button
          class="rounded px-0.5 py-0.5 text-xs font-semibold uppercase tracking-wide text-[#2296f3] transition-colors hover:text-[#55b4ff] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2296f3]/70"
          onclick={action.onClick}
        >
          {action.label}
        </button>
      {/each}
    </div>
  {/if}

</div>

<style>
  .toast-card {
    background: rgb(45 50 54 / 0.98);
    box-shadow: 0 12px 28px rgb(0 0 0 / 0.26), 0 1px 0 rgb(255 255 255 / 0.035) inset;
    backdrop-filter: blur(12px);
  }

  @keyframes slide-in {
    from {
      transform: translate3d(12px, 0, 0);
      opacity: 0;
    }
    to {
      transform: translate3d(0, 0, 0);
      opacity: 1;
    }
  }

  .animate-slide-in {
    animation: slide-in 0.2s ease-out;
  }

  @media (prefers-reduced-motion: reduce) {
    .animate-slide-in {
      animation: none;
    }
  }
</style>
