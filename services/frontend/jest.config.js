export default {
    transform: {
        "^.+\\.svelte$": "svelte-jester",
        "^.+\\.ts$": ["ts-jest", {
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
    moduleFileExtensions: ["js", "ts", "svelte"],
    testEnvironment: "jsdom",
    setupFilesAfterEnv: ["<rootDir>/jest.setup.js"],
    testMatch: ["**/*.test.ts", "**/*.test.js"],
    moduleNameMapper: {
        "^\\$lib/(.*)$": "<rootDir>/src/lib/$1"
    },
    extensionsToTreatAsEsm: ['.ts', '.svelte']
}; 