<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useSearchPalette } from '@/composables/useSearchPalette'

const router = useRouter()
const palette = useSearchPalette()

const inputRef = ref<HTMLInputElement | null>(null)

const sectionLabels: Record<'resource' | 'incident' | 'page', string> = {
  resource: 'RESOURCES',
  incident: 'INCIDENTS',
  page: 'PAGES',
}

const sectionIcons: Record<'resource' | 'incident' | 'page', string> = {
  resource: 'i-lucide-globe',
  incident: 'i-lucide-circle-alert',
  page: 'i-lucide-layout',
}

const sectionTints: Record<'resource' | 'incident' | 'page', string> = {
  resource: 'bg-primary/10 text-primary',
  incident: 'bg-error/10 text-error',
  page: 'bg-elevated text-muted',
}

const orderedSections = computed(() => {
  const out: Array<{
    key: 'resource' | 'incident' | 'page'
    label: string
    items: ReturnType<typeof palette.results.value.slice>
  }> = []
  for (const key of ['resource', 'incident', 'page'] as const) {
    const items = palette.groupedResults.value[key]
    if (items.length > 0) {
      out.push({ key, label: sectionLabels[key], items })
    }
  }
  return out
})

// ⌘K / open is owned by the global shortcuts registry (installKeyboardShortcuts).
// This local listener only handles the palette's own internal navigation while open.
function onKeydown(event: KeyboardEvent) {
  if (!palette.open.value) return

  if (event.key === 'Escape') {
    event.preventDefault()
    palette.setOpen(false)
    return
  }
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    palette.moveHighlight(1)
    return
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    palette.moveHighlight(-1)
    return
  }
  if (event.key === 'Enter') {
    event.preventDefault()
    palette.activate((to) => router.push(to))
  }
}

onMounted(() => {
  document.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
})

watch(
  () => palette.open.value,
  (now) => {
    if (now) {
      // Defer focus until the modal is in the DOM.
      requestAnimationFrame(() => inputRef.value?.focus())
    }
  },
)

const counterText = computed(() => {
  if (palette.searching.value) return 'Searching…'
  const total = palette.results.value.length
  return `${total} result${total === 1 ? '' : 's'} · ${palette.lastQueryDurationMs.value}ms`
})

function activateRow(index: number) {
  palette.highlightIndex.value = index
  palette.activate((to) => router.push(to))
}
</script>

<template>
  <UModal
    :open="palette.open.value"
    :ui="{ content: 'max-w-2xl' }"
    @update:open="palette.setOpen($event)"
  >
    <template #content>
      <div class="flex flex-col bg-default" role="dialog" aria-label="Search palette">
        <div class="flex items-center gap-3 px-4 py-3 border-b border-default">
          <UIcon name="i-lucide-search" class="size-4 text-muted" />
          <input
            ref="inputRef"
            v-model="palette.query.value"
            type="text"
            placeholder="Search resources, incidents, pages…"
            class="flex-1 bg-transparent outline-none text-sm text-default placeholder:text-muted"
            @input="palette.highlightIndex.value = 0"
          />
          <kbd
            class="px-1.5 py-0.5 text-[10px] font-medium rounded border border-default text-muted"
            >Esc</kbd
          >
        </div>

        <div class="max-h-96 overflow-y-auto py-2 bg-default">
          <div v-if="palette.loadingMore.value" class="px-4 py-3 text-xs text-muted">
            Loading…
          </div>
          <div v-else-if="palette.results.value.length === 0" class="px-4 py-8 text-center">
            <UIcon name="i-lucide-search-x" class="size-6 text-muted mx-auto mb-2" />
            <p class="text-sm text-muted">No results</p>
            <p class="text-xs text-muted mt-1">Try a different query.</p>
          </div>
          <template v-else>
            <div v-for="section in orderedSections" :key="section.key">
              <div
                class="px-4 pt-3 pb-1 text-[10px] font-semibold tracking-wider text-muted"
              >
                {{ section.label }}
              </div>
              <button
                v-for="item in section.items"
                :key="item.id"
                type="button"
                class="w-full flex items-center gap-3 px-4 py-2 text-left transition-colors"
                :class="
                  palette.results.value.indexOf(item) === palette.highlightIndex.value
                    ? 'bg-elevated'
                    : 'hover:bg-muted'
                "
                @click="activateRow(palette.results.value.indexOf(item))"
                @mouseenter="palette.highlightIndex.value = palette.results.value.indexOf(item)"
              >
                <span class="size-6 rounded flex items-center justify-center" :class="sectionTints[section.key]">
                  <UIcon :name="sectionIcons[section.key]" class="size-3.5" />
                </span>
                <span class="flex-1 min-w-0">
                  <span class="block text-sm text-default truncate">
                    {{ item.label }}
                  </span>
                  <span
                    v-if="item.meta"
                    class="block text-xs text-muted truncate"
                  >
                    {{ item.meta }}
                  </span>
                </span>
                <kbd
                  v-if="palette.results.value.indexOf(item) === palette.highlightIndex.value"
                  class="px-1.5 py-0.5 text-[10px] font-medium rounded border border-default text-muted"
                  >⏎</kbd
                >
              </button>
            </div>
          </template>
        </div>

        <div
          class="flex items-center justify-between px-4 py-2 border-t border-default text-[11px] text-muted bg-default"
        >
          <div class="flex items-center gap-3">
            <span class="flex items-center gap-1"
              ><kbd class="px-1 py-0.5 rounded border border-default">↑↓</kbd>
              navigate</span
            >
            <span class="flex items-center gap-1"
              ><kbd class="px-1 py-0.5 rounded border border-default">⏎</kbd>
              open</span
            >
            <span class="flex items-center gap-1"
              ><kbd class="px-1 py-0.5 rounded border border-default">⌘K</kbd>
              search</span
            >
          </div>
          <span data-testid="palette-counter">{{ counterText }}</span>
        </div>
      </div>
    </template>
  </UModal>
</template>
