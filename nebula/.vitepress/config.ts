import { defineConfig } from 'vitepress'

// Ogoune public docs. Covers Community (CE) + Enterprise (EE) — same codebase,
// EE features documented publicly (Portainer model). Cloud = the managed offering.
export default defineConfig({
  title: 'Ogoune',
  description: 'Uptime monitoring that confirms before it cries wolf.',
  lang: 'en-US',
  cleanUrls: true,
  lastUpdated: true,

  head: [
    ['link', { rel: 'icon', href: '/logo.png' }],
  ],

  // api/reference.md imports ../../api/openapi/v1.json (repo root, outside srcDir).
  vite: {
    server: {
      fs: {
        allow: ['..'],
      },
    },
  },

  themeConfig: {
    logo: '/logo.png',

    nav: [
      { text: 'Guide', link: '/guide/' },
      { text: 'Self-host', link: '/self-host/' },
      { text: 'Enterprise', link: '/enterprise/' },
      { text: 'API', link: '/api/' },
      { text: 'Cloud', link: '/cloud/' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Getting started',
          items: [
            { text: 'Introduction', link: '/guide/' },
            { text: 'Quickstart', link: '/guide/quickstart' },
            { text: 'Core concepts', link: '/guide/concepts' },
          ],
        },
        {
          text: 'Monitoring',
          items: [
            { text: 'Monitor types', link: '/guide/monitor-types' },
            { text: 'Incidents & confirmation', link: '/guide/incidents' },
            { text: 'Notifications', link: '/guide/notifications' },
          ],
        },
      ],
      '/self-host/': [
        {
          text: 'Self-hosting',
          items: [
            { text: 'Overview', link: '/self-host/' },
            { text: 'Community (SQLite)', link: '/self-host/community' },
            { text: 'Production (Postgres + Redis)', link: '/self-host/production' },
            { text: 'Configuration', link: '/self-host/configuration' },
            { text: 'Host agent', link: '/self-host/agent' },
          ],
        },
      ],
      '/enterprise/': [
        {
          text: 'Enterprise Edition',
          items: [
            { text: 'Overview', link: '/enterprise/' },
            { text: 'Licensing', link: '/enterprise/licensing' },
          ],
        },
      ],
      '/api/': [
        {
          text: 'API',
          items: [
            { text: 'Overview', link: '/api/' },
            { text: 'Reference', link: '/api/reference' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/denisakp/ogoune' },
    ],

    editLink: {
      pattern: 'https://github.com/denisakp/ogoune/edit/main/nebula/:path',
      text: 'Edit this page on GitHub',
    },

    search: {
      provider: 'local',
    },

    footer: {
      message: 'Community Edition under Apache 2.0.',
      copyright: 'Copyright © Ogoune',
    },
  },
})
