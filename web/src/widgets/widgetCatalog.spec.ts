import { beforeEach, describe, expect, it } from 'vitest'
import {
  defineWidget,
  getWidgetDefinition,
  listWidgets,
  widgetCatalog,
  __resetWidgetCatalogForTests,
} from './widgetCatalog'

describe('widgetCatalog', () => {
  it('exposes the 4 MVP widgets covering the 4 archetypes', () => {
    const widgets = listWidgets()
    expect(widgets.length).toBe(4)
    const archetypes = new Set(widgets.map((w) => w.archetype))
    expect(archetypes).toEqual(new Set(['stat', 'list', 'chart', 'grid']))
  })

  it('exposes UptimeStat / IncidentsList / ResponseTime / ResourceStatusGrid', () => {
    const ids = listWidgets().map((w) => w.id)
    expect(ids).toContain('uptime-stat')
    expect(ids).toContain('incidents-list')
    expect(ids).toContain('response-time')
    expect(ids).toContain('resource-status-grid')
  })

  it('each registry entry lazy-loads its component', () => {
    for (const def of listWidgets()) {
      expect(typeof def.component).toBe('function')
    }
  })

  it('getWidgetDefinition resolves a known id', () => {
    const def = getWidgetDefinition('uptime-stat')
    expect(def).toBeDefined()
    expect(def?.archetype).toBe('stat')
  })

  it('widgetCatalog is keyed by id', () => {
    expect(widgetCatalog.size).toBe(4)
    expect(widgetCatalog.has('uptime-stat')).toBe(true)
  })

  describe('defineWidget()', () => {
    beforeEach(() => {
      __resetWidgetCatalogForTests()
    })

    it('rejects duplicate ids', () => {
      defineWidget({
        id: 'uptime-stat',
        name: 'one',
        icon: 'i-lucide-trending-up',
        archetype: 'stat',
        defaultConfig: {},
        component: () => Promise.resolve({ default: {} as never }),
      })
      expect(() =>
        defineWidget({
          id: 'uptime-stat',
          name: 'duplicate',
          icon: 'i-lucide-trending-up',
          archetype: 'stat',
          defaultConfig: {},
          component: () => Promise.resolve({ default: {} as never }),
        }),
      ).toThrow(/duplicate/)
    })
  })
})
