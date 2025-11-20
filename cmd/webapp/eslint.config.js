import js from '@eslint/js';
import ts from '@typescript-eslint/eslint-plugin';
import tsParser from '@typescript-eslint/parser';
import sveltePlugin from 'eslint-plugin-svelte';
import svelteParser from 'svelte-eslint-parser';

export default [
  js.configs.recommended,
  ...sveltePlugin.configs['flat/recommended'],
  {
    ignores: ['node_modules', 'dist', 'package-lock.json', '.svelte-kit']
  },
  {
    files: ['**/*.ts'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module'
      },
      globals: {
        URL: 'readonly',
        URLSearchParams: 'readonly',
        window: 'readonly',
        document: 'readonly',
        CustomEvent: 'readonly',
        MouseEvent: 'readonly',
        KeyboardEvent: 'readonly',
        MessageEvent: 'readonly',
        CloseEvent: 'readonly',
        HTMLElement: 'readonly',
        HTMLDivElement: 'readonly',
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
    plugins: {
      '@typescript-eslint': ts
    },
    rules: {
      ...ts.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': [
        'warn',
        {
          argsIgnorePattern: '^_'
        }
      ]
    }
  },
  {
    files: ['**/*.svelte'],
    languageOptions: {
      parser: svelteParser,
      parserOptions: {
        parser: tsParser,
        ecmaVersion: 'latest',
        sourceType: 'module'
      },
      globals: {
        URL: 'readonly',
        URLSearchParams: 'readonly',
        window: 'readonly',
        document: 'readonly',
        CustomEvent: 'readonly',
        MouseEvent: 'readonly',
        KeyboardEvent: 'readonly',
        MessageEvent: 'readonly',
        CloseEvent: 'readonly',
        HTMLElement: 'readonly',
        HTMLDivElement: 'readonly',
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
    files: ['**/*.js', '**/*.ts', '**/*.svelte'],
    rules: {
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
