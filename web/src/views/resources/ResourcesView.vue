<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'

import { useResourceStore } from '@/stores/resourceStore'
import { useComponentStore } from '@/stores/componentStore'
import { useConfirm } from '@/composables/useConfirm'
import { useResourceFilters } from '@/composables/useResourceFilters'
import ComponentGroupHeader from '@/components/resources/ComponentGroupHeader.vue'
import ResourceListItem from '@/components/resources/ResourceListItem.vue'
import ResourceModal from '@/components/resources/ResourceModal.vue'
import GroupResourcesModal from '@/components/modals/GroupResourcesModal.vue'
import { bulkRemoveFromComponent } from '@/services/componentService'
import { exportManifest } from '@/services/resourceImportService'
import type { Resource } from '@/types'

const resourceStore = useResourceStore()
const componentStore = useComponentStore()
const router = useRouter()
const route = useRoute()

const filters = useResourceFilters()

const showModal = ref(false)
const editingResource = ref<Resource | null>(null)
// Seed for "create from Toolbox" CTAs: /resources?create=1&type=&target=
const createSeed = ref<{ type?: string; target?: string; name?: string } | null>(null)
const collapsedGroups = ref<Record<string, boolean>>({})

const selectedIds = ref<string[]>([])
const showGroupModal = ref(false)

