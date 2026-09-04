<script lang="ts">
  // Avatar circle — colored initials with consistent color hash from email.
  // Uses the SAME theme classes (.avatar-1 .. .avatar-14) defined in
  // frontend/src/themes/_utilities.css that the mail UI uses, so the palette
  // matches mail automatically (and stays matched even though the JS is
  // duplicated — see project_extension_sdk_pattern memory for rationale).

  interface Props {
    /** Email address used as the color-hash seed. */
    email: string
    /** Optional display name. If absent, initials derive from email. */
    name?: string
    /** Density preset. */
    density?: 'micro' | 'compact' | 'standard' | 'large'
    /** Override the density-derived pixel size (rare). */
    size?: number
    /** Inline base64-encoded photo bytes. When set with photoMediaType, the
     *  avatar renders as an <img> instead of initials. Falls back to initials
     *  on image load error. */
    photoData?: string
    /** Photo media type (e.g. "image/jpeg"). Required alongside photoData. */
    photoMediaType?: string
    /** Inline base64-encoded sender brand logo. Used only when there is no
     *  personal contact photo. */
    logoData?: string
    /** Brand logo media type. Required alongside logoData. */
    logoMediaType?: string
    /** Optional domain favicon used when no local contact photo is available. */
    faviconUrl?: string
  }

  const { email, name, density = 'standard', size, photoData, photoMediaType, logoData, logoMediaType, faviconUrl }: Props = $props()

  // Photo rendering state: true when we have data+media-type AND the img loaded
  // successfully. On error (broken base64, unsupported MIME, etc.), falls back
  // to initials.
  let photoFailed = $state(false)
  let logoFailed = $state(false)
  let faviconFailed = $state(false)
  const showPhoto = $derived(!!photoData && !!photoMediaType && !photoFailed)
  const showLogo = $derived(!!logoData && !!logoMediaType && !logoFailed && !showPhoto)
  const showFavicon = $derived(!!faviconUrl && !faviconFailed && !showPhoto && !showLogo)

  // DJB2-style hash. Bit-for-bit the same as mail's getAvatarColor() in
  // ConversationRow.svelte:172-180 so an extension's contact and a mail
  // sender with the same email render the same color.
  function colorClass(seed: string): string {
    let hash = 0
    for (let i = 0; i < seed.length; i++) {
      hash = seed.charCodeAt(i) + ((hash << 5) - hash)
    }
    return `avatar-${(Math.abs(hash) % 14) + 1}`
  }

  // Ported verbatim from mail's getInitials in ConversationRow.svelte:158-170.
  // Split-on-single-space (not whitespace regex), map to first char, join +
  // uppercase + slice(0,2). Kept identical so an extension's contact and a
  // mail sender with the same display name render the same letters.
  function initials(displayName: string | undefined, fallbackEmail: string): string {
    if (!displayName && !fallbackEmail) return '?'
    const name = displayName || fallbackEmail
    return name.trim().charAt(0).toUpperCase() || '?'
  }

  function handleFaviconLoad(event: Event) {
    const image = event.currentTarget as HTMLImageElement
    // Google's favicon endpoint returns a small generic globe when a domain
    // has no usable icon. Treat that response as a miss and use initials.
    if (image.naturalWidth <= 16 || image.naturalHeight <= 16) {
      faviconFailed = true
    }
  }

  // Density → pixel size table. Tuned to match mail UI's density visual weight.
  const DENSITY_SIZE: Record<NonNullable<Props['density']>, number> = {
    micro: 24,
    compact: 28,
    standard: 32,
    large: 40,
  }

  const px = $derived(size ?? DENSITY_SIZE[density])
  const fontPx = $derived(Math.round(px * 0.4))
  const cls = $derived(colorClass(email || ''))
  const text = $derived(initials(name, email))
</script>

<div
  class="rounded-full flex-shrink-0 inline-flex items-center justify-center font-medium overflow-hidden {showPhoto || showLogo ? '' : showFavicon ? 'bg-white' : cls}"
  style:width="{px}px"
  style:height="{px}px"
  style:font-size="{fontPx}px"
  aria-hidden="true"
>
  {#if showPhoto}
    <img
      src="data:{photoMediaType};base64,{photoData}"
      alt=""
      class="w-full h-full object-cover"
      onerror={() => { photoFailed = true }}
    />
  {:else if showLogo}
    <img
      src="data:{logoMediaType};base64,{logoData}"
      alt=""
      class="w-full h-full object-contain bg-white"
      onerror={() => { logoFailed = true }}
    />
  {:else if showFavicon}
    <img
      src={faviconUrl}
      alt=""
      class="w-full h-full object-contain"
      onload={handleFaviconLoad}
      onerror={() => { faviconFailed = true }}
    />
  {:else}
    {text}
  {/if}
</div>
