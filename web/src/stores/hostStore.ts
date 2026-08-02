import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { listHosts } from '@/services/hostsService'
import type { Host } from '@/types'

/**
 * Host fleet store. Holds the raw host list plus a lightweight
 * polling loop so the Hosts list stays fresh without manual refresh. Derived
 * fields (serviceCount, worstLoad, trouble-first sort) live in `useHosts`.
 */
export const useHostStore = defineStore('host', () => {
  const hosts = ref<Host[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  const count = computed(() => hosts.value.length)

  let timer: ReturnType<typeof setInterval> | null = null

  async function fetch() {
    loading.value = true
    error.value = null
    try {
      hosts.value = await listHosts()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load hosts'
    } finally {
      loading.value = false
    }
  }

  function startPolling(ms = 12000) {
    stopPolling()
    void fetch()
    timer = setInterval(() => {
      void fetch()
    }, ms)
  }

  function stopPolling() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  return { hosts, loading, error, count, fetch, startPolling, stopPolling }
})
