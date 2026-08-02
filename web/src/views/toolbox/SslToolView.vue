<script setup lang="ts">
/**
 * SSL Checker tool.
 * Inspects a TLS certificate, warns when it expires within 14 days, lists
 * passive vulnerability indicators, and offers "Add as monitor".
 */
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '@nuxt/ui/composables/useToast'
import { sslCheckSchema } from '@/schemas/toolbox-ssl.schema'
import { sslCheck } from '@/services/toolboxService'
import type { SslCheckResponse } from '@/types/toolbox'

const toast = useToast()
const router = useRouter()

const state = reactive<{ domain: string; port: number }>({ domain: '', port: 443 })

const loading = ref(false)
const result = ref<SslCheckResponse | null>(null)
let controller: AbortController | null = null

async function run() {
  loading.value = true
  controller = new AbortController()
  try {
    result.value = await sslCheck({ domain: state.domain.trim(), port: state.port }, controller.signal)
  } catch (e) {
    if ((e as Error)?.name !== 'AbortError') {
      result.value = null
      toast.add({ title: 'SSL check failed', description: (e as Error)?.message, color: 'error' })
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

function addAsMonitor() {
  router.push({
    path: '/resources',
    query: { create: '1', type: 'http', target: `https://${state.domain.trim()}` },
  })
}

function vulnColor(status: string): 'success' | 'warning' {
  return status === 'pass' ? 'success' : 'warning'
}
</script>

<template>
  <div class="flex flex-col lg:flex-row gap-6">
    <div class="w-full lg:w-[380px] shrink-0 flex flex-col gap-4">
      <UForm :schema="sslCheckSchema" :state="state" class="flex flex-col gap-4" @submit="run">
        <UFormField label="Domain" name="domain">
          <UInput v-model="state.domain" placeholder="example.com" class="w-full" />
        </UFormField>
        <UFormField label="Port" name="port">
          <UInput v-model.number="state.port" type="number" :min="1" :max="65535" class="w-full" />
        </UFormField>
        <div class="flex gap-2">
          <UButton type="submit" :loading="loading" icon="i-lucide-play">Run</UButton>
          <UButton v-if="loading" color="neutral" variant="subtle" @click="cancel">Cancel</UButton>
        </div>
      </UForm>
    </div>

    <div class="flex-1 min-w-0">
      <div v-if="result" class="flex flex-col gap-4">
        <UAlert
          v-if="result.expiring_soon"
          icon="i-lucide-alert-triangle"
          color="warning"
          variant="subtle"
          :title="`Certificate expires in ${result.days_to_expiry} days`"
          :description="`Renew before ${result.certificate.valid_to}`"
        >
          <template #actions>
            <UButton size="xs" color="warning" @click="addAsMonitor">Add as monitor</UButton>
          </template>
        </UAlert>

        <!-- Certificate details -->
        <div class="rounded-lg border border-default p-4 flex flex-col gap-2 text-sm">
          <div class="font-medium text-highlighted">Certificate</div>
          <dl class="grid grid-cols-[120px_1fr] gap-y-1">
            <dt class="text-muted">Subject</dt><dd class="font-mono break-all">{{ result.certificate.subject }}</dd>
            <dt class="text-muted">Issuer</dt><dd>{{ result.certificate.issuer }}</dd>
            <dt class="text-muted">Valid from</dt><dd>{{ result.certificate.valid_from }}</dd>
            <dt class="text-muted">Valid to</dt><dd>{{ result.certificate.valid_to }}</dd>
            <dt class="text-muted">Cipher</dt><dd class="font-mono">{{ result.certificate.cipher }}</dd>
            <dt class="text-muted">SANs</dt><dd class="font-mono break-all">{{ result.certificate.sans.join(', ') || '—' }}</dd>
            <dt class="text-muted">Chain</dt><dd class="font-mono break-all">{{ result.certificate.chain.join(' → ') || '—' }}</dd>
          </dl>
        </div>

        <!-- Vulnerability checks -->
        <div class="rounded-lg border border-default p-4 flex flex-col gap-2 text-sm">
          <div class="font-medium text-highlighted">Vulnerability checks</div>
          <div class="flex flex-wrap gap-2">
            <div v-for="v in result.vulnerabilities" :key="v.name" class="flex items-center gap-2">
              <UBadge :color="vulnColor(v.status)" variant="subtle" size="sm">
                {{ v.name }}: {{ v.status }}
              </UBadge>
            </div>
          </div>
        </div>
      </div>
      <div v-else-if="!loading" class="flex items-center justify-center h-40 text-muted text-sm">
        Enter a domain to inspect its certificate.
      </div>
    </div>
  </div>
</template>
