import Fuse, { type IFuseOptions } from 'fuse.js'
import { computed, ref, watch, type ComputedRef } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { storeToRefs } from 'pinia'
import { useResourceStore } from '@/stores/resourceStore'
import { useIncidentStore } from '@/stores/incidentStore'
import { searchPalette as searchPaletteApi } from '@/services/searchService'
import type { SearchResult } from '@/types'

interface StaticPage {
  label: string
  meta?: string
  route: RouteLocationRaw
}

// Local nav targets. Mirrors the server-side `searchPages` list in
// internal/service/search_service.go — kept here for the empty-query "browse"
// state and the offline Fuse fallback (the backend owns them for real queries).
const STATIC_PAGES: StaticPage[] = [
  { label: 'Overview', meta: 'Dashboard', route: '/overview' },
  { label: 'Resources', meta: 'All monitors', route: '/resources' },
  { label: 'Incidents', meta: 'Incident list', route: '/incidents' },
  { label: 'Components', meta: 'Status grouping', route: '/components' },
  { label: 'Maintenance', meta: 'Planned windows', route: '/maintenance' },
  { label: 'Notifications', meta: 'Channels', route: '/notifications' },
  { label: 'Escalation', meta: 'Policies', route: '/escalation' },
  { label: 'API keys', meta: 'Settings', route: '/api-keys' },
  { label: 'Account', meta: 'Settings', route: '/settings/account' },
  { label: 'Sessions', meta: 'Settings', route: '/settings/sessions' },
]

const FUSE_OPTIONS: IFuseOptions<SearchResult> = {
  keys: [
    { name: 'label', weight: 3 },
    { name: 'meta', weight: 1 },
  ],
  threshold: 0.4,
  includeScore: true,
  ignoreLocation: true,
}

// Server search kicks in at 2 chars (matches the backend's min-length guard);
// shorter non-empty queries filter the in-memory corpus locally.
const SEARCH_MIN_LEN = 2
const DEBOUNCE_MS = 150
const BROWSE_LIMIT = 20
const RESULT_LIMIT = 30

const open = ref(false)
const query = ref('')
const highlightIndex = ref(0)
const loadingMore = ref(false)
const searching = ref(false)
const lastQueryDurationMs = ref(0)
// Results for queries >= SEARCH_MIN_LEN, populated by the debounced backend call
// (or the Fuse fallback when the endpoint is unreachable).
const remoteResults = ref<SearchResult[]>([])

// Singleton — palette state is global across the app.
let initialized = false
let debounceTimer: ReturnType<typeof setTimeout> | null = null
let requestSeq = 0
let stopQueryWatch: (() => void) | null = null

