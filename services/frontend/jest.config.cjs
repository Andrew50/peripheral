module.exports = {
    preset: 'ts-jest/presets/default-esm',
    testEnvironment: 'jsdom',
    extensionsToTreatAsEsm: ['.ts', '.svelte'],
    transform: {
        '^.+\\.svelte$': 'svelte-jester',
        '^.+\\.ts$': ['ts-jest', {
            useESM: true,
            tsconfig: {
                allowJs: true,
                checkJs: true,
                esModuleInterop: true,
                forceConsistentCasingInFileNames: true,
                resolveJsonModule: true,
                skipLibCheck: true,
                sourceMap: true,
                strict: true,
                moduleResolution: "bundler",
                target: "ES2020",
                module: "ESNext",
                paths: {
                    "$lib": ["<rootDir>/src/lib"],
                    "$lib/*": ["<rootDir>/src/lib/*"]
                }
            }
        }]
    },
    moduleFileExtensions: ['js', 'ts', 'svelte'],
    setupFilesAfterEnv: ['<rootDir>/jest.setup.cjs'],
    testMatch: ['**/*.test.ts', '**/*.test.js'],
    moduleNameMapper: {
        '^\\$lib/(.*)$': '<rootDir>/src/lib/$1'
    },
    transformIgnorePatterns: [
        'node_modules/(?!(@testing-library/jest-dom|.*\\.esm\\.js$))'
    ]
}; 