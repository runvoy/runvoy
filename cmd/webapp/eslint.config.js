import js from '@eslint/js';
import sveltePlugin from 'eslint-plugin-svelte';
import svelteParser from 'svelte-eslint-parser';

export default [
  js.configs.recommended,
  ...sveltePlugin.configs['flat/recommended'],
  {
    ignores: ['node_modules', 'dist', 'package-lock.json']
  },
  {
    files: ['**/*.svelte'],
    languageOptions: {
      parser: svelteParser,
      parserOptions: {
        parser: null
      },
      globals: {
        URL: 'readonly',
        URLSearchParams: 'readonly',
        window: 'readonly',
        document: 'readonly',
        CustomEvent: 'readonly',
        clearTimeout: 'readonly',
        setTimeout: 'readonly',
        setInterval: 'readonly',
        clearInterval: 'readonly',
        localStorage: 'readonly',
        Blob: 'readonly',
        WebSocket: 'readonly',
        fetch: 'readonly',
        console: 'readonly'
      }
    },
    rules: {
      'svelte/valid-compile': 'error',
      'svelte/no-at-debug-tags': 'warn',
      'svelte/no-at-html-tags': 'warn'
    }
  },
  {
    files: ['**/*.js'],
    languageOptions: {
      globals: {
        URL: 'readonly',
        URLSearchParams: 'readonly',
        window: 'readonly',
        document: 'readonly',
        CustomEvent: 'readonly',
        clearTimeout: 'readonly',
        setTimeout: 'readonly',
        setInterval: 'readonly',
        clearInterval: 'readonly',
        localStorage: 'readonly',
        Blob: 'readonly',
        WebSocket: 'readonly',
        fetch: 'readonly',
        console: 'readonly'
      }
    }
  },
  {
    files: ['**/*.js', '**/*.svelte'],
    rules: {
      'no-unused-vars': [
        'warn',
        {
          argsIgnorePattern: '^_'
        }
      ],
      'no-console': 'warn',
      'no-debugger': 'error',
      'semi': ['error', 'always'],
      'quotes': ['error', 'single', { avoidEscape: true }],
      'indent': ['error', 4],
      'comma-dangle': ['error', 'never'],
      'no-trailing-spaces': 'error',
      'eol-last': ['error', 'always'],
      'object-shorthand': 'error',
      'prefer-const': 'error',
      'prefer-arrow-callback': 'error'
    }
  }
];
