import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ui from '@nuxt/ui/vue-plugin'

import App from './App.vue'
import router from './router'
import './style.css'
import { loadRuntimeConfig } from '@/composables/useRuntimeConfig'
import { installErrorBoundary } from '@/plugins/errorBoundary'
import { installKeyboardShortcuts } from '@/composables/useKeyboardShortcuts'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ui)

// global error boundary. Installed before mount so it catches any
// error raised during the first render cycle.
installErrorBoundary(app, router)

// Global keyboard shortcut registry — owns ⌘K, ?, and chord nav (G O/R/I/S).
// Single document keydown listener shared across overlays.
installKeyboardShortcuts(router)

// Prefetch runtime config (SSL provider + edition) so first paint can render
// the correct UI wording. Failure falls back to safe defaults.
loadRuntimeConfig().finally(() => {
  app.mount('#app')
})
