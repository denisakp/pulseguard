<script setup lang="ts">
/**
 * Channel modal — create/edit a notification channel.
 * Spec 059 US3. Tabs by type, swap subcomponent, Send test inline.
 */
import { computed, ref, watch } from 'vue'
import {
  CHANNEL_TYPES,
  channelSchemaFor,
  emptyConfigForType,
  type ChannelType,
  type NotificationChannelInput,
} from '@/schemas/notification-channel.schema'
import ChannelFormEmail from './ChannelFormEmail.vue'
import ChannelFormSlack from './ChannelFormSlack.vue'
import ChannelFormWebhook from './ChannelFormWebhook.vue'
import { testChannel, type ChannelTestResult } from '@/services/notificationChannelService'

const formComponents: Record<ChannelType, unknown> = {
  smtp: ChannelFormEmail,
  slack: ChannelFormSlack,
  webhook: ChannelFormWebhook,
}

interface Props {
  open: boolean
  initial?: Partial<NotificationChannelInput> & { id?: string }
}
const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:open', v: boolean): void
  (e: 'submit', v: NotificationChannelInput): void
}>()

const type = ref<ChannelType>((props.initial?.type as ChannelType) ?? 'smtp')
const name = ref<string>(props.initial?.name ?? '')
const isDefault = ref<boolean>(Boolean(props.initial?.is_default))
const isActive = ref<boolean>(props.initial?.is_active !== false)
const config = ref<NotificationChannelInput['config']>(
  (props.initial?.config as NotificationChannelInput['config']) ?? emptyConfigForType(type.value),
)

const fieldError = ref<Record<string, string>>({})
const submitting = ref(false)
const testResult = ref<ChannelTestResult | null>(null)
const testing = ref(false)

// Internal flag — when set, suppress the type-watcher's config wipe so the
// initial-driven resync below is not clobbered by an interactive tab swap.
let resyncing = false

// Resync local refs when the parent passes a new `initial` (Edit/Create switch)
// or when the modal reopens. Without this, the refs only capture initial at first mount.
watch(
  () => [props.open, props.initial] as const,
  ([open]) => {
    if (!open) return
    resyncing = true
    type.value = (props.initial?.type as ChannelType) ?? 'smtp'
    name.value = props.initial?.name ?? ''
    isDefault.value = Boolean(props.initial?.is_default)
    isActive.value = props.initial?.is_active !== false
    config.value =
      (props.initial?.config as NotificationChannelInput['config']) ??
      emptyConfigForType(type.value)
    fieldError.value = {}
    testResult.value = null
    // release the guard after the type-watcher has had a chance to fire
    queueMicrotask(() => {
      resyncing = false
    })
  },
  { immediate: true, deep: true },
)

// Tab swap clears the form payload so previous-type config never leaks.
// Skipped while a resync is in flight (Edit/Create swap should keep its config).
watch(type, (next, prev) => {
  if (next === prev) return
  if (resyncing) return
  config.value = emptyConfigForType(next)
  fieldError.value = {}
  testResult.value = null
})

const formComponent = computed(() => formComponents[type.value])

function validate(): NotificationChannelInput | null {
  const candidate = {
    type: type.value,
    name: name.value,
    is_default: isDefault.value,
    is_active: isActive.value,
    config: config.value,
  } as NotificationChannelInput
  // On edit the schema relaxes secret fields — GET masks them, so the operator
  // sees them blank and may leave them blank to keep the stored value.
  const schema = channelSchemaFor(props.initial?.id ? 'edit' : 'create')
  const r = schema.safeParse(candidate)
  if (!r.success) {
    const errs: Record<string, string> = {}
    for (const issue of r.error.issues) {
      errs[issue.path.join('.')] = issue.message
    }
    fieldError.value = errs
    return null
  }
  fieldError.value = {}
  return r.data
}

async function onSubmit() {
  const payload = validate()
  if (!payload) return
  submitting.value = true
  try {
    emit('submit', payload)
  } finally {
    submitting.value = false
  }
}

async function onSendTest() {
  if (!props.initial?.id) {
    testResult.value = {
      delivered: false,
      error: 'Save the channel before testing.',
      latency_ms: 0,
    }
    return
  }
  testing.value = true
  testResult.value = null
  try {
    testResult.value = await testChannel(props.initial.id)
  } finally {
    testing.value = false
  }
}

function close() {
  emit('update:open', false)
}

defineExpose({
  type,
  name,
  config,
  fieldError,
  testResult,
  validate,
  onSubmit,
  onSendTest,
})
</script>

<template>
  <UModal
    :open="open"
    :title="initial?.id ? 'Edit notification channel' : 'New notification channel'"
    :description="
      initial?.id
        ? 'Update credentials or recipient.'
        : 'Pick a channel type and fill the destination details.'
    "
    :ui="{
      content: 'sm:max-w-2xl !bg-white dark:!bg-gray-900 !divide-y-0',
      header: '!border-b-0',
      body: '!bg-white dark:!bg-gray-900 !border-y-0',
      footer: '!border-t-0',
    }"
    @update:open="emit('update:open', $event)"
  >
    <template #body>
      <div class="space-y-5 bg-white dark:bg-gray-900 relative isolate">
        <UFormField label="Channel type">
          <UTabs
            v-model="type"
            variant="pill"
            size="sm"
            :items="
              CHANNEL_TYPES.map((t) => ({
                label: t.label,
                value: t.value,
                icon: t.icon,
                disabled: !!initial?.id && t.value !== initial?.type,
              }))
            "
            :content="false"
            class="w-full"
          />
          <p v-if="initial?.id" class="text-xs text-muted mt-1.5">
            Channel type is locked in edit mode. Create a new channel to use a different type.
          </p>
        </UFormField>

        <UFormField label="Name" required :error="fieldError['name']">
          <UInput v-model="name" placeholder="Ops mailbox" class="w-full" />
        </UFormField>

        <div class="rounded-md border border-default/60 bg-default p-4 space-y-3">
          <h3 class="text-xs font-semibold text-muted uppercase tracking-wide">
            Channel configuration
          </h3>
          <p v-if="initial?.id" class="text-xs text-muted">
            Secrets are hidden. Leave a secret field blank to keep the current value.
          </p>
          <component :is="formComponent" v-model="config" :field-errors="fieldError" />
        </div>

        <div class="flex flex-wrap items-center gap-x-6 gap-y-2 pt-1">
          <UCheckbox v-model="isDefault" label="Set as default" />
          <UCheckbox v-model="isActive" label="Active" />
        </div>

        <UAlert
          v-if="testResult"
          :color="testResult.delivered ? 'success' : 'error'"
          variant="subtle"
          :icon="testResult.delivered ? 'i-lucide-check-circle' : 'i-lucide-alert-triangle'"
          :title="
            testResult.delivered ? `Test delivered in ${testResult.latency_ms} ms` : 'Test failed'
          "
          :description="!testResult.delivered && testResult.error ? testResult.error : undefined"
        />
      </div>
    </template>

    <template #footer>
      <div class="flex justify-between w-full">
        <UButton variant="ghost" :loading="testing" :disabled="!initial?.id" @click="onSendTest">
          Send test
        </UButton>
        <div class="flex gap-2">
          <UButton variant="ghost" @click="close">Cancel</UButton>
          <UButton color="primary" :loading="submitting" @click="onSubmit">Save</UButton>
        </div>
      </div>
    </template>
  </UModal>
</template>
