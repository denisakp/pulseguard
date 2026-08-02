/**
 * Cross-cutting EE upsell hygiene.
 *
 * Verifies the three CE-facing affordances that surface EE-only features:
 *   1. /reports — EE upsell banner + Upgrade CTA wires to /settings/account?tab=plan
 *   2. Wizard Step 3 — Team & Public visibility cards render disabled with EE badge
 *      + "Available on Enterprise" tooltip; clicks do not navigate or change state.
 *   3. /dashboards — Shared filter empty state matches FR-013 copy.
 *
 * No affordance must trigger navigation when activated by a CE user.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import type { ReportHistoryEntry } from '@/types'

// --- shared router stub ---------------------------------------------------
const pushMock = vi.fn()
const replaceMock = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock, replace: replaceMock }),
  useRoute: () => ({ query: {}, params: {}, path: '/', name: '_' }),
  useLink: () => ({ href: { value: '#' }, navigate: vi.fn(), isActive: { value: false } }),
  RouterLink: { template: '<a><slot /></a>' },
}))

// --- /reports mocks -------------------------------------------------------
const monthlyRef = await vi.hoisted(async () => (await import('vue')).ref<unknown>(null))
const historyRef = await vi.hoisted(
  async () => (await import('vue')).ref<ReportHistoryEntry[]>([]),
)
const latestDeliveredRef = await vi.hoisted(async () =>
  (await import('vue')).computed(() => historyRef.value.find((h) => h.status === 'delivered') ?? null),
)
vi.mock('@/composables/useReports', () => ({
  useReports: () => ({
    monthly: monthlyRef,
    history: historyRef,
    latestDelivered: latestDeliveredRef,
    loadAll: vi.fn(async () => {}),
    toggleMonthly: vi.fn(),
  }),
}))

// --- wizard mocks ---------------------------------------------------------
vi.mock('@/stores/authStore', () => ({
  useAuthStore: () => ({ userId: 'user-default', email: 'me@example.com' }),
}))
vi.mock('@/composables/useDashboards', () => {
  const filteredSorted = { value: [] as unknown[] }
  return {
    useDashboards: () => ({
      dashboards: { value: [] },
      filteredSorted,
      filter: { value: 'shared' },
      sort: { value: 'updated' },
      load: vi.fn(async () => {}),
      setFilter: vi.fn(),
      setSort: vi.fn(),
      create: vi.fn(),
      isStarred: () => false,
      toggleStar: vi.fn(),
    }),
  }
})
vi.mock('@/composables/useConfirm', () => ({ useConfirm: vi.fn().mockResolvedValue(true) }))
vi.mock('@/widgets/widgetCatalog', () => ({
  listWidgets: () => [
    {
      id: 'uptime-stat',
      name: 'Uptime',
      icon: 'i-lucide-trending-up',
      archetype: 'stat',
      defaultConfig: {},
      component: () => Promise.resolve({ default: {} }),
    },
  ],
}))

import ReportsView from '@/views/reports/ReportsView.vue'
import DashboardWizardModal from '@/components/dashboards/DashboardWizardModal.vue'
import DashboardsView from '@/views/dashboards/DashboardsView.vue'

const sharedStubs = {
  UIcon: { template: '<span />', props: ['name'] },
  UCard: { template: '<div><slot /></div>', props: ['ui'] },
  UBadge: { template: '<span><slot /></span>', props: ['color', 'variant', 'size'] },
  USwitch: {
    template: '<button :disabled="disabled"><slot /></button>',
    props: ['modelValue', 'disabled'],
    emits: ['update:modelValue'],
  },
  UAlert: {
    template: '<div :data-testid="dataTestid"><slot /><slot name="actions" /></div>',
    props: ['color', 'variant', 'icon', 'title', 'description'],
    computed: {
      dataTestid(this: { $attrs: Record<string, unknown> }) {
        return (this.$attrs['data-testid'] as string) ?? ''
      },
    },
  },
  UButton: {
    template: '<a :href="to" v-bind="$attrs" @click="$emit(\'click\', $event)"><slot /></a>',
    props: ['color', 'variant', 'size', 'icon', 'to', 'disabled', 'loading'],
    inheritAttrs: false,
    emits: ['click'],
  },
  UEditionBadge: {
    template: '<span data-testid="edition-badge" :data-edition="edition">{{ edition }}</span>',
    props: ['edition', 'size'],
  },
  UTooltip: { template: '<div><slot /></div>', props: ['text'] },
  UModal: {
    template: '<div v-if="open" data-testid="modal-stub"><slot name="content" /><slot /></div>',
    props: ['open', 'ui'],
    emits: ['update:open'],
  },
  DashboardScopeResolver: {
    template:
      '<button data-testid="scope-resolver-trigger" type="button" @click="emitMatch">Trigger</button>',
    props: ['modelValue'],
    emits: ['update:modelValue', 'update:matchCount'],
    methods: {
      emitMatch(this: { $emit: (e: string, v: unknown) => void }) {
        this.$emit('update:modelValue', { mode: 'tag', payload: { tagIds: ['t1'] } })
        this.$emit('update:matchCount', 3)
      },
    },
  },
  DashboardCard: {
    template: '<button :data-testid="`dashboard-card-${dashboard.id}`" />',
    props: ['dashboard', 'health'],
  },
  DashboardWizardModal: {
    template: '<div v-if="open" data-testid="wizard-mount" />',
    props: ['open'],
    emits: ['update:open'],
  },
}

beforeEach(() => {
  pushMock.mockClear()
  replaceMock.mockClear()
  document.body.innerHTML = ''
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('EE upsell hygiene — Reports page banner (FR US5 / surface 1)', () => {
  beforeEach(() => {
    monthlyRef.value = {
      enabled: false,
      recipientEmail: 'admin@example.com',
      schedule: 'monthly-1st',
      scope: 'all-resources',
      lastSentAt: null,
    }
    historyRef.value = []
  })

  it('renders the upsell banner and Upgrade CTA pointing at /settings/account?tab=plan', async () => {
    const wrapper = mount(ReportsView, { global: { stubs: sharedStubs } })
    await nextTick()
    expect(wrapper.find('[data-testid="reports-ee-banner"]').exists()).toBe(true)
    const cta = wrapper.find('[data-testid="reports-upgrade-cta"]')
    expect(cta.exists()).toBe(true)
    expect(wrapper.html()).toContain('/settings/account?tab=plan')
  })

  it('clicking the Upgrade CTA does not call router.push (it is a link, not a SPA nav)', async () => {
    const wrapper = mount(ReportsView, { global: { stubs: sharedStubs } })
    await nextTick()
    await wrapper.find('[data-testid="reports-upgrade-cta"]').trigger('click')
    expect(pushMock).not.toHaveBeenCalled()
  })
})

describe('EE upsell hygiene — Wizard Step 3 visibility cards (surface 2)', () => {
  async function settle() {
    await nextTick()
    await nextTick()
    await new Promise((r) => setTimeout(r, 0))
  }
  async function openStep3(): Promise<ReturnType<typeof mount>> {
    const wrapper = mount(DashboardWizardModal, {
      global: { stubs: sharedStubs },
      props: { open: true },
      attachTo: document.body,
    })
    await settle()
    const $ = (sel: string) => document.body.querySelector(sel) as HTMLElement | null
    const nameInput = $('[data-testid="wizard-name-input"]') as HTMLInputElement | null
    if (nameInput) {
      nameInput.value = 'EE-test'
      nameInput.dispatchEvent(new Event('input', { bubbles: true }))
    }
    ;($('[data-testid="scope-resolver-trigger"]') as HTMLButtonElement | null)?.click()
    await settle()
    ;($('[data-testid="wizard-continue"]') as HTMLButtonElement | null)?.click()
    await settle()
    ;($('[data-testid="wizard-widget-uptime-stat"]') as HTMLButtonElement | null)?.click()
    await settle()
    ;($('[data-testid="wizard-continue"]') as HTMLButtonElement | null)?.click()
    await settle()
    return wrapper
  }

  it('renders Team + Public cards disabled with EE badge inside an "Available on Enterprise" tooltip', async () => {
    const wrapper = await openStep3()
    const team = document.body.querySelector(
      '[data-testid="wizard-visibility-team"]',
    ) as HTMLButtonElement | null
    const pub = document.body.querySelector(
      '[data-testid="wizard-visibility-public"]',
    ) as HTMLButtonElement | null

    expect(team).not.toBeNull()
    expect(pub).not.toBeNull()
    expect(team!.disabled).toBe(true)
    expect(pub!.disabled).toBe(true)

    // Each disabled card surfaces the "Available on Enterprise" tooltip copy
    // (native `title` for now — graduates to UTooltip once NuxtUI overlays are
    // wired into the test harness — and a matching aria-label).
    expect(team!.getAttribute('title')).toBe('Available on Enterprise')
    expect(pub!.getAttribute('title')).toBe('Available on Enterprise')
    expect(team!.getAttribute('aria-label')).toContain('Available on Enterprise')
    expect(pub!.getAttribute('aria-label')).toContain('Available on Enterprise')

    // EE badge present inside each disabled card.
    expect(team!.querySelector('[data-testid="edition-badge"][data-edition="ee"]')).not.toBeNull()
    expect(pub!.querySelector('[data-testid="edition-badge"][data-edition="ee"]')).not.toBeNull()

    wrapper.unmount()
  })

  it('clicking the disabled Team / Public cards does not navigate', async () => {
    const wrapper = await openStep3()
    ;(
      document.body.querySelector('[data-testid="wizard-visibility-team"]') as HTMLButtonElement
    )?.click()
    ;(
      document.body.querySelector('[data-testid="wizard-visibility-public"]') as HTMLButtonElement
    )?.click()
    await nextTick()
    expect(pushMock).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})

describe('EE upsell hygiene — Gallery Shared filter empty state (surface 3 / FR-013)', () => {
  it('shows the FR-013-compliant empty copy when filter=shared on CE', async () => {
    const wrapper = mount(DashboardsView, { global: { stubs: sharedStubs } })
    await nextTick()
    const empty = wrapper.find('[data-testid="dashboards-shared-empty"]')
    expect(empty.exists()).toBe(true)
    const text = empty.text()
    expect(text).toContain('Shared dashboards are an Enterprise feature')
    expect(text.toLowerCase()).toContain('upgrade')
  })

  it('the Shared empty state contains no broken navigation (no router.push on mount)', async () => {
    mount(DashboardsView, { global: { stubs: sharedStubs } })
    await nextTick()
    expect(pushMock).not.toHaveBeenCalled()
  })
})
