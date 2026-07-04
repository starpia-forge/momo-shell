import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import boundaries from 'eslint-plugin-boundaries'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import globals from 'globals'

// FSD layer elements. Patterns are relative to this file's directory
// (frontend/), which is eslint-plugin-boundaries' default root path.
const elements = [
  { type: 'app', pattern: 'src/app/**' },
  { type: 'pages', pattern: 'src/pages/*/**', capture: ['family'] },
  { type: 'widgets', pattern: 'src/widgets/*/**', capture: ['family'] },
  { type: 'features', pattern: 'src/features/*/**', capture: ['family'] },
  { type: 'entities', pattern: 'src/entities/*/**', capture: ['family'] },
  { type: 'shared', pattern: 'src/shared/**' },
  { type: 'wailsjs', pattern: 'wailsjs/**' },
]

export default tseslint.config(
  { ignores: ['dist', 'wailsjs'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  reactHooks.configs['recommended-latest'],
  reactRefresh.configs.vite,
  {
    files: ['src/**/*.{ts,tsx}'],
    languageOptions: {
      globals: globals.browser,
    },
    plugins: { boundaries },
    settings: {
      'boundaries/elements': elements,
      // boundaries resolves import targets via eslint-plugin-import's
      // resolver settings -- without this, extensionless .ts/.tsx imports
      // (i.e. everything except wailsjs's plain .js) fail to resolve to a
      // real file, and boundary rules silently no-op for them.
      'import/resolver': {
        typescript: true,
      },
    },
    rules: {
      // app -> pages -> widgets -> features -> entities -> shared, one-way only.
      // A slice may always import its own files (same captured family) --
      // this rule is about crossing layers/slices, not internal structure.
      // wailsjs (generated Wails bindings) may only be reached from shared/api.
      'boundaries/element-types': [
        'error',
        {
          default: 'disallow',
          rules: [
            { from: 'app', allow: ['pages', 'widgets', 'features', 'entities', 'shared'] },
            {
              from: 'pages',
              allow: ['widgets', 'features', 'entities', 'shared', ['pages', { family: '${from.family}' }]],
            },
            {
              from: 'widgets',
              allow: ['features', 'entities', 'shared', ['widgets', { family: '${from.family}' }]],
            },
            {
              from: 'features',
              allow: ['entities', 'shared', ['features', { family: '${from.family}' }]],
            },
            {
              from: 'entities',
              allow: ['shared', ['entities', { family: '${from.family}' }]],
            },
            { from: 'shared', allow: ['shared', 'wailsjs'] },
          ],
        },
      ],
      // Slices expose only index.ts as public API to OTHER slices -- no deep
      // imports into another slice's internals. A slice's own files may
      // freely import each other regardless of subfolder (lib/model/ui),
      // so the same-family rule is listed first and allows any path.
      // shared/app/wailsjs have no restriction at all.
      'boundaries/entry-point': [
        'error',
        {
          default: 'disallow',
          rules: [
            {
              target: [
                ['pages', { family: '${from.family}' }],
                ['widgets', { family: '${from.family}' }],
                ['features', { family: '${from.family}' }],
                ['entities', { family: '${from.family}' }],
              ],
              allow: '**',
            },
            { target: ['pages', 'widgets', 'features', 'entities'], allow: '**/index.ts' },
            { target: ['shared', 'app', 'wailsjs'], allow: '**' },
          ],
        },
      ],
    },
  }
)
