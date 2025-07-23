import typescriptParser from '@typescript-eslint/parser';
import typescriptPlugin from '@typescript-eslint/eslint-plugin';
import sveltePlugin from 'eslint-plugin-svelte';
import svelteParser from 'svelte-eslint-parser';

export default [
    {
        files: ['**/*.{js,ts,mjs,cjs}'],
        languageOptions: {
            parser: typescriptParser,
            parserOptions: {
                ecmaVersion: 'latest',
                sourceType: 'module'
            },
            globals: {
                console: 'readonly',
                process: 'readonly',
                Buffer: 'readonly',
                __dirname: 'readonly',
                __filename: 'readonly',
                global: 'readonly',
                window: 'readonly',
                document: 'readonly',
                navigator: 'readonly',
                location: 'readonly'
            }
        },
        plugins: {
            '@typescript-eslint': typescriptPlugin
        },
        rules: {
            // Enforce camelCase for all identifiers
            /*camelcase: ['error', { properties: 'always' }],

            // TypeScript naming conventions - enforce camelCase and disallow snake_case
            '@typescript-eslint/naming-convention': [
                'error',
                { selector: 'default', format: ['camelCase'], leadingUnderscore: 'forbid', trailingUnderscore: 'forbid' },
                { selector: 'variableLike', format: ['camelCase'], leadingUnderscore: 'forbid', trailingUnderscore: 'forbid' },
                { selector: 'typeLike', format: ['PascalCase'] },
                { selector: 'enumMember', format: ['PascalCase'] },
                { selector: 'parameter', format: ['camelCase'], leadingUnderscore: 'allow' },
                { selector: 'memberLike', modifiers: ['private'], format: ['camelCase'], leadingUnderscore: 'require' }
            ],*/

            // Additional rules for code quality
            'no-unused-vars': 'off',
            '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
            'prefer-const': 'error',
            'no-var': 'error',

            // Basic recommended rules
            ...typescriptPlugin.configs.recommended.rules
        }
    },
    {
        files: ['**/*.svelte'],
        languageOptions: {
            parser: svelteParser,
            parserOptions: {
                parser: typescriptParser,
                extraFileExtensions: ['.svelte']
            }
        },
        plugins: {
            svelte: sveltePlugin,
            '@typescript-eslint': typescriptPlugin
        },
        rules: {
            ...sveltePlugin.configs.recommended.rules,
            // Apply the same camelCase rules to Svelte files
            /*camelcase: ['error', { properties: 'always' }],
            '@typescript-eslint/naming-convention': [
                'error',
                { selector: 'default', format: ['camelCase'], leadingUnderscore: 'forbid', trailingUnderscore: 'forbid' },
                { selector: 'variableLike', format: ['camelCase'], leadingUnderscore: 'forbid', trailingUnderscore: 'forbid' },
                { selector: 'typeLike', format: ['PascalCase'] },
                { selector: 'enumMember', format: ['PascalCase'] },
                { selector: 'parameter', format: ['camelCase'], leadingUnderscore: 'allow' },
                { selector: 'memberLike', modifiers: ['private'], format: ['camelCase'], leadingUnderscore: 'require' }
            ]*/
        }
    },
    {
        ignores: [
            '.svelte-kit/**',
            'build/**',
            'dist/**',
            'node_modules/**',
            '*.config.js',
            '*.config.ts',
            'vite.config.ts.timestamp-*'
        ]
    }
]; 