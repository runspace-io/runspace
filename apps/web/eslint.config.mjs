import eslint from '@eslint/js';
import { FlatCompat } from '@eslint/eslintrc';

const compat = new FlatCompat({ baseDirectory: import.meta.dirname });

const config = [
  {
    ignores: ['.next/**', 'next-env.d.ts'],
  },
  eslint.configs.recommended,
  ...compat.extends('next/core-web-vitals'),
  {
    rules: {
      complexity: ['error', { max: 10 }],
      'max-depth': ['error', 4],
      'max-lines': ['error', { max: 300, skipBlankLines: true, skipComments: true }],
      'max-lines-per-function': ['error', { max: 145, skipBlankLines: true, skipComments: true }],
      'max-params': ['error', 5],
    },
  },
  {
    files: ['**/*.ts', '**/*.tsx'],
    rules: {
      'no-unused-vars': 'off',
    },
  },
];

export default config;
