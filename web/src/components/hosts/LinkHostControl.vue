<script setup lang="ts">
/**
 * LinkHostControl — link/unlink a monitor to a host.
 *
 * Frontend-only. Renders a USelect of hosts plus a link/unlink button. Linking
 * calls `linkMonitorToHost`, unlinking calls `unlinkMonitorFromHost`; a `changed`
 * event is emitted on success with the new host id (or null when unlinked).
 *
 * There is no client-side read/write gate in this SPA — the control is always
 * shown. A backend 403 (ForbiddenError) is caught and surfaced as a toast; it
 * never crashes the component.
 */
import { onMounted, ref } from 'vue'
import { useToast } from '@nuxt/ui/composables/useToast'
import { listHosts, linkMonitorToHost, unlinkMonitorFromHost } from '@/services/hostsService'
import { ForbiddenError } from '@/core/errors'
import type { Host } from '@/types'

interface Props {
  monitorId: string
  currentHostId: string | null
}
const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'changed', hostId: string | null): void
}>()

const toast = useToast()

const hosts = ref<Host[]>([])
const loadingHosts = ref(false)
const busy = ref(false)
/** The host id currently selected in the picker (defaults to the linked one). */
const selectedHostId = ref<string | null>(props.currentHostId)

async function loadHosts() {
  loadingHosts.value = true
  try {
    hosts.value = await listHosts()
  } catch {
    hosts.value = []
  } finally {
    loadingHosts.value = false
  }
}

onMounted(loadHosts)

function handleForbidden(e: unknown, fallback: string) {
  if (e instanceof ForbiddenError) {
    toast.add({
      title: 'Not allowed',
      description: 'You do not have permission to change the host link.',
      color: 'error',
    })
    return
  }
  toast.add({
    title: fallback,
    description: e instanceof Error ? e.message : String(e),
    color: 'error',
  })
}

async function link() {
  if (!selectedHostId.value || busy.value) return
  const target = selectedHostId.value
  busy.value = true
  try {
    await linkMonitorToHost(props.monitorId, target)
    emit('changed', target)
  } catch (e) {
    handleForbidden(e, 'Failed to link host')
  } finally {
    busy.value = false
  }
}

async function unlink() {
  if (busy.value) return
  busy.value = true
  try {
    await unlinkMonitorFromHost(props.monitorId)
    selectedHostId.value = null
    emit('changed', null)
  } catch (e) {
    handleForbidden(e, 'Failed to unlink host')
  } finally {
    busy.value = false
  }
}

defineExpose({ hosts, selectedHostId, link, unlink, loadHosts })
</script>

<template>
  <div class="flex items-center gap-2">
    <USelect
      v-model="selectedHostId"
      :items="hosts.map((h) => ({ label: h.name, value: h.id }))"
      :loading="loadingHosts"
      placeholder="Select a host"
      icon="i-lucide-server"
      class="min-w-56"
      data-testid="link-host-select"
    />

    <UButton
      color="primary"
      icon="i-lucide-link"
      :loading="busy"
      :disabled="!selectedHostId || selectedHostId === currentHostId"
      data-testid="link-host-btn"
      @click="link"
    >
      Link
    </UButton>

    <UButton
      v-if="currentHostId"
      color="neutral"
      variant="ghost"
      icon="i-lucide-unlink"
      :loading="busy"
      data-testid="unlink-host-btn"
      @click="unlink"
    >
      Unlink
    </UButton>
  </div>
</template>
