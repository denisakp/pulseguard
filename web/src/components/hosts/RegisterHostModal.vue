<script setup lang="ts">
/**
 * Register-host onboarding modal (spec 081). Collects the host name, calls
 * `registerHost`, then swaps the form for a one-time credential reveal.
 * Server-side ValidationError.fieldErrors are mapped back onto the form via
 * `formRef.setErrors` (UFormExample oracle pattern).
 */
import { computed, ref, watch } from 'vue'
import { ValidationError } from '@/core/errors'
import { registerHost } from '@/services/hostsService'
import { hostSchema, type HostInput } from '@/schemas/host'
import type { Host } from '@/types'
import HostCredentialReveal from './HostCredentialReveal.vue'

interface Props {
  open?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  open: false,
})

const emit = defineEmits<{
  registered: [host: Host]
  close: []
}>()

const isOpen = computed({
  get: () => props.open,
  set: (v) => {
    if (!v) emit('close')
  },
})

const formRef = ref<{ setErrors: (errs: Array<{ path: string; message: string }>) => void } | null>(
  null,
)
const state = ref<HostInput>({ name: '' })
const submitting = ref(false)

// After a successful register we hold the raw credential locally to reveal it
// once. Never persisted anywhere else.
const revealed = ref<{ credential: string; prefix: string } | null>(null)

// Reset transient state each time the modal is (re)opened.
watch(
  () => props.open,
  (open) => {
    if (open) {
      state.value = { name: '' }
      revealed.value = null
      submitting.value = false
    }
  },
)

async function onSubmit(payload: { data: HostInput }) {
  submitting.value = true
  try {
    const result = await registerHost({ name: payload.data.name })
    emit('registered', result.host)
    revealed.value = { credential: result.credential, prefix: result.prefix }
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

function onRevealClose() {
  revealed.value = null
  isOpen.value = false
}
</script>

<template>
  <UModal v-model:open="isOpen" title="Register host" :ui="{ content: 'sm:max-w-lg' }">
    <template #body>
      <HostCredentialReveal
        v-if="revealed"
        :credential="revealed.credential"
        :prefix="revealed.prefix"
        @close="onRevealClose"
      />

      <UForm
        v-else
        ref="formRef"
        :schema="hostSchema"
        :state="state"
        class="space-y-4"
        @submit="onSubmit"
      >
        <p class="text-sm text-muted">
          Give the host a name. After registering, you'll get a one-time credential to install the
          agent on the server.
        </p>

        <UFormField label="Host name" name="name">
          <UInput v-model="state.name" placeholder="web-1" autofocus />
        </UFormField>

        <div class="flex justify-end gap-2 pt-2">
          <UButton color="neutral" variant="ghost" @click="isOpen = false">Cancel</UButton>
          <UButton type="submit" color="primary" :loading="submitting">Register host</UButton>
        </div>
      </UForm>
    </template>
  </UModal>
</template>
