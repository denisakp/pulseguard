<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'

import {
  resourceSchema,
  type ResourceInput,
  monitorTypes,
  httpMethods,
} from '@/schemas/resource.schema'
import { ValidationError } from '@/core/errors'
import * as resourceService from '@/services/resourceService'
import * as tagService from '@/services/tagService'
import HeadersEditor from './HeadersEditor.vue'
import type { Resource, Tag } from '@/types'

interface Props {
  resource?: Resource | null
  // Seed for "create from toolbox" — pre-fills type/target/name on a
  // NEW monitor without entering edit mode.
  initial?: { type?: string; target?: string; name?: string } | null
}

const props = withDefaults(defineProps<Props>(), { resource: null, initial: null })
const emit = defineEmits<{ submit: []; cancel: [] }>()

const formRef = ref<{
  setErrors: (errs: Array<{ path: string; message: string }>) => void
} | null>(null)

const submitting = ref(false)
const showAdvanced = ref(false)
const showTags = ref(false)
const tagInput = ref('')
const availableTags = ref<Tag[]>([])

function onTagsToggle(open: boolean) {
  if (open) void loadTags()
}

async function loadTags() {
  if (availableTags.value.length > 0) return
  try {
    availableTags.value = await tagService.fetchTags()
  } catch {
    availableTags.value = []
  }
}

function tagName(id: string): string {
  return availableTags.value.find((t) => t.id === id)?.name ?? id
}

async function addTagFromInput() {
  const name = tagInput.value.trim()
  if (!name) return
  const existing = availableTags.value.find((t) => t.name.toLowerCase() === name.toLowerCase())
  let tagId: string
  if (existing) {
    tagId = existing.id
  } else {
    const created = await tagService.createTag({ name })
    availableTags.value.push(created)
    tagId = created.id
  }
  const tags = ((state as unknown as { tags?: string[] }).tags ??= [])
  if (!tags.includes(tagId)) tags.push(tagId)
  tagInput.value = ''
}

function removeTag(id: string) {
  const tags = (state as unknown as { tags?: string[] }).tags
  if (tags) (state as unknown as { tags: string[] }).tags = tags.filter((t) => t !== id)
}

type FormState = Record<string, unknown> & { type: ResourceInput['type'] }

function targetToPerType(type: string, target: string | undefined): Record<string, unknown> {
  const t = target ?? ''
  switch (type) {
    case 'http':
    case 'keyword':
      return { url: t }
    case 'tcp':
    case 'protocol': {
      const idx = t.lastIndexOf(':')
      if (idx === -1) return { host: t }
      const port = Number(t.slice(idx + 1))
      return { host: t.slice(0, idx), port: Number.isFinite(port) ? port : undefined }
    }
    case 'dns':
    case 'icmp':
      return { host: t }
    default:
      return {}
  }
}

function perTypeToTarget(s: FormState): string {
  const r = s as unknown as { url?: string; host?: string; port?: number }
  switch (s.type) {
    case 'http':
    case 'keyword':
      return r.url ?? ''
    case 'tcp':
    case 'protocol':
      return r.port ? `${r.host ?? ''}:${r.port}` : (r.host ?? '')
    case 'dns':
    case 'icmp':
      return r.host ?? ''
    default:
      return ''
  }
}

// toIdList normalizes an API relation (array of {id} objects, or already-string
// IDs) into the string[] the form state and schema expect.
function toIdList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value
    .map((item) => (typeof item === 'string' ? item : ((item as { id?: string })?.id ?? '')))
    .filter(Boolean)
}

function initialState(): FormState {
  if (props.resource) {
    const r = props.resource as unknown as Record<string, unknown>
    return {
      ...r,
      // The API returns tags / notification_channels as objects, but the form
      // (and its schema) work with string IDs. Without this the schema fails and
      // UForm silently blocks the submit on edit.
      tags: toIdList(r.tags),
      notification_channels: toIdList(r.notification_channels),
      ...targetToPerType(String(r.type), r.target as string | undefined),
    } as unknown as FormState
  }
  const base: FormState = {
    type: 'http',
    name: '',
    interval: 60,
    url: '',
    method: 'GET',
    expected_status: 200,
    follow_redirects: true,
    headers: {},
    tags: [],
    notification_channels: [],
  }
  if (props.initial) {
    const seedType = props.initial.type || 'http'
    return {
      ...base,
      type: seedType as ResourceInput['type'],
      name: props.initial.name ?? base.name,
      ...targetToPerType(seedType, props.initial.target),
    } as FormState
  }
  return base
}

