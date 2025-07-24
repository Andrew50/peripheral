export default {
    env: {
        browser: true,
        es2021: true,
        node: true
    },
    extends: [
        'eslint:recommended',
        '@typescript-eslint/recommended'
    ],
    parser: '@typescript-eslint/parser',
    parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module'
    },
    plugins: [
        '@typescript-eslint'
    ],
    rules: {
        // Enforce camelCase for all identifiers
        camelcase: ['error', { properties: 'always' }],

        // TypeScript naming conventions - enforce camelCase and disallow snake_case
        '@typescript-eslint/naming-convention': [
            'error',
            { selector: 'default', format: ['camelCase'], leadingUnderscore: 'forbid', trailingUnderscore: 'forbid' },
            { selector: 'variableLike', format: ['camelCase'], leadingUnderscore: 'forbid', trailingUnderscore: 'forbid' },
            { selector: 'typeLike', format: ['PascalCase'] },
            { selector: 'enumMember', format: ['PascalCase'] },
            { selector: 'parameter', format: ['camelCase'], leadingUnderscore: 'allow' },
            { selector: 'memberLike', modifiers: ['private'], format: ['camelCase'], leadingUnderscore: 'require' }
        ],

        // Additional rules for code quality
        'no-unused-vars': 'off',
        '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
        'prefer-const': 'error',
        'no-var': 'error'
    },
    overrides: [
        {
            files: ['*.svelte'],
            processor: 'svelte/svelte',
            extends: ['plugin:svelte/recommended']
        }
    ]
}; 