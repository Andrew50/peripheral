// eslint.config.js
import sveltePlugin from 'eslint-plugin-svelte';
import typescriptParser from '@typescript-eslint/parser';

export default [
    // ❶ Pull in the full recommended Svelte flat config (rules *and* processor)
    ...sveltePlugin.configs['flat/recommended'],

    // ❷ Your own tweaks can follow
    {
        files: ['**/*.svelte'],
        languageOptions: {
            parser: typescriptParser,
            parserOptions: { extraFileExtensions: ['.svelte'] }
        },
        // optional: additional project‑specific rules
        rules: {
            // keep the rule on, but only warn
            'svelte/no-at-html-tags': 'warn'
        }
    }
];