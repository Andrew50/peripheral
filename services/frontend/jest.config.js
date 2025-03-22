export default {
    transform: {
        "^.+\\.svelte$": "svelte-jester",
        "^.+\\.ts$": ["ts-jest", {
            useESM: true,
            tsconfig: 'tsconfig.json'
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