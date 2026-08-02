/**
 * Static integration assets for the Prometheus metrics page:
 * - a shareable Grafana dashboard model (import via Grafana → Import → paste JSON)
 * - example Prometheus alerting rules (evaluated by Prometheus, routed by Alertmanager)
 *
 * Built over the ogoune_* metrics emitted by GET /metrics.
 *
 * Now the STATIC FALLBACK: the Download buttons fetch config-derived
 * artifacts from /api/v1/integrations/* and only use these when that fails.
 * NOTE (D1): the 5-panel dashboard shape below is duplicated with the backend
 * generator (internal/service/integrations/dashboard.go). Keep them in sync.
 */

const DS = '${DS_PROMETHEUS}'

export const GRAFANA_DASHBOARD = {
  __inputs: [
    {
      name: 'DS_PROMETHEUS',
      label: 'Prometheus',
      description: 'Prometheus data source scraping the Ogoune /metrics endpoint',
      type: 'datasource',
      pluginId: 'prometheus',
      pluginName: 'Prometheus',
    },
  ],
  __requires: [
    { type: 'grafana', id: 'grafana', name: 'Grafana', version: '10.0.0' },
    { type: 'datasource', id: 'prometheus', name: 'Prometheus', version: '1.0.0' },
  ],
  title: 'Ogoune — Uptime & Monitoring',
  uid: 'ogoune-overview',
  tags: ['ogoune', 'uptime'],
  schemaVersion: 39,
  version: 1,
  time: { from: 'now-24h', to: 'now' },
  refresh: '30s',
  templating: {
    list: [
      {
        name: 'resource',
        type: 'query',
        datasource: DS,
        query: 'label_values(ogoune_resource_up, name)',
        includeAll: true,
        multi: true,
        refresh: 2,
      },
    ],
  },
  panels: [
    {
      id: 1,
      title: 'Resources up',
      type: 'stat',
      datasource: DS,
      gridPos: { h: 4, w: 6, x: 0, y: 0 },
      options: { reduceOptions: { calcs: ['lastNotNull'] } },
      targets: [{ expr: 'sum(ogoune_resource_up)', refId: 'A' }],
    },
    {
      id: 2,
      title: 'Active incidents',
      type: 'stat',
      datasource: DS,
      gridPos: { h: 4, w: 6, x: 6, y: 0 },
      fieldConfig: { defaults: { thresholds: { steps: [{ color: 'green', value: null }, { color: 'red', value: 1 }] } } },
      targets: [{ expr: 'sum(ogoune_incidents_active)', refId: 'A' }],
    },
    {
      id: 3,
      title: 'Uptime % (24h) by resource',
      type: 'timeseries',
      datasource: DS,
      gridPos: { h: 8, w: 12, x: 12, y: 0 },
      fieldConfig: { defaults: { unit: 'percent', min: 0, max: 100 } },
      targets: [
        { expr: 'ogoune_uptime_ratio{window="24h", name=~"$resource"}', legendFormat: '{{name}}', refId: 'A' },
      ],
    },
    {
      id: 4,
      title: 'Check success rate',
      type: 'timeseries',
      datasource: DS,
      gridPos: { h: 8, w: 12, x: 0, y: 8 },
      fieldConfig: { defaults: { unit: 'percentunit', min: 0, max: 1 } },
      targets: [
        {
          expr: 'sum(rate(ogoune_checks_total{status="success"}[5m])) / clamp_min(sum(rate(ogoune_checks_total[5m])), 1)',
          refId: 'A',
        },
      ],
    },
    {
      id: 5,
      title: 'Check duration p95 (s)',
      type: 'timeseries',
      datasource: DS,
      gridPos: { h: 8, w: 12, x: 12, y: 8 },
      fieldConfig: { defaults: { unit: 's' } },
      targets: [
        {
          expr: 'histogram_quantile(0.95, sum(rate(ogoune_check_duration_seconds_bucket[5m])) by (le, name))',
          legendFormat: '{{name}}',
          refId: 'A',
        },
      ],
    },
  ],
} as const

export const ALERT_RULES_YAML = `# Ogoune — example Prometheus alerting rules.
# Evaluated by Prometheus; firing alerts are routed by Alertmanager.
# Load via prometheus.yml -> rule_files, then point Prometheus at Alertmanager.
groups:
  - name: ogoune
    rules:
      - alert: OgouneResourceDown
        expr: ogoune_resource_up == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "{{ $labels.name }} is down"
          description: "Resource {{ $labels.name }} ({{ $labels.type }}) has been down for 5 minutes."

      - alert: OgouneLowUptime24h
        expr: ogoune_uptime_ratio{window="24h"} < 99
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "{{ $labels.name }} uptime below 99% (24h)"
          description: "24h uptime is {{ $value | printf \\"%.2f\\" }}%."

      - alert: OgouneActiveIncident
        expr: ogoune_incidents_active > 0
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Open incident on {{ $labels.name }}"

      - alert: OgouneHighFailureRate
        expr: sum by (name) (rate(ogoune_checks_total{status="failure"}[5m])) > 0
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "{{ $labels.name }} is failing checks"
          description: "Sustained check failures over the last 10 minutes."
`