const state = reactive<FormState>(initialState())

watch(
  () => [props.resource, props.initial],
  () => {
    Object.assign(state, initialState())
  },
)

const typeOptions = monitorTypes.map((t) => ({ label: t.toUpperCase(), value: t }))
const methodOptions = httpMethods.map((m) => ({ label: m, value: m }))
const dnsRecordTypes = ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS']
const protocols = ['imap', 'smtp', 'pop3', 'ssh', 'mysql', 'postgres']

const allowedByType: Record<string, string[]> = {
  http: ['url', 'method', 'expected_status', 'follow_redirects', 'headers'],
  tcp: ['host', 'port'],
  dns: ['host', 'record_type'],
  icmp: ['host'],
  keyword: ['url', 'keyword', 'case_sensitive'],
  heartbeat: ['grace_seconds'],
  protocol: ['protocol', 'host', 'port'],
}
const baseKeys = [
  'type',
  'name',
  'interval',
  'confirmation_interval',
  'tags',
  'notification_channels',
]

function stripExtras() {
  const keep = new Set([...baseKeys, ...(allowedByType[state.type] ?? [])])
  for (const k of Object.keys(state)) {
    if (!keep.has(k)) delete state[k]
  }
}

watch(() => state.type, stripExtras)

const isEdit = computed(() => !!props.resource)

async function onSubmit() {
  stripExtras()
  const parsed = resourceSchema.safeParse(state as unknown as ResourceInput)
  if (!parsed.success) {
    const errors = parsed.error.issues.map((i) => ({
      path: i.path.join('.'),
      message: i.message,
    }))
    formRef.value?.setErrors(errors)
    return
  }

  // Backend uses a single canonical `target` field — serialize per-type fields into it.
  const payload = {
    ...(parsed.data as unknown as Record<string, unknown>),
    target: perTypeToTarget(state),
  }

  submitting.value = true
  try {
    if (isEdit.value && props.resource) {
      await resourceService.updateResource(
        props.resource.id,
        payload as unknown as Parameters<typeof resourceService.updateResource>[1],
      )
    } else {
      await resourceService.createResource(
        payload as unknown as Parameters<typeof resourceService.createResource>[0],
      )
    }
    emit('submit')
  } catch (e) {
    if (e instanceof ValidationError) {
      formRef.value?.setErrors(
        Object.entries(e.fieldErrors).map(([path, msgs]) => ({
          path,
          message: msgs[0] ?? 'Invalid',
        })),
      )
    } else {
      throw e
    }
  } finally {
    submitting.value = false
  }
}

defineExpose({ state, onSubmit, formRef, stripExtras })
</script>

