// Session cache for cosmetic sender-brand logos. The backend owns the durable
// 14-day cache; this store only prevents duplicate Wails calls while navigating.
//
// State semantics per domain:
//   missing — never consulted (no cache entry)
//   pending — a GetSenderLogos call is in flight for it
//   found   — cache holds a Logo
//   none    — consulted, no logo exists (negative cached as null)

// @ts-ignore - wailsjs bindings
import { GetSenderLogos } from '../../../wailsjs/go/app/App.js'

type Logo = { data: string; mediaType: string }
type LogoResult = Logo & { domain: string }

let cache = $state<Record<string, Logo | null>>({})
const inflight = new Set<string>()

export function domainFromEmail(email: string): string {
  const normalized = email.trim().toLowerCase()
  const at = normalized.lastIndexOf('@')
  return at > 0 && at < normalized.length - 1 ? normalized.slice(at + 1) : ''
}

async function ensure(domains: string[]): Promise<void> {
  const dedup = new Set<string>()
  const missing: string[] = []
  for (const raw of domains) {
    const domain = raw.trim().toLowerCase()
    if (!domain || dedup.has(domain)) continue
    dedup.add(domain)
    // Cached covers BOTH found logos and negative (null) entries — a domain
    // with no logo is never re-fetched within this session.
    if (domain in cache) {
      continue
    }
    if (inflight.has(domain)) {
      continue
    }
    missing.push(domain)
  }
  if (missing.length === 0) return

  for (const domain of missing) inflight.add(domain)
  try {
    const results: LogoResult[] = (await GetSenderLogos(missing)) || []
    const found = new Map<string, Logo>()
    for (const result of results) {
      if (result?.domain && result.data && result.mediaType) {
        found.set(result.domain.toLowerCase(), { data: result.data, mediaType: result.mediaType })
      }
    }
    // Seed EVERY requested domain: the found logo, or null as negative cache.
    const updates: Record<string, Logo | null> = {}
    for (const domain of missing) updates[domain] = found.get(domain) ?? null
    cache = { ...cache, ...updates }
  } catch (err) {
    // Sender logos are cosmetic. Keep the initials fallback and avoid noisy UI.
    // Errors are intentionally NOT negative-cached: a transient network failure
    // must not suppress the logo for the rest of the session.
    console.debug('Failed to fetch sender logos:', err)
  } finally {
    for (const domain of missing) inflight.delete(domain)
  }
}

function get(domain: string): Logo | undefined {
  // null (negative cache) collapses to undefined — rows only care whether a
  // logo exists; undefined and null both mean "use the initials fallback".
  return cache[domain.trim().toLowerCase()] ?? undefined
}

export const senderLogos = { ensure, get }
