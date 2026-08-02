import { describe, expect, it, vi, beforeEach } from 'vitest'

/**
 * FR-036 regression — every destructive flow in Settings must route
 * through `useConfirm`. Failing this test means a destructive action is
 * silently skipping the confirm modal.
 */

const confirmMock = vi.fn()
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: (opts: unknown) => confirmMock(opts),
}))

vi.mock('@/services/accountService', () => ({
  default: {
    listAPIKeys: vi.fn().mockResolvedValue([]),
    createAPIKey: vi.fn(),
    revokeAPIKey: vi.fn().mockResolvedValue({ message: 'ok' }),
    deleteAccount: vi.fn().mockResolvedValue({ message: 'ok' }),
  },
}))

vi.mock('@/services/sessionsService', () => ({
  default: {
    list: vi.fn().mockResolvedValue([]),
    revoke: vi.fn().mockResolvedValue(undefined),
    revokeOthers: vi.fn().mockResolvedValue(undefined),
  },
}))

vi.mock('@/services/twoFactorService', () => ({
  default: {
    setup: vi.fn(),
    verify: vi.fn(),
    disable: vi.fn().mockResolvedValue(undefined),
    requestReset: vi.fn(),
    confirmReset: vi.fn(),
  },
}))

vi.mock('@/services/notificationChannelService', () => ({
  fetchChannels: vi.fn().mockResolvedValue([]),
  createChannel: vi.fn(),
  updateChannel: vi.fn(),
  deleteChannel: vi.fn().mockResolvedValue(undefined),
  setDefault: vi.fn(),
}))

vi.mock('@/services/escalationService', () => ({
  default: {
    list: vi.fn().mockResolvedValue([]),
    create: vi.fn(),
    update: vi.fn().mockResolvedValue({ id: 'p', is_active: false }),
    delete: vi.fn().mockResolvedValue(undefined),
    reorder: vi.fn(),
  },
}))

vi.mock('@/services/customDomainService', () => ({
  default: {
    list: vi.fn().mockResolvedValue([]),
    create: vi.fn(),
    verify: vi.fn(),
    delete: vi.fn().mockResolvedValue(undefined),
  },
}))

vi.mock('@/services/tagService', () => ({
  fetchTags: vi.fn().mockResolvedValue([]),
  createTag: vi.fn(),
  updateTag: vi.fn(),
  deleteTag: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('@/services/maintenanceService', () => ({
  fetchMaintenances: vi.fn().mockResolvedValue([]),
  createMaintenance: vi.fn(),
  cancelMaintenance: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), resolve: () => ({ href: '#' }) }),
  useRoute: () => ({ path: '/', params: {}, query: {}, name: 'x' }),
  useLink: () => ({ href: { value: '#' }, navigate: vi.fn(), isActive: { value: false } }),
  RouterLink: { template: '<a><slot /></a>' },
}))

import { useAuthStore } from '@/stores/authStore'
vi.mock('@/stores/authStore', () => ({
  useAuthStore: vi.fn().mockReturnValue({
    user: { email: 'me@x.test' },
    email: 'me@x.test',
    verify: vi.fn(),
    logout: vi.fn(),
  }),
}))

beforeEach(() => {
  confirmMock.mockReset()
  ;(useAuthStore as unknown as { mockReturnValue: (v: unknown) => void }).mockReturnValue({
    user: { email: 'me@x.test' },
    email: 'me@x.test',
    verify: vi.fn(),
    logout: vi.fn(),
  })
})