<template>
  <UForm ref="formRef" :schema="resourceSchema" :state="state" class="space-y-4" @submit="onSubmit">
    <UFormField name="type" label="Type">
      <USelect v-model="state.type" :items="typeOptions" class="w-full" />
    </UFormField>

    <UFormField name="name" label="Name">
      <UInput
        v-model="(state as unknown as { name: string }).name"
        placeholder="api.acme.com"
        class="w-full"
      />
    </UFormField>

    <UFormField v-if="state.type === 'http' || state.type === 'keyword'" name="url" label="URL">
      <UInput
        v-model="(state as unknown as { url: string }).url"
        placeholder="https://api.acme.com/health"
        class="w-full"
      />
    </UFormField>

    <UFormField v-if="state.type === 'keyword'" name="keyword" label="Keyword">
      <UInput v-model="(state as unknown as { keyword: string }).keyword" class="w-full" />
    </UFormField>

    <UFormField
      v-if="['tcp', 'protocol', 'dns', 'icmp'].includes(state.type as string)"
      name="host"
      label="Host"
    >
      <UInput
        v-model="(state as unknown as { host: string }).host"
        placeholder="db.acme.com"
        class="w-full"
      />
    </UFormField>

    <UFormField v-if="state.type === 'tcp' || state.type === 'protocol'" name="port" label="Port">
      <UInput
        v-model.number="(state as unknown as { port: number }).port"
        type="number"
        :min="1"
        :max="65535"
        class="w-full"
      />
    </UFormField>

    <UFormField v-if="state.type === 'dns'" name="record_type" label="Record type">
      <USelect
        v-model="(state as unknown as { record_type: string }).record_type"
        :items="dnsRecordTypes"
        class="w-full"
      />
    </UFormField>

    <UFormField v-if="state.type === 'protocol'" name="protocol" label="Protocol">
      <USelect
        v-model="(state as unknown as { protocol: string }).protocol"
        :items="protocols"
        class="w-full"
      />
    </UFormField>

    <UFormField
      v-if="state.type === 'heartbeat'"
      name="grace_seconds"
      label="Grace period (seconds)"
    >
      <UInput
        v-model.number="(state as unknown as { grace_seconds: number }).grace_seconds"
        type="number"
        :min="30"
        :max="86400"
        class="w-full"
      />
    </UFormField>

    <UCollapsible v-model:open="showAdvanced">
      <UButton
        variant="ghost"
        color="neutral"
        size="xs"
        :trailing-icon="showAdvanced ? 'i-lucide-chevron-down' : 'i-lucide-chevron-right'"
        class="font-medium"
      >
        Advanced
      </UButton>
      <template #content>
        <div class="space-y-4 pt-2 pl-4 border-l border-default">
          <UFormField name="interval" label="Check interval (seconds)">
            <UInput
              v-model.number="(state as unknown as { interval: number }).interval"
              type="number"
              :min="30"
              :max="86400"
              class="w-full"
            />
          </UFormField>
          <template v-if="state.type === 'http'">
            <UFormField label="Method">
              <USelect
                v-model="(state as unknown as { method: string }).method"
                :items="methodOptions"
                class="w-full"
              />
            </UFormField>
            <UFormField label="Expected status">
              <UInput
                v-model.number="(state as unknown as { expected_status: number }).expected_status"
                type="number"
                :min="100"
                :max="599"
                class="w-full"
              />
            </UFormField>
            <UFormField label="Headers">
              <HeadersEditor
                v-model="(state as unknown as { headers: Record<string, string> }).headers"
              />
            </UFormField>
          </template>
        </div>
      </template>
    </UCollapsible>

    <UCollapsible v-model:open="showTags" @update:open="onTagsToggle">
      <UButton
        variant="ghost"
        color="neutral"
        size="xs"
        :trailing-icon="showTags ? 'i-lucide-chevron-down' : 'i-lucide-chevron-right'"
        class="font-medium"
      >
        Tags
      </UButton>
      <template #content>
        <div class="space-y-2 pt-2 pl-4 border-l border-default">
          <div class="flex flex-wrap gap-1.5">
            <UBadge
              v-for="id in (state as unknown as { tags?: string[] }).tags ?? []"
              :key="id"
              variant="subtle"
              color="neutral"
              size="md"
            >
              {{ tagName(id) }}
              <UButton
                variant="ghost"
                color="neutral"
                size="2xs"
                icon="i-lucide-x"
                :aria-label="`Remove tag ${tagName(id)}`"
                class="-mr-1"
                @click="removeTag(id)"
              />
            </UBadge>
          </div>
          <div class="flex items-center gap-2">
            <UInput
              v-model="tagInput"
              placeholder="Type a tag name and press Enter"
              size="sm"
              class="flex-1"
              @keydown.enter.prevent="addTagFromInput"
            />
            <UButton
              color="neutral"
              variant="outline"
              size="xs"
              icon="i-lucide-plus"
              @click="addTagFromInput"
            >
              Add
            </UButton>
          </div>
        </div>
      </template>
    </UCollapsible>

    <div class="flex justify-end gap-2 pt-4 border-t border-default">
      <UButton color="neutral" variant="ghost" @click="emit('cancel')">Cancel</UButton>
      <UButton type="submit" color="primary" :loading="submitting">
        {{ isEdit ? 'Save changes' : 'Create monitor' }}
      </UButton>
    </div>
  </UForm>
</template>
