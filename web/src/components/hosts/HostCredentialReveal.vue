<script setup lang="ts">
/**
 * One-time credential reveal. Shows the raw agent credential
 * exactly once, with a copy button and install instructions. The secret lives
 * ONLY in this component's local state — never in Pinia/localStorage — and is
 * wiped on close so it cannot be recovered after the operator dismisses it.
 */
import { ref } from 'vue'

interface Props {
  credential: string
  prefix: string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  close: []
}>()

// Local copy so we can clear it on close (props are read-only but the parent
// should also drop its reference — this guards the in-component view).
const secret = ref(props.credential)
const copied = ref(false)

async function copy() {
  try {
    await navigator.clipboard.writeText(secret.value)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {
    /* clipboard unavailable — operator can still select manually */
  }
}

function onClose() {
  secret.value = ''
  emit('close')
}
</script>

<template>
  <div class="space-y-4">
    <UAlert
      color="warning"
      icon="i-lucide-triangle-alert"
      title="Copy this credential now"
      description="It is shown only once. If you lose it, rotate the credential to get a new one."
    />

    <div>
      <p class="text-xs font-medium text-muted mb-1">Agent credential ({{ prefix }})</p>
      <div class="flex items-center gap-2">
        <code
          data-testid="host-credential"
          class="flex-1 truncate rounded-md bg-elevated px-3 py-2 font-mono text-sm"
        >
          {{ secret }}
        </code>
        <UButton
          icon="i-lucide-copy"
          color="neutral"
          variant="subtle"
          aria-label="Copy credential"
          @click="copy"
        >
          {{ copied ? 'Copied' : 'Copy' }}
        </UButton>
      </div>
    </div>

    <div class="rounded-md border border-default p-4 text-sm space-y-2">
      <p class="font-medium">Install the agent on your host</p>
      <ol class="list-decimal list-inside text-muted space-y-1">
        <li>
          Run the image (or a release binary):
          <code class="font-mono">docker run --pid=host --network=host …
            ghcr.io/denisakp/ogoune-agent:latest</code>
        </li>
        <li>
          Set <code class="font-mono">OGOUNE_BACKEND_URL</code> and the
          <code class="font-mono">OGOUNE_CREDENTIAL</code> above (env, or
          <code class="font-mono">/etc/ogoune/agent.cfg</code>).
        </li>
        <li>
          Or as a service:
          <code class="font-mono">sudo systemctl enable --now ogoune-agent</code>
        </li>
      </ol>
      <p class="text-xs text-muted">
        Full instructions: Self-host → Host agent in the documentation.
      </p>
    </div>

    <div class="flex justify-end">
      <UButton color="primary" @click="onClose">Done</UButton>
    </div>
  </div>
</template>
