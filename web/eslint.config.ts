import { globalIgnores } from 'eslint/config'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import pluginVue from 'eslint-plugin-vue'
import pluginOxlint from 'eslint-plugin-oxlint'
import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'

// To allow more languages other than `ts` in `.vue` files, uncomment the following lines:
// import { configureVueProject } from '@vue/eslint-config-typescript'
// configureVueProject({ scriptLangs: ['ts', 'tsx'] })
// More info at https://github.com/vuejs/eslint-config-typescript/#advanced-setup

export default defineConfigWithVueTs(
  {
    name: 'app/files-to-lint',
    files: ['**/*.{ts,mts,tsx,vue}'],
  },

  globalIgnores([
    '**/node_modules/**',
    '**/dist/**',
    '**/dist-ssr/**',
    '**/build/**',
    '**/coverage/**',
    '**/*.min.js',
  ]),

  pluginVue.configs['flat/essential'],
  vueTsConfigs.recommended,
  ...pluginOxlint.configs['flat/recommended'],
  skipFormatting,

  // Spec 073 — guard the completed migration: forbid re-introducing the retired
  // Ant Design Vue / Axios stack. Use NuxtUI components + Iconify, and the Ky
  // client (`@/core/http/client`) instead.
  {
    name: 'app/no-legacy-stack',
    rules: {
      'no-restricted-imports': [
        'error',
        {
          paths: [
            { name: 'ant-design-vue', message: 'Ant Design Vue was removed. Use NuxtUI components.' },
            { name: 'axios', message: 'Axios was removed. Use the Ky client at @/core/http/client.' },
          ],
          patterns: [
            { group: ['ant-design-vue', 'ant-design-vue/*', '@ant-design/*'], message: 'Ant Design Vue / its icons were removed. Use NuxtUI + Iconify.' },
          ],
        },
      ],
    },
  },
)
