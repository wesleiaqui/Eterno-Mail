// Session cache for cosmetic sender-brand logos. The backend owns the durable
// 14-day cache; this store only prevents duplicate Wails calls while navigating.

// @ts-ignore - wailsjs bindings
import { GetSenderLogos } from '../../../wailsjs/go/app/App.js'

type Logo = { data: string; mediaType: string }
type LogoResult = Logo & { domain: string }

let cache = $state<Record<string, Logo>>({})
const inflight = new Set<string>()

export function domainFromEmail(email: string): string {
  const normalized = email.trim().toLowerCase()
  const at = normalized.lastIndexOf('@')
  return at > 0 && at < normalized.length - 1 ? normalized.slice(at + 1) : ''
}

async function ensure(domains: string[]): Promise<void> {
  const missing: string[] = []
  const seen = new Set<string>()
  for (const raw of domains) {
    const domain = raw.trim().toLowerCase()
    if (!domain || seen.has(domain)) continue
    seen.add(domain)
    if (domain in cache || inflight.has(domain)) continue
    missing.push(domain)
  }
  if (missing.length === 0) return

  for (const domain of missing) inflight.add(domain)
  try {
    const results: LogoResult[] = (await GetSenderLogos(missing)) || []
    const updates: Record<string, Logo> = {}
    for (const result of results) {
      if (result?.domain && result.data && result.mediaType) {
        updates[result.domain.toLowerCase()] = { data: result.data, mediaType: result.mediaType }
      }
    }
    cache = { ...cache, ...updates }
  } catch (err) {
    // Sender logos are cosmetic. Keep the initials fallback and avoid noisy UI.
    console.debug('Failed to fetch sender logos:', err)
  } finally {
    for (const domain of missing) inflight.delete(domain)
  }
}

function get(domain: string): Logo | undefined {
  return cache[domain.trim().toLowerCase()]
}

export const senderLogos = { ensure, get }
