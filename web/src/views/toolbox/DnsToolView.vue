<script setup lang="ts">
/**
 * DNS Lookup tool.
 * Form (domain + record types + resolver) → results table + session history.
 */
import { reactive, ref, computed } from 'vue'
import { useToast } from '@nuxt/ui/composables/useToast'
import { dnsLookupSchema, dnsRecordTypes } from '@/schemas/toolbox-dns.schema'
import { dnsLookup } from '@/services/toolboxService'
import type { DnsLookupRequest, DnsRecord, DnsHistoryEntry, DnsRecordType, DnsResolver } from '@/types/toolbox'

const toast = useToast()

const state = reactive<{
  domain: string
  record_types: DnsRecordType[]
  resolver: DnsResolver
  custom_resolver: string
}>({
  domain: '',
  record_types: ['A'],
  resolver: 'cloudflare',
  custom_resolver: '',
})

const recordTypeItems = dnsRecordTypes.map((t) => ({ label: t, value: t }))
const resolverItems = [
  { label: '1.1.1.1 (Cloudflare)', value: 'cloudflare' },
  { label: '8.8.8.8 (Google)', value: 'google' },
  { label: '9.9.9.9 (Quad9)', value: 'quad9' },
  { label: 'Custom…', value: 'custom' },
]

const loading = ref(false)
const records = ref<DnsRecord[]>([])
const queryMs = ref<number | null>(null)
const resolverUsed = ref('')
const ran = ref(false)
const history = ref<DnsHistoryEntry[]>(loadHistory())
let controller: AbortController | null = null

const summary = computed(() =>
  queryMs.value === null ? '' : `${records.value.length} records · ${queryMs.value}ms`,
)

function loadHistory(): DnsHistoryEntry[] {
  try {
    const raw = sessionStorage.getItem('toolbox.dns.history')
    return raw ? (JSON.parse(raw) as DnsHistoryEntry[]) : []
  } catch {
    return []
  }
}

function pushHistory(req: DnsLookupRequest) {
  history.value = [{ request: req, at: Date.now() }, ...history.value].slice(0, 10)
  try {
    sessionStorage.setItem('toolbox.dns.history', JSON.stringify(history.value))
  } catch {
    /* sessionStorage may be unavailable; non-fatal */
  }
}

async function run() {
  const payload: DnsLookupRequest = {
    domain: state.domain.trim(),
    record_types: state.record_types,
    resolver: state.resolver,
    custom_resolver: state.resolver === 'custom' ? state.custom_resolver.trim() : undefined,
  }
  loading.value = true
  ran.value = true
  controller = new AbortController()
  try {
    const res = await dnsLookup(payload, controller.signal)
    records.value = res.records
    queryMs.value = res.query_ms
    resolverUsed.value = res.resolver_used
    pushHistory(payload)
  } catch (e) {
    if ((e as Error)?.name !== 'AbortError') {
      toast.add({ title: 'DNS lookup failed', description: (e as Error)?.message, color: 'error' })
    }
  } finally {
    loading.value = false
    controller = null
  }
}

function cancel() {
  controller?.abort()
  loading.value = false
}

function rerun(entry: DnsHistoryEntry) {
  state.domain = entry.request.domain
  state.record_types = entry.request.record_types
  state.resolver = entry.request.resolver
  state.custom_resolver = entry.request.custom_resolver ?? ''
  void run()
}

async function copy(value: string) {
  try {
    await navigator.clipboard.writeText(value)
    toast.add({ title: 'Copied', color: 'success' })
  } catch {
    toast.add({ title: 'Copy failed', color: 'error' })
  }
}
</script>

<template>
  <div class="flex flex-col lg:flex-row gap-6">
    <!-- Form -->
    <div class="w-full lg:w-[380px] shrink-0 flex flex-col gap-4">
      <UForm :schema="dnsLookupSchema" :state="state" class="flex flex-col gap-4" @submit="run">
        <UFormField label="Domain" name="domain">
          <UInput v-model="state.domain" placeholder="example.com" class="w-full" />
        </UFormField>

        <UFormField label="Record types" name="record_types">
          <USelect
            v-model="state.record_types"
            :items="recordTypeItems"
            multiple
            class="w-full"
          />
        </UFormField>

        <UFormField label="Resolver" name="resolver">
          <USelect v-model="state.resolver" :items="resolverItems" class="w-full" />
        </UFormField>

        <UFormField
          v-if="state.resolver === 'custom'"
          label="Custom resolver"
          name="custom_resolver"
        >
          <UInput v-model="state.custom_resolver" placeholder="192.0.2.1" class="w-full" />
        </UFormField>

        <div class="flex gap-2">
          <UButton type="submit" :loading="loading" icon="i-lucide-play">Run Lookup</UButton>
          <UButton v-if="loading" color="neutral" variant="subtle" @click="cancel">Cancel</UButton>
        </div>
      </UForm>

      <UAlert
        icon="i-lucide-info"
        color="info"
        variant="subtle"
        title="Monitor this continuously?"
        description="Save it as a DNS monitor to track changes over time."
      />

      <!-- History -->
      <div v-if="history.length" class="flex flex-col gap-2">
        <div class="text-xs font-medium text-muted uppercase tracking-wide">Recent lookups</div>
        <button
          v-for="(entry, i) in history"
          :key="i"
          class="flex items-center justify-between px-2 py-1.5 rounded-md text-sm text-muted hover:bg-elevated hover:text-default transition-colors text-left"
          @click="rerun(entry)"
        >
          <span class="font-mono truncate">{{ entry.request.domain }}</span>
          <span class="text-xs">{{ entry.request.record_types.join(', ') }}</span>
        </button>
      </div>
    </div>

    <!-- Results -->
    <div class="flex-1 min-w-0">
      <div v-if="ran && !loading" class="flex flex-col gap-3">
        <div class="flex items-center gap-2">
          <UBadge v-if="records.length" color="success" variant="subtle">{{ summary }}</UBadge>
          <UBadge v-else color="neutral" variant="subtle">No records found</UBadge>
          <span v-if="resolverUsed" class="text-xs text-muted font-mono">via {{ resolverUsed }}</span>
        </div>

        <div v-if="records.length" class="overflow-x-auto rounded-lg border border-default">
          <table class="w-full text-sm">
            <thead class="bg-elevated text-muted">
              <tr>
                <th class="text-left font-medium px-3 py-2">Type</th>
                <th class="text-left font-medium px-3 py-2">Value</th>
                <th class="text-left font-medium px-3 py-2">TTL</th>
                <th class="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(rec, i) in records" :key="i" class="border-t border-default">
                <td class="px-3 py-2"><UBadge variant="subtle" size="sm">{{ rec.type }}</UBadge></td>
                <td class="px-3 py-2 font-mono break-all">{{ rec.value }}</td>
                <td class="px-3 py-2 text-muted">{{ rec.ttl || '—' }}</td>
                <td class="px-3 py-2 text-right">
                  <UButton
                    icon="i-lucide-copy"
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    @click="copy(rec.value)"
                  />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-else-if="!ran" class="flex items-center justify-center h-40 text-muted text-sm">
        Enter a domain and run a lookup.
      </div>
    </div>
  </div>
</template>
