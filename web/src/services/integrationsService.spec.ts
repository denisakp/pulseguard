import { describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'
import { createRemoteIntegrationsFeed } from './integrationsService'
import { server } from '@/test/msw/server'

describe('integrationsService', () => {
  it('fetchAlertRules reads the raw text/yaml body and forwards the threshold', async () => {
    let url = ''
    server.use(
      http.get('*/v1/integrations/alert-rules', ({ request }) => {
        url = request.url
        return new HttpResponse('groups:\n- name: ogoune\n', {
          headers: { 'Content-Type': 'text/yaml' },
        })
      }),
    )
    const yaml = await createRemoteIntegrationsFeed().fetchAlertRules(95)
    expect(url).toContain('uptimeThreshold=95')
    expect(yaml).toContain('name: ogoune')
  })

  it('fetchGrafanaDashboard reads JSON', async () => {
    server.use(
      http.get('*/v1/integrations/grafana-dashboard', () =>
        HttpResponse.json({ title: 'Ogoune — Uptime & Monitoring', panels: [] }),
      ),
    )
    const dash = (await createRemoteIntegrationsFeed().fetchGrafanaDashboard()) as { title: string }
    expect(dash.title).toContain('Ogoune')
  })

  it('rejects on a server error (drives the fallback)', async () => {
    server.use(
      http.get('*/v1/integrations/alert-rules', () => new HttpResponse(null, { status: 500 })),
    )
    await expect(createRemoteIntegrationsFeed().fetchAlertRules()).rejects.toBeTruthy()
  })
})
