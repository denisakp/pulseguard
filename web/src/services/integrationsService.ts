import { getAuthenticatedClient } from '@/core/http/client'

/**
 * Config-derived observability assets. The alert-rules endpoint
 * returns raw `text/yaml` (the shared `request<T>` helper always `.json()`s, so
 * we read `.text()` off the ky instance directly); the dashboard returns JSON.
 * Callers fall back to the bundled static assets on any error.
 */
export interface IntegrationsFeed {
  fetchAlertRules(uptimeThreshold?: number): Promise<string>
  fetchGrafanaDashboard(): Promise<unknown>
}

export function createRemoteIntegrationsFeed(): IntegrationsFeed {
  const client = () => getAuthenticatedClient()
  return {
    async fetchAlertRules(uptimeThreshold?: number): Promise<string> {
      const q = uptimeThreshold ? `?uptimeThreshold=${uptimeThreshold}` : ''
      return await client()(`v1/integrations/alert-rules${q}`).text()
    },
    async fetchGrafanaDashboard(): Promise<unknown> {
      return await client()('v1/integrations/grafana-dashboard').json()
    },
  }
}

let activeFeed: IntegrationsFeed = createRemoteIntegrationsFeed()

const integrationsService: IntegrationsFeed = {
  fetchAlertRules: (t) => activeFeed.fetchAlertRules(t),
  fetchGrafanaDashboard: () => activeFeed.fetchGrafanaDashboard(),
}

export default integrationsService

// Test-only seam.
export function __setIntegrationsFeedForTests(feed: IntegrationsFeed): void {
  activeFeed = feed
}

export function __resetIntegrationsForTests(): void {
  activeFeed = createRemoteIntegrationsFeed()
}