describe('FR-036 destructive flows route through useConfirm', () => {
  it('Delete account → confirm modal mounted', async () => {
    confirmMock.mockResolvedValue(false)
    const { default: cmp } = await import('@/components/settings/account/DangerZoneSection.vue')
    const { mount } = await import('@vue/test-utils')
    const { setActivePinia, createPinia } = await import('pinia')
    setActivePinia(createPinia())
    const w = mount(cmp)
    type Vm = { typed: string; onConfirm: () => Promise<void> }
    const vm = w.vm as unknown as Vm
    vm.typed = 'me@x.test'
    await vm.onConfirm()
    expect(confirmMock).toHaveBeenCalled()
  })

  it('Revoke session → confirm modal mounted', async () => {
    confirmMock.mockResolvedValue(false)
    const { default: cmp } = await import('@/views/settings/SessionsView.vue')
    const { mount, flushPromises } = await import('@vue/test-utils')
    const w = mount(cmp)
    await flushPromises()
    type Vm = {
      onRevoke: (id: string) => Promise<void>
      sessions: { id: string; browser: string; os: string }[]
    }
    const vm = w.vm as unknown as Vm
    vm.sessions = [
      {
        id: 's1',
        browser: 'X',
        os: 'Y',
      } as unknown as Vm['sessions'][number],
    ]
    await vm.onRevoke('s1')
    expect(confirmMock).toHaveBeenCalled()
  })

  it('Disable 2FA → confirm modal mounted', async () => {
    confirmMock.mockResolvedValue(false)
    const { default: cmp } = await import('@/views/settings/TwoFactorSetupView.vue')
    const { mount } = await import('@vue/test-utils')
    const { setActivePinia, createPinia } = await import('pinia')
    setActivePinia(createPinia())
    const w = mount(cmp)
    type Vm = { onDisable: () => Promise<void> }
    const vm = w.vm as unknown as Vm
    await vm.onDisable()
    expect(confirmMock).toHaveBeenCalled()
  })

  it('Revoke API key → confirm modal mounted', async () => {
    confirmMock.mockResolvedValue(false)
    const { default: cmp } = await import('@/views/settings/ApiKeysView.vue')
    const { mount, flushPromises } = await import('@vue/test-utils')
    const { setActivePinia, createPinia } = await import('pinia')
    setActivePinia(createPinia())
    const w = mount(cmp)
    await flushPromises()
    type Vm = { onRevoke: (k: { id: string; name: string }) => Promise<void> }
    const vm = w.vm as unknown as Vm
    await vm.onRevoke({ id: 'k1', name: 'CI' })
    expect(confirmMock).toHaveBeenCalled()
  })

  it('Delete notification channel → confirm modal mounted', async () => {
    confirmMock.mockResolvedValue(false)
    const { default: cmp } = await import('@/views/settings/NotificationsView.vue')
    const { mount, flushPromises } = await import('@vue/test-utils')
    const w = mount(cmp)
    await flushPromises()
    type Vm = {
      onDelete: (c: {
        id: string
        name: string
        type: 'smtp' | 'slack' | 'webhook'
        config: Record<string, unknown>
        enabled_by_default: boolean
      }) => Promise<void>
    }
    const vm = w.vm as unknown as Vm
    await vm.onDelete({ id: 'c1', name: 'X', type: 'slack', config: {}, enabled_by_default: false })
    expect(confirmMock).toHaveBeenCalled()
  })

  it('Delete escalation policy → confirm modal mounted', async () => {
    confirmMock.mockResolvedValue(false)
    const { default: cmp } = await import('@/views/settings/EscalationView.vue')
    const { mount, flushPromises } = await import('@vue/test-utils')
    const w = mount(cmp)
    await flushPromises()
    type Vm = {
      onDelete: (p: {
        id: string
        name: string
        scope: { kind: 'component'; value: string }
        is_active: boolean
        priority: number
        steps: { delay_minutes: number; channel_ids: string[] }[]
      }) => Promise<void>
    }
    const vm = w.vm as unknown as Vm
    await vm.onDelete({
      id: 'p1',
      name: 'X',
      scope: { kind: 'component', value: 'c' },
      is_active: true,
      priority: 1,
      steps: [{ delay_minutes: 5, channel_ids: ['ch'] }],
    })
    expect(confirmMock).toHaveBeenCalled()
  })

  // Custom-domain flow folded into Status Page settings:
  // domain config has no standalone delete confirm — the user just clears
  // the field and saves. Test removed.

  // Tag CRUD UI dropped from spec 059 — tag management is deferred to a
  // resource-form inline flow + a future light hygiene panel.

  it('Remove status page logo → confirm modal mounted', async () => {
    confirmMock.mockResolvedValue(false)
    vi.doMock('@/services/statusPageSettingsService', () => ({
      uploadStatusPageLogo: vi.fn(),
      deleteStatusPageLogo: vi.fn(),
    }))
    vi.doMock('@nuxt/ui/composables/useToast', () => ({
      useToast: () => ({ add: vi.fn() }),
    }))
    const { default: BrandingSection } = await import(
      '@/components/settings/branding/BrandingSection.vue'
    )
    const { mount } = await import('@vue/test-utils')
    const w = mount(BrandingSection, {
      props: {
        settings: {
          id: 'sp-1',
          name: 'Acme',
          homepage_url: '',
          custom_domain: '',
          umami_website_id: '',
          umami_script_url: '',
          enable_details_page: true,
          show_uptime_percentage: true,
          hide_paused_monitors: true,
          show_incident_history: true,
          custom_domain_status: 'pending',
          custom_domain_ssl_status: 'none',
          custom_domain_dns_records: [],
          logo_url_light: '/static/uploads/statuspage/light-x.png',
          logo_url_dark: '',
          favicon_url: '',
          primary_color: '#4f46e5',
          theme_overrides: {},
          created_at: '',
          updated_at: '',
        },
        primaryColor: '#4f46e5',
        themeOverrides: {},
      },
    })
    type Vm = { onDelete: (slot: 'light' | 'dark' | 'favicon') => Promise<void> }
    const vm = w.vm as unknown as Vm
    await vm.onDelete('light')
    expect(confirmMock).toHaveBeenCalled()
  })
})