const exporting = ref(false)
async function onExport() {
  exporting.value = true
  try {
    const yaml = await exportManifest()
    const blob = new Blob([yaml], { type: 'text/yaml' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'ogoune-monitors.yaml'
    a.click()
    URL.revokeObjectURL(url)
  } finally {
    exporting.value = false
  }
}

function toggleSelect(id: string) {
  const i = selectedIds.value.indexOf(id)
  if (i === -1) selectedIds.value.push(id)
  else selectedIds.value.splice(i, 1)
}

function clearSelection() {
  selectedIds.value = []
}

async function onBulkGroup() {
  showGroupModal.value = true
}

async function onBulkRemove() {
  if (selectedIds.value.length === 0) return
  await bulkRemoveFromComponent({ resource_ids: selectedIds.value })
  clearSelection()
  await resourceStore.loadResources()
}

async function onGroupSuccess() {
  clearSelection()
  await resourceStore.loadResources()
}

const filtered = computed<Resource[]>(() => {
  let out = resourceStore.resources
  if (filters.search.value) {
    const needle = filters.search.value.toLowerCase()
    out = out.filter((r) => r.name.toLowerCase().includes(needle))
  }
  if (filters.type.value.length) {
    out = out.filter((r) => filters.type.value.includes(r.type))
  }
  if (filters.status.value.length) {
    out = out.filter((r) => filters.status.value.includes(r.status))
  }
  if (filters.tag.value.length) {
    out = out.filter((r) =>
      (r.tags ?? []).some((t) =>
        filters.tag.value.includes((t as unknown as { id: string }).id ?? (t as unknown as string)),
      ),
    )
  }
  if (filters.component.value.length) {
    out = out.filter((r) => r.component_id && filters.component.value.includes(r.component_id))
  }
  return out
})

interface Group {
  key: string
  component: { id?: string; name?: string } | null
  resources: Resource[]
}

const groups = computed<Group[]>(() => {
  if (filters.view.value !== 'byComponent') return []
  const map = new Map<string | null, Resource[]>()
  for (const r of filtered.value) {
    const key = r.component_id ?? null
    if (!map.has(key)) map.set(key, [])
    map.get(key)!.push(r)
  }
  const out: Group[] = []
  for (const c of componentStore.components) {
    const list = map.get(c.id) ?? []
    if (list.length > 0) {
      out.push({ key: c.id, component: { id: c.id, name: c.name }, resources: list })
    }
  }
  const standalone = map.get(null) ?? []
  if (standalone.length > 0) {
    out.push({ key: '__standalone__', component: null, resources: standalone })
  }
  return out
})

function isCollapsed(key: string) {
  return collapsedGroups.value[key] === true
}

function setCollapsed(key: string, v: boolean) {
  collapsedGroups.value = { ...collapsedGroups.value, [key]: v }
}

function openCreate() {
  editingResource.value = null
  createSeed.value = null
  showModal.value = true
}

async function onAction(p: { kind: 'view' | 'edit' | 'pause' | 'delete'; resource: Resource }) {
  if (p.kind === 'view') {
    router.push({ name: 'ResourceDetail', params: { id: p.resource.id } })
  } else if (p.kind === 'edit') {
    editingResource.value = p.resource
    showModal.value = true
  } else if (p.kind === 'pause') {
    if (p.resource.status === 'paused') {
      await resourceStore.resumeMonitoring(p.resource.id)
    } else {
      await resourceStore.pauseMonitoring(p.resource.id)
    }
  } else if (p.kind === 'delete') {
    const ok = await useConfirm({
      kind: 'destructive',
      title: 'Delete monitor?',
      body: `${p.resource.name} will stop being checked immediately.`,
      ctaLabel: 'Delete',
    })
    if (ok) await resourceStore.removeResource(p.resource.id)
  }
}

async function onFormSubmit() {
  showModal.value = false
  await resourceStore.loadResources()
}

onMounted(async () => {
  await Promise.all([resourceStore.loadResources(), componentStore.loadComponents()])
  // Open the create modal pre-seeded when arriving from a Toolbox "Create monitor" CTA.
  if (route.query.create === '1') {
    createSeed.value = {
      type: typeof route.query.type === 'string' ? route.query.type : undefined,
      target: typeof route.query.target === 'string' ? route.query.target : undefined,
      name: typeof route.query.name === 'string' ? route.query.name : undefined,
    }
    editingResource.value = null
    showModal.value = true
    // Drop the query so a refresh / back doesn't reopen it.
    void router.replace({ query: {} })
  }
})

defineExpose({
  filters,
  filtered,
  groups,
  onAction,
  selectedIds,
  toggleSelect,
  clearSelection,
  onBulkGroup,
  onBulkRemove,
  onGroupSuccess,
})
</script>

<template>
  <div class="bg-default text-default min-h-full">
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-2xl font-semibold text-highlighted">Resources</h1>
        <p class="text-sm text-muted mt-1">Track uptime and performance of your resources</p>
      </div>
      <div class="flex items-center gap-2">
        <UButton
          color="neutral"
          variant="ghost"
          size="sm"
          icon="i-lucide-download"
          :loading="exporting"
          @click="onExport"
        >
          Export
        </UButton>
        <UButton
          color="neutral"
          variant="ghost"
          size="sm"
          icon="i-lucide-upload"
          @click="router.push({ name: 'ResourceImport' })"
        >
          Import
        </UButton>
        <UButton color="primary" size="sm" icon="i-lucide-plus" @click="openCreate">
          New monitor
        </UButton>
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-2 mb-3">
      <UInput
        v-model="filters.search.value"
        placeholder="Search by name"
        icon="i-lucide-search"
        size="sm"
        class="flex-1 min-w-[220px]"
      />
      <USelectMenu
        v-model="filters.type.value"
        :items="['http', 'tcp', 'dns', 'icmp', 'keyword', 'heartbeat', 'protocol']"
        placeholder="All types"
        multiple
        size="sm"
      />
      <USelectMenu
        v-model="filters.status.value"
        :items="['up', 'down', 'flapping', 'paused', 'pending']"
        placeholder="All statuses"
        multiple
        size="sm"
      />
      <USelectMenu
        v-model="filters.component.value"
        :items="componentStore.components.map((c) => ({ label: c.name, value: c.id }))"
        placeholder="All components"
        multiple
        size="sm"
      />
      <UTabs
        v-model="filters.view.value"
        :items="[
          { label: 'Flat', value: 'flat', icon: 'i-lucide-list' },
          { label: 'By Component', value: 'byComponent', icon: 'i-lucide-folder' },
          { label: 'By Tag', value: 'byTag', icon: 'i-lucide-tag' },
        ]"
        size="sm"
      />
    </div>

    <div
      v-if="selectedIds.length > 0"
      class="flex items-center gap-3 mb-3 px-4 py-2.5 bg-primary-50 border border-primary-200 rounded-md text-sm sticky top-0 z-10"
    >
      <span class="text-primary-900 font-medium"> {{ selectedIds.length }} selected </span>
      <div class="flex-1" />
      <UButton color="primary" size="xs" icon="i-lucide-folder-plus" @click="onBulkGroup">
        Group into component
      </UButton>
      <UButton
        color="neutral"
        variant="outline"
        size="xs"
        icon="i-lucide-folder-minus"
        @click="onBulkRemove"
      >
        Remove from component
      </UButton>
      <UButton color="neutral" variant="ghost" size="xs" @click="clearSelection"> Clear </UButton>
    </div>

    <div
      v-if="filters.chips.value.length || filters.search.value"
      class="flex flex-wrap items-center gap-2 mb-3"
    >
      <UFilterChip
        v-for="c in filters.chips.value"
        :key="`${c.kind}:${c.value}`"
        :kind="c.kind"
        :value="c.value"
        @remove="filters.removeChip(c)"
      />
      <button
        type="button"
        class="text-xs text-primary-600 underline ml-2"
        @click="filters.clear()"
      >
        Clear all
      </button>
    </div>

    <div class="bg-default rounded-lg border border-default overflow-hidden">
      <div
        class="grid grid-cols-[28px_1fr_80px_90px_90px_90px_140px_40px] gap-2 px-4 py-2.5 bg-muted text-xs font-medium text-muted border-b border-default"
      >
        <span />
        <span>Name</span>
        <span>Type</span>
        <span>Status</span>
        <span>Uptime 7d</span>
        <span>Incidents 30d</span>
        <span>Last check</span>
        <span />
      </div>

      <div v-if="resourceStore.loading" class="px-6 py-12 text-center text-sm text-muted">
        Loading…
      </div>
      <UEmpty
        v-else-if="filtered.length === 0 && resourceStore.resources.length === 0"
        icon="i-lucide-radar"
        title="No monitors yet"
        description="Add your first monitor to start tracking uptime and performance."
        :actions="[
          { label: 'New monitor', icon: 'i-lucide-plus', color: 'primary', onClick: openCreate },
        ]"
      />
      <UEmpty
        v-else-if="filtered.length === 0"
        icon="i-lucide-search"
        title="No resources match the current filters"
        description="Try removing a filter or clearing your search."
        :actions="[
          {
            label: 'Clear all',
            icon: 'i-lucide-x',
            variant: 'outline',
            color: 'neutral',
            onClick: filters.clear,
          },
        ]"
      />
      <template v-else-if="filters.view.value === 'flat' || groups.length === 0">
        <ResourceListItem
          v-for="r in filtered"
          :key="r.id"
          :resource="r"
          :selected="selectedIds.includes(r.id)"
          @action="onAction"
          @toggle-select="toggleSelect(r.id)"
        />
      </template>
      <template v-else>
        <div v-for="g in groups" :key="g.key">
          <ComponentGroupHeader
            :component="g.component"
            :resources="g.resources"
            :collapsed="isCollapsed(g.key)"
            @update:collapsed="(v) => setCollapsed(g.key, v)"
          />
          <template v-if="!isCollapsed(g.key)">
            <ResourceListItem
              v-for="r in g.resources"
              :key="r.id"
              :resource="r"
              :selected="selectedIds.includes(r.id)"
              @action="onAction"
              @toggle-select="toggleSelect(r.id)"
            />
          </template>
        </div>
      </template>
    </div>

    <ResourceModal
      v-model:open="showModal"
      :resource="editingResource"
      :initial="createSeed"
      @submit="onFormSubmit"
    />
    <GroupResourcesModal
      v-model:open="showGroupModal"
      :selected-ids="selectedIds"
      @success="onGroupSuccess"
    />
  </div>
</template>