export function useSearchPalette() {
  const resourceStore = useResourceStore()
  const incidentStore = useIncidentStore()
  const { resources } = storeToRefs(resourceStore)
  const { incidents } = storeToRefs(incidentStore)

  // In-memory corpus: hydrated stores + static pages. Used for the empty-query
  // browse list, sub-2-char filtering, and the offline fallback.
  const corpus: ComputedRef<SearchResult[]> = computed(() => {
    const items: SearchResult[] = []
    for (const r of resources.value) {
      items.push({
        id: `resource:${r.id}`,
        category: 'resource',
        label: r.name,
        meta: r.target,
        route: `/resources/${r.id}`,
        score: 0,
      })
    }
    for (const i of incidents.value) {
      items.push({
        id: `incident:${i.id}`,
        category: 'incident',
        label: i.cause || i.reason || 'Incident',
        meta: i.resource?.name ?? i.resource_id,
        route: `/incidents/${i.id}`,
        score: 0,
      })
    }
    for (const p of STATIC_PAGES) {
      items.push({
        id: `page:${p.label}`,
        category: 'page',
        label: p.label,
        meta: p.meta,
        route: p.route,
        score: 0,
      })
    }
    return items
  })

  function localSearch(q: string): SearchResult[] {
    const fuse = new Fuse(corpus.value, FUSE_OPTIONS)
    return fuse.search(q, { limit: RESULT_LIMIT }).map((m) => ({ ...m.item, score: m.score ?? 0 }))
  }

  const results: ComputedRef<SearchResult[]> = computed(() => {
    const q = query.value.trim()
    if (q.length === 0) return corpus.value.slice(0, BROWSE_LIMIT)
    // Below the server minimum: filter the local corpus instantly (the backend
    // would reject the query anyway).
    if (q.length < SEARCH_MIN_LEN) return localSearch(q)
    return remoteResults.value
  })

  const groupedResults = computed(() => ({
    resource: results.value.filter((r) => r.category === 'resource'),
    incident: results.value.filter((r) => r.category === 'incident'),
    page: results.value.filter((r) => r.category === 'page'),
  }))

  // Fetch results for queries >= SEARCH_MIN_LEN. Falls back to a local Fuse pass
  // over the corpus if the backend is unreachable (resilience — spec 084 PRD).
  async function runRemoteQuery(q: string): Promise<void> {
    const mySeq = ++requestSeq
    searching.value = true
    const t0 = performance.now()
    try {
      const outcome = await searchPaletteApi(q, { limit: RESULT_LIMIT })
      if (mySeq !== requestSeq) return // a newer query superseded this one
      remoteResults.value = outcome.results
      lastQueryDurationMs.value = Math.max(1, Math.round(outcome.durationMs))
    } catch {
      if (mySeq !== requestSeq) return
      remoteResults.value = localSearch(q)
      lastQueryDurationMs.value = Math.max(1, Math.round(performance.now() - t0))
    } finally {
      if (mySeq === requestSeq) {
        searching.value = false
        highlightIndex.value = 0
      }
    }
  }

  if (!initialized) {
    initialized = true
    stopQueryWatch?.()
    stopQueryWatch = watch(query, (raw) => {
      const q = raw.trim()
      if (debounceTimer) {
        clearTimeout(debounceTimer)
        debounceTimer = null
      }
      if (q.length < SEARCH_MIN_LEN) {
        // Browse / local-filter mode: cancel any in-flight request and let the
        // `results` computed serve from the corpus.
        requestSeq++
        searching.value = false
        lastQueryDurationMs.value = 0
        return
      }
      debounceTimer = setTimeout(() => {
        debounceTimer = null
        void runRemoteQuery(q)
      }, DEBOUNCE_MS)
    })
  }

  async function hydrateIfEmpty() {
    if (resources.value.length === 0 && !loadingMore.value) {
      loadingMore.value = true
      try {
        await resourceStore.loadResources()
      } finally {
        loadingMore.value = false
      }
    }
  }

  function setOpen(v: boolean) {
    open.value = v
    if (v) {
      highlightIndex.value = 0
      void hydrateIfEmpty()
    } else {
      query.value = ''
    }
  }

  function toggle() {
    setOpen(!open.value)
  }

  function moveHighlight(delta: number) {
    const total = results.value.length
    if (total === 0) return
    highlightIndex.value = (highlightIndex.value + delta + total) % total
  }

  function activate(routerPush: (to: RouteLocationRaw) => unknown): boolean {
    const item = results.value[highlightIndex.value]
    if (!item) return false
    routerPush(item.route)
    setOpen(false)
    return true
  }

  return {
    open,
    query,
    highlightIndex,
    loadingMore,
    searching,
    lastQueryDurationMs,
    results,
    groupedResults,
    setOpen,
    toggle,
    moveHighlight,
    activate,
    hydrateIfEmpty,
  }
}

export type UseSearchPaletteReturn = ReturnType<typeof useSearchPalette>

// Test-only reset.
export function __resetSearchPaletteForTests(): void {
  stopQueryWatch?.()
  stopQueryWatch = null
  if (debounceTimer) {
    clearTimeout(debounceTimer)
    debounceTimer = null
  }
  requestSeq = 0
  open.value = false
  query.value = ''
  highlightIndex.value = 0
  loadingMore.value = false
  searching.value = false
  lastQueryDurationMs.value = 0
  remoteResults.value = []
  initialized = false
}
